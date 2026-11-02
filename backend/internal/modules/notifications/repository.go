package notifications

import (
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

func (r *Repository) List(recipientID uuid.UUID, filter NotificationFilter) ([]model.Notification, int64, error) {
	query := r.db.Model(&model.Notification{}).Where("recipient_id = ?", recipientID)

	if filter.UnreadOnly {
		query = query.Where("read_at IS NULL")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
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

	var list []model.Notification
	err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error

	return list, total, err
}

func (r *Repository) GetUnreadCount(recipientID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("recipient_id = ? AND read_at IS NULL", recipientID).
		Count(&count).Error
	return count, err
}

func (r *Repository) MarkAsRead(recipientID, notificationID uuid.UUID) (*model.Notification, error) {
	var notif model.Notification
	if err := r.db.Where("id = ? AND recipient_id = ?", notificationID, recipientID).First(&notif).Error; err != nil {
		return nil, err
	}

	if notif.ReadAt == nil {
		now := time.Now().UTC()
		notif.ReadAt = &now
		if err := r.db.Model(&notif).Update("read_at", now).Error; err != nil {
			return nil, err
		}
	}
	return &notif, nil
}

func (r *Repository) MarkAllAsRead(recipientID uuid.UUID) (int64, error) {
	now := time.Now().UTC()
	result := r.db.Model(&model.Notification{}).
		Where("recipient_id = ? AND read_at IS NULL", recipientID).
		Update("read_at", now)
	return result.RowsAffected, result.Error
}

func (r *Repository) Create(notif *model.Notification) error {
	return r.db.Create(notif).Error
}
