package rbac

import (
	"fmt"

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

func (r *Repository) ListPermissions() ([]model.Permission, error) {
	var list []model.Permission
	err := r.db.Order("module ASC, code ASC").Find(&list).Error
	return list, err
}

func (r *Repository) GetPermissionOptions() ([]PermissionOption, error) {
	var perms []model.Permission
	if err := r.db.Order("module ASC, code ASC").Find(&perms).Error; err != nil {
		return nil, err
	}

	options := make([]PermissionOption, len(perms))
	for i, p := range perms {
		options[i] = PermissionOption{
			Value:  p.Code,
			Label:  fmt.Sprintf("%s (%s)", p.Name, p.Code),
			Module: p.Module,
		}
	}
	return options, nil
}

func (r *Repository) CreatePermission(perm *model.Permission) error {
	return r.db.Create(perm).Error
}

func (r *Repository) ListRoles() ([]model.Role, error) {
	var roles []model.Role
	err := r.db.Order("created_at ASC").Find(&roles).Error
	return roles, err
}

func (r *Repository) GetRoleByID(roleID uuid.UUID) (*model.Role, error) {
	var role model.Role
	if err := r.db.Where("id = ?", roleID).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *Repository) GetRolePermissions(roleID uuid.UUID) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.Table("permissions").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id = ? AND role_permissions.deleted_at IS NULL", roleID).
		Order("permissions.module ASC, permissions.code ASC").
		Find(&perms).Error
	return perms, err
}

func (r *Repository) SetRolePermissions(roleID uuid.UUID, permissionCodes []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Xóa các quyền cũ của role (hard delete bảng trung gian)
		if err := tx.Unscoped().Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}

		if len(permissionCodes) == 0 {
			return nil
		}

		// 2. Lấy ID của các permission tương ứng
		var perms []model.Permission
		if err := tx.Where("code IN ?", permissionCodes).Find(&perms).Error; err != nil {
			return err
		}

		// 3. Insert các bản ghi liên kết mới
		var rolePerms []model.RolePermission
		for _, p := range perms {
			rolePerms = append(rolePerms, model.RolePermission{
				BaseModel:    model.BaseModel{ID: uuid.New()},
				RoleID:       roleID,
				PermissionID: p.ID,
			})
		}

		if len(rolePerms) > 0 {
			return tx.Create(&rolePerms).Error
		}
		return nil
	})
}

func (r *Repository) GetPermissionsForRoles(roles []string) ([]string, error) {
	for _, roleCode := range roles {
		if roleCode == "system_admin" {
			var allPerms []model.Permission
			if err := r.db.Find(&allPerms).Error; err != nil {
				return nil, err
			}
			res := make([]string, len(allPerms))
			for i, p := range allPerms {
				res[i] = p.Code
			}
			return res, nil
		}
	}

	var perms []string
	err := r.db.Table("role_permissions").
		Joins("JOIN roles ON roles.id = role_permissions.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("roles.code IN ? AND role_permissions.deleted_at IS NULL AND roles.deleted_at IS NULL AND permissions.deleted_at IS NULL", roles).
		Pluck("DISTINCT permissions.code", &perms).Error
	return perms, err
}

