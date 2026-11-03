package tasks_test

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/config"
	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/tasks"
	"taskmgmt/internal/platform/storage"
	"taskmgmt/internal/testutil"
)

func setupTaskService(t *testing.T) (*tasks.Service, *model.Organization, *model.User, *model.User, string) {
	db := testutil.SetupTestDB(t)
	org, manager := testutil.CreateTestOrgAndUser(t, db, "manager", "department_head")
	_, employee := testutil.CreateTestOrgAndUser(t, db, "employee", "employee")

	tempDir, err := os.MkdirTemp("", "taskmgmt-service-test-*")
	require.NoError(t, err)

	storageSvc, err := storage.NewStorage(&config.Config{
		StorageDriver: "local",
		UploadDir:     tempDir,
	})
	require.NoError(t, err)

	repo := tasks.NewRepository(db)
	svc := tasks.NewService(repo, storageSvc)
	return svc, org, manager, employee, tempDir
}

func TestTask_CreateRootTask(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	start := time.Now().UTC()
	due := start.Add(48 * time.Hour)

	req := tasks.CreateTaskRequest{
		Title:       "Dự án Nâng cấp Hạ tầng",
		Description: "Mô tả chi tiết nhiệm vụ",
		Priority:    2,
		StartAt:     &start,
		DueAt:       &due,
		InitialAssignment: &tasks.AssignmentInput{
			Type:              "user",
			ID:                employee.ID,
			ResponsibleUserID: &employee.ID,
		},
	}

	task, err := svc.CreateTask(org.ID, manager.ID, req)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Dự án Nâng cấp Hạ tầng", task.Title)
	assert.Equal(t, "assigned", task.Status)
	assert.True(t, task.IsAssigned)
	assert.Equal(t, int64(1), task.Version)
	assert.Equal(t, task.ID, *task.RootTaskID)
	assert.Nil(t, task.ParentID)
}

func TestTask_CreateChildTask_DeadlineValidation(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	now := time.Now().UTC()
	parentDue := now.Add(24 * time.Hour)

	parent, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ cha",
		Priority: 2,
		DueAt:    &parentDue,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)

	// Case 1: Child deadline EXCEEDS parent deadline (48 hours > 24 hours) -> Must fail
	childExceedDue := now.Add(48 * time.Hour)
	_, err = svc.CreateChildTask(org.ID, manager.ID, parent.ID, tasks.CreateChildTaskRequest{
		Title:    "Việc con quá hạn cha",
		Priority: 2,
		Weight:   1,
		DueAt:    &childExceedDue,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot exceed")

	// Case 2: Child deadline within parent deadline (12 hours <= 24 hours) -> Must succeed
	childValidDue := now.Add(12 * time.Hour)
	child, err := svc.CreateChildTask(org.ID, manager.ID, parent.ID, tasks.CreateChildTaskRequest{
		Title:    "Việc con hợp lệ",
		Priority: 2,
		Weight:   1,
		DueAt:    &childValidDue,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, parent.ID, *child.ParentID)
	assert.Equal(t, *parent.RootTaskID, *child.RootTaskID)
}

func TestTask_OptimisticLocking(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Kiểm tra Lock",
		Priority: 2,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), task.Version)

	// Concurrent update 1 with version 1 -> succeeds, version becomes 2
	newTitle1 := "Tiêu đề cập nhật lần 1"
	updatedTask1, err := svc.UpdateTask(org.ID, manager.ID, task.ID, tasks.UpdateTaskRequest{
		Version: 1,
		Title:   &newTitle1,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), updatedTask1.Version)
	assert.Equal(t, "Tiêu đề cập nhật lần 1", updatedTask1.Title)

	// Concurrent update 2 still sending outdated version 1 -> must fail with conflict!
	newTitle2 := "Tiêu đề xung đột"
	_, err = svc.UpdateTask(org.ID, manager.ID, task.ID, tasks.UpdateTaskRequest{
		Version: 1, // OUTDATED VERSION
		Title:   &newTitle2,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version conflict")
}

func TestTask_SubmitProgress(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ tiến độ",
		Priority: 2,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int16(0), task.Progress)

	// Employee reports 50% progress
	update, err := svc.SubmitProgress(org.ID, employee.ID, task.ID, tasks.CreateTaskUpdateRequest{
		Progress: 50,
		Content:  "Đã hoàn thành giai đoạn 1 của công việc",
	})
	require.NoError(t, err)
	assert.Equal(t, int16(50), update.Progress)

	// Fetch task and verify progress and status updated to in_progress
	detail, err := svc.GetTaskByID(org.ID, task.ID)
	require.NoError(t, err)
	assert.Equal(t, int16(50), detail.Progress)
	assert.Equal(t, "in_progress", detail.Status)
}

func TestTask_Draft_Publish_And_Delete(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	// 1. Create task as draft
	draftTask, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Bản nháp kế hoạch Q4",
		Priority: 2,
		IsDraft:  true,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "draft", draftTask.Status)

	// 2. Publish draft -> becomes assigned
	published, err := svc.PublishTask(org.ID, manager.ID, draftTask.ID)
	require.NoError(t, err)
	assert.Equal(t, "assigned", published.Status)
	assert.True(t, published.IsAssigned)

	// 3. Trying to delete a published/dispatched task must be forbidden
	err = svc.DeleteTask(org.ID, manager.ID, published.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only drafts may be deleted")

	// 4. Create another draft and delete it directly -> succeeds
	draftTask2, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Bản nháp 2",
		Priority: 1,
		IsDraft:  true,
	})
	require.NoError(t, err)
	assert.Equal(t, "draft", draftTask2.Status)

	err = svc.DeleteTask(org.ID, manager.ID, draftTask2.ID)
	require.NoError(t, err)

	_, err = svc.GetTaskByID(org.ID, draftTask2.ID)
	assert.Error(t, err)
}

