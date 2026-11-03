package menus

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

func (s *Service) GetUserMenus(userPerms []string, isSystemAdmin bool) ([]AntdMenuItem, error) {
	rootMenus, err := s.repo.GetActiveRootMenus()
	if err != nil {
		return nil, err
	}

	permMap := make(map[string]bool, len(userPerms))
	for _, p := range userPerms {
		permMap[p] = true
	}

	var filterAndTransform func(items []model.Menu) []AntdMenuItem
	filterAndTransform = func(items []model.Menu) []AntdMenuItem {
		var result []AntdMenuItem

		for _, item := range items {
			// Kiểm tra quyền của chính menu này
			hasPerm := isSystemAdmin || item.RequiredPermission == "" || permMap[item.RequiredPermission]

			// Xử lý menu con
			var transformedChildren []AntdMenuItem
			if len(item.Children) > 0 {
				transformedChildren = filterAndTransform(item.Children)
			}

			// Điều kiện hiển thị:
			// 1. Menu lá (không có con): Phải có quyền
			// 2. Menu cha (có con): Phải có ít nhất 1 menu con thỏa mãn quyền
			if len(item.Children) > 0 {
				if len(transformedChildren) > 0 {
					key := item.Path
					if key == "" {
						key = item.Code
					}
					result = append(result, AntdMenuItem{
						Key:      key,
						Label:    item.Title,
						Icon:     item.Icon,
						Path:     item.Path,
						Children: transformedChildren,
					})
				}
			} else if hasPerm {
				key := item.Path
				if key == "" {
					key = item.Code
				}
				result = append(result, AntdMenuItem{
					Key:   key,
					Label: item.Title,
					Icon:  item.Icon,
					Path:  item.Path,
				})
			}
		}

		if result == nil {
			result = []AntdMenuItem{}
		}
		return result
	}

	res := filterAndTransform(rootMenus)
	if res == nil {
		res = []AntdMenuItem{}
	}
	return res, nil
}

func (s *Service) GetAllMenus() ([]model.Menu, error) {
	menus, err := s.repo.GetAllRootMenus()
	if err != nil {
		return nil, err
	}
	if menus == nil {
		menus = []model.Menu{}
	}
	return menus, nil
}

func (s *Service) CreateMenu(req CreateMenuRequest) (*model.Menu, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	isHidden := false
	if req.IsHidden != nil {
		isHidden = *req.IsHidden
	}

	menu := &model.Menu{
		BaseModel:          model.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		ParentID:           req.ParentID,
		Code:               req.Code,
		Title:              req.Title,
		Icon:               req.Icon,
		Path:               req.Path,
		RequiredPermission: req.RequiredPermission,
		SortOrder:          req.SortOrder,
		IsActive:           isActive,
		IsHidden:           isHidden,
	}

	if err := s.repo.Create(menu); err != nil {
		return nil, err
	}
	return menu, nil
}

func (s *Service) UpdateMenu(id uuid.UUID, req UpdateMenuRequest) (*model.Menu, error) {
	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Path != nil {
		updates["path"] = *req.Path
	}
	if req.RequiredPermission != nil {
		updates["required_permission"] = *req.RequiredPermission
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsHidden != nil {
		updates["is_hidden"] = *req.IsHidden
	}
	if req.ParentID != nil {
		updates["parent_id"] = req.ParentID
	}
	updates["updated_at"] = time.Now().UTC()

	return s.repo.Update(id, updates)
}

func (s *Service) DeleteMenu(id uuid.UUID) error {
	return s.repo.Delete(id)
}

