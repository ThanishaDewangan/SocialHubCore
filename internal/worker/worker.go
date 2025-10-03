package worker

import (
	"context"
	"time"

	"stories-service/internal/db"
	"stories-service/internal/metrics"

	"go.uber.org/zap"
)

type Worker struct {
	db     *db.DB
	logger *zap.Logger
}

func NewWorker(database *db.DB, logger *zap.Logger) *Worker {
	return &Worker{
		db:     database,
		logger: logger,
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	w.logger.Info("worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return
		case <-ticker.C:
			w.expireStories()
		}
	}
}

func (w *Worker) expireStories() {
	start := time.Now()

	result, err := w.db.Exec(`
		UPDATE stories
		SET deleted_at = NOW()
		WHERE expires_at < NOW()
		  AND deleted_at IS NULL
	`)

	duration := time.Since(start)

	if err != nil {
		w.logger.Error("failed to expire stories", zap.Error(err))
		return
	}

	count, _ := result.RowsAffected()
	if count > 0 {
		metrics.StoriesExpiredTotal.Add(float64(count))
		w.logger.Info("stories expired",
			zap.Int64("count", count),
			zap.Duration("duration", duration))
	}

	metrics.WorkerLatencySeconds.Observe(duration.Seconds())
}
