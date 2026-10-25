package service

import (
	"context"

	"invoicekeeper/internal/models"
	"invoicekeeper/internal/store"
)

// NotificationService — колокольчик веб-панели: у каждого пользователя свои уведомления.
type NotificationService struct{ st *store.Store }

func NewNotificationService(st *store.Store) *NotificationService {
	return &NotificationService{st: st}
}

type NotificationList struct {
	Items  []models.Notification `json:"items"`
	Unread int64                 `json:"unread"`
}

func (s *NotificationService) List(ctx context.Context, a Actor, unreadOnly bool, limit int) (*NotificationList, error) {
	if a.UserID == nil {
		return &NotificationList{Items: []models.Notification{}}, nil
	}
	limit = min(max(limit, 1), 100)
	items, err := s.st.ListNotifications(ctx, *a.UserID, unreadOnly, limit)
	if err != nil {
		return nil, err
	}
	n, err := s.st.UnreadNotifications(ctx, *a.UserID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.Notification{}
	}
	return &NotificationList{Items: items, Unread: n}, nil
}

// MarkRead — отметить прочитанными (ids пусто — все).
func (s *NotificationService) MarkRead(ctx context.Context, a Actor, ids []uint) error {
	if a.UserID == nil {
		return nil
	}
	if len(ids) > 200 {
		return Invalid("too many ids")
	}
	return s.st.MarkNotificationsRead(ctx, *a.UserID, ids)
}

// Clear — удалить прочитанные уведомления.
func (s *NotificationService) Clear(ctx context.Context, a Actor) error {
	if a.UserID == nil {
		return nil
	}
	return s.st.DeleteNotifications(ctx, *a.UserID, true)
}
