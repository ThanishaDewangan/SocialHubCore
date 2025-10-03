package handlers

import (
	"context"
	"fmt"
	"net/http"

	"stories-service/internal/models"
	"stories-service/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UploadHandler struct {
	storage *storage.Storage
	logger  *zap.Logger
}

func NewUploadHandler(stor *storage.Storage, logger *zap.Logger) *UploadHandler {
	return &UploadHandler{
		storage: stor,
		logger:  logger,
	}
}

func (h *UploadHandler) GetPresignedURL(c *gin.Context) {
	var req models.PresignedUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mediaKey := fmt.Sprintf("uploads/%s-%s", uuid.New().String(), req.FileName)

	url, err := h.storage.GeneratePresignedUploadURL(context.Background(), mediaKey, req.ContentType, 10*1024*1024)
	if err != nil {
		h.logger.Error("failed to generate presigned URL", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("presigned URL generated", zap.String("media_key", mediaKey))

	c.JSON(http.StatusOK, models.PresignedUploadResponse{
		UploadURL: url,
		MediaKey:  mediaKey,
	})
}
