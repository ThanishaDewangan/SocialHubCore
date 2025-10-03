package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"stories-service/internal/db"
	"stories-service/internal/worker"
	"stories-service/pkg/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	godotenv.Load()

	logger, err := logger.NewLogger()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	database, err := db.NewDB(os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer database.Close()

	w := worker.NewWorker(database, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down worker...")
	cancel()
}
