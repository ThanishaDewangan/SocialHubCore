package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Story struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	AuthorID   uuid.UUID  `json:"author_id" db:"author_id"`
	Text       *string    `json:"text,omitempty" db:"text"`
	MediaKey   *string    `json:"media_key,omitempty" db:"media_key"`
	Visibility string     `json:"visibility" db:"visibility"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

type Follow struct {
	FollowerID uuid.UUID `json:"follower_id" db:"follower_id"`
	FolloweeID uuid.UUID `json:"followee_id" db:"followee_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type StoryView struct {
	StoryID  uuid.UUID `json:"story_id" db:"story_id"`
	ViewerID uuid.UUID `json:"viewer_id" db:"viewer_id"`
	ViewedAt time.Time `json:"viewed_at" db:"viewed_at"`
}

type Reaction struct {
	ID        uuid.UUID `json:"id" db:"id"`
	StoryID   uuid.UUID `json:"story_id" db:"story_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Emoji     string    `json:"emoji" db:"emoji"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type StoryAudience struct {
	StoryID uuid.UUID `json:"story_id" db:"story_id"`
	UserID  uuid.UUID `json:"user_id" db:"user_id"`
}

type CreateStoryRequest struct {
	Text            *string     `json:"text"`
	MediaKey        *string     `json:"media_key"`
	Visibility      string      `json:"visibility" binding:"required,oneof=public friends private"`
	AudienceUserIDs []uuid.UUID `json:"audience_user_ids,omitempty"`
}

type ReactRequest struct {
	Emoji string `json:"emoji" binding:"required,oneof=👍 ❤️ 😂 😮 😢 🔥"`
}

type StatsResponse struct {
	Posted        int                `json:"posted"`
	Views         int                `json:"views"`
	UniqueViewers int                `json:"unique_viewers"`
	Reactions     map[string]int     `json:"reactions"`
}

type PresignedUploadRequest struct {
	ContentType string `json:"content_type" binding:"required"`
	FileName    string `json:"file_name" binding:"required"`
}

type PresignedUploadResponse struct {
	UploadURL string `json:"upload_url"`
	MediaKey  string `json:"media_key"`
}
