package rbac

import (
	"time"

	"github.com/google/uuid"

	"taskmgmt/internal/model"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListPermissions() ([]ModuleGroupedPermissions, error) {
	list, err := s.repo.ListPermissions()
	if err != nil {
		return nil, err
	}

	// Group permissions by module for UI display
	moduleMap := make(map[string][]model.Permission)
	var moduleOrder []string

	for _, p := range list {
		if _, exists := moduleMap[p.Module]; !exists {
			moduleOrder = append(moduleOrder, p.Module)
		}
		moduleMap[p.Module] = append(moduleMap[p.Module], p)
	}

	grouped := make([]ModuleGroupedPermissions, 0, len(moduleOrder))
	for _, m := range moduleOrder {
		grouped = append(grouped, ModuleGroupedPermissions{
			Module:      m,
			Permissions: moduleMap[m],
		})
	}

	return grouped, nil
}

func (s *Service) GetPermissionOptions() ([]PermissionOption, error) {
	return s.repo.GetPermissionOptions()
}

func (s *Service) CreatePermission(req CreatePermissionRequest) (*model.Permission, error) {
	perm := &model.Permission{
		BaseModel:   model.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		Code:        req.Code,
		Name:        req.Name,
		Module:      req.Module,
		Description: req.Description,
	}

	if err := s.repo.CreatePermission(perm); err != nil {
		return nil, err
	}
	return perm, nil
}

func (s *Service) ListRoles() ([]model.Role, error) {
	return s.repo.ListRoles()
}

func (s *Service) GetRoleWithPermissions(roleID uuid.UUID) (*RoleWithPermissionsResponse, error) {
	role, err := s.repo.GetRoleByID(roleID)
	if err != nil {
		return nil, err
	}

	perms, err := s.repo.GetRolePermissions(roleID)
	if err != nil {
		return nil, err
	}

	return &RoleWithPermissionsResponse{
		Role:        *role,
		Permissions: perms,
	}, nil
}

func (s *Service) SetRolePermissions(roleID uuid.UUID, permissionCodes []string) error {
	// Verify role exists
	if _, err := s.repo.GetRoleByID(roleID); err != nil {
		return err
	}
	return s.repo.SetRolePermissions(roleID, permissionCodes)
}

func (s *Service) GetPermissionsForRoles(roles []string) ([]string, error) {
	return s.repo.GetPermissionsForRoles(roles)
}

