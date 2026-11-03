package reports

import (
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"taskmgmt/internal/model"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserDepartmentIDs(orgID, userID uuid.UUID) ([]uuid.UUID, error) {
	var deptIDs []uuid.UUID
	err := r.db.Model(&model.DepartmentMember{}).
		Where("organization_id = ? AND user_id = ? AND (effective_to IS NULL OR effective_to > ?)", orgID, userID, time.Now().UTC()).
		Pluck("department_id", &deptIDs).Error
	return deptIDs, err
}

func (r *Repository) baseScopeQuery(orgID, userID uuid.UUID, isAllScope bool, deptIDs []uuid.UUID) *gorm.DB {
	query := r.db.Model(&model.Task{}).Where("tasks.organization_id = ?", orgID)
	if isAllScope {
		return query
	}

	// Scope filter: user is assignee, or dept is assignee, or user is creator
	subQueryAssignee := r.db.Model(&model.TaskAssignment{}).
		Select("task_id").
		Where("organization_id = ?", orgID)

	if len(deptIDs) > 0 {
		subQueryAssignee = subQueryAssignee.Where("assignee_user_id = ? OR assignee_department_id IN (?)", userID, deptIDs)
	} else {
		subQueryAssignee = subQueryAssignee.Where("assignee_user_id = ?", userID)
	}

	return query.Where("tasks.id IN (?) OR tasks.created_by = ?", subQueryAssignee, userID)
}

func (r *Repository) GetDashboardSummary(orgID, userID uuid.UUID, isAllScope bool, deptIDs []uuid.UUID) (*DashboardSummaryResponse, error) {
	baseQuery := r.baseScopeQuery(orgID, userID, isAllScope, deptIDs)

	var totalTasks int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&totalTasks).Error; err != nil {
		return nil, err
	}

	type statusCount struct {
		Status string
		Total  int64
	}
	var statusCounts []statusCount
	err := baseQuery.Session(&gorm.Session{}).
		Select("status, count(id) as total").
		Group("status").
		Scan(&statusCounts).Error
	if err != nil {
		return nil, err
	}

	res := &DashboardSummaryResponse{
		TotalTasks: totalTasks,
	}

	for _, sc := range statusCounts {
		switch sc.Status {
		case "in_progress":
			res.InProgressTasks = sc.Total
		case "assigned":
			res.AssignedTasks = sc.Total
		case "completed":
			res.CompletedTasks = sc.Total
		case "blocked":
			res.BlockedTasks = sc.Total
		case "revision_required":
			res.RevisionRequiredTasks = sc.Total
		case "cancelled":
			res.CancelledTasks = sc.Total
		}
	}

	now := time.Now().UTC()
	var overdueCount int64
	err = baseQuery.Session(&gorm.Session{}).
		Where("due_at IS NOT NULL AND due_at < ? AND status NOT IN ('completed', 'cancelled')", now).
		Count(&overdueCount).Error
	if err != nil {
		return nil, err
	}
	res.OverdueTasks = overdueCount

	upcomingLimit := now.Add(48 * time.Hour)
	var upcomingCount int64
	err = baseQuery.Session(&gorm.Session{}).
		Where("due_at IS NOT NULL AND due_at >= ? AND due_at <= ? AND status NOT IN ('completed', 'cancelled')", now, upcomingLimit).
		Count(&upcomingCount).Error
	if err != nil {
		return nil, err
	}
	res.UpcomingDeadlineTasks = upcomingCount

	var avgProgress float64
	_ = baseQuery.Session(&gorm.Session{}).
		Where("status != 'cancelled'").
		Select("coalesce(avg(progress), 0)").
		Scan(&avgProgress).Error
	res.AvgProgress = math.Round(avgProgress*10) / 10

	validTotal := totalTasks - res.CancelledTasks
	if validTotal > 0 {
		res.CompletionRate = math.Round((float64(res.CompletedTasks)/float64(validTotal))*1000) / 10
	}

	return res, nil
}

func (r *Repository) GetMyTasks(orgID, userID uuid.UUID, deptIDs []uuid.UUID, limit int) ([]MyTaskItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	subQuery := r.db.Model(&model.TaskAssignment{}).
		Select("task_id").
		Where("organization_id = ?", orgID)

	if len(deptIDs) > 0 {
		subQuery = subQuery.Where("assignee_user_id = ? OR assignee_department_id IN (?)", userID, deptIDs)
	} else {
		subQuery = subQuery.Where("assignee_user_id = ?", userID)
	}

	var tasks []model.Task
	err := r.db.Where("organization_id = ? AND id IN (?) AND status NOT IN ('completed', 'cancelled')", orgID, subQuery).
		Order("priority ASC, due_at ASC, created_at DESC").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result := make([]MyTaskItem, 0, len(tasks))

	for _, t := range tasks {
		isOverdue := t.DueAt != nil && t.DueAt.Before(now)
		item := MyTaskItem{
			ID:        t.ID,
			Code:      t.Code,
			Title:     t.Title,
			Status:    t.Status,
			Priority:  t.Priority,
			Progress:  t.Progress,
			DueAt:     t.DueAt,
			IsOverdue: isOverdue,
			CreatedAt: t.CreatedAt,
		}

		if t.ParentID != nil {
			var parent model.Task
			if err := r.db.Select("id, title").Where("id = ?", *t.ParentID).First(&parent).Error; err == nil {
				item.ParentTitle = parent.Title
			}
		}

		result = append(result, item)
	}

	return result, nil
}