func TestTask_Claim_Unassigned(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	// Create task without initial assignment
	unassignedTask, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:             "Nhiệm vụ chưa giao ai",
		Priority:          2,
		InitialAssignment: nil,
	})
	require.NoError(t, err)
	assert.Equal(t, "unassigned", unassignedTask.Status)
	assert.False(t, unassignedTask.IsAssigned)

	// Employee self-claims the task
	claimed, err := svc.ClaimTask(org.ID, employee.ID, unassignedTask.ID)
	require.NoError(t, err)
	assert.Equal(t, "assigned", claimed.Status)
	assert.True(t, claimed.IsAssigned)

	// Attempting to claim again must fail
	_, err = svc.ClaimTask(org.ID, manager.ID, unassignedTask.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already assigned")
}

func TestTask_Block_And_Unblock(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ lắp đặt",
		Priority: 2,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)

	// Block task
	blocked, err := svc.BlockTask(org.ID, employee.ID, task.ID, "Thiếu linh kiện phần cứng máy chủ")
	require.NoError(t, err)
	assert.Equal(t, "blocked", blocked.Status)
	assert.Equal(t, "Thiếu linh kiện phần cứng máy chủ", blocked.BlockedReason)

	// Unblock task
	unblocked, err := svc.UnblockTask(org.ID, manager.ID, task.ID, "Đã nhập đủ linh kiện bổ sung")
	require.NoError(t, err)
	assert.Equal(t, "in_progress", unblocked.Status)
	assert.Empty(t, unblocked.BlockedReason)
}

func TestTask_Submit_And_AutoSubmit(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ kiểm thử",
		Priority: 2,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)

	// Employee updates progress to 100% -> Auto moves to submitted because ApprovalRequired=true
	_, err = svc.SubmitProgress(org.ID, employee.ID, task.ID, tasks.CreateTaskUpdateRequest{
		Progress: 100,
		Content:  "Đã hoàn thành 100% kiểm thử",
	})
	require.NoError(t, err)

	detail, err := svc.GetTaskByID(org.ID, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "submitted", detail.Status)
	assert.Equal(t, int16(100), detail.Progress)
}

