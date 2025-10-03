package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RootHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "Stories Service API",
		"version": "1.0.0",
		"status":  "running",
		"endpoints": gin.H{
			"auth": []string{
				"POST /signup",
				"POST /login",
			},
			"stories": []string{
				"POST /stories",
				"GET /stories/:id",
				"GET /feed",
				"POST /stories/:id/view",
				"POST /stories/:id/reactions",
			},
			"social": []string{
				"POST /follow/:user_id",
				"DELETE /follow/:user_id",
			},
			"user": []string{
				"GET /me/stats",
			},
			"upload": []string{
				"POST /upload/presigned",
			},
			"websocket": []string{
				"GET /ws",
			},
			"system": []string{
				"GET /healthz",
				"GET /metrics",
			},
		},
		"documentation": "See README.md for full API documentation",
	})
}
