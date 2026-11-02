package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"taskmgmt/internal/model"
	appErrors "taskmgmt/internal/platform/errors"
)

type Notifier interface {
	Notify(orgID, recipientID uuid.UUID, taskID *uuid.UUID, notifType, title, body string) (*model.Notification, error)
}

type Service struct {
	repo     *Repository
	notifier Notifier
}

func NewService(repo *Repository, notifier ...Notifier) *Service {
	s := &Service{repo: repo}
	if len(notifier) > 0 {
		s.notifier = notifier[0]
	}
	return s
}

func (s *Service) notifyUser(orgID, recipientID uuid.UUID, taskID *uuid.UUID, notifType, title, body string) {
	if s.notifier != nil && recipientID != uuid.Nil {
		go func() {
			_, _ = s.notifier.Notify(orgID, recipientID, taskID, notifType, title, body)
		}()
	}
}

func (s *Service) CreateTask(orgID, actorID uuid.UUID, req CreateTaskRequest) (*model.Task, error) {
	taskID := uuid.New()
	code := fmt.Sprintf("TASK-%d-%05d", time.Now().Year(), time.Now().Unix()%100000)

	approvalReq := true
	if req.ApprovalRequired != nil {
		approvalReq = *req.ApprovalRequired
	}

	task := &model.Task{
		BaseModel:        model.BaseModel{ID: taskID},
		OrganizationID:   orgID,
		ParentID:         nil,
		RootTaskID:       &taskID,
		Code:             code,
		Title:            req.Title,
		Description:      req.Description,
		Status:           "assigned",
		Priority:         req.Priority,
		Progress:         0,
		StartAt:          req.StartAt,
		DueAt:            req.DueAt,
		CreatedBy:        actorID,
		ApprovalRequired: approvalReq,
		Weight:           1,
		Version:          1,
	}

	assignment := &model.TaskAssignment{
		OrganizationID: orgID,
		TaskID:         taskID,
		IsPrimary:      true,
		AssignmentRole: "responsible",
		AssignedBy:     actorID,
	}

	if req.InitialAssignment.Type == "user" {
		assignment.AssigneeUserID = &req.InitialAssignment.ID
	} else if req.InitialAssignment.Type == "department" {
		assignment.AssigneeDepartmentID = &req.InitialAssignment.ID
		assignment.ResponsibleUserID = req.InitialAssignment.ResponsibleUserID
	}

	eventData, _ := json.Marshal(map[string]interface{}{
		"action":     "create_root_task",
		"title":      task.Title,
		"assigned":   req.InitialAssignment,
		"created_by": actorID,
	})

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateTaskTx(tx, task, assignment); err != nil {
			return err
		}

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.created",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	if s.notifier != nil {
		if req.InitialAssignment.Type == "user" {
			s.notifyUser(orgID, req.InitialAssignment.ID, &taskID, "task_assigned",
				"Bạn được giao công việc mới: "+task.Title,
				fmt.Sprintf("Mã công việc: %s", task.Code))
		} else if req.InitialAssignment.Type == "department" && req.InitialAssignment.ResponsibleUserID != nil {
			s.notifyUser(orgID, *req.InitialAssignment.ResponsibleUserID, &taskID, "task_assigned",
				"Đơn vị bạn được giao việc mới: "+task.Title,
				fmt.Sprintf("Mã công việc: %s. Bạn được chỉ định phụ trách chính.", task.Code))
		}
	}

	return s.repo.GetByID(orgID, taskID)
}

