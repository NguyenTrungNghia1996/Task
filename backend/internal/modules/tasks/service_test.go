package tasks_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/tasks"
	"taskmgmt/internal/testutil"
)

func setupTaskService(t *testing.T) (*tasks.Service, *model.Organization, *model.User, *model.User) {
	db := testutil.SetupTestDB(t)
	org, manager := testutil.CreateTestOrgAndUser(t, db, "manager", "department_head")
	_, employee := testutil.CreateTestOrgAndUser(t, db, "employee", "employee")

	repo := tasks.NewRepository(db)
	svc := tasks.NewService(repo)
	return svc, org, manager, employee
}

func TestTask_CreateRootTask(t *testing.T) {
	svc, org, manager, employee := setupTaskService(t)

	start := time.Now().UTC()
	due := start.Add(48 * time.Hour)

	req := tasks.CreateTaskRequest{
		Title:       "Dự án Nâng cấp Hạ tầng",
		Description: "Mô tả chi tiết nhiệm vụ",
		Priority:    2,
		StartAt:     &start,
		DueAt:       &due,
		InitialAssignment: tasks.AssignmentInput{
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
	assert.Equal(t, int64(1), task.Version)
	assert.Equal(t, task.ID, *task.RootTaskID)
	assert.Nil(t, task.ParentID)
}

func TestTask_CreateChildTask_DeadlineValidation(t *testing.T) {
	svc, org, manager, employee := setupTaskService(t)

	now := time.Now().UTC()
	parentDue := now.Add(24 * time.Hour)

	parent, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ cha",
		Priority: 2,
		DueAt:    &parentDue,
		InitialAssignment: tasks.AssignmentInput{
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
		InitialAssignment: tasks.AssignmentInput{
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
		InitialAssignment: tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, parent.ID, *child.ParentID)
	assert.Equal(t, *parent.RootTaskID, *child.RootTaskID)
}

func TestTask_OptimisticLocking(t *testing.T) {
	svc, org, manager, employee := setupTaskService(t)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Kiểm tra Lock",
		Priority: 2,
		InitialAssignment: tasks.AssignmentInput{
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
	svc, org, manager, employee := setupTaskService(t)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ tiến độ",
		Priority: 2,
		InitialAssignment: tasks.AssignmentInput{
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

func TestTask_CancelTask(t *testing.T) {
	svc, org, manager, employee := setupTaskService(t)

	task, err := svc.CreateTask(org.ID, manager.ID, tasks.CreateTaskRequest{
		Title:    "Nhiệm vụ sẽ hủy",
		Priority: 1,
		InitialAssignment: tasks.AssignmentInput{
			Type: "user",
			ID:   employee.ID,
		},
	})
	require.NoError(t, err)

	cancelled, err := svc.CancelTask(org.ID, manager.ID, task.ID, "Kế hoạch thay đổi theo chỉ thị mới")
	require.NoError(t, err)
	assert.Equal(t, "cancelled", cancelled.Status)
}