func TestTask_MoveTask_And_CycleCheck(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	now := time.Now().UTC()
	parentDue := now.Add(48 * time.Hour)
	childDue := now.Add(24 * time.Hour)

	// Parent task
	parent, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Công việc cha",
		Priority: 2,
		DueAt:    &parentDue,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   manager.ID,
		},
	})
	require.NoError(t, err)

	// Child task
	child, err := svc.CreateChildTask(org.ID, manager.ID, parent.ID, tasks.CreateChildTaskRequest{
		Title:    "Công việc con",
		Priority: 2,
		Weight:   1,
		DueAt:    &childDue,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)

	// Move task: reorder and change status
	newStatus := "in_progress"
	newSortOrder := 5
	moved, err := svc.MoveTask(org.ID, manager.ID, child.ID, tasks.MoveTaskRequest{
		NewStatus: &newStatus,
		SortOrder: &newSortOrder,
		Version:   child.Version,
	})
	require.NoError(t, err)
	assert.Equal(t, "in_progress", moved.Status)
	assert.Equal(t, 5, moved.SortOrder)

	// Test cycle detection: Cannot move parent to become child of its own child!
	_, err = svc.MoveTask(org.ID, manager.ID, parent.ID, tasks.MoveTaskRequest{
		ParentID: &child.ID,
		Version:  parent.Version,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cyclic")

	// Verify recursive tree loading
	tree, err := svc.GetTaskTree(org.ID, parent.ID)
	require.NoError(t, err)
	assert.Equal(t, parent.ID, tree.ID)
	assert.Len(t, tree.Children, 1)
	assert.Equal(t, child.ID, tree.Children[0].ID)
}

func TestTask_Attachments_Local(t *testing.T) {
	svc, org, manager, employee, tempDir := setupTaskService(t)
	defer os.RemoveAll(tempDir)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ có tệp",
		Priority: 2,
		InitialAssignment: &tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)

	// 1. Direct upload attachment
	fileContent := []byte("Nội dung tài liệu đính kèm kiểm thử")
	att, err := svc.DirectUploadAttachment(org.ID, employee.ID, task.ID, "report.pdf", "application/pdf", int64(len(fileContent)), bytes.NewReader(fileContent))
	require.NoError(t, err)
	assert.Equal(t, "report.pdf", att.FileName)
	assert.Equal(t, "active", att.Status)

	// 2. List attachments
	list, err := svc.GetTaskAttachments(org.ID, task.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, att.ID, list[0].ID)

	// 3. Download attachment
	downloadedAtt, rc, _, err := svc.DownloadAttachment(org.ID, task.ID, att.ID)
	require.NoError(t, err)
	assert.Equal(t, "report.pdf", downloadedAtt.FileName)
	readBytes, err := io.ReadAll(rc)
	rc.Close()
	require.NoError(t, err)
	assert.Equal(t, fileContent, readBytes)

	// 4. Submit progress with attachment ID
	update, err := svc.SubmitProgress(org.ID, employee.ID, task.ID, tasks.CreateTaskUpdateRequest{
		Progress:      30,
		Content:       "Đã tải lên tài liệu báo cáo",
		AttachmentIDs: []uuid.UUID{att.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, int16(30), update.Progress)

	// Verify attachment is linked to update
	listAfter, err := svc.GetTaskAttachments(org.ID, task.ID)
	require.NoError(t, err)
	assert.Equal(t, &update.ID, listAfter[0].UpdateID)

	// 5. Delete attachment
	err = svc.DeleteAttachment(org.ID, employee.ID, task.ID, att.ID)
	require.NoError(t, err)

	listEmpty, err := svc.GetTaskAttachments(org.ID, task.ID)
	require.NoError(t, err)
	assert.Len(t, listEmpty, 0)
}
