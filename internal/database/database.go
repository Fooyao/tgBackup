package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"tgbackup/internal/models"
)

type DB struct {
	*sql.DB
}

func InitDB() (*DB, error) {
	db, err := sql.Open("sqlite3", "./tgbackup.db")
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	dbWrapper := &DB{db}
	if err := dbWrapper.createTables(); err != nil {
		return nil, err
	}

	return dbWrapper, nil
}

func (db *DB) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			first_name TEXT,
			last_name TEXT,
			username TEXT,
			phone TEXT,
			is_active BOOLEAN DEFAULT FALSE,
			last_sync_time DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			username TEXT,
			avatar_url TEXT,
			access_hash TEXT,
			last_message TEXT,
			last_time DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			conversation_id INTEGER NOT NULL,
			message_id INTEGER NOT NULL,
			from_id INTEGER,
			from_username TEXT,
			from_first_name TEXT,
			from_last_name TEXT,
			content TEXT NOT NULL,
			message_type TEXT DEFAULT 'text',
			media_url TEXT,
			timestamp DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (conversation_id) REFERENCES conversations(id),
			UNIQUE(user_id, conversation_id, message_id)
		)`,
		`CREATE TABLE IF NOT EXISTS auth_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			phone_code TEXT,
			is_active BOOLEAN DEFAULT FALSE,
			session_data TEXT,
			app_id INTEGER,
			app_hash TEXT,
			phone TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS updates_state (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			pts INTEGER DEFAULT 0,
			qts INTEGER DEFAULT 0,
			date INTEGER DEFAULT 0,
			seq INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			UNIQUE(user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	return nil
}

func (db *DB) SaveUser(user *models.User) error {
	query := `INSERT OR REPLACE INTO users 
		(id, first_name, last_name, username, phone, is_active, last_sync_time, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, user.ID, user.FirstName, user.LastName, user.Username, 
		user.Phone, user.IsActive, user.LastSyncTime, time.Now())
	return err
}

func (db *DB) GetUsers() ([]models.User, error) {
	query := `SELECT id, COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(username, ''), 
		COALESCE(phone, ''), is_active, last_sync_time, created_at, updated_at 
		FROM users ORDER BY updated_at DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var lastSyncTime sql.NullTime
		err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Username, 
			&user.Phone, &user.IsActive, &lastSyncTime, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if lastSyncTime.Valid {
			user.LastSyncTime = lastSyncTime.Time
		}
		users = append(users, user)
	}

	return users, nil
}

func (db *DB) GetUserByID(userID int64) (*models.User, error) {
	query := `SELECT id, COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(username, ''), 
		COALESCE(phone, ''), is_active, last_sync_time, created_at, updated_at 
		FROM users WHERE id = ?`

	var user models.User
	var lastSyncTime sql.NullTime
	err := db.QueryRow(query, userID).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Username, 
		&user.Phone, &user.IsActive, &lastSyncTime, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	
	if lastSyncTime.Valid {
		user.LastSyncTime = lastSyncTime.Time
	}

	return &user, nil
}

