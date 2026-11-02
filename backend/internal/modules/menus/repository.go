package menus

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"taskmgmt/internal/model"
)

var ErrMenuNotFound = errors.New("menu not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetActiveRootMenus() ([]model.Menu, error) {
	var list []model.Menu
	err := r.db.Where("parent_id IS NULL AND is_active = ? AND is_hidden = ?", true, false).
		Order("sort_order ASC, created_at ASC").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ? AND is_hidden = ?", true, false).
				Order("sort_order ASC, created_at ASC")
		}).
		Preload("Children.Children", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ? AND is_hidden = ?", true, false).
				Order("sort_order ASC, created_at ASC")
		}).
		Find(&list).Error
	return list, err
}

func (r *Repository) GetAllRootMenus() ([]model.Menu, error) {
	var list []model.Menu
	err := r.db.Where("parent_id IS NULL").
		Order("sort_order ASC, created_at ASC").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).
		Preload("Children.Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).
		Find(&list).Error
	return list, err
}

func (r *Repository) GetByID(id uuid.UUID) (*model.Menu, error) {
	var menu model.Menu
	if err := r.db.Where("id = ?", id).First(&menu).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}
	return &menu, nil
}

func (r *Repository) Create(menu *model.Menu) error {
	return r.db.Create(menu).Error
}

func (r *Repository) Update(id uuid.UUID, updates map[string]interface{}) (*model.Menu, error) {
	result := r.db.Model(&model.Menu{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrMenuNotFound
	}
	return r.GetByID(id)
}

func (r *Repository) Delete(id uuid.UUID) error {
	// Check if has children
	var childCount int64
	if err := r.db.Model(&model.Menu{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return err
	}
	if childCount > 0 {
		return errors.New("cannot delete menu that contains sub-menus")
	}

	result := r.db.Where("id = ?", id).Delete(&model.Menu{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMenuNotFound
	}
	return nil
}

