package tasks

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"taskmgmt/internal/model"
)

var (
	ErrTaskNotFound     = errors.New("task not found")
	ErrVersionConflict = errors.New("version conflict: task was modified by another transaction")
	ErrAttachmentNotFound = errors.New("attachment not found")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) CreateTaskTx(tx *gorm.DB, task *model.Task, assignment *model.TaskAssignment) error {
	if err := tx.Create(task).Error; err != nil {
		return err
	}

	if assignment != nil {
		assignment.TaskID = task.ID
		assignment.OrganizationID = task.OrganizationID
		if err := tx.Create(assignment).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetByID(orgID, taskID uuid.UUID) (*model.Task, error) {
	var task model.Task
	err := r.db.Where("organization_id = ? AND id = ?", orgID, taskID).
		Preload("Assignments.AssigneeUser").
		Preload("Assignments.AssigneeDepartment").
		Preload("Assignments.ResponsibleUser").
		Preload("Attachments.Uploader").
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (r *Repository) GetByIDForUpdate(tx *gorm.DB, orgID, taskID uuid.UUID) (*model.Task, error) {
	var task model.Task
	err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("organization_id = ? AND id = ?", orgID, taskID).
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (r *Repository) UpdateWithVersion(tx *gorm.DB, taskID uuid.UUID, expectedVersion int64, updates map[string]interface{}) error {
	updates["version"] = gorm.Expr("version + 1")
	updates["updated_at"] = time.Now().UTC()

	result := tx.Model(&model.Task{}).
		Where("id = ? AND version = ?", taskID, expectedVersion).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (r *Repository) SoftDeleteTaskTx(tx *gorm.DB, taskID uuid.UUID) error {
	result := tx.Where("id = ?", taskID).Delete(&model.Task{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (r *Repository) GetDirectChildren(orgID, parentID uuid.UUID) ([]model.Task, error) {
	var children []model.Task
	err := r.db.Where("organization_id = ? AND parent_id = ?", orgID, parentID).
		Order("sort_order ASC, created_at ASC").
		Find(&children).Error
	return children, err
}

func (r *Repository) CountDirectChildren(tx *gorm.DB, parentID uuid.UUID) (int64, error) {
	var count int64
	err := tx.Model(&model.Task{}).Where("parent_id = ?", parentID).Count(&count).Error
	return count, err
}

func (r *Repository) CountIncompleteRequiredChildren(tx *gorm.DB, taskID uuid.UUID) (int64, error) {
	var count int64
	err := tx.Model(&model.Task{}).
		Where("parent_id = ? AND status != ? AND status != ?", taskID, "completed", "cancelled").
		Count(&count).Error
	return count, err
}

func (r *Repository) CheckHasExceededChildDue(tx *gorm.DB, parentID uuid.UUID, newDueAt time.Time) (bool, error) {
	var count int64
	err := tx.Model(&model.Task{}).
		Where("parent_id = ? AND due_at > ? AND status != ?", parentID, newDueAt, "cancelled").
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) CheckIsDescendant(tx *gorm.DB, orgID, ancestorID, targetID uuid.UUID) (bool, error) {
	if ancestorID == targetID {
		return true, nil
	}

	db := r.db
	if tx != nil {
		db = tx
	}

	currID := &targetID
	visited := make(map[uuid.UUID]bool)

	for currID != nil {
		if visited[*currID] {
			return true, nil // Loop detected in existing data
		}
		visited[*currID] = true

		if *currID == ancestorID {
			return true, nil // targetID is indeed a descendant of ancestorID
		}

		var t model.Task
		err := db.Select("id", "parent_id").
			Where("organization_id = ? AND id = ?", orgID, *currID).
			First(&t).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			return false, err
		}
		currID = t.ParentID
	}

	return false, nil
}

func (r *Repository) RollupParentProgressTx(tx *gorm.DB, parentID uuid.UUID) error {
	currentParentID := &parentID

	for currentParentID != nil {
		var parent model.Task
		if err := tx.Where("id = ?", *currentParentID).First(&parent).Error; err != nil {
			return err
		}

		// Fetch all direct children
		var children []model.Task
		if err := tx.Where("parent_id = ?", *currentParentID).Find(&children).Error; err != nil {
			return err
		}

		var totalWeight int64 = 0
		var weightedProgressSum float64 = 0

		for _, child := range children {
			// Rule: Exclude cancelled tasks from progress formula to prevent deadlock
			if child.Status == "cancelled" {
				continue
			}
			weight := int64(child.Weight)
			if weight <= 0 {
				weight = 1
			}
			totalWeight += weight
			weightedProgressSum += float64(weight) * float64(child.Progress)
		}

		var newProgress int16 = parent.Progress
		if totalWeight > 0 {
			calc := math.Round(weightedProgressSum / float64(totalWeight))
			newProgress = int16(calc)
			if newProgress > 100 {
				newProgress = 100
			} else if newProgress < 0 {
				newProgress = 0
			}
		}

		if err := tx.Model(&model.Task{}).
			Where("id = ?", parent.ID).
			Updates(map[string]interface{}{
				"progress":   newProgress,
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		currentParentID = parent.ParentID
	}

	return nil
}

func (r *Repository) ListTasks(orgID uuid.UUID, filter TaskFilter) ([]model.Task, int64, error) {
	query := r.db.Model(&model.Task{}).Where("organization_id = ?", orgID)

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Priority > 0 {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.IsAssigned != nil {
		query = query.Where("is_assigned = ?", *filter.IsAssigned)
	}
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		query = query.Where("(title ILIKE ? OR code ILIKE ?)", like, like)
	}
	if filter.ParentID != nil {
		query = query.Where("parent_id = ?", filter.ParentID)
	}
	if filter.RootTaskID != nil {
		query = query.Where("root_task_id = ?", filter.RootTaskID)
	}
	if filter.DueBefore != nil {
		query = query.Where("due_at <= ?", filter.DueBefore)
	}
	if filter.DueAfter != nil {
		query = query.Where("due_at >= ?", filter.DueAfter)
	}
	if filter.Overdue {
		now := time.Now().UTC()
		query = query.Where("due_at < ? AND status NOT IN ('completed', 'cancelled')", now)
	}
	if filter.AssigneeID != nil {
		query = query.Joins("JOIN task_assignments ON task_assignments.task_id = tasks.id").
			Where("task_assignments.assignee_user_id = ?", *filter.AssigneeID)
	}
	if filter.DepartmentID != nil {
		query = query.Joins("JOIN task_assignments ON task_assignments.task_id = tasks.id").
			Where("task_assignments.assignee_department_id = ?", *filter.DepartmentID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Sorting
	sortBy := "created_at"
	allowedSorts := map[string]bool{
		"created_at": true, "due_at": true, "priority": true, "progress": true, "title": true, "sort_order": true,
	}
	if allowedSorts[filter.SortBy] {
		sortBy = filter.SortBy
	}
	sortOrder := "DESC"
	if filter.SortOrder == "asc" || filter.SortOrder == "ASC" {
		sortOrder = "ASC"
	}

	var list []model.Task
	err := query.Order(sortBy + " " + sortOrder).
		Limit(pageSize).
		Offset(offset).
		Preload("Assignments.AssigneeUser").
		Preload("Assignments.AssigneeDepartment").
		Find(&list).Error

	return list, total, err
}

func (r *Repository) GetLatestReview(tx *gorm.DB, taskID uuid.UUID) (*model.TaskReview, error) {
	var review model.TaskReview
	err := tx.Where("task_id = ?", taskID).Order("created_at DESC").First(&review).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &review, nil
}

func (r *Repository) GetUpdates(taskID uuid.UUID) ([]model.TaskUpdate, error) {
	var updates []model.TaskUpdate
	err := r.db.Where("task_id = ?", taskID).
		Preload("Submitter").
		Preload("Attachments").
		Order("submitted_at DESC").
		Find(&updates).Error
	return updates, err
}

func (r *Repository) GetReviews(taskID uuid.UUID) ([]model.TaskReview, error) {
	var reviews []model.TaskReview
	err := r.db.Where("task_id = ?", taskID).
		Preload("Reviewer").
		Order("reviewed_at DESC").
		Find(&reviews).Error
	return reviews, err
}

func (r *Repository) GetEvents(taskID uuid.UUID) ([]model.TaskEvent, error) {
	var events []model.TaskEvent
	err := r.db.Where("task_id = ?", taskID).
		Preload("Actor").
		Order("created_at DESC").
		Find(&events).Error
	return events, err
}

func (r *Repository) GetComments(taskID uuid.UUID) ([]model.TaskComment, error) {
	var comments []model.TaskComment
	err := r.db.Where("task_id = ?", taskID).
		Preload("Author").
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

func (r *Repository) CreateComment(comment *model.TaskComment) error {
	return r.db.Create(comment).Error
}

func (r *Repository) GetAssignments(taskID uuid.UUID) ([]model.TaskAssignment, error) {
	var list []model.TaskAssignment
	err := r.db.Where("task_id = ?", taskID).
		Preload("AssigneeUser").
		Preload("AssigneeDepartment").
		Preload("ResponsibleUser").
		Find(&list).Error
	return list, err
}

func (r *Repository) CreateAssignment(assignment *model.TaskAssignment) error {
	return r.db.Create(assignment).Error
}

func (r *Repository) DeleteAssignment(taskID, assignmentID uuid.UUID) error {
	result := r.db.Where("task_id = ? AND id = ?", taskID, assignmentID).Delete(&model.TaskAssignment{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetTaskTree loads an arbitrary-depth task tree with assignments preloaded.
func (r *Repository) GetTaskTree(orgID, taskID uuid.UUID) (*model.Task, error) {
	var target model.Task
	if err := r.db.Where("organization_id = ? AND id = ?", orgID, taskID).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	rootID := taskID
	if target.RootTaskID != nil && *target.RootTaskID != uuid.Nil {
		rootID = *target.RootTaskID
	}

	var allTasks []model.Task
	err := r.db.Where("organization_id = ? AND (id = ? OR root_task_id = ?)", orgID, rootID, rootID).
		Preload("Assignments.AssigneeUser").
		Preload("Assignments.AssigneeDepartment").
		Preload("Assignments.ResponsibleUser").
		Order("sort_order ASC, created_at ASC").
		Find(&allTasks).Error
	if err != nil {
		return nil, err
	}

	// Map tasks by ID and build tree in-memory
	nodeMap := make(map[uuid.UUID]*model.Task, len(allTasks))
	for i := range allTasks {
		allTasks[i].Children = make([]model.Task, 0)
		nodeMap[allTasks[i].ID] = &allTasks[i]
	}

	for i := range allTasks {
		curr := &allTasks[i]
		if curr.ParentID != nil {
			if parent, exists := nodeMap[*curr.ParentID]; exists {
				parent.Children = append(parent.Children, *curr)
			}
		}
	}

	res, exists := nodeMap[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	// Helper to recursively sort children by sort_order ASC
	var sortChildren func(n *model.Task)
	sortChildren = func(n *model.Task) {
		sort.SliceStable(n.Children, func(i, j int) bool {
			if n.Children[i].SortOrder == n.Children[j].SortOrder {
				return n.Children[i].CreatedAt.Before(n.Children[j].CreatedAt)
			}
			return n.Children[i].SortOrder < n.Children[j].SortOrder
		})
		for i := range n.Children {
			if childInMap, ok := nodeMap[n.Children[i].ID]; ok {
				sortChildren(childInMap)
				n.Children[i] = *childInMap
			}
		}
	}
	sortChildren(res)

	return res, nil
}

// --- Attachment Repository Methods ---

func (r *Repository) CreateAttachment(tx *gorm.DB, att *model.TaskAttachment) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.Create(att).Error
}

func (r *Repository) GetAttachments(taskID uuid.UUID) ([]model.TaskAttachment, error) {
	var attachments []model.TaskAttachment
	err := r.db.Where("task_id = ?", taskID).
		Preload("Uploader").
		Order("created_at DESC").
		Find(&attachments).Error
	return attachments, err
}

func (r *Repository) GetAttachmentByID(taskID, attID uuid.UUID) (*model.TaskAttachment, error) {
	var att model.TaskAttachment
	err := r.db.Where("task_id = ? AND id = ?", taskID, attID).
		Preload("Uploader").
		First(&att).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAttachmentNotFound
		}
		return nil, err
	}
	return &att, nil
}

func (r *Repository) ConfirmAttachment(taskID, attID uuid.UUID, checksum string) error {
	updates := map[string]interface{}{
		"status":     "active",
		"updated_at": time.Now().UTC(),
	}
	if checksum != "" {
		updates["checksum"] = checksum
	}
	result := r.db.Model(&model.TaskAttachment{}).
		Where("task_id = ? AND id = ?", taskID, attID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAttachmentNotFound
	}
	return nil
}

func (r *Repository) DeleteAttachment(taskID, attID uuid.UUID) error {
	result := r.db.Where("task_id = ? AND id = ?", taskID, attID).Delete(&model.TaskAttachment{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAttachmentNotFound
	}
	return nil
}

func (r *Repository) LinkAttachmentsToUpdate(tx *gorm.DB, taskID, updateID uuid.UUID, attIDs []uuid.UUID) error {
	if len(attIDs) == 0 {
		return nil
	}
	return tx.Model(&model.TaskAttachment{}).
		Where("task_id = ? AND id IN ?", taskID, attIDs).
		Updates(map[string]interface{}{
			"update_id":  updateID,
			"status":     "active",
			"updated_at": time.Now().UTC(),
		}).Error
}