func (s *Service) CreateChildTask(orgID, actorID, parentID uuid.UUID, req CreateChildTaskRequest) (*model.Task, error) {
	parent, err := s.repo.GetByID(orgID, parentID)
	if err != nil {
		return nil, err
	}

	if parent.Status == "completed" || parent.Status == "cancelled" {
		return nil, appErrors.ErrTaskCompleted
	}

	// Rule: Child start date cannot precede parent start date
	if parent.StartAt != nil && req.StartAt != nil && req.StartAt.Before(*parent.StartAt) {
		return nil, appErrors.ErrParentStartExceeded
	}

	// Rule: Child due date cannot exceed parent due date
	if parent.DueAt != nil {
		if req.DueAt == nil || req.DueAt.After(*parent.DueAt) {
			return nil, appErrors.ErrParentDueExceeded
		}
	}

	taskID := uuid.New()
	code := fmt.Sprintf("%s-%02d", parent.Code, time.Now().Unix()%100)

	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}

	approvalReq := true
	if req.ApprovalRequired != nil {
		approvalReq = *req.ApprovalRequired
	}

	child := &model.Task{
		BaseModel:        model.BaseModel{ID: taskID},
		OrganizationID:   orgID,
		ParentID:         &parentID,
		RootTaskID:       parent.RootTaskID,
		Code:             code,
		Title:            req.Title,
		Description:      req.Description,
		Status:           "assigned",
		Priority:         req.Priority,
		Progress:         0,
		StartAt:          req.StartAt,
		DueAt:            req.DueAt,
		CreatedBy:        actorID,
		ApprovalRequired: approvalReq,
		Weight:           weight,
		Version:          1,
	}

	assignment := &model.TaskAssignment{
		OrganizationID: orgID,
		TaskID:         taskID,
		IsPrimary:      true,
		AssignmentRole: "responsible",
		AssignedBy:     actorID,
	}

	if req.InitialAssignment.Type == "user" {
		assignment.AssigneeUserID = &req.InitialAssignment.ID
	} else if req.InitialAssignment.Type == "department" {
		assignment.AssigneeDepartmentID = &req.InitialAssignment.ID
		assignment.ResponsibleUserID = req.InitialAssignment.ResponsibleUserID
	}

	eventData, _ := json.Marshal(map[string]interface{}{
		"action":    "create_child_task",
		"parent_id": parentID,
		"weight":    weight,
		"due_at":    req.DueAt,
	})

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateTaskTx(tx, child, assignment); err != nil {
			return err
		}

		// Rollup parent progress after new child added
		if err := s.repo.RollupParentProgressTx(tx, parentID); err != nil {
			return err
		}

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.child_created",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	if s.notifier != nil {
		if req.InitialAssignment.Type == "user" {
			s.notifyUser(orgID, req.InitialAssignment.ID, &taskID, "task_assigned",
				"Bạn được giao công việc con mới: "+child.Title,
				fmt.Sprintf("Mã: %s. Thuộc nhiệm vụ cha: %s", child.Code, parent.Title))
		} else if req.InitialAssignment.Type == "department" && req.InitialAssignment.ResponsibleUserID != nil {
			s.notifyUser(orgID, *req.InitialAssignment.ResponsibleUserID, &taskID, "task_assigned",
				"Đơn vị bạn được giao việc con mới: "+child.Title,
				fmt.Sprintf("Mã: %s. Thuộc nhiệm vụ cha: %s. Bạn phụ trách chính.", child.Code, parent.Title))
		}
	}

	return s.repo.GetByID(orgID, taskID)
}

func (s *Service) UpdateTask(orgID, actorID, taskID uuid.UUID, req UpdateTaskRequest) (*model.Task, error) {
	updates := make(map[string]interface{})

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if req.DueAt != nil {
			// Check if any child tasks exceed the new due date
			hasExceeded, err := s.repo.CheckHasExceededChildDue(tx, taskID, *req.DueAt)
			if err != nil {
				return err
			}
			if hasExceeded {
				return appErrors.ErrParentDueExceeded
			}

			// If this task has a parent, check if new due date exceeds parent due date
			if task.ParentID != nil {
				var parent model.Task
				if err := tx.Where("id = ?", *task.ParentID).First(&parent).Error; err == nil {
					if parent.DueAt != nil && req.DueAt.After(*parent.DueAt) {
						return appErrors.ErrParentDueExceeded
					}
				}
			}
			updates["due_at"] = *req.DueAt
		}

		if err := s.repo.UpdateWithVersion(tx, taskID, req.Version, updates); err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":  "update_task",
			"updates": updates,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.updated",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(orgID, taskID)
}

func (s *Service) SubmitProgress(orgID, actorID, taskID uuid.UUID, req CreateTaskUpdateRequest) (*model.TaskUpdate, error) {
	var updateRecord model.TaskUpdate

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status == "completed" || task.Status == "cancelled" {
			return appErrors.ErrTaskCompleted
		}

		// Check if task has children: if it has active children, direct progress override is disallowed
		var childCount int64
		_ = tx.Model(&model.Task{}).Where("parent_id = ? AND status != 'cancelled'", taskID).Count(&childCount).Error

		newProgress := req.Progress
		if childCount > 0 {
			// Cannot set leaf progress directly when having children
			newProgress = task.Progress
		}

		updateRecord = model.TaskUpdate{
			BaseModel:   model.BaseModel{ID: uuid.New()},
			TaskID:      taskID,
			SubmittedBy: actorID,
			Progress:    newProgress,
			Content:     req.Content,
			SubmittedAt: time.Now().UTC(),
		}
		if err := tx.Create(&updateRecord).Error; err != nil {
			return err
		}

		// Update task status and leaf progress
		taskUpdates := map[string]interface{}{
			"updated_at": time.Now().UTC(),
		}
		if childCount == 0 {
			taskUpdates["progress"] = newProgress
			if task.Status == "assigned" {
				taskUpdates["status"] = "in_progress"
			}
		}

		if err := tx.Model(&model.Task{}).Where("id = ?", taskID).Updates(taskUpdates).Error; err != nil {
			return err
		}

		// Rollup parent progress
		if task.ParentID != nil {
			if err := s.repo.RollupParentProgressTx(tx, *task.ParentID); err != nil {
				return err
			}
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":    "submit_progress",
			"progress":  newProgress,
			"content":   req.Content,
			"update_id": updateRecord.ID,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.progress_updated",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	if s.notifier != nil {
		task, _ := s.repo.GetByID(orgID, taskID)
		if task != nil && task.CreatedBy != actorID {
			s.notifyUser(orgID, task.CreatedBy, &taskID, "task_progress",
				fmt.Sprintf("Tiến độ mới cập nhật cho %s: %d%%", task.Code, req.Progress),
				req.Content)
		}
	}

	return &updateRecord, nil
}

