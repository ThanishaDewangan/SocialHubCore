package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"stories-service/internal/cache"
	"stories-service/internal/db"
	"stories-service/internal/metrics"
	"stories-service/internal/middleware"
	"stories-service/internal/models"
	"stories-service/internal/storage"
	"stories-service/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type StoriesHandler struct {
	db      *db.DB
	storage *storage.Storage
	cache   *cache.Cache
	hub     *websocket.Hub
	logger  *zap.Logger
}

func NewStoriesHandler(database *db.DB, stor *storage.Storage, cach *cache.Cache, hub *websocket.Hub, logger *zap.Logger) *StoriesHandler {
	return &StoriesHandler{
		db:      database,
		storage: stor,
		cache:   cach,
		hub:     hub,
		logger:  logger,
	}
}

func (h *StoriesHandler) CreateStory(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	allowed, err := h.cache.CheckRateLimit(c.Request.Context(), userID, "create_story", 20, time.Minute)
	if err != nil {
		h.logger.Error("rate limit check failed", zap.Error(err))
	}
	if !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}

	var req models.CreateStoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Text == nil && req.MediaKey == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text or media_key required"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		h.logger.Error("failed to begin transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer tx.Rollback()

	var storyID uuid.UUID
	expiresAt := time.Now().Add(24 * time.Hour)

	err = tx.QueryRow(`
		INSERT INTO stories (author_id, text, media_key, visibility, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, userID, req.Text, req.MediaKey, req.Visibility, expiresAt).Scan(&storyID)

	if err != nil {
		h.logger.Error("failed to create story", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if req.Visibility == "friends" && len(req.AudienceUserIDs) > 0 {
		for _, audienceUserID := range req.AudienceUserIDs {
			_, err = tx.Exec(`
				INSERT INTO story_audience (story_id, user_id) VALUES ($1, $2)
			`, storyID, audienceUserID)
			if err != nil {
				h.logger.Error("failed to insert audience", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
		}
	}

	if err = tx.Commit(); err != nil {
		h.logger.Error("failed to commit transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	metrics.StoriesCreatedTotal.Inc()
	h.logger.Info("story created",
		zap.String("story_id", storyID.String()),
		zap.String("author_id", userID.String()),
		zap.String("visibility", req.Visibility))

	story := models.Story{
		ID:         storyID,
		AuthorID:   userID,
		Text:       req.Text,
		MediaKey:   req.MediaKey,
		Visibility: req.Visibility,
		CreatedAt:  time.Now(),
		ExpiresAt:  expiresAt,
	}

	c.JSON(http.StatusCreated, story)
}

func (h *StoriesHandler) GetStory(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	storyIDStr := c.Param("id")
	storyID, err := uuid.Parse(storyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid story id"})
		return
	}

	var story models.Story
	err = h.db.QueryRow(`
		SELECT id, author_id, text, media_key, visibility, created_at, expires_at, deleted_at
		FROM stories
		WHERE id = $1 AND deleted_at IS NULL
	`, storyID).Scan(&story.ID, &story.AuthorID, &story.Text, &story.MediaKey,
		&story.Visibility, &story.CreatedAt, &story.ExpiresAt, &story.DeletedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "story not found"})
		return
	}
	if err != nil {
		h.logger.Error("failed to get story", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	canView := false
	if story.AuthorID == userID {
		canView = true
	} else if story.Visibility == "public" {
		canView = true
	} else if story.Visibility == "friends" {
		var isFollowing bool
		err = h.db.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)
		`, userID, story.AuthorID).Scan(&isFollowing)
		if err == nil && isFollowing {
			canView = true
		}
	}

	if !canView {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, story)
}

