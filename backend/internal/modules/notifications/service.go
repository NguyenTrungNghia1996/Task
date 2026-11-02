package notifications

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"taskmgmt/internal/model"
)

type Broker struct {
	mu      sync.RWMutex
	clients map[uuid.UUID][]chan *model.Notification
}

func newBroker() *Broker {
	return &Broker{
		clients: make(map[uuid.UUID][]chan *model.Notification),
	}
}

func (b *Broker) subscribe(recipientID uuid.UUID) chan *model.Notification {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *model.Notification, 10)
	b.clients[recipientID] = append(b.clients[recipientID], ch)
	return ch
}

func (b *Broker) unsubscribe(recipientID uuid.UUID, ch chan *model.Notification) {
	b.mu.Lock()
	defer b.mu.Unlock()

	channels := b.clients[recipientID]
	for i, c := range channels {
		if c == ch {
			b.clients[recipientID] = append(channels[:i], channels[i+1:]...)
			close(c)
			break
		}
	}
	if len(b.clients[recipientID]) == 0 {
		delete(b.clients, recipientID)
	}
}

func (b *Broker) publish(notif *model.Notification) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	channels, ok := b.clients[notif.RecipientID]
	if !ok {
		return
	}
	for _, ch := range channels {
		select {
		case ch <- notif:
		default:
		}
	}
}

type Service struct {
	repo   *Repository
	broker *Broker
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo:   repo,
		broker: newBroker(),
	}
}

func (s *Service) List(recipientID uuid.UUID, filter NotificationFilter) ([]model.Notification, int64, error) {
	return s.repo.List(recipientID, filter)
}

func (s *Service) GetUnreadCount(recipientID uuid.UUID) (int64, error) {
	return s.repo.GetUnreadCount(recipientID)
}

func (s *Service) MarkAsRead(recipientID, notificationID uuid.UUID) (*model.Notification, error) {
	return s.repo.MarkAsRead(recipientID, notificationID)
}

func (s *Service) MarkAllAsRead(recipientID uuid.UUID) (int64, error) {
	return s.repo.MarkAllAsRead(recipientID)
}

func (s *Service) Notify(orgID, recipientID uuid.UUID, taskID *uuid.UUID, notifType, title, body string) (*model.Notification, error) {
	notif := &model.Notification{
		BaseModel:      model.BaseModel{ID: uuid.New(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		OrganizationID: orgID,
		RecipientID:    recipientID,
		TaskID:         taskID,
		Type:           notifType,
		Title:          title,
		Body:           body,
		ReadAt:         nil,
	}

	if err := s.repo.Create(notif); err != nil {
		return nil, err
	}

	s.broker.publish(notif)
	return notif, nil
}

func (s *Service) Subscribe(recipientID uuid.UUID) chan *model.Notification {
	return s.broker.subscribe(recipientID)
}

func (s *Service) Unsubscribe(recipientID uuid.UUID, ch chan *model.Notification) {
	s.broker.unsubscribe(recipientID, ch)
}