func (s *Service) ReviewTask(orgID, actorID, taskID uuid.UUID, req CreateTaskReviewRequest) (*model.TaskReview, error) {
	var review model.TaskReview

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status == "completed" || task.Status == "cancelled" {
			return fmt.Errorf("task cannot be reviewed in status: %s", task.Status)
		}

		if req.Decision == "approved" {
			// Check that all required children are completed
			incompleteCount, err := s.repo.CountIncompleteRequiredChildren(tx, taskID)
			if err != nil {
				return err
			}
			if incompleteCount > 0 {
				return appErrors.ErrReviewIncomplete
			}
		}

		review = model.TaskReview{
			BaseModel:  model.BaseModel{ID: uuid.New()},
			TaskID:     taskID,
			UpdateID:   req.UpdateID,
			ReviewerID: actorID,
			Decision:   req.Decision,
			Comment:    req.Comment,
			ReviewedAt: time.Now().UTC(),
		}
		if err := tx.Create(&review).Error; err != nil {
			return err
		}

		newStatus := "completed"
		if req.Decision == "revision_required" {
			newStatus = "revision_required"
		}

		if err := tx.Model(&model.Task{}).
			Where("id = ?", taskID).
			Updates(map[string]interface{}{
				"status":     newStatus,
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":    "review_task",
			"decision":  req.Decision,
			"comment":   req.Comment,
			"review_id": review.ID,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.reviewed",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	if s.notifier != nil {
		task, _ := s.repo.GetByID(orgID, taskID)
		if task != nil {
			decisionText := "đã được nghiệm thu đạt yêu cầu"
			if req.Decision == "revision_required" {
				decisionText = "yêu cầu điều chỉnh, bổ sung nội dung"
			}
			for _, a := range task.Assignments {
				if a.AssigneeUserID != nil && *a.AssigneeUserID != actorID {
					s.notifyUser(orgID, *a.AssigneeUserID, &taskID, "task_reviewed",
						fmt.Sprintf("Công việc %s %s", task.Code, decisionText),
						req.Comment)
				}
			}
		}
	}

	return &review, nil
}

func (s *Service) ReopenTask(orgID, actorID, taskID uuid.UUID, req ReopenTaskRequest) (*model.Task, error) {
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status != "completed" {
			return fmt.Errorf("only completed tasks may be reopened")
		}

		latestReview, err := s.repo.GetLatestReview(tx, taskID)
		if err != nil {
			return err
		}

		// Rule: Only the latest reviewer or a higher manager may reopen
		if latestReview != nil && latestReview.ReviewerID != actorID {
			// shortcut: In MVP, permit task creator or latest reviewer; full RBAC scope hierarchy in phase 3
			if task.CreatedBy != actorID {
				return appErrors.ErrReopenUnauthorized
			}
		}

		prevStatus := task.Status
		newStatus := "in_progress"

		if err := tx.Model(&model.Task{}).
			Where("id = ?", taskID).
			Updates(map[string]interface{}{
				"status":     newStatus,
				"version":    gorm.Expr("version + 1"),
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":      "reopen_task",
			"reason":      req.Reason,
			"prev_status": prevStatus,
			"new_status":  newStatus,
			"actor_id":    actorID,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.reopened",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	task, err := s.repo.GetByID(orgID, taskID)
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		for _, a := range task.Assignments {
			if a.AssigneeUserID != nil && *a.AssigneeUserID != actorID {
				s.notifyUser(orgID, *a.AssigneeUserID, &taskID, "task_reopened",
					fmt.Sprintf("Công việc %s đã được mở lại", task.Code),
					req.Reason)
			}
		}
	}
	return task, nil
}

func (s *Service) CancelTask(orgID, actorID, taskID uuid.UUID, reason string) (*model.Task, error) {
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status == "completed" {
			return fmt.Errorf("cannot cancel an already completed task")
		}

		prevStatus := task.Status
		if err := tx.Model(&model.Task{}).
			Where("id = ?", taskID).
			Updates(map[string]interface{}{
				"status":     "cancelled",
				"version":    gorm.Expr("version + 1"),
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}

		// Recalculate parent progress with cancelled child excluded
		if task.ParentID != nil {
			if err := s.repo.RollupParentProgressTx(tx, *task.ParentID); err != nil {
				return err
			}
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":      "cancel_task",
			"reason":      reason,
			"prev_status": prevStatus,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.cancelled",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	task, err := s.repo.GetByID(orgID, taskID)
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		for _, a := range task.Assignments {
			if a.AssigneeUserID != nil && *a.AssigneeUserID != actorID {
				s.notifyUser(orgID, *a.AssigneeUserID, &taskID, "task_cancelled",
					fmt.Sprintf("Công việc %s đã bị hủy", task.Code),
					reason)
			}
		}
	}
	return task, nil
}

func (s *Service) ListTasks(orgID uuid.UUID, filter TaskFilter) ([]model.Task, int64, error) {
	return s.repo.ListTasks(orgID, filter)
}

func (s *Service) GetTaskByID(orgID, taskID uuid.UUID) (*model.Task, error) {
	return s.repo.GetByID(orgID, taskID)
}

func (s *Service) GetTaskChildren(orgID, taskID uuid.UUID) ([]model.Task, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetDirectChildren(orgID, taskID)
}

func (s *Service) GetTaskUpdates(orgID, taskID uuid.UUID) ([]model.TaskUpdate, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetUpdates(taskID)
}

func (s *Service) GetTaskReviews(orgID, taskID uuid.UUID) ([]model.TaskReview, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetReviews(taskID)
}

func (s *Service) GetTaskEvents(orgID, taskID uuid.UUID) ([]model.TaskEvent, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetEvents(taskID)
}

func (s *Service) GetTaskComments(orgID, taskID uuid.UUID) ([]model.TaskComment, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetComments(taskID)
}

func (s *Service) AddComment(orgID, actorID, taskID uuid.UUID, content string) (*model.TaskComment, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	comment := &model.TaskComment{
		BaseModel: model.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		TaskID:    taskID,
		AuthorID:  actorID,
		Content:   content,
	}
	if err := s.repo.CreateComment(comment); err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *Service) GetTaskTree(orgID, taskID uuid.UUID) (*model.Task, error) {
	return s.repo.GetTaskTree(orgID, taskID)
}

func (s *Service) GetAssignments(orgID, taskID uuid.UUID) ([]model.TaskAssignment, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetAssignments(taskID)
}

func (s *Service) AddAssignment(orgID, actorID, taskID uuid.UUID, req CreateAssignmentRequest) (*model.TaskAssignment, error) {
	task, err := s.repo.GetByID(orgID, taskID)
	if err != nil {
		return nil, err
	}

	role := "responsible"
	if req.AssignmentRole != "" {
		role = req.AssignmentRole
	}

	assignment := &model.TaskAssignment{
		BaseModel:      model.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		OrganizationID: orgID,
		TaskID:         taskID,
		IsPrimary:      req.IsPrimary,
		AssignmentRole: role,
		AssignedBy:     actorID,
		Note:           req.Note,
	}

	if req.Type == "user" {
		assignment.AssigneeUserID = &req.ID
	} else if req.Type == "department" {
		assignment.AssigneeDepartmentID = &req.ID
		assignment.ResponsibleUserID = req.ResponsibleUserID
	}

	if err := s.repo.CreateAssignment(assignment); err != nil {
		return nil, err
	}

	if s.notifier != nil {
		if req.Type == "user" {
			s.notifyUser(orgID, req.ID, &taskID, "task_assigned",
				"Bạn được phân công công việc: "+task.Title, req.Note)
		} else if req.Type == "department" && req.ResponsibleUserID != nil {
			s.notifyUser(orgID, *req.ResponsibleUserID, &taskID, "task_assigned",
				"Đơn vị bạn được phân công việc: "+task.Title, req.Note)
		}
	}

	return assignment, nil
}

func (s *Service) DeleteAssignment(orgID, taskID, assignmentID uuid.UUID) error {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return err
	}
	return s.repo.DeleteAssignment(taskID, assignmentID)
}
