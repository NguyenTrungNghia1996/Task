package notifications_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/notifications"
	"taskmgmt/internal/testutil"
)

func setupNotificationService(t *testing.T) (*notifications.Service, *model.Organization, *model.User) {
	db := testutil.SetupTestDB(t)
	org, user := testutil.CreateTestOrgAndUser(t, db, "notif_user", "employee")
	repo := notifications.NewRepository(db)
	svc := notifications.NewService(repo)
	return svc, org, user
}

func TestNotification_NotifyAndUnreadCount(t *testing.T) {
	svc, org, user := setupNotificationService(t)

	// Send 3 notifications
	for i := 1; i <= 3; i++ {
		_, err := svc.Notify(org.ID, user.ID, nil, "task_assigned", "Thông báo mới", "Nội dung công việc")
		require.NoError(t, err)
	}

	// Check unread count
	count, err := svc.GetUnreadCount(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// List notifications
	items, total, err := svc.List(user.ID, notifications.NotificationFilter{
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 3)
}

func TestNotification_MarkAsRead(t *testing.T) {
	svc, org, user := setupNotificationService(t)

	n1, err := svc.Notify(org.ID, user.ID, nil, "task_assigned", "Thông báo 1", "Nội dung 1")
	require.NoError(t, err)
	_, err = svc.Notify(org.ID, user.ID, nil, "task_assigned", "Thông báo 2", "Nội dung 2")
	require.NoError(t, err)

	// Mark 1 as read
	updated, err := svc.MarkAsRead(user.ID, n1.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.ReadAt)

	// Unread count should now be 1
	count, err := svc.GetUnreadCount(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestNotification_MarkAllAsRead(t *testing.T) {
	svc, org, user := setupNotificationService(t)

	for i := 0; i < 5; i++ {
		_, err := svc.Notify(org.ID, user.ID, nil, "alert", "Cảnh báo", "Nội dung")
		require.NoError(t, err)
	}

	count, err := svc.GetUnreadCount(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// Mark all as read
	rows, err := svc.MarkAllAsRead(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5), rows)

	// Unread count is now 0
	count, err = svc.GetUnreadCount(user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestNotification_RealtimeBroker(t *testing.T) {
	svc, org, user := setupNotificationService(t)

	// Subscribe to realtime channel
	ch := svc.Subscribe(user.ID)
	defer svc.Unsubscribe(user.ID, ch)

	// Send notification in background
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = svc.Notify(org.ID, user.ID, nil, "live_event", "Tin khẩn", "Chi tiết tin")
	}()

	select {
	case notif := <-ch:
		assert.Equal(t, "Tin khẩn", notif.Title)
		assert.Equal(t, "live_event", notif.Type)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for realtime notification")
	}
}
