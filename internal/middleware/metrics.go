package middleware

import (
	"strconv"

	"stories-service/internal/metrics"

	"github.com/gin-gonic/gin"
)

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		
		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		
		metrics.HTTPRequestsTotal.WithLabelValues(
			route,
			c.Request.Method,
			strconv.Itoa(c.Writer.Status()),
		).Inc()
	}
}
