package reports

import (
	"time"

	"github.com/google/uuid"
)

type DashboardSummaryResponse struct {
	TotalTasks            int64   `json:"total_tasks"`
	InProgressTasks       int64   `json:"in_progress_tasks"`
	AssignedTasks         int64   `json:"assigned_tasks"`
	CompletedTasks        int64   `json:"completed_tasks"`
	BlockedTasks          int64   `json:"blocked_tasks"`
	RevisionRequiredTasks int64   `json:"revision_required_tasks"`
	CancelledTasks        int64   `json:"cancelled_tasks"`
	OverdueTasks          int64   `json:"overdue_tasks"`
	UpcomingDeadlineTasks int64   `json:"upcoming_deadline_tasks"`
	AvgProgress           float64 `json:"avg_progress"`
	CompletionRate        float64 `json:"completion_rate"`
}

type MyTaskItem struct {
	ID          uuid.UUID  `json:"id"`
	Code        string     `json:"code"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Priority    int16      `json:"priority"`
	Progress    int16      `json:"progress"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	IsOverdue   bool       `json:"is_overdue"`
	ParentTitle string     `json:"parent_title,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type TaskReportFilter struct {
	FromDate     *time.Time `query:"from_date"`
	ToDate       *time.Time `query:"to_date"`
	DepartmentID *uuid.UUID `query:"department_id"`
}

type TaskReportResponse struct {
	TotalTasks     int64            `json:"total_tasks"`
	CompletedTasks int64            `json:"completed_tasks"`
	OverdueTasks   int64            `json:"overdue_tasks"`
	AvgProgress    float64          `json:"avg_progress"`
	CompletionRate float64          `json:"completion_rate"`
	ByStatus       map[string]int64 `json:"by_status"`
	ByPriority     map[int16]int64  `json:"by_priority"`
}

type DepartmentPerformanceItem struct {
	DepartmentID    uuid.UUID `json:"department_id"`
	DepartmentCode  string    `json:"department_code"`
	DepartmentName  string    `json:"department_name"`
	TotalTasks      int64     `json:"total_tasks"`
	CompletedTasks  int64     `json:"completed_tasks"`
	InProgressTasks int64     `json:"in_progress_tasks"`
	OverdueTasks    int64     `json:"overdue_tasks"`
	AvgProgress     float64   `json:"avg_progress"`
	CompletionRate  float64   `json:"completion_rate"`
}

