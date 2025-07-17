package models

import (
	"time"
)

type User struct {
	ID           int64     `json:"id" db:"id"`                     // Telegram User ID
	FirstName    string    `json:"first_name" db:"first_name"`
	LastName     string    `json:"last_name" db:"last_name"`
	Username     string    `json:"username" db:"username"`
	Phone        string    `json:"phone" db:"phone"`
	IsActive     bool      `json:"is_active" db:"is_active"`       // 当前是否可以使用(session有效)
	LastSyncTime time.Time `json:"last_sync_time" db:"last_sync_time"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Conversation struct {
	ID          int64     `json:"id" db:"id"`
	UserID      int64     `json:"user_id" db:"user_id"`           // 关联的Telegram用户ID
	Type        string    `json:"type" db:"type"` // user, bot, channel, group
	Title       string    `json:"title" db:"title"`
	Username    string    `json:"username" db:"username"`
	AvatarURL   string    `json:"avatar_url" db:"avatar_url"`
	AccessHash  string    `json:"access_hash" db:"access_hash"`
	LastMessage string    `json:"last_message" db:"last_message"`
	LastTime    time.Time `json:"last_time" db:"last_time"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type Message struct {
	ID             int64     `json:"id" db:"id"`
	UserID         int64     `json:"user_id" db:"user_id"`           // 关联的Telegram用户ID
	ConversationID int64     `json:"conversation_id" db:"conversation_id"`
	MessageID      int       `json:"message_id" db:"message_id"`
	FromID         int64     `json:"from_id" db:"from_id"`
	FromUsername   string    `json:"from_username" db:"from_username"`
	FromFirstName  string    `json:"from_first_name" db:"from_first_name"`
	FromLastName   string    `json:"from_last_name" db:"from_last_name"`
	Content        string    `json:"content" db:"content"`
	MessageType    string    `json:"message_type" db:"message_type"` // text, photo, video, document, etc.
	MediaURL       string    `json:"media_url" db:"media_url"`
	Timestamp      time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

type AuthSession struct {
	ID           int       `json:"id" db:"id"`
	UserID       int64     `json:"user_id" db:"user_id"`           // 关联的Telegram用户ID
	PhoneCode    string    `json:"phone_code" db:"phone_code"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	SessionData  string    `json:"session_data" db:"session_data"` // Store session file path or data
	AppID        int       `json:"app_id" db:"app_id"`
	AppHash      string    `json:"app_hash" db:"app_hash"`
	Phone        string    `json:"phone" db:"phone"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type SyncStatus struct {
	IsRunning        bool      `json:"is_running"`
	LastSyncTime     time.Time `json:"last_sync_time"`
	TotalMessages    int       `json:"total_messages"`
	TotalConversations int     `json:"total_conversations"`
}