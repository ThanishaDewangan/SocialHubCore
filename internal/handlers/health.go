package handlers

import (
	"context"
	"net/http"
	"time"

	"stories-service/internal/cache"
	"stories-service/internal/db"
	"stories-service/internal/storage"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db      *db.DB
	cache   *cache.Cache
	storage *storage.Storage
}

func NewHealthHandler(database *db.DB, cach *cache.Cache, stor *storage.Storage) *HealthHandler {
	return &HealthHandler{
		db:      database,
		cache:   cach,
		storage: stor,
	}
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checks := map[string]string{
		"database": "ok",
		"redis":    "ok",
		"storage":  "ok",
	}

	if err := h.db.Ping(); err != nil {
		checks["database"] = err.Error()
	}

	testKey := "health_check"
	if err := h.cache.Set(ctx, testKey, "test", time.Second); err != nil {
		checks["redis"] = err.Error()
	}
	h.cache.Delete(ctx, testKey)

	healthy := true
	for _, status := range checks {
		if status != "ok" {
			healthy = false
			break
		}
	}

	statusCode := http.StatusOK
	if !healthy {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status": checks,
		"healthy": healthy,
	})
}