func (db *DB) SaveConversation(conv *models.Conversation) error {
	query := `INSERT OR REPLACE INTO conversations 
		(id, user_id, type, title, username, avatar_url, access_hash, last_message, last_time, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, conv.ID, conv.UserID, conv.Type, conv.Title, conv.Username, 
		conv.AvatarURL, conv.AccessHash, conv.LastMessage, conv.LastTime, time.Now())
	return err
}

func (db *DB) GetConversations() ([]models.Conversation, error) {
	query := `SELECT id, user_id, type, title, username, COALESCE(avatar_url, ''), COALESCE(access_hash, ''), 
		COALESCE(last_message, ''), last_time, created_at, updated_at 
		FROM conversations ORDER BY last_time DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []models.Conversation
	for rows.Next() {
		var conv models.Conversation
		err := rows.Scan(&conv.ID, &conv.UserID, &conv.Type, &conv.Title, &conv.Username, 
			&conv.AvatarURL, &conv.AccessHash, &conv.LastMessage, &conv.LastTime, &conv.CreatedAt, &conv.UpdatedAt)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

func (db *DB) GetConversationsByUserID(userID int64) ([]models.Conversation, error) {
	query := `SELECT id, user_id, type, title, username, COALESCE(avatar_url, ''), COALESCE(access_hash, ''), 
		COALESCE(last_message, ''), last_time, created_at, updated_at 
		FROM conversations WHERE user_id = ? ORDER BY last_time DESC`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []models.Conversation
	for rows.Next() {
		var conv models.Conversation
		err := rows.Scan(&conv.ID, &conv.UserID, &conv.Type, &conv.Title, &conv.Username, 
			&conv.AvatarURL, &conv.AccessHash, &conv.LastMessage, &conv.LastTime, &conv.CreatedAt, &conv.UpdatedAt)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

func (db *DB) SaveMessage(msg *models.Message) error {
	query := `INSERT OR REPLACE INTO messages 
		(user_id, conversation_id, message_id, from_id, from_username, from_first_name, from_last_name, 
		content, message_type, media_url, timestamp) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, msg.UserID, msg.ConversationID, msg.MessageID, msg.FromID, 
		msg.FromUsername, msg.FromFirstName, msg.FromLastName, msg.Content, 
		msg.MessageType, msg.MediaURL, msg.Timestamp)
	return err
}

func (db *DB) GetMessages(conversationID int64, limit, offset int) ([]models.Message, error) {
	query := `SELECT id, user_id, conversation_id, message_id, from_id, from_username, from_first_name, 
		from_last_name, content, message_type, media_url, timestamp, created_at 
		FROM messages WHERE conversation_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`

	rows, err := db.Query(query, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		err := rows.Scan(&msg.ID, &msg.UserID, &msg.ConversationID, &msg.MessageID, &msg.FromID, 
			&msg.FromUsername, &msg.FromFirstName, &msg.FromLastName, &msg.Content, 
			&msg.MessageType, &msg.MediaURL, &msg.Timestamp, &msg.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func (db *DB) GetMessagesByUserAndConversation(userID, conversationID int64, limit, offset int) ([]models.Message, error) {
	query := `SELECT id, user_id, conversation_id, message_id, from_id, from_username, from_first_name, 
		from_last_name, content, message_type, media_url, timestamp, created_at 
		FROM messages WHERE user_id = ? AND conversation_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`

	rows, err := db.Query(query, userID, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		err := rows.Scan(&msg.ID, &msg.UserID, &msg.ConversationID, &msg.MessageID, &msg.FromID, 
			&msg.FromUsername, &msg.FromFirstName, &msg.FromLastName, &msg.Content, 
			&msg.MessageType, &msg.MediaURL, &msg.Timestamp, &msg.CreatedAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func (db *DB) SaveAuthSession(session *models.AuthSession) error {
	query := `INSERT OR REPLACE INTO auth_sessions (user_id, phone_code, is_active, session_data, app_id, app_hash, phone, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, session.UserID, session.PhoneCode, session.IsActive, session.SessionData, 
		session.AppID, session.AppHash, session.Phone, time.Now())
	return err
}

func (db *DB) GetActiveAuthSession() (*models.AuthSession, error) {
	query := `SELECT id, COALESCE(user_id, 0), COALESCE(phone_code, ''), is_active, COALESCE(session_data, ''), 
		COALESCE(app_id, 0), COALESCE(app_hash, ''), COALESCE(phone, ''), created_at, updated_at 
		FROM auth_sessions WHERE is_active = 1 ORDER BY created_at DESC LIMIT 1`

	var session models.AuthSession
	err := db.QueryRow(query).Scan(&session.ID, &session.UserID, &session.PhoneCode, &session.IsActive, 
		&session.SessionData, &session.AppID, &session.AppHash, &session.Phone,
		&session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (db *DB) GetActiveAuthSessionByUserID(userID int64) (*models.AuthSession, error) {
	query := `SELECT id, user_id, COALESCE(phone_code, ''), is_active, COALESCE(session_data, ''), 
		COALESCE(app_id, 0), COALESCE(app_hash, ''), COALESCE(phone, ''), created_at, updated_at 
		FROM auth_sessions WHERE user_id = ? AND is_active = 1 ORDER BY created_at DESC LIMIT 1`

	var session models.AuthSession
	err := db.QueryRow(query, userID).Scan(&session.ID, &session.UserID, &session.PhoneCode, &session.IsActive, 
		&session.SessionData, &session.AppID, &session.AppHash, &session.Phone,
		&session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (db *DB) GetPendingAuthSession() (*models.AuthSession, error) {
	query := `SELECT id, COALESCE(user_id, 0), phone_code, is_active, session_data, app_id, app_hash, phone, created_at, updated_at 
		FROM auth_sessions WHERE phone_code IS NOT NULL AND phone_code != '' ORDER BY created_at DESC LIMIT 1`

	var session models.AuthSession
	err := db.QueryRow(query).Scan(&session.ID, &session.UserID, &session.PhoneCode, &session.IsActive, 
		&session.SessionData, &session.AppID, &session.AppHash, &session.Phone,
		&session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// SaveUpdatesState saves the current updates state for a user
func (db *DB) SaveUpdatesState(userID int64, pts, qts, date, seq int) error {
	query := `INSERT OR REPLACE INTO updates_state (user_id, pts, qts, date, seq, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?)`
	
	_, err := db.Exec(query, userID, pts, qts, date, seq, time.Now())
	return err
}

// GetUpdatesState gets the stored updates state for a user
func (db *DB) GetUpdatesState(userID int64) (pts, qts, date, seq int, err error) {
	query := `SELECT pts, qts, date, seq FROM updates_state WHERE user_id = ?`
	
	err = db.QueryRow(query, userID).Scan(&pts, &qts, &date, &seq)
	if err == sql.ErrNoRows {
		// No state found, return zeros (first sync)
		return 0, 0, 0, 0, nil
	}
	
	return pts, qts, date, seq, err
}