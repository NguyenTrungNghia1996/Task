package reports_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/reports"
	"taskmgmt/internal/testutil"
)

func TestReports_DashboardSummary_And_MyTasks(t *testing.T) {
	db := testutil.SetupTestDB(t)
	org, admin := testutil.CreateTestOrgAndUser(t, db, "admin_user", "system_admin")

	// Create a department
	dept := &model.Department{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Code:           "PKH",
		Name:           "Phòng Kế hoạch",
		DepartmentType: "department",
		IsActive:       true,
	}
	require.NoError(t, db.Create(dept).Error)

	now := time.Now().UTC()
	pastDue := now.Add(-24 * time.Hour)
	futureDue := now.Add(24 * time.Hour)

	// Task 1: in_progress, overdue, assigned to admin
	t1 := &model.Task{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Code:           "TASK-001",
		Title:          "Nhiệm vụ 1 khẩn cấp",
		Status:         "in_progress",
		Priority:       1,
		Progress:       50,
		DueAt:          &pastDue,
		CreatedBy:      admin.ID,
	}
	require.NoError(t, db.Create(t1).Error)
	require.NoError(t, db.Create(&model.TaskAssignment{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		TaskID:         t1.ID,
		AssigneeUserID: &admin.ID,
		AssignmentRole: "responsible",
		IsPrimary:      true,
	}).Error)

	// Task 2: completed, assigned to dept
	t2 := &model.Task{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Code:           "TASK-002",
		Title:          "Nhiệm vụ 2 hoàn thành",
		Status:         "completed",
		Priority:       2,
		Progress:       100,
		DueAt:          &futureDue,
		CreatedBy:      admin.ID,
	}
	require.NoError(t, db.Create(t2).Error)
	require.NoError(t, db.Create(&model.TaskAssignment{
		BaseModel:            model.BaseModel{ID: uuid.New()},
		OrganizationID:       org.ID,
		TaskID:               t2.ID,
		AssigneeDepartmentID: &dept.ID,
		AssignmentRole:       "responsible",
		IsPrimary:            true,
	}).Error)

	// Task 3: assigned, upcoming deadline
	t3 := &model.Task{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Code:           "TASK-003",
		Title:          "Nhiệm vụ 3 sắp đến hạn",
		Status:         "assigned",
		Priority:       2,
		Progress:       0,
		DueAt:          &futureDue,
		CreatedBy:      admin.ID,
	}
	require.NoError(t, db.Create(t3).Error)

	repo := reports.NewRepository(db)
	svc := reports.NewService(repo)

	// 1. Dashboard Summary as Admin (All scope)
	summary, err := svc.GetDashboardSummary(org.ID, admin.ID, []string{"system_admin"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), summary.TotalTasks)
	assert.Equal(t, int64(1), summary.InProgressTasks)
	assert.Equal(t, int64(1), summary.CompletedTasks)
	assert.Equal(t, int64(1), summary.AssignedTasks)
	assert.Equal(t, int64(1), summary.OverdueTasks)
	assert.Equal(t, int64(1), summary.UpcomingDeadlineTasks) // Task 3 is in next 48h
	assert.Equal(t, 50.0, summary.AvgProgress)
	assert.Equal(t, 33.3, summary.CompletionRate) // 1 completed out of 3 total = 33.3%

	// 2. My Tasks
	myTasks, err := svc.GetMyTasks(org.ID, admin.ID, 10)
	require.NoError(t, err)
	require.Len(t, myTasks, 1) // only t1 is assigned directly to admin and not completed
	assert.Equal(t, "TASK-001", myTasks[0].Code)
	assert.True(t, myTasks[0].IsOverdue)

	// 3. Task Report
	report, err := svc.GetTaskReport(org.ID, reports.TaskReportFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(3), report.TotalTasks)
	assert.Equal(t, int64(1), report.CompletedTasks)
	assert.Equal(t, int64(1), report.ByStatus["in_progress"])
	assert.Equal(t, int64(1), report.ByStatus["completed"])
	assert.Equal(t, int64(1), report.ByStatus["assigned"])

	// 4. Department Performance
	perf, err := svc.GetDepartmentPerformance(org.ID)
	require.NoError(t, err)
	require.Len(t, perf, 1)
	assert.Equal(t, "PKH", perf[0].DepartmentCode)
	assert.Equal(t, int64(1), perf[0].TotalTasks)
	assert.Equal(t, int64(1), perf[0].CompletedTasks)
	assert.Equal(t, 100.0, perf[0].CompletionRate)

	// 5. Export Tasks Excel
	excelTasksBytes, err := svc.ExportTasksExcel(org.ID, reports.TaskReportFilter{})
	require.NoError(t, err)
	assert.NotEmpty(t, excelTasksBytes)
	assert.Greater(t, len(excelTasksBytes), 1000) // Valid .xlsx zip binary

	// 6. Export Departments Excel
	excelDeptsBytes, err := svc.ExportDepartmentsExcel(org.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, excelDeptsBytes)
	assert.Greater(t, len(excelDeptsBytes), 1000)
}

