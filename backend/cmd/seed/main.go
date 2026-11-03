package main

import (
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"taskmgmt/internal/config"
	"taskmgmt/internal/database"
	"taskmgmt/internal/model"
	"taskmgmt/internal/platform/auth"
)

func main() {
	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("AutoMigrate error: %v", err)
	}

	log.Println("🌱 Seeding initial data...")

	// 1. Organization
	org := model.Organization{
		Code:     "ORG_ALPHA",
		Name:     "Bộ Chỉ Huy / Đơn Vị Mẫu",
		IsActive: true,
	}
	db.Where("code = ?", org.Code).FirstOrCreate(&org)

	// 2. Roles
	roleDefs := []struct {
		Code string
		Name string
	}{
		{"system_admin", "Quản trị hệ thống"},
		{"commander", "Chỉ huy đơn vị"},
		{"department_head", "Trưởng phòng/ban"},
		{"manager", "Quản lý nhóm/tổ"},
		{"employee", "Nhân viên phụ trách"},
		{"team_member", "Tổ viên thực hiện"},
	}

	roleMap := make(map[string]uuid.UUID)
	for _, r := range roleDefs {
		role := model.Role{
			Code: r.Code,
			Name: r.Name,
		}
		db.Where("code = ?", r.Code).FirstOrCreate(&role)
		roleMap[r.Code] = role.ID
	}

	// 3. Hierarchical Departments (Khối -> Phòng -> Ban / Tổ)
	// 3.1 Khối Tác Chiến (Division)
	divTC := model.Department{
		OrganizationID: org.ID,
		Code:           "KTC",
		Name:           "Khối Tác Chiến",
		DepartmentType: "division",
		IsActive:       true,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, divTC.Code).FirstOrCreate(&divTC)

	// 3.2 Phòng Tham Mưu (Department under KTC)
	deptTM := model.Department{
		OrganizationID: org.ID,
		ParentID:       &divTC.ID,
		Code:           "PTM",
		Name:           "Phòng Tham Mưu Tác Chiến",
		DepartmentType: "department",
		IsActive:       true,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, deptTM.Code).FirstOrCreate(&deptTM)

	// 3.3 Ban Thông Tin (Section under PTM)
	secTT := model.Department{
		OrganizationID: org.ID,
		ParentID:       &deptTM.ID,
		Code:           "BTT",
		Name:           "Ban Thông Tin Liên Lạc",
		DepartmentType: "section",
		IsActive:       true,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, secTT.Code).FirstOrCreate(&secTT)

	// 3.4 Tổ Kỹ Thuật (Team under PTM)
	teamKT := model.Department{
		OrganizationID: org.ID,
		ParentID:       &deptTM.ID,
		Code:           "TKT",
		Name:           "Tổ Kỹ Thuật Cơ Động",
		DepartmentType: "team",
		IsActive:       true,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, teamKT.Code).FirstOrCreate(&teamKT)

	// 3.5 Phòng Kế Hoạch & Hậu Cần (Department under KTC)
	deptKH := model.Department{
		OrganizationID: org.ID,
		ParentID:       &divTC.ID,
		Code:           "PKH",
		Name:           "Phòng Kế Hoạch & Hậu Cần",
		DepartmentType: "department",
		IsActive:       true,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, deptKH.Code).FirstOrCreate(&deptKH)

	// 4. Users
	hashedPass, _ := auth.HashPassword("password123")

	usersDef := []struct {
		Username      string
		FullName      string
		Email         string
		RoleCode      string
		DeptID        *uuid.UUID
		PositionTitle string
	}{
		{"admin", "Quản Trị Viên Hệ Thống", "admin@taskmgmt.vn", "system_admin", nil, "Quản trị viên"},
		{"commander", "Nguyễn Văn Chỉ Huy", "commander@taskmgmt.vn", "commander", &divTC.ID, "Chỉ huy trưởng"},
		{"truongphong", "Trần Văn Trưởng Phòng", "truongphong@taskmgmt.vn", "department_head", &deptTM.ID, "Trưởng phòng Tham Mưu"},
		{"phophong", "Phạm Thị Phó Phòng", "phophong@taskmgmt.vn", "department_head", &deptKH.ID, "Phó trưởng phòng Kế Hoạch"},
		{"nhanvien", "Lê Văn Nhân Viên", "nhanvien@taskmgmt.vn", "employee", &secTT.ID, "Chuyên viên Ban Thông Tin"},
		{"tovien", "Hoàng Văn Tổ Viên", "tovien@taskmgmt.vn", "team_member", &teamKT.ID, "Kỹ thuật viên"},
	}

	userMap := make(map[string]model.User)
	for _, u := range usersDef {
		user := model.User{
			OrganizationID: org.ID,
			Username:       u.Username,
			FullName:       u.FullName,
			Email:          u.Email,
			PasswordHash:   hashedPass,
			IsActive:       true,
		}
		db.Where("organization_id = ? AND username = ?", org.ID, u.Username).FirstOrCreate(&user)
		userMap[u.Username] = user

		// User Role
		ur := model.UserRole{
			OrganizationID: org.ID,
			UserID:         user.ID,
			RoleID:         roleMap[u.RoleCode],
		}
		db.Where("user_id = ? AND role_id = ?", user.ID, ur.RoleID).FirstOrCreate(&ur)

		// Department Member
		if u.DeptID != nil {
			var managerID *uuid.UUID
			if u.Username == "nhanvien" {
				tp := userMap["truongphong"].ID
				managerID = &tp
			} else if u.Username == "tovien" {
				nv := userMap["nhanvien"].ID
				managerID = &nv
			}

			dm := model.DepartmentMember{
				OrganizationID: org.ID,
				DepartmentID:   *u.DeptID,
				UserID:         user.ID,
				ManagerUserID:  managerID,
				PositionTitle:  u.PositionTitle,
				IsPrimary:      true,
				EffectiveFrom:  time.Now().UTC().Add(-30 * 24 * time.Hour),
			}
			db.Where("user_id = ? AND department_id = ?", user.ID, *u.DeptID).FirstOrCreate(&dm)
		}
	}

	// 5. Seed Permissions
	permDefs := []struct {
		Code   string
		Name   string
		Module string
		Desc   string
	}{
		// Quản lý công việc
		{"tasks:view", "Xem danh sách công việc", "Quản lý công việc", "Xem danh sách và chi tiết công việc"},
		{"tasks:view_all", "Xem toàn bộ công việc đơn vị", "Quản lý công việc", "Xem công việc xuyên suốt toàn đơn vị"},
		{"tasks:view_dept", "Xem công việc phòng ban", "Quản lý công việc", "Xem công việc thuộc phòng ban phụ trách"},
		{"tasks:create", "Tạo nhiệm vụ mới", "Quản lý công việc", "Khởi tạo công việc hoặc nhiệm vụ mới"},
		{"tasks:edit", "Sửa thông tin công việc", "Quản lý công việc", "Sửa nội dung, gia hạn thời gian công việc"},
		{"tasks:delete", "Hủy hoặc xóa công việc", "Quản lý công việc", "Hủy hoặc xóa mềm công việc theo thẩm quyền"},
		{"tasks:review", "Nghiệm thu công việc", "Quản lý công việc", "Đánh giá nghiệm thu đạt hoặc yêu cầu làm lại"},
		{"tasks:reopen", "Mở lại công việc", "Quản lý công việc", "Mở lại công việc đã nghiệm thu có kèm lý do"},
		{"tasks:update_progress", "Báo cáo tiến độ", "Quản lý công việc", "Gửi báo cáo tiến độ và tài liệu đính kèm"},

		// Cơ cấu tổ chức
		{"org:view_tree", "Xem sơ đồ cây tổ chức", "Cơ cấu tổ chức", "Xem danh bạ và cơ cấu phòng ban"},
		{"org:manage", "Quản lý phòng ban", "Cơ cấu tổ chức", "Thêm, sửa, đóng cơ cấu phòng ban và bổ nhiệm"},

		// Nhân sự
		{"users:view", "Xem danh sách nhân sự", "Quản trị người dùng", "Xem hồ sơ nhân sự cơ quan"},
		{"users:manage", "Quản lý tài khoản nhân sự", "Quản trị người dùng", "Thêm, sửa, đổi mật khẩu và gán vai trò nhân sự"},

		// Báo cáo & Dashboard
		{"reports:view", "Xem báo cáo tiến độ", "Báo cáo & Thống kê", "Xem báo cáo tiến độ và đánh giá công việc"},
		{"reports:export", "Xuất file báo cáo", "Báo cáo & Thống kê", "Xuất báo cáo dạng Excel / PDF"},

		// Quản trị hệ thống
		{"system:settings", "Cấu hình tham số hệ thống", "Quản trị hệ thống", "Quản trị hệ thống toàn diện"},
		{"roles:manage", "Quản lý vai trò & phân quyền", "Quản trị hệ thống", "Gán và thu hồi quyền của từng vai trò"},
		{"menus:manage", "Quản lý Menu điều hướng", "Quản trị hệ thống", "Cấu hình thêm/sửa/xóa các mục menu"},
	}

	permMap := make(map[string]uuid.UUID)
	for _, p := range permDefs {
		perm := model.Permission{
			Code:        p.Code,
			Name:        p.Name,
			Module:      p.Module,
			Description: p.Desc,
		}
		db.Where("code = ?", p.Code).FirstOrCreate(&perm)
		permMap[p.Code] = perm.ID
	}

	// 6. Role - Permission mapping
	rolePermMappings := map[string][]string{
		"system_admin": {
			"tasks:view", "tasks:view_all", "tasks:view_dept", "tasks:create", "tasks:edit", "tasks:delete",
			"tasks:review", "tasks:reopen", "tasks:update_progress", "org:view_tree", "org:manage",
			"users:view", "users:manage", "reports:view", "reports:export", "system:settings",
			"roles:manage", "menus:manage",
		},
		"commander": {
			"tasks:view", "tasks:view_all", "tasks:view_dept", "tasks:create", "tasks:edit", "tasks:delete",
			"tasks:review", "tasks:reopen", "tasks:update_progress", "org:view_tree", "users:view",
			"reports:view", "reports:export",
		},
		"department_head": {
			"tasks:view", "tasks:view_dept", "tasks:create", "tasks:edit", "tasks:delete",
			"tasks:review", "tasks:reopen", "tasks:update_progress", "org:view_tree", "users:view",
			"reports:view",
		},
		"manager": {
			"tasks:view", "tasks:view_dept", "tasks:create", "tasks:edit",
			"tasks:review", "tasks:update_progress", "org:view_tree", "users:view",
		},
		"employee": {
			"tasks:view", "tasks:update_progress", "org:view_tree",
		},
		"team_member": {
			"tasks:view", "tasks:update_progress", "org:view_tree",
		},
	}

	for roleCode, permCodes := range rolePermMappings {
		roleID, ok := roleMap[roleCode]
		if !ok {
			continue
		}
		for _, pCode := range permCodes {
			pID, exists := permMap[pCode]
			if !exists {
				continue
			}
			var existingRP model.RolePermission
			if err := db.Where("role_id = ? AND permission_id = ?", roleID, pID).First(&existingRP).Error; err != nil {
				rp := model.RolePermission{
					RoleID:       roleID,
					PermissionID: pID,
				}
				db.Create(&rp)
			}
		}
	}

	// 7. Seed Dynamic Menus
	type menuDef struct {
		Code               string
		Title              string
		Icon               string
		Path               string
		RequiredPermission string
		SortOrder          int
		Children           []menuDef
	}

	menusTree := []menuDef{
		{
			Code:      "dashboard",
			Title:     "Bảng tổng quan",
			Icon:      "DashboardOutlined",
			Path:      "/dashboard",
			SortOrder: 1,
		},
		{
			Code:               "tasks",
			Title:              "Quản lý công việc",
			Icon:               "ProjectOutlined",
			Path:               "/tasks",
			RequiredPermission: "tasks:view",
			SortOrder:          2,
			Children: []menuDef{
				{
					Code:               "tasks_all",
					Title:              "Tất cả công việc",
					Icon:               "UnorderedListOutlined",
					Path:               "/tasks/all",
					RequiredPermission: "tasks:view_all",
					SortOrder:          1,
				},
				{
					Code:               "tasks_dept",
					Title:              "Việc phòng ban",
					Icon:               "ApartmentOutlined",
					Path:               "/tasks/department",
					RequiredPermission: "tasks:view_dept",
					SortOrder:          2,
				},
				{
					Code:               "tasks_my",
					Title:              "Việc của tôi",
					Icon:               "CheckSquareOutlined",
					Path:               "/tasks/my-tasks",
					RequiredPermission: "tasks:view",
					SortOrder:          3,
				},
			},
		},
		{
			Code:               "org",
			Title:              "Cơ cấu tổ chức",
			Icon:               "BankOutlined",
			Path:               "/organization",
			RequiredPermission: "org:view_tree",
			SortOrder:          3,
			Children: []menuDef{
				{
					Code:               "org_tree",
					Title:              "Sơ đồ đơn vị",
					Icon:               "BranchesOutlined",
					Path:               "/organization/tree",
					RequiredPermission: "org:view_tree",
					SortOrder:          1,
				},
				{
					Code:               "org_depts",
					Title:              "Quản lý phòng ban",
					Icon:               "ClusterOutlined",
					Path:               "/organization/departments",
					RequiredPermission: "org:manage",
					SortOrder:          2,
				},
			},
		},
		{
			Code:               "reports",
			Title:              "Báo cáo & Thống kê",
			Icon:               "BarChartOutlined",
			Path:               "/reports",
			RequiredPermission: "reports:view",
			SortOrder:          4,
		},
		{
			Code:               "admin",
			Title:              "Quản trị hệ thống",
			Icon:               "SettingOutlined",
			Path:               "/admin",
			RequiredPermission: "system:settings",
			SortOrder:          5,
			Children: []menuDef{
				{
					Code:               "admin_users",
					Title:              "Quản lý nhân sự",
					Icon:               "TeamOutlined",
					Path:               "/admin/users",
					RequiredPermission: "users:manage",
					SortOrder:          1,
				},
				{
					Code:               "admin_roles",
					Title:              "Vai trò & Phân quyền",
					Icon:               "SafetyCertificateOutlined",
					Path:               "/admin/roles",
					RequiredPermission: "roles:manage",
					SortOrder:          2,
				},
				{
					Code:               "admin_menus",
					Title:              "Cấu hình Menu",
					Icon:               "MenuOutlined",
					Path:               "/admin/menus",
					RequiredPermission: "menus:manage",
					SortOrder:          3,
				},
			},
		},
	}

	for _, m := range menusTree {
		parent := model.Menu{
			Code:               m.Code,
			Title:              m.Title,
			Icon:               m.Icon,
			Path:               m.Path,
			RequiredPermission: m.RequiredPermission,
			SortOrder:          m.SortOrder,
			IsActive:           true,
			IsHidden:           false,
		}
		db.Where("code = ?", m.Code).FirstOrCreate(&parent)

		for _, ch := range m.Children {
			chMenu := model.Menu{
				ParentID:           &parent.ID,
				Code:               ch.Code,
				Title:              ch.Title,
				Icon:               ch.Icon,
				Path:               ch.Path,
				RequiredPermission: ch.RequiredPermission,
				SortOrder:          ch.SortOrder,
				IsActive:           true,
				IsHidden:           false,
			}
			db.Where("code = ?", ch.Code).FirstOrCreate(&chMenu)
		}
	}

	// 8. Seed Sample Tasks & Multi-Level Hierarchy
	now := time.Now().UTC()
	startDay := now.Add(-5 * 24 * time.Hour)
	dueRoot1 := now.Add(20 * 24 * time.Hour)
	dueChild11 := now.Add(7 * 24 * time.Hour)
	dueChild12 := now.Add(3 * 24 * time.Hour)

	commanderUser := userMap["commander"]
	tpUser := userMap["truongphong"]
	nvUser := userMap["nhanvien"]
	tvUser := userMap["tovien"]
	phophongUser := userMap["phophong"]

	// Root Task 1: Kế hoạch diễn tập tác chiến
	task1 := model.Task{
		OrganizationID:   org.ID,
		Code:             "TASK-2026-00001",
		Title:            "Kế hoạch diễn tập tác chiến diện rộng quý IV",
		Description:      "Chỉ huy giao Phòng Tham Mưu chủ trì phối hợp diễn tập hiệp đồng tác chiến diện rộng.",
		Status:           "in_progress",
		Priority:         1,
		Progress:         87, // (80*2 + 100*1)/3 = 86.6 -> 87%
		StartAt:          &startDay,
		DueAt:            &dueRoot1,
		CreatedBy:        commanderUser.ID,
		ApprovalRequired: true,
		Weight:           1,
		Version:          1,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, task1.Code).FirstOrCreate(&task1)
	if task1.RootTaskID == nil {
		task1.RootTaskID = &task1.ID
		db.Save(&task1)
	}

	// Assign Task 1 to Department PTM with responsible user = truongphong
	assignTask(db, org.ID, task1.ID, nil, &deptTM.ID, &tpUser.ID, commanderUser.ID, "responsible", true)

	// Child Task 1.1: Khảo sát và thiết lập mạng thông tin
	child11 := model.Task{
		OrganizationID:   org.ID,
		ParentID:         &task1.ID,
		RootTaskID:       &task1.ID,
		Code:             "TASK-2026-00002",
		Title:            "Khảo sát và thiết lập mạng thông tin liên lạc",
		Description:      "Triển khai mạng liên lạc dã chiến thông suốt giữa các mũi tác chiến.",
		Status:           "in_progress",
		Priority:         1,
		Progress:         80,
		StartAt:          &startDay,
		DueAt:            &dueChild11,
		CreatedBy:        tpUser.ID,
		ApprovalRequired: true,
		Weight:           2,
		Version:          1,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, child11.Code).FirstOrCreate(&child11)
	assignTask(db, org.ID, child11.ID, &nvUser.ID, nil, nil, tpUser.ID, "responsible", true)

	// Progress update on 1.1
	up11 := model.TaskUpdate{
		TaskID:      child11.ID,
		SubmittedBy: nvUser.ID,
		Progress:    80,
		Content:     "Đã hoàn thành lắp đặt trạm trung gian tiếp sóng, tín hiệu thông suốt 24/7.",
		SubmittedAt: now.Add(-1 * 24 * time.Hour),
	}
	db.Where("task_id = ? AND progress = ?", child11.ID, 80).FirstOrCreate(&up11)

	// Comment on 1.1
	cm11 := model.TaskComment{
		TaskID:   child11.ID,
		AuthorID: tpUser.ID,
		Content:  "Đề nghị kiểm tra thêm phương án dự phòng khi mất điện lưới cục bộ.",
	}
	db.Where("task_id = ? AND author_id = ?", child11.ID, tpUser.ID).FirstOrCreate(&cm11)

	// Child Task 1.2: Kiểm tra khí tài kỹ thuật (Completed)
	child12 := model.Task{
		OrganizationID:   org.ID,
		ParentID:         &task1.ID,
		RootTaskID:       &task1.ID,
		Code:             "TASK-2026-00003",
		Title:            "Kiểm tra bảo dưỡng khí tài kỹ thuật tác chiến",
		Description:      "Bảo dưỡng định kỳ toàn bộ khí tài trang bị cho kíp cơ động.",
		Status:           "completed",
		Priority:         2,
		Progress:         100,
		StartAt:          &startDay,
		DueAt:            &dueChild12,
		CreatedBy:        tpUser.ID,
		ApprovalRequired: true,
		Weight:           1,
		Version:          2,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, child12.Code).FirstOrCreate(&child12)
	assignTask(db, org.ID, child12.ID, &tvUser.ID, nil, nil, tpUser.ID, "responsible", true)

	up12 := model.TaskUpdate{
		TaskID:      child12.ID,
		SubmittedBy: tvUser.ID,
		Progress:    100,
		Content:     "Đã kiểm tra 100% khí tài, các thông số kỹ thuật đều đạt yêu cầu nhiệm vụ.",
		SubmittedAt: now.Add(-2 * 24 * time.Hour),
	}
	db.Where("task_id = ? AND progress = ?", child12.ID, 100).FirstOrCreate(&up12)

	rev12 := model.TaskReview{
		TaskID:     child12.ID,
		UpdateID:   up12.ID,
		ReviewerID: tpUser.ID,
		Decision:   "approved",
		Comment:    "Nghiệm thu đạt chất lượng kỹ thuật, đưa vào biên chế phục vụ diễn tập.",
		ReviewedAt: now.Add(-1 * 24 * time.Hour),
	}
	db.Where("task_id = ? AND reviewer_id = ?", child12.ID, tpUser.ID).FirstOrCreate(&rev12)

	// Root Task 2: Bảo đảm hậu cần thường xuyên
	dueRoot2 := now.Add(30 * 24 * time.Hour)
	task2 := model.Task{
		OrganizationID:   org.ID,
		Code:             "TASK-2026-00004",
		Title:            "Kế hoạch bảo đảm vật tư hậu cần kỹ thuật thường xuyên",
		Description:      "Bảo đảm đầy đủ định mức vật tư, hậu cần cho các đơn vị trực sẵn sàng chiến đấu.",
		Status:           "assigned",
		Priority:         2,
		Progress:         0,
		StartAt:          &now,
		DueAt:            &dueRoot2,
		CreatedBy:        commanderUser.ID,
		ApprovalRequired: true,
		Weight:           1,
		Version:          1,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, task2.Code).FirstOrCreate(&task2)
	if task2.RootTaskID == nil {
		task2.RootTaskID = &task2.ID
		db.Save(&task2)
	}
	assignTask(db, org.ID, task2.ID, nil, &deptKH.ID, &phophongUser.ID, commanderUser.ID, "responsible", true)

	// Root Task 3: Báo cáo sơ kết chuyển đổi số (Quá hạn - Demo Overdue metric)
	pastDue3 := now.Add(-3 * 24 * time.Hour)
	task3 := model.Task{
		OrganizationID:   org.ID,
		Code:             "TASK-2026-00005",
		Title:            "Báo cáo sơ kết chuyển đổi số và an toàn thông tin năm 2026",
		Description:      "Tổng hợp số liệu chuyển đổi số toàn đơn vị gửi cấp trên theo định kỳ.",
		Status:           "in_progress",
		Priority:         1,
		Progress:         40,
		DueAt:            &pastDue3, // Overdue
		CreatedBy:        commanderUser.ID,
		ApprovalRequired: true,
		Weight:           1,
		Version:          1,
	}
	db.Where("organization_id = ? AND code = ?", org.ID, task3.Code).FirstOrCreate(&task3)
	if task3.RootTaskID == nil {
		task3.RootTaskID = &task3.ID
		db.Save(&task3)
	}
	assignTask(db, org.ID, task3.ID, &nvUser.ID, nil, nil, commanderUser.ID, "responsible", true)

	// 9. Seed Sample Notifications
	notifications := []model.Notification{
		{
			OrganizationID: org.ID,
			RecipientID:    tpUser.ID,
			TaskID:         &task1.ID,
			Type:           "task_assigned",
			Title:          "Nhiệm vụ mới được giao",
			Body:           "Chỉ huy trưởng vừa giao nhiệm vụ: 'Kế hoạch diễn tập tác chiến diện rộng quý IV' cho phòng của đồng chí.",
		},
		{
			OrganizationID: org.ID,
			RecipientID:    nvUser.ID,
			TaskID:         &child11.ID,
			Type:           "task_assigned",
			Title:          "Phân công nhiệm vụ",
			Body:           "Trưởng phòng vừa phân công đồng chí phụ trách: 'Khảo sát và thiết lập mạng thông tin liên lạc'.",
		},
		{
			OrganizationID: org.ID,
			RecipientID:    tpUser.ID,
			TaskID:         &child12.ID,
			Type:           "review_submitted",
			Title:          "Báo cáo hoàn thành chờ nghiệm thu",
			Body:           "Đồng chí Hoàng Văn Tổ Viên đã gửi báo cáo hoàn thành nhiệm vụ 'Kiểm tra bảo dưỡng khí tài kỹ thuật tác chiến'.",
			ReadAt:         &now,
		},
	}

	for _, n := range notifications {
		db.Where("organization_id = ? AND recipient_id = ? AND title = ?", n.OrganizationID, n.RecipientID, n.Title).FirstOrCreate(&n)
	}

	// 10. Audit Event Sample
	event := model.TaskEvent{
		ID:             uuid.New(),
		OrganizationID: org.ID,
		TaskID:         task1.ID,
		ActorID:        commanderUser.ID,
		EventType:      "task.created",
		Data: datatypes.JSON([]byte(`{
			"code": "TASK-2026-00001",
			"title": "Kế hoạch diễn tập tác chiến diện rộng quý IV",
			"priority": 1,
			"status": "in_progress"
		}`)),
		CreatedAt: now.Add(-5 * 24 * time.Hour),
	}
	db.Where("organization_id = ? AND task_id = ? AND event_type = ?", org.ID, task1.ID, "task.created").FirstOrCreate(&event)

	log.Println("✅ Seeding complete successfully! Dữ liệu mẫu đã sẵn sàng.")
	log.Println("======================================================================")
	log.Println("🔑 TÀI KHOẢN MẪU (Mật khẩu chung cho tất cả: password123)")
	log.Println("1. admin        (Quản trị hệ thống toàn diện - system_admin)")
	log.Println("2. commander    (Nguyễn Văn Chỉ Huy - commander)")
	log.Println("3. truongphong  (Trần Văn Trưởng Phòng - department_head / Phòng Tham Mưu)")
	log.Println("4. phophong     (Phạm Thị Phó Phòng - department_head / Phòng Kế Hoạch)")
	log.Println("5. nhanvien     (Lê Văn Nhân Viên - employee / Ban Thông Tin)")
	log.Println("6. tovien       (Hoàng Văn Tổ Viên - team_member / Tổ Kỹ Thuật)")
	log.Println("======================================================================")
}

func assignTask(db *gorm.DB, orgID, taskID uuid.UUID, userID, deptID, respUserID *uuid.UUID, assignedBy uuid.UUID, role string, isPrimary bool) {
	var existing model.TaskAssignment
	q := db.Where("task_id = ?", taskID)
	if userID != nil {
		q = q.Where("assignee_user_id = ?", *userID)
	}
	if deptID != nil {
		q = q.Where("assignee_department_id = ?", *deptID)
	}
	if err := q.First(&existing).Error; err != nil {
		assign := model.TaskAssignment{
			OrganizationID:       orgID,
			TaskID:               taskID,
			AssigneeUserID:       userID,
			AssigneeDepartmentID: deptID,
			ResponsibleUserID:    respUserID,
			AssignmentRole:       role,
			IsPrimary:            isPrimary,
			AssignedBy:           assignedBy,
		}
		db.Create(&assign)
	}
}
