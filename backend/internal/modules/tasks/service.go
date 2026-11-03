package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"taskmgmt/internal/config"
	"taskmgmt/internal/model"
	appErrors "taskmgmt/internal/platform/errors"
	"taskmgmt/internal/platform/storage"
)

type Notifier interface {
	Notify(orgID, recipientID uuid.UUID, taskID *uuid.UUID, notifType, title, body string) (*model.Notification, error)
}

type Service struct {
	repo     *Repository
	notifier Notifier
	storage  storage.StorageService
}

func NewService(repo *Repository, args ...interface{}) *Service {
	s := &Service{repo: repo}
	for _, arg := range args {
		switch v := arg.(type) {
		case Notifier:
			s.notifier = v
		case storage.StorageService:
			s.storage = v
		}
	}
	if s.storage == nil {
		s.storage, _ = storage.NewStorage(&config.Config{
			StorageDriver: "local",
			UploadDir:     "./uploads",
		})
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

	status := "unassigned"
	isAssigned := false

	if req.IsDraft {
		status = "draft"
		if req.InitialAssignment != nil && req.InitialAssignment.ID != uuid.Nil {
			isAssigned = true
		}
	} else if req.InitialAssignment != nil && req.InitialAssignment.ID != uuid.Nil {
		status = "assigned"
		isAssigned = true
	}

	task := &model.Task{
		BaseModel:        model.BaseModel{ID: taskID},
		OrganizationID:   orgID,
		ParentID:         nil,
		RootTaskID:       &taskID,
		Code:             code,
		Title:            req.Title,
		Description:      req.Description,
		Status:           status,
		Priority:         req.Priority,
		Progress:         0,
		StartAt:          req.StartAt,
		DueAt:            req.DueAt,
		CreatedBy:        actorID,
		ApprovalRequired: approvalReq,
		Weight:           1,
		Version:          1,
		SortOrder:        req.SortOrder,
		IsAssigned:       isAssigned,
	}

	var assignment *model.TaskAssignment
	if req.InitialAssignment != nil && req.InitialAssignment.ID != uuid.Nil {
		assignment = &model.TaskAssignment{
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
	}

	actionType := "create_root_task"
	eventType := "task.created"
	if req.IsDraft {
		actionType = "create_draft_task"
		eventType = "task.draft_created"
	}

	eventData, _ := json.Marshal(map[string]interface{}{
		"action":     actionType,
		"title":      task.Title,
		"is_draft":   req.IsDraft,
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
			EventType:      eventType,
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	if s.notifier != nil && status == "assigned" && req.InitialAssignment != nil {
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

	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}

	approvalReq := true
	if req.ApprovalRequired != nil {
		approvalReq = *req.ApprovalRequired
	}

	status := "unassigned"
	isAssigned := false

	if req.IsDraft {
		status = "draft"
		if req.InitialAssignment != nil && req.InitialAssignment.ID != uuid.Nil {
			isAssigned = true
		}
	} else if req.InitialAssignment != nil && req.InitialAssignment.ID != uuid.Nil {
		status = "assigned"
		isAssigned = true
	}

	var childCount int64
	_ = s.repo.DB().Model(&model.Task{}).Where("parent_id = ?", parentID).Count(&childCount).Error
	code := fmt.Sprintf("%s-%02d", parent.Code, childCount+1)

	child := &model.Task{
		BaseModel:        model.BaseModel{ID: taskID},
		OrganizationID:   orgID,
		ParentID:         &parentID,
		RootTaskID:       parent.RootTaskID,
		Code:             code,
		Title:            req.Title,
		Description:      req.Description,
		Status:           status,
		Priority:         req.Priority,
		Progress:         0,
		StartAt:          req.StartAt,
		DueAt:            req.DueAt,
		CreatedBy:        actorID,
		ApprovalRequired: approvalReq,
		Weight:           weight,
		Version:          1,
		SortOrder:        req.SortOrder,
		IsAssigned:       isAssigned,
	}

	var assignment *model.TaskAssignment
	if req.InitialAssignment != nil && req.InitialAssignment.ID != uuid.Nil {
		assignment = &model.TaskAssignment{
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
	}

	actionType := "create_child_task"
	eventType := "task.child_created"
	if req.IsDraft {
		actionType = "create_draft_child_task"
		eventType = "task.draft_child_created"
	}

	eventData, _ := json.Marshal(map[string]interface{}{
		"action":    actionType,
		"parent_id": parentID,
		"weight":    weight,
		"due_at":    req.DueAt,
		"is_draft":  req.IsDraft,
	})

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateTaskTx(tx, child, assignment); err != nil {
			return err
		}

		// Rollup parent progress after new child added (only if not draft)
		if !req.IsDraft {
			if err := s.repo.RollupParentProgressTx(tx, parentID); err != nil {
				return err
			}
		}

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      eventType,
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	if s.notifier != nil && status == "assigned" && req.InitialAssignment != nil {
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

func (s *Service) PublishTask(orgID, actorID, taskID uuid.UUID) (*model.Task, error) {
	var task *model.Task

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		t, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}
		if t.Status != "draft" {
			return appErrors.ErrTaskNotDraft
		}

		var assignCount int64
		_ = tx.Model(&model.TaskAssignment{}).Where("task_id = ?", taskID).Count(&assignCount).Error

		newStatus := "unassigned"
		isAssigned := false
		if assignCount > 0 {
			newStatus = "assigned"
			isAssigned = true
		}

		updates := map[string]interface{}{
			"status":      newStatus,
			"is_assigned": isAssigned,
		}
		if err := s.repo.UpdateWithVersion(tx, taskID, t.Version, updates); err != nil {
			return err
		}

		// Rollup progress if this task has a parent
		if t.ParentID != nil {
			if err := s.repo.RollupParentProgressTx(tx, *t.ParentID); err != nil {
				return err
			}
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":      "publish_task",
			"new_status":  newStatus,
			"is_assigned": isAssigned,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.published",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})

	if err != nil {
		return nil, err
	}

	task, err = s.repo.GetByID(orgID, taskID)
	if err != nil {
		return nil, err
	}

	if s.notifier != nil && task.IsAssigned {
		for _, a := range task.Assignments {
			if a.AssigneeUserID != nil {
				s.notifyUser(orgID, *a.AssigneeUserID, &taskID, "task_published",
					"Nhiệm vụ đã được phát hành: "+task.Title,
					fmt.Sprintf("Mã công việc: %s", task.Code))
			}
		}
	}

	return task, nil
}

func (s *Service) ClaimTask(orgID, actorID, taskID uuid.UUID) (*model.Task, error) {
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status == "completed" || task.Status == "cancelled" {
			return appErrors.ErrTaskCompleted
		}
		if task.Status == "draft" {
			return fmt.Errorf("không thể nhận công việc đang ở trạng thái bản nháp")
		}
		if task.IsAssigned {
			return appErrors.ErrTaskAlreadyAssigned
		}

		assignment := &model.TaskAssignment{
			BaseModel:      model.BaseModel{ID: uuid.New()},
			OrganizationID: orgID,
			TaskID:         taskID,
			AssigneeUserID: &actorID,
			IsPrimary:      true,
			AssignmentRole: "responsible",
			AssignedBy:     actorID,
			Note:           "Tự nhận công việc qua hệ thống",
		}
		if err := tx.Create(assignment).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"status":      "assigned",
			"is_assigned": true,
		}
		if err := s.repo.UpdateWithVersion(tx, taskID, task.Version, updates); err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":   "claim_task",
			"actor_id": actorID,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.claimed",
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

	if s.notifier != nil && task.CreatedBy != actorID {
		s.notifyUser(orgID, task.CreatedBy, &taskID, "task_claimed",
			fmt.Sprintf("Công việc %s đã có người nhận", task.Code),
			fmt.Sprintf("Nhân sự đã nhận nhiệm vụ %s", task.Title))
	}

	return task, nil
}

func (s *Service) BlockTask(orgID, actorID, taskID uuid.UUID, reason string) (*model.Task, error) {
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status == "completed" || task.Status == "cancelled" {
			return appErrors.ErrTaskCompleted
		}
		if task.Status == "draft" {
			return fmt.Errorf("công việc bản nháp không thể báo vướng mắc")
		}

		updates := map[string]interface{}{
			"status":         "blocked",
			"blocked_reason": reason,
		}
		if err := s.repo.UpdateWithVersion(tx, taskID, task.Version, updates); err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action": "block_task",
			"reason": reason,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.blocked",
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

	if s.notifier != nil && task.CreatedBy != actorID {
		s.notifyUser(orgID, task.CreatedBy, &taskID, "task_blocked",
			fmt.Sprintf("Vướng mắc trong công việc %s: %s", task.Code, task.Title),
			reason)
	}

	return task, nil
}

func (s *Service) UnblockTask(orgID, actorID, taskID uuid.UUID, note string) (*model.Task, error) {
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status != "blocked" {
			return fmt.Errorf("công việc hiện không ở trạng thái vướng mắc (blocked)")
		}

		updates := map[string]interface{}{
			"status":         "in_progress",
			"blocked_reason": "",
		}
		if err := s.repo.UpdateWithVersion(tx, taskID, task.Version, updates); err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action": "unblock_task",
			"note":   note,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.unblocked",
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
				s.notifyUser(orgID, *a.AssigneeUserID, &taskID, "task_unblocked",
					fmt.Sprintf("Công việc %s đã được gỡ vướng mắc", task.Code),
					note)
			}
		}
	}

	return task, nil
}

func (s *Service) SubmitTask(orgID, actorID, taskID uuid.UUID) (*model.Task, error) {
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status == "completed" || task.Status == "cancelled" || task.Status == "draft" {
			return appErrors.ErrInvalidTaskState
		}

		updates := map[string]interface{}{
			"status": "submitted",
		}
		if err := s.repo.UpdateWithVersion(tx, taskID, task.Version, updates); err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action": "submit_task",
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.submitted",
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

	if s.notifier != nil && task.CreatedBy != actorID {
		s.notifyUser(orgID, task.CreatedBy, &taskID, "task_submitted",
			fmt.Sprintf("Công việc %s đã nộp chờ nghiệm thu", task.Code),
			fmt.Sprintf("Nhiệm vụ %s đã hoàn tất và sẵn sàng kiểm tra nghiệm thu.", task.Title))
	}

	return task, nil
}

func (s *Service) MoveTask(orgID, actorID, taskID uuid.UUID, req MoveTaskRequest) (*model.Task, error) {
	var oldParentID *uuid.UUID
	var newParentID *uuid.UUID
	hasParentChange := false

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		updates := make(map[string]interface{})

		if req.SortOrder != nil {
			updates["sort_order"] = *req.SortOrder
		}

		if req.NewStatus != nil {
			updates["status"] = *req.NewStatus
		}

		if req.ParentID != nil {
			oldParentID = task.ParentID
			hasParentChange = true

			if *req.ParentID == uuid.Nil {
				// Move to root
				updates["parent_id"] = nil
				updates["root_task_id"] = &task.ID
				newParentID = nil
			} else {
				// Moving under new parent
				targetParentID := *req.ParentID
				if targetParentID == taskID {
					return appErrors.ErrCyclicDependency
				}

				// Check cyclic dependency
				isDescendant, err := s.repo.CheckIsDescendant(tx, orgID, taskID, targetParentID)
				if err != nil {
					return err
				}
				if isDescendant {
					return appErrors.ErrCyclicDependency
				}

				var parent model.Task
				if err := tx.Where("organization_id = ? AND id = ?", orgID, targetParentID).First(&parent).Error; err != nil {
					return fmt.Errorf("new parent not found: %w", err)
				}
				if parent.Status == "completed" || parent.Status == "cancelled" {
					return appErrors.ErrTaskCompleted
				}

				// Deadline check
				if parent.DueAt != nil && task.DueAt != nil && task.DueAt.After(*parent.DueAt) {
					return appErrors.ErrParentDueExceeded
				}

				newRootID := parent.RootTaskID
				if newRootID == nil || *newRootID == uuid.Nil {
					newRootID = &parent.ID
				}

				updates["parent_id"] = &targetParentID
				updates["root_task_id"] = newRootID
				newParentID = &targetParentID
			}
		}

		if err := s.repo.UpdateWithVersion(tx, taskID, req.Version, updates); err != nil {
			return err
		}

		// Rollup parent progress
		if hasParentChange {
			if oldParentID != nil {
				_ = s.repo.RollupParentProgressTx(tx, *oldParentID)
			}
			if newParentID != nil {
				_ = s.repo.RollupParentProgressTx(tx, *newParentID)
			}
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":  "move_task",
			"updates": updates,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.moved",
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

func (s *Service) DeleteTask(orgID, actorID, taskID uuid.UUID) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		task, err := s.repo.GetByIDForUpdate(tx, orgID, taskID)
		if err != nil {
			return err
		}

		if task.Status != "draft" {
			return appErrors.ErrCannotDeleteDispatchedTask
		}

		if err := s.repo.SoftDeleteTaskTx(tx, taskID); err != nil {
			return err
		}

		eventData, _ := json.Marshal(map[string]interface{}{
			"action":  "delete_draft_task",
			"task_id": taskID,
		})

		event := model.TaskEvent{
			ID:             uuid.New(),
			OrganizationID: orgID,
			TaskID:         taskID,
			ActorID:        actorID,
			EventType:      "task.deleted",
			Data:           datatypes.JSON(eventData),
			CreatedAt:      time.Now().UTC(),
		}
		return tx.Create(&event).Error
	})
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

		// Link attachments to this update
		if len(req.AttachmentIDs) > 0 {
			if err := s.repo.LinkAttachmentsToUpdate(tx, taskID, updateRecord.ID, req.AttachmentIDs); err != nil {
				return err
			}
		}

		// Update task status and leaf progress
		taskUpdates := map[string]interface{}{
			"updated_at": time.Now().UTC(),
		}
		if childCount == 0 {
			taskUpdates["progress"] = newProgress
			if newProgress == 100 {
				if task.ApprovalRequired {
					taskUpdates["status"] = "submitted"
				} else {
					taskUpdates["status"] = "completed"
				}
			} else if task.Status == "assigned" || task.Status == "revision_required" {
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
			"action":         "submit_progress",
			"progress":       newProgress,
			"content":        req.Content,
			"update_id":      updateRecord.ID,
			"attachment_ids": req.AttachmentIDs,
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

		if latestReview != nil && latestReview.ReviewerID != actorID {
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

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(assignment).Error; err != nil {
			return err
		}
		// If task was unassigned, promote to assigned
		if !task.IsAssigned {
			updates := map[string]interface{}{
				"is_assigned": true,
				"updated_at":  time.Now().UTC(),
			}
			if task.Status == "unassigned" {
				updates["status"] = "assigned"
			}
			return tx.Model(&model.Task{}).Where("id = ?", taskID).Updates(updates).Error
		}
		return nil
	})
	if err != nil {
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

// --- Attachment Service Methods ---

func (s *Service) PresignAttachmentUpload(orgID, actorID, taskID uuid.UUID, req PresignUploadRequest) (*PresignUploadResponse, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}

	attID := uuid.New()
	fileName := filepath.Base(req.FileName)
	objectKey := fmt.Sprintf("%s/%s/%s_%s", orgID, taskID, attID, fileName)

	att := &model.TaskAttachment{
		BaseModel:   model.BaseModel{ID: attID},
		TaskID:      taskID,
		UploadedBy:  actorID,
		FileName:    fileName,
		ObjectKey:   objectKey,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		Status:      "pending",
	}

	if err := s.repo.CreateAttachment(nil, att); err != nil {
		return nil, err
	}

	driver := s.storage.Driver()
	uploadURL := ""
	expiresInSec := 900 // 15 mins

	if driver == "minio" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		u, err := s.storage.PresignUpload(ctx, objectKey, req.ContentType, time.Duration(expiresInSec)*time.Second)
		if err != nil {
			return nil, err
		}
		uploadURL = u
	} else {
		// Local driver: return backend upload endpoint
		uploadURL = fmt.Sprintf("/api/v1/tasks/%s/attachments/upload", taskID)
	}

	return &PresignUploadResponse{
		AttachmentID:  attID,
		UploadURL:     uploadURL,
		ObjectKey:     objectKey,
		ExpiresInSec:  expiresInSec,
		StorageDriver: driver,
	}, nil
}

func (s *Service) ConfirmAttachmentUpload(orgID, actorID, taskID, attID uuid.UUID, checksum string) error {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return err
	}
	return s.repo.ConfirmAttachment(taskID, attID, checksum)
}

func (s *Service) DirectUploadAttachment(orgID, actorID, taskID uuid.UUID, fileName, contentType string, sizeBytes int64, fileReader io.Reader) (*model.TaskAttachment, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}

	attID := uuid.New()
	cleanName := filepath.Base(fileName)
	objectKey := fmt.Sprintf("%s/%s/%s_%s", orgID, taskID, attID, cleanName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.storage.UploadStream(ctx, objectKey, fileReader, sizeBytes, contentType); err != nil {
		return nil, err
	}

	att := &model.TaskAttachment{
		BaseModel:   model.BaseModel{ID: attID},
		TaskID:      taskID,
		UploadedBy:  actorID,
		FileName:    cleanName,
		ObjectKey:   objectKey,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		Status:      "active",
	}

	if err := s.repo.CreateAttachment(nil, att); err != nil {
		_ = s.storage.Delete(context.Background(), objectKey)
		return nil, err
	}

	return att, nil
}

func (s *Service) GetTaskAttachments(orgID, taskID uuid.UUID) ([]model.TaskAttachment, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetAttachments(taskID)
}

func (s *Service) DownloadAttachment(orgID, taskID, attID uuid.UUID) (*model.TaskAttachment, io.ReadCloser, string, error) {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return nil, nil, "", err
	}

	att, err := s.repo.GetAttachmentByID(taskID, attID)
	if err != nil {
		return nil, nil, "", err
	}

	if s.storage.Driver() == "minio" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		presignedURL, err := s.storage.PresignDownload(ctx, att.ObjectKey, 15*time.Minute)
		if err != nil {
			return nil, nil, "", err
		}
		return att, nil, presignedURL, nil
	}

	// Local driver: open stream
	rc, err := s.storage.Open(context.Background(), att.ObjectKey)
	if err != nil {
		return nil, nil, "", err
	}
	return att, rc, "", nil
}

func (s *Service) DeleteAttachment(orgID, actorID, taskID, attID uuid.UUID) error {
	if _, err := s.repo.GetByID(orgID, taskID); err != nil {
		return err
	}

	att, err := s.repo.GetAttachmentByID(taskID, attID)
	if err != nil {
		return err
	}

	_ = s.storage.Delete(context.Background(), att.ObjectKey)
	return s.repo.DeleteAttachment(taskID, attID)
}