func (r *Repository) GetTaskReport(orgID uuid.UUID, filter TaskReportFilter) (*TaskReportResponse, error) {
	query := r.db.Model(&model.Task{}).Where("organization_id = ?", orgID)

	if filter.FromDate != nil {
		query = query.Where("created_at >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where("created_at <= ?", *filter.ToDate)
	}
	if filter.DepartmentID != nil {
		subQuery := r.db.Model(&model.TaskAssignment{}).
			Select("task_id").
			Where("organization_id = ? AND assignee_department_id = ?", orgID, *filter.DepartmentID)
		query = query.Where("id IN (?)", subQuery)
	}

	var totalTasks int64
	if err := query.Session(&gorm.Session{}).Count(&totalTasks).Error; err != nil {
		return nil, err
	}

	type statusRow struct {
		Status string
		Total  int64
	}
	var statusRows []statusRow
	_ = query.Session(&gorm.Session{}).Select("status, count(id) as total").Group("status").Scan(&statusRows).Error
	byStatus := make(map[string]int64)
	for _, sr := range statusRows {
		byStatus[sr.Status] = sr.Total
	}

	type priorityRow struct {
		Priority int16
		Total    int64
	}
	var priorityRows []priorityRow
	_ = query.Session(&gorm.Session{}).Select("priority, count(id) as total").Group("priority").Scan(&priorityRows).Error
	byPriority := make(map[int16]int64)
	for _, pr := range priorityRows {
		byPriority[pr.Priority] = pr.Total
	}

	now := time.Now().UTC()
	var overdueCount int64
	_ = query.Session(&gorm.Session{}).
		Where("due_at IS NOT NULL AND due_at < ? AND status NOT IN ('completed', 'cancelled')", now).
		Count(&overdueCount).Error

	var avgProgress float64
	_ = query.Session(&gorm.Session{}).
		Where("status != 'cancelled'").
		Select("coalesce(avg(progress), 0)").
		Scan(&avgProgress).Error

	completedTasks := byStatus["completed"]
	cancelledTasks := byStatus["cancelled"]
	validTotal := totalTasks - cancelledTasks
	var completionRate float64
	if validTotal > 0 {
		completionRate = math.Round((float64(completedTasks)/float64(validTotal))*1000) / 10
	}

	return &TaskReportResponse{
		TotalTasks:     totalTasks,
		CompletedTasks: completedTasks,
		OverdueTasks:   overdueCount,
		AvgProgress:    math.Round(avgProgress*10) / 10,
		CompletionRate: completionRate,
		ByStatus:       byStatus,
		ByPriority:     byPriority,
	}, nil
}

func (r *Repository) GetDepartmentPerformance(orgID uuid.UUID) ([]DepartmentPerformanceItem, error) {
	var depts []model.Department
	err := r.db.Where("organization_id = ? AND is_active = ?", orgID, true).
		Order("name ASC").
		Find(&depts).Error
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result := make([]DepartmentPerformanceItem, 0, len(depts))

	for _, d := range depts {
		subQuery := r.db.Model(&model.TaskAssignment{}).
			Select("task_id").
			Where("organization_id = ? AND assignee_department_id = ?", orgID, d.ID)

		query := r.db.Model(&model.Task{}).Where("organization_id = ? AND id IN (?)", orgID, subQuery)

		var total int64
		_ = query.Session(&gorm.Session{}).Count(&total).Error

		var completed int64
		_ = query.Session(&gorm.Session{}).Where("status = 'completed'").Count(&completed).Error

		var inProgress int64
		_ = query.Session(&gorm.Session{}).Where("status = 'in_progress'").Count(&inProgress).Error

		var overdue int64
		_ = query.Session(&gorm.Session{}).
			Where("due_at IS NOT NULL AND due_at < ? AND status NOT IN ('completed', 'cancelled')", now).
			Count(&overdue).Error

		var avgProgress float64
		_ = query.Session(&gorm.Session{}).
			Where("status != 'cancelled'").
			Select("coalesce(avg(progress), 0)").
			Scan(&avgProgress).Error

		var completionRate float64
		if total > 0 {
			completionRate = math.Round((float64(completed)/float64(total))*1000) / 10
		}

		result = append(result, DepartmentPerformanceItem{
			DepartmentID:    d.ID,
			DepartmentCode:  d.Code,
			DepartmentName:  d.Name,
			TotalTasks:      total,
			CompletedTasks:  completed,
			InProgressTasks: inProgress,
			OverdueTasks:    overdue,
			AvgProgress:     math.Round(avgProgress*10) / 10,
			CompletionRate:  completionRate,
		})
	}

	return result, nil
}

func (r *Repository) GetTasksForExport(orgID uuid.UUID, filter TaskReportFilter) ([]model.Task, error) {
	query := r.db.Model(&model.Task{}).Where("organization_id = ?", orgID)
	if filter.FromDate != nil {
		query = query.Where("created_at >= ?", filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where("created_at <= ?", filter.ToDate)
	}
	if filter.DepartmentID != nil {
		query = query.Joins("JOIN task_assignments ON task_assignments.task_id = tasks.id").
			Where("task_assignments.assignee_department_id = ?", *filter.DepartmentID)
	}

	var list []model.Task
	err := query.Order("created_at DESC").
		Preload("Assignments.AssigneeUser").
		Preload("Assignments.AssigneeDepartment").
		Find(&list).Error
	return list, err
}

