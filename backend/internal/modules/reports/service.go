package reports

import (
	"bytes"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetDashboardSummary(orgID, userID uuid.UUID, roles []string) (*DashboardSummaryResponse, error) {
	isAllScope := false
	for _, r := range roles {
		if r == "system_admin" || r == "commander" {
			isAllScope = true
			break
		}
	}

	var deptIDs []uuid.UUID
	if !isAllScope {
		var err error
		deptIDs, err = s.repo.GetUserDepartmentIDs(orgID, userID)
		if err != nil {
			return nil, err
		}
	}

	return s.repo.GetDashboardSummary(orgID, userID, isAllScope, deptIDs)
}

func (s *Service) GetMyTasks(orgID, userID uuid.UUID, limit int) ([]MyTaskItem, error) {
	deptIDs, err := s.repo.GetUserDepartmentIDs(orgID, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetMyTasks(orgID, userID, deptIDs, limit)
}

func (s *Service) GetTaskReport(orgID uuid.UUID, filter TaskReportFilter) (*TaskReportResponse, error) {
	return s.repo.GetTaskReport(orgID, filter)
}

func (s *Service) GetDepartmentPerformance(orgID uuid.UUID) ([]DepartmentPerformanceItem, error) {
	return s.repo.GetDepartmentPerformance(orgID)
}

func formatPriorityVN(p int16) string {
	switch p {
	case 1:
		return "Khẩn cấp"
	case 2:
		return "Bình thường"
	case 3:
		return "Thấp"
	default:
		return "Bình thường"
	}
}

func formatStatusVN(st string) string {
	switch st {
	case "draft":
		return "Bản nháp"
	case "unassigned":
		return "Chờ nhận việc"
	case "assigned":
		return "Đã giao việc"
	case "in_progress":
		return "Đang thực hiện"
	case "blocked":
		return "Vướng mắc"
	case "submitted":
		return "Chờ nghiệm thu"
	case "revision_required":
		return "Yêu cầu sửa đổi"
	case "completed":
		return "Đã hoàn thành"
	case "cancelled":
		return "Đã hủy"
	default:
		return st
	}
}

func (s *Service) ExportTasksExcel(orgID uuid.UUID, filter TaskReportFilter) ([]byte, error) {
	tasks, err := s.repo.GetTasksForExport(orgID, filter)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Báo cáo công việc"
	f.SetSheetName("Sheet1", sheet)

	// Styles
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F5597"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "top", Style: 1, Color: "D9D9D9"},
			{Type: "bottom", Style: 1, Color: "D9D9D9"},
			{Type: "left", Style: 1, Color: "D9D9D9"},
			{Type: "right", Style: 1, Color: "D9D9D9"},
		},
	})

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1F497D", Size: 15},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	centerStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Style: 1, Color: "E0E0E0"},
			{Type: "bottom", Style: 1, Color: "E0E0E0"},
			{Type: "left", Style: 1, Color: "E0E0E0"},
			{Type: "right", Style: 1, Color: "E0E0E0"},
		},
	})

	dataStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Style: 1, Color: "E0E0E0"},
			{Type: "bottom", Style: 1, Color: "E0E0E0"},
			{Type: "left", Style: 1, Color: "E0E0E0"},
			{Type: "right", Style: 1, Color: "E0E0E0"},
		},
	})

	// Header Title Block
	_ = f.MergeCell(sheet, "A1", "I1")
	_ = f.SetCellValue(sheet, "A1", "BÁO CÁO THỐNG KÊ DANH SÁCH NHIỆM VỤ & CÔNG VIỆC")
	_ = f.SetCellStyle(sheet, "A1", "I1", titleStyle)
	_ = f.SetRowHeight(sheet, 1, 35)

	_ = f.SetCellValue(sheet, "A2", fmt.Sprintf("Thời gian xuất: %s | Tổng số: %d nhiệm vụ",
		time.Now().Format("02/01/2006 15:04"), len(tasks)))
	_ = f.SetRowHeight(sheet, 2, 20)

	// Column Headers
	headers := []string{"STT", "Mã công việc", "Tiêu đề nhiệm vụ", "Ưu tiên", "Tiến độ", "Trạng thái", "Bắt đầu", "Hạn chót", "Đơn vị/Người phụ trách"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 4)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	_ = f.SetRowHeight(sheet, 4, 28)

	// Data rows
	for i, t := range tasks {
		row := i + 5
		_ = f.SetRowHeight(sheet, row, 22)

		assigneeText := "Chưa gán"
		for _, a := range t.Assignments {
			if a.IsPrimary {
				if a.AssigneeUser != nil {
					assigneeText = a.AssigneeUser.FullName
				} else if a.AssigneeDepartment != nil {
					assigneeText = a.AssigneeDepartment.Name
				}
				break
			}
		}

		startStr := "-"
		if t.StartAt != nil {
			startStr = t.StartAt.Format("02/01/2006")
		}
		dueStr := "-"
		if t.DueAt != nil {
			dueStr = t.DueAt.Format("02/01/2006")
		}

		c1, _ := excelize.CoordinatesToCellName(1, row)
		_ = f.SetCellValue(sheet, c1, i+1)
		_ = f.SetCellStyle(sheet, c1, c1, centerStyle)

		c2, _ := excelize.CoordinatesToCellName(2, row)
		_ = f.SetCellValue(sheet, c2, t.Code)
		_ = f.SetCellStyle(sheet, c2, c2, centerStyle)

		c3, _ := excelize.CoordinatesToCellName(3, row)
		_ = f.SetCellValue(sheet, c3, t.Title)
		_ = f.SetCellStyle(sheet, c3, c3, dataStyle)

		c4, _ := excelize.CoordinatesToCellName(4, row)
		_ = f.SetCellValue(sheet, c4, formatPriorityVN(t.Priority))
		_ = f.SetCellStyle(sheet, c4, c4, centerStyle)

		c5, _ := excelize.CoordinatesToCellName(5, row)
		_ = f.SetCellValue(sheet, c5, fmt.Sprintf("%d%%", t.Progress))
		_ = f.SetCellStyle(sheet, c5, c5, centerStyle)

		c6, _ := excelize.CoordinatesToCellName(6, row)
		_ = f.SetCellValue(sheet, c6, formatStatusVN(t.Status))
		_ = f.SetCellStyle(sheet, c6, c6, centerStyle)

		c7, _ := excelize.CoordinatesToCellName(7, row)
		_ = f.SetCellValue(sheet, c7, startStr)
		_ = f.SetCellStyle(sheet, c7, c7, centerStyle)

		c8, _ := excelize.CoordinatesToCellName(8, row)
		_ = f.SetCellValue(sheet, c8, dueStr)
		_ = f.SetCellStyle(sheet, c8, c8, centerStyle)

		c9, _ := excelize.CoordinatesToCellName(9, row)
		_ = f.SetCellValue(sheet, c9, assigneeText)
		_ = f.SetCellStyle(sheet, c9, c9, dataStyle)
	}

	// Set column widths
	_ = f.SetColWidth(sheet, "A", "A", 8)
	_ = f.SetColWidth(sheet, "B", "B", 18)
	_ = f.SetColWidth(sheet, "C", "C", 40)
	_ = f.SetColWidth(sheet, "D", "D", 14)
	_ = f.SetColWidth(sheet, "E", "E", 12)
	_ = f.SetColWidth(sheet, "F", "F", 18)
	_ = f.SetColWidth(sheet, "G", "H", 15)
	_ = f.SetColWidth(sheet, "I", "I", 25)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Service) ExportDepartmentsExcel(orgID uuid.UUID) ([]byte, error) {
	perf, err := s.repo.GetDepartmentPerformance(orgID)
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Hiệu suất phòng ban"
	f.SetSheetName("Sheet1", sheet)

	// Styles
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "top", Style: 1, Color: "D9D9D9"},
			{Type: "bottom", Style: 1, Color: "D9D9D9"},
			{Type: "left", Style: 1, Color: "D9D9D9"},
			{Type: "right", Style: 1, Color: "D9D9D9"},
		},
	})

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1F497D", Size: 15},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	centerStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Style: 1, Color: "E0E0E0"},
			{Type: "bottom", Style: 1, Color: "E0E0E0"},
			{Type: "left", Style: 1, Color: "E0E0E0"},
			{Type: "right", Style: 1, Color: "E0E0E0"},
		},
	})

	dataStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Style: 1, Color: "E0E0E0"},
			{Type: "bottom", Style: 1, Color: "E0E0E0"},
			{Type: "left", Style: 1, Color: "E0E0E0"},
			{Type: "right", Style: 1, Color: "E0E0E0"},
		},
	})

	totalRowStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "top", Style: 1, Color: "000000"},
			{Type: "bottom", Style: 1, Color: "000000"},
			{Type: "left", Style: 1, Color: "D9D9D9"},
			{Type: "right", Style: 1, Color: "D9D9D9"},
		},
	})

	// Header Title Block
	_ = f.MergeCell(sheet, "A1", "I1")
	_ = f.SetCellValue(sheet, "A1", "BÁO CÁO ĐÁNH GIÁ HIỆU SUẤT CÔNG VIỆC THEO PHÒNG BAN")
	_ = f.SetCellStyle(sheet, "A1", "I1", titleStyle)
	_ = f.SetRowHeight(sheet, 1, 35)

	_ = f.SetCellValue(sheet, "A2", fmt.Sprintf("Thời gian xuất: %s", time.Now().Format("02/01/2006 15:04")))
	_ = f.SetRowHeight(sheet, 2, 20)

	// Column Headers
	headers := []string{"STT", "Mã đơn vị", "Tên đơn vị/phòng ban", "Tổng nhiệm vụ", "Đang làm", "Hoàn thành", "Quá hạn", "Tiến độ TB", "Tỷ lệ hoàn thành"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 4)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	_ = f.SetRowHeight(sheet, 4, 28)

	var totalTasks, totalInProgress, totalCompleted, totalOverdue int64

	for i, item := range perf {
		row := i + 5
		_ = f.SetRowHeight(sheet, row, 22)

		totalTasks += item.TotalTasks
		totalInProgress += item.InProgressTasks
		totalCompleted += item.CompletedTasks
		totalOverdue += item.OverdueTasks

		c1, _ := excelize.CoordinatesToCellName(1, row)
		_ = f.SetCellValue(sheet, c1, i+1)
		_ = f.SetCellStyle(sheet, c1, c1, centerStyle)

		c2, _ := excelize.CoordinatesToCellName(2, row)
		_ = f.SetCellValue(sheet, c2, item.DepartmentCode)
		_ = f.SetCellStyle(sheet, c2, c2, centerStyle)

		c3, _ := excelize.CoordinatesToCellName(3, row)
		_ = f.SetCellValue(sheet, c3, item.DepartmentName)
		_ = f.SetCellStyle(sheet, c3, c3, dataStyle)

		c4, _ := excelize.CoordinatesToCellName(4, row)
		_ = f.SetCellValue(sheet, c4, item.TotalTasks)
		_ = f.SetCellStyle(sheet, c4, c4, centerStyle)

		c5, _ := excelize.CoordinatesToCellName(5, row)
		_ = f.SetCellValue(sheet, c5, item.InProgressTasks)
		_ = f.SetCellStyle(sheet, c5, c5, centerStyle)

		c6, _ := excelize.CoordinatesToCellName(6, row)
		_ = f.SetCellValue(sheet, c6, item.CompletedTasks)
		_ = f.SetCellStyle(sheet, c6, c6, centerStyle)

		c7, _ := excelize.CoordinatesToCellName(7, row)
		_ = f.SetCellValue(sheet, c7, item.OverdueTasks)
		_ = f.SetCellStyle(sheet, c7, c7, centerStyle)

		c8, _ := excelize.CoordinatesToCellName(8, row)
		_ = f.SetCellValue(sheet, c8, fmt.Sprintf("%.1f%%", item.AvgProgress))
		_ = f.SetCellStyle(sheet, c8, c8, centerStyle)

		c9, _ := excelize.CoordinatesToCellName(9, row)
		_ = f.SetCellValue(sheet, c9, fmt.Sprintf("%.1f%%", item.CompletionRate))
		_ = f.SetCellStyle(sheet, c9, c9, centerStyle)
	}

	// Total Row
	totRow := len(perf) + 5
	_ = f.SetRowHeight(sheet, totRow, 24)

	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", totRow), fmt.Sprintf("C%d", totRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", totRow), "TỔNG CỘNG")
	for c := 1; c <= 3; c++ {
		cell, _ := excelize.CoordinatesToCellName(c, totRow)
		_ = f.SetCellStyle(sheet, cell, cell, totalRowStyle)
	}

	_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", totRow), totalTasks)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("D%d", totRow), fmt.Sprintf("D%d", totRow), totalRowStyle)

	_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", totRow), totalInProgress)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("E%d", totRow), fmt.Sprintf("E%d", totRow), totalRowStyle)

	_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", totRow), totalCompleted)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("F%d", totRow), fmt.Sprintf("F%d", totRow), totalRowStyle)

	_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", totRow), totalOverdue)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("G%d", totRow), fmt.Sprintf("G%d", totRow), totalRowStyle)

	overallRate := 0.0
	if totalTasks > 0 {
		overallRate = float64(totalCompleted) / float64(totalTasks) * 100.0
	}
	_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", totRow), "-")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("H%d", totRow), fmt.Sprintf("H%d", totRow), totalRowStyle)

	_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", totRow), fmt.Sprintf("%.1f%%", overallRate))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("I%d", totRow), fmt.Sprintf("I%d", totRow), totalRowStyle)

	// Set column widths
	_ = f.SetColWidth(sheet, "A", "A", 8)
	_ = f.SetColWidth(sheet, "B", "B", 16)
	_ = f.SetColWidth(sheet, "C", "C", 32)
	_ = f.SetColWidth(sheet, "D", "G", 15)
	_ = f.SetColWidth(sheet, "H", "I", 18)

	buf := new(bytes.Buffer)
	if err := f.Write(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
