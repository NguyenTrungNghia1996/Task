package notifications

type NotificationFilter struct {
	Page       int  `query:"page"`
	PageSize   int  `query:"page_size"`
	UnreadOnly bool `query:"unread_only"`
}

type UnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

type MarkAllReadResponse struct {
	Success      bool  `json:"success"`
	UpdatedCount int64 `json:"updated_count"`
}
