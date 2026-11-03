package users

import (
	"fmt"
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

func (r *Repository) ListUsers(orgID uuid.UUID, filter UserFilter) ([]UserListItem, int64, error) {
	query := r.db.Model(&model.User{}).Where("users.organization_id = ?", orgID)

	if filter.IsActive != nil {
		query = query.Where("users.is_active = ?", *filter.IsActive)
	}

	if filter.Query != "" {
		likeTerm := "%" + filter.Query + "%"
		query = query.Where("(users.username LIKE ? OR users.full_name LIKE ? OR users.email LIKE ?)", likeTerm, likeTerm, likeTerm)
	}

	if filter.DepartmentID != nil {
		query = query.Joins("JOIN department_members ON department_members.user_id = users.id AND department_members.deleted_at IS NULL").
			Where("department_members.department_id = ?", *filter.DepartmentID)
	}

	if filter.RoleCode != "" {
		query = query.Joins("JOIN user_roles ON user_roles.user_id = users.id AND user_roles.deleted_at IS NULL").
			Joins("JOIN roles ON roles.id = user_roles.role_id AND roles.deleted_at IS NULL").
			Where("roles.code = ?", filter.RoleCode)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Distinct("users.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var users []model.User
	if err := query.Select("users.*").Order("users.created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	if len(users) == 0 {
		return []UserListItem{}, total, nil
	}

	userIDs := make([]uuid.UUID, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	// Fetch roles for these users
	var userRoles []model.UserRole
	_ = r.db.Preload("Role").Where("user_id IN ? AND deleted_at IS NULL", userIDs).Find(&userRoles).Error
	rolesMap := make(map[uuid.UUID][]string)
	for _, ur := range userRoles {
		if ur.Role != nil {
			rolesMap[ur.UserID] = append(rolesMap[ur.UserID], ur.Role.Code)
		}
	}

	// Fetch primary departments for these users
	var deptMembers []model.DepartmentMember
	_ = r.db.Preload("Department").Where("user_id IN ? AND is_primary = ? AND deleted_at IS NULL", userIDs, true).Find(&deptMembers).Error
	deptMap := make(map[uuid.UUID]*DepartmentInfo)
	for _, dm := range deptMembers {
		if dm.Department != nil {
			deptMap[dm.UserID] = &DepartmentInfo{
				ID:            dm.DepartmentID,
				Name:          dm.Department.Name,
				PositionTitle: dm.PositionTitle,
				IsPrimary:     dm.IsPrimary,
			}
		}
	}

	items := make([]UserListItem, len(users))
	for i, u := range users {
		rList := rolesMap[u.ID]
		if rList == nil {
			rList = []string{}
		}

		items[i] = UserListItem{
			ID:         u.ID,
			Username:   u.Username,
			FullName:   u.FullName,
			Email:      u.Email,
			IsActive:   u.IsActive,
			Roles:      rList,
			Department: deptMap[u.ID],
			CreatedAt:  u.CreatedAt,
		}
	}

	return items, total, nil
}

func (r *Repository) GetUserByID(orgID, userID uuid.UUID) (*UserDetailResponse, error) {
	var user model.User
	if err := r.db.Where("id = ? AND organization_id = ?", userID, orgID).First(&user).Error; err != nil {
		return nil, err
	}

	var userRoles []model.UserRole
	_ = r.db.Preload("Role").Where("user_id = ? AND deleted_at IS NULL", user.ID).Find(&userRoles).Error
	roles := make([]string, 0, len(userRoles))
	for _, ur := range userRoles {
		if ur.Role != nil {
			roles = append(roles, ur.Role.Code)
		}
	}

	var deptMembers []model.DepartmentMember
	_ = r.db.Preload("Department").Where("user_id = ? AND deleted_at IS NULL", user.ID).Find(&deptMembers).Error
	depts := make([]DepartmentInfo, 0, len(deptMembers))
	for _, dm := range deptMembers {
		name := ""
		if dm.Department != nil {
			name = dm.Department.Name
		}
		depts = append(depts, DepartmentInfo{
			ID:            dm.DepartmentID,
			Name:          name,
			PositionTitle: dm.PositionTitle,
			IsPrimary:     dm.IsPrimary,
		})
	}

	return &UserDetailResponse{
		ID:          user.ID,
		Username:    user.Username,
		FullName:    user.FullName,
		Email:       user.Email,
		IsActive:    user.IsActive,
		Roles:       roles,
		Departments: depts,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (r *Repository) FindByUsername(orgID uuid.UUID, username string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("organization_id = ? AND username = ?", orgID, username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) CreateUser(orgID uuid.UUID, user *model.User, roleCodes []string, deptID *uuid.UUID, positionTitle string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// Gán vai trò
		if len(roleCodes) > 0 {
			var roles []model.Role
			if err := tx.Where("code IN ?", roleCodes).Find(&roles).Error; err != nil {
				return err
			}
			for _, r := range roles {
				ur := model.UserRole{
					BaseModel:      model.BaseModel{ID: uuid.New()},
					OrganizationID: orgID,
					UserID:         user.ID,
					RoleID:         r.ID,
				}
				if err := tx.Create(&ur).Error; err != nil {
					return err
				}
			}
		}

		// Gán phòng ban chính nếu có
		if deptID != nil {
			dm := model.DepartmentMember{
				BaseModel:      model.BaseModel{ID: uuid.New()},
				OrganizationID: orgID,
				DepartmentID:   *deptID,
				UserID:         user.ID,
				PositionTitle:  positionTitle,
				IsPrimary:      true,
				EffectiveFrom:  time.Now().UTC(),
			}
			if err := tx.Create(&dm).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) UpdateUser(orgID, userID uuid.UUID, req UpdateUserRequest) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		updates := make(map[string]interface{})
		if req.FullName != nil {
			updates["full_name"] = *req.FullName
		}
		if req.Email != nil {
			updates["email"] = *req.Email
		}
		if req.IsActive != nil {
			updates["is_active"] = *req.IsActive
		}

		if len(updates) > 0 {
			updates["updated_at"] = time.Now().UTC()
			if err := tx.Model(&model.User{}).Where("id = ? AND organization_id = ?", userID, orgID).Updates(updates).Error; err != nil {
				return err
			}
		}

		// Cập nhật phòng ban nếu có
		if req.DepartmentID != nil {
			pos := ""
			if req.PositionTitle != nil {
				pos = *req.PositionTitle
			}

			// Bỏ primary cũ
			_ = tx.Model(&model.DepartmentMember{}).
				Where("user_id = ? AND organization_id = ?", userID, orgID).
				Update("is_primary", false)

			// Tạo hoặc đặt làm primary cho phòng ban mới
			var existing model.DepartmentMember
			err := tx.Where("user_id = ? AND department_id = ?", userID, *req.DepartmentID).First(&existing).Error
			if err == nil {
				existing.IsPrimary = true
				if pos != "" {
					existing.PositionTitle = pos
				}
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
			} else {
				dm := model.DepartmentMember{
					BaseModel:      model.BaseModel{ID: uuid.New()},
					OrganizationID: orgID,
					DepartmentID:   *req.DepartmentID,
					UserID:         userID,
					PositionTitle:  pos,
					IsPrimary:      true,
					EffectiveFrom:  time.Now().UTC(),
				}
				if err := tx.Create(&dm).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (r *Repository) SetUserRoles(orgID, userID uuid.UUID, roleCodes []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Hard delete old user_roles
		if err := tx.Unscoped().Where("user_id = ? AND organization_id = ?", userID, orgID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}

		if len(roleCodes) == 0 {
			return nil
		}

		var roles []model.Role
		if err := tx.Where("code IN ?", roleCodes).Find(&roles).Error; err != nil {
			return err
		}

		for _, r := range roles {
			ur := model.UserRole{
				BaseModel:      model.BaseModel{ID: uuid.New()},
				OrganizationID: orgID,
				UserID:         userID,
				RoleID:         r.ID,
			}
			if err := tx.Create(&ur).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) ResetPassword(orgID, userID uuid.UUID, passwordHash string) error {
	return r.db.Model(&model.User{}).
		Where("id = ? AND organization_id = ?", userID, orgID).
		Updates(map[string]interface{}{
			"password_hash": passwordHash,
			"updated_at":    time.Now().UTC(),
		}).Error
}

func (r *Repository) GetUserOptions(orgID uuid.UUID) ([]UserOption, error) {
	var users []model.User
	if err := r.db.Where("organization_id = ? AND is_active = ?", orgID, true).
		Order("full_name ASC").
		Find(&users).Error; err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return []UserOption{}, nil
	}

	userIDs := make([]uuid.UUID, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	var deptMembers []model.DepartmentMember
	_ = r.db.Preload("Department").
		Where("user_id IN ? AND is_primary = ? AND deleted_at IS NULL", userIDs, true).
		Find(&deptMembers).Error

	deptMap := make(map[uuid.UUID]*DepartmentInfo)
	for _, dm := range deptMembers {
		if dm.Department != nil {
			deptMap[dm.UserID] = &DepartmentInfo{
				ID:            dm.DepartmentID,
				Name:          dm.Department.Name,
				PositionTitle: dm.PositionTitle,
			}
		}
	}

	options := make([]UserOption, len(users))
	for i, u := range users {
		deptName := ""
		posTitle := ""
		if d, ok := deptMap[u.ID]; ok && d != nil {
			deptName = d.Name
			posTitle = d.PositionTitle
		}

		label := fmt.Sprintf("%s (%s)", u.FullName, u.Username)
		if deptName != "" {
			label = fmt.Sprintf("%s (%s) - %s", u.FullName, u.Username, deptName)
		}

		options[i] = UserOption{
			Value:          u.ID,
			Label:          label,
			Username:       u.Username,
			FullName:       u.FullName,
			DepartmentName: deptName,
			PositionTitle:  posTitle,
		}
	}

	return options, nil
}
