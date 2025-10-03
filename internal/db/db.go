package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func NewDB(connString string) (*DB, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

func (db *DB) InitSchema() error {
	schema := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS follows (
		follower_id UUID REFERENCES users(id) ON DELETE CASCADE,
		followee_id UUID REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (follower_id, followee_id)
	);

	CREATE TABLE IF NOT EXISTS stories (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		author_id UUID REFERENCES users(id) ON DELETE CASCADE,
		text TEXT,
		media_key TEXT,
		visibility TEXT CHECK (visibility IN ('public', 'friends', 'private')),
		created_at TIMESTAMPTZ DEFAULT NOW(),
		expires_at TIMESTAMPTZ NOT NULL,
		deleted_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS story_audience (
		story_id UUID REFERENCES stories(id) ON DELETE CASCADE,
		user_id UUID REFERENCES users(id) ON DELETE CASCADE,
		PRIMARY KEY (story_id, user_id)
	);

	CREATE TABLE IF NOT EXISTS story_views (
		story_id UUID REFERENCES stories(id) ON DELETE CASCADE,
		viewer_id UUID REFERENCES users(id) ON DELETE CASCADE,
		viewed_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (story_id, viewer_id)
	);

	CREATE TABLE IF NOT EXISTS reactions (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		story_id UUID REFERENCES stories(id) ON DELETE CASCADE,
		user_id UUID REFERENCES users(id) ON DELETE CASCADE,
		emoji TEXT NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_stories_author_created ON stories(author_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_stories_expires_at ON stories(expires_at);
	CREATE INDEX IF NOT EXISTS idx_stories_active ON stories(expires_at) WHERE deleted_at IS NULL;
	CREATE INDEX IF NOT EXISTS idx_story_views_story ON story_views(story_id);
	CREATE INDEX IF NOT EXISTS idx_reactions_story ON reactions(story_id);
	CREATE INDEX IF NOT EXISTS idx_follows_follower ON follows(follower_id);
	`

	_, err := db.Exec(schema)
	return err
}
