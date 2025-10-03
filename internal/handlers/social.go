package handlers

import (
	"net/http"

	"stories-service/internal/db"
	"stories-service/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SocialHandler struct {
	db     *db.DB
	logger *zap.Logger
}

func NewSocialHandler(database *db.DB, logger *zap.Logger) *SocialHandler {
	return &SocialHandler{
		db:     database,
		logger: logger,
	}
}

func (h *SocialHandler) Follow(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	followeeIDStr := c.Param("user_id")
	followeeID, err := uuid.Parse(followeeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if userID == followeeID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot follow yourself"})
		return
	}

	_, err = h.db.Exec(`
		INSERT INTO follows (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT (follower_id, followee_id) DO NOTHING
	`, userID, followeeID)

	if err != nil {
		h.logger.Error("failed to follow user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	h.logger.Info("user followed",
		zap.String("follower_id", userID.String()),
		zap.String("followee_id", followeeID.String()))

	c.JSON(http.StatusOK, gin.H{"message": "followed successfully"})
}

func (h *SocialHandler) Unfollow(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	followeeIDStr := c.Param("user_id")
	followeeID, err := uuid.Parse(followeeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	_, err = h.db.Exec(`
		DELETE FROM follows
		WHERE follower_id = $1 AND followee_id = $2
	`, userID, followeeID)

	if err != nil {
		h.logger.Error("failed to unfollow user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	h.logger.Info("user unfollowed",
		zap.String("follower_id", userID.String()),
		zap.String("followee_id", followeeID.String()))

	c.JSON(http.StatusOK, gin.H{"message": "unfollowed successfully"})
}
