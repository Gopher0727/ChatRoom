-- Active: 1759919848086@@127.0.0.1@5432@chatroom@public

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    nickname VARCHAR(100),
    avatar_url VARCHAR(255),
    status VARCHAR(20) DEFAULT 'offline', -- online, offline
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 群组表
CREATE TABLE IF NOT EXISTS GROUPS (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    avatar_url VARCHAR(255),
    owner_id INTEGER NOT NULL REFERENCES users (id),
    invite_code VARCHAR(50) UNIQUE NOT NULL, -- 邀请码，用于加入群组
    max_members INTEGER DEFAULT 10000,
    member_count INTEGER DEFAULT 1,
    status VARCHAR(20) DEFAULT 'active', -- active, archived
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 群组成员表
CREATE TABLE IF NOT EXISTS group_members (
    id SERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES GROUPS (id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ROLE VARCHAR(20) DEFAULT 'member', -- admin, member
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_read_msg_id BIGINT DEFAULT 0,
    deleted_at TIMESTAMP,
    UNIQUE (group_id, user_id)
);

-- 消息表
CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES GROUPS (id) ON DELETE CASCADE,
    sender_id INTEGER NOT NULL REFERENCES users (id),
    CONTENT TEXT NOT NULL,
    msg_type VARCHAR(20) DEFAULT 'text', -- text, image, file, system
    sequence_id BIGINT NOT NULL, -- 群内消息序列号，保证消息有序性
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 创建索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

CREATE INDEX IF NOT EXISTS idx_groups_owner_id ON GROUPS (owner_id);

CREATE INDEX IF NOT EXISTS idx_groups_invite_code ON GROUPS (invite_code);

CREATE INDEX IF NOT EXISTS idx_group_members_group_id ON group_members (group_id);

CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members (user_id);

CREATE INDEX IF NOT EXISTS idx_messages_group_id ON messages (group_id);

CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages (sender_id);

CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages (created_at);

CREATE INDEX IF NOT EXISTS idx_messages_sequence_id ON messages (group_id, sequence_id);