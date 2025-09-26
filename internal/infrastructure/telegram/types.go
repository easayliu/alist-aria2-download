package telegram

import "time"

type NotificationMessage struct {
	Type      string                 `json:"type"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}