func (h *StoriesHandler) GetFeed(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	cacheKey := fmt.Sprintf("feed:%s", userID.String())
	var stories []models.Story
	err := h.cache.Get(c.Request.Context(), cacheKey, &stories)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"stories": stories, "cached": true})
		return
	}

	rows, err := h.db.Query(`
		SELECT DISTINCT s.id, s.author_id, s.text, s.media_key, s.visibility, s.created_at, s.expires_at
		FROM stories s
		LEFT JOIN follows f ON s.author_id = f.followee_id AND f.follower_id = $1
		WHERE s.deleted_at IS NULL
		  AND s.expires_at > NOW()
		  AND (
		    s.visibility = 'public'
		    OR (s.visibility = 'friends' AND f.follower_id IS NOT NULL)
		    OR s.author_id = $1
		  )
		ORDER BY s.created_at DESC
		LIMIT 50
	`, userID)

	if err != nil {
		h.logger.Error("failed to get feed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	defer rows.Close()

	stories = []models.Story{}
	for rows.Next() {
		var story models.Story
		err := rows.Scan(&story.ID, &story.AuthorID, &story.Text, &story.MediaKey,
			&story.Visibility, &story.CreatedAt, &story.ExpiresAt)
		if err != nil {
			h.logger.Error("failed to scan story", zap.Error(err))
			continue
		}
		stories = append(stories, story)
	}

	h.cache.Set(c.Request.Context(), cacheKey, stories, 30*time.Second)

	c.JSON(http.StatusOK, gin.H{"stories": stories})
}

func (h *StoriesHandler) ViewStory(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	storyIDStr := c.Param("id")
	storyID, err := uuid.Parse(storyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid story id"})
		return
	}

	var authorID uuid.UUID
	err = h.db.QueryRow("SELECT author_id FROM stories WHERE id = $1", storyID).Scan(&authorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "story not found"})
		return
	}

	_, err = h.db.Exec(`
		INSERT INTO story_views (story_id, viewer_id, viewed_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (story_id, viewer_id) DO NOTHING
	`, storyID, userID)

	if err != nil {
		h.logger.Error("failed to record view", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	metrics.StoryViewsTotal.Inc()

	h.hub.SendToUser(authorID, websocket.Event{
		Type: "story.viewed",
		Payload: websocket.ViewEvent{
			StoryID:  storyID,
			ViewerID: userID,
			ViewedAt: time.Now().Format(time.RFC3339),
		},
	})

	h.logger.Info("story viewed",
		zap.String("story_id", storyID.String()),
		zap.String("viewer_id", userID.String()))

	c.JSON(http.StatusOK, gin.H{"message": "view recorded"})
}

func (h *StoriesHandler) AddReaction(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	allowed, err := h.cache.CheckRateLimit(c.Request.Context(), userID, "react", 60, time.Minute)
	if err != nil {
		h.logger.Error("rate limit check failed", zap.Error(err))
	}
	if !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
		return
	}

	storyIDStr := c.Param("id")
	storyID, err := uuid.Parse(storyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid story id"})
		return
	}

	var req models.ReactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var authorID uuid.UUID
	err = h.db.QueryRow("SELECT author_id FROM stories WHERE id = $1", storyID).Scan(&authorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "story not found"})
		return
	}

	var reactionID uuid.UUID
	err = h.db.QueryRow(`
		INSERT INTO reactions (story_id, user_id, emoji)
		VALUES ($1, $2, $3)
		RETURNING id
	`, storyID, userID, req.Emoji).Scan(&reactionID)

	if err != nil {
		h.logger.Error("failed to add reaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	metrics.ReactionsTotal.Inc()

	h.hub.SendToUser(authorID, websocket.Event{
		Type: "story.reacted",
		Payload: websocket.ReactionEvent{
			StoryID: storyID,
			UserID:  userID,
			Emoji:   req.Emoji,
		},
	})

	h.logger.Info("reaction added",
		zap.String("story_id", storyID.String()),
		zap.String("user_id", userID.String()),
		zap.String("emoji", req.Emoji))

	c.JSON(http.StatusCreated, gin.H{"id": reactionID, "emoji": req.Emoji})
}

func (h *StoriesHandler) GetStats(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var posted int
	h.db.QueryRow(`
		SELECT COUNT(*) FROM stories 
		WHERE author_id = $1 AND created_at > NOW() - INTERVAL '7 days'
	`, userID).Scan(&posted)

	var views int
	h.db.QueryRow(`
		SELECT COUNT(*) FROM story_views sv
		JOIN stories s ON sv.story_id = s.id
		WHERE s.author_id = $1 AND sv.viewed_at > NOW() - INTERVAL '7 days'
	`, userID).Scan(&views)

	var uniqueViewers int
	h.db.QueryRow(`
		SELECT COUNT(DISTINCT sv.viewer_id) FROM story_views sv
		JOIN stories s ON sv.story_id = s.id
		WHERE s.author_id = $1 AND sv.viewed_at > NOW() - INTERVAL '7 days'
	`, userID).Scan(&uniqueViewers)

	rows, err := h.db.Query(`
		SELECT r.emoji, COUNT(*) as count
		FROM reactions r
		JOIN stories s ON r.story_id = s.id
		WHERE s.author_id = $1 AND r.created_at > NOW() - INTERVAL '7 days'
		GROUP BY r.emoji
	`, userID)

	reactions := make(map[string]int)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var emoji string
			var count int
			rows.Scan(&emoji, &count)
			reactions[emoji] = count
		}
	}

	c.JSON(http.StatusOK, models.StatsResponse{
		Posted:        posted,
		Views:         views,
		UniqueViewers: uniqueViewers,
		Reactions:     reactions,
	})
}
