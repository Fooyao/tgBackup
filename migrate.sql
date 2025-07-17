-- Migration script to add multi-user support

-- Create users table if not exists
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    first_name TEXT,
    last_name TEXT,
    username TEXT,
    phone TEXT,
    is_active BOOLEAN DEFAULT FALSE,
    last_sync_time DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Add user_id column to conversations table
ALTER TABLE conversations ADD COLUMN user_id INTEGER DEFAULT 0;

-- Add user_id column to messages table
ALTER TABLE messages ADD COLUMN user_id INTEGER DEFAULT 0;

-- Add user_id column to auth_sessions table if not exists
ALTER TABLE auth_sessions ADD COLUMN user_id INTEGER DEFAULT 0;

-- Create a default user from existing auth session
INSERT OR IGNORE INTO users (id, first_name, last_name, username, phone, is_active, last_sync_time, created_at, updated_at)
SELECT 
    COALESCE((SELECT app_id FROM auth_sessions WHERE is_active = 1 LIMIT 1), 1) as id,
    '未知用户' as first_name,
    '' as last_name,
    '' as username,
    COALESCE((SELECT phone FROM auth_sessions WHERE is_active = 1 LIMIT 1), '') as phone,
    1 as is_active,
    datetime('now') as last_sync_time,
    datetime('now') as created_at,
    datetime('now') as updated_at
WHERE EXISTS (SELECT 1 FROM conversations WHERE user_id = 0);

-- Update existing conversations to associate with default user
UPDATE conversations 
SET user_id = (
    SELECT COALESCE((SELECT app_id FROM auth_sessions WHERE is_active = 1 LIMIT 1), 1)
) 
WHERE user_id = 0;

-- Update existing messages to associate with default user
UPDATE messages 
SET user_id = (
    SELECT COALESCE((SELECT app_id FROM auth_sessions WHERE is_active = 1 LIMIT 1), 1)
) 
WHERE user_id = 0;

-- Update auth_sessions with user_id
UPDATE auth_sessions 
SET user_id = app_id 
WHERE user_id = 0 AND app_id IS NOT NULL;