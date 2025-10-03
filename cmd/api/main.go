package main

import (
        "context"
        "log"
        "net/http"
        "os"
        "os/signal"
        "syscall"
        "time"

        "stories-service/internal/cache"
        "stories-service/internal/db"
        "stories-service/internal/handlers"
        "stories-service/internal/middleware"
        "stories-service/internal/storage"
        "stories-service/internal/websocket"
        "stories-service/pkg/logger"

        "github.com/gin-gonic/gin"
        ws "github.com/gorilla/websocket"
        "github.com/joho/godotenv"
        "github.com/prometheus/client_golang/prometheus/promhttp"
        "go.uber.org/zap"
)

var upgrader = ws.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
                return true
        },
}

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

        if err := database.InitSchema(); err != nil {
                logger.Fatal("failed to initialize schema", zap.Error(err))
        }

        var redisCache *cache.Cache
        if os.Getenv("REDIS_ADDR") != "" {
                redisCache, err = cache.NewCache(
                        os.Getenv("REDIS_ADDR"),
                        os.Getenv("REDIS_PASSWORD"),
                        0,
                )
                if err != nil {
                        logger.Warn("failed to connect to redis, continuing without cache", zap.Error(err))
                        redisCache = nil
                } else {
                        defer redisCache.Close()
                }
        } else {
                logger.Warn("REDIS_ADDR not set, running without cache")
        }

        var stor *storage.Storage
        if os.Getenv("MINIO_ENDPOINT") != "" && os.Getenv("MINIO_BUCKET") != "" {
                stor, err = storage.NewStorage(
                        os.Getenv("MINIO_ENDPOINT"),
                        os.Getenv("MINIO_ACCESS_KEY"),
                        os.Getenv("MINIO_SECRET_KEY"),
                        os.Getenv("MINIO_BUCKET"),
                        os.Getenv("MINIO_USE_SSL") == "true",
                )
                if err != nil {
                        logger.Warn("failed to create storage, continuing without presigned uploads", zap.Error(err))
                        stor = nil
                }
        } else {
                logger.Warn("MINIO_ENDPOINT or MINIO_BUCKET not set, running without storage")
        }

        hub := websocket.NewHub()
        go hub.Run()

        jwtSecret := os.Getenv("JWT_SECRET")
        if jwtSecret == "" {
                logger.Fatal("JWT_SECRET not set")
        }

        router := gin.New()
        router.Use(gin.Recovery())
        router.Use(middleware.MetricsMiddleware())

        router.Use(func(c *gin.Context) {
                c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
                c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
                if c.Request.Method == "OPTIONS" {
                        c.AbortWithStatus(204)
                        return
                }
                c.Next()
        })

        router.GET("/", handlers.RootHandler)

        authHandler := handlers.NewAuthHandler(database, jwtSecret, logger)
        router.POST("/signup", authHandler.Signup)
        router.POST("/login", authHandler.Login)

        uploadHandler := handlers.NewUploadHandler(stor, logger)
        storiesHandler := handlers.NewStoriesHandler(database, stor, redisCache, hub, logger)
        socialHandler := handlers.NewSocialHandler(database, logger)
        healthHandler := handlers.NewHealthHandler(database, redisCache, stor)

        router.GET("/healthz", healthHandler.Health)
        router.GET("/metrics", gin.WrapH(promhttp.Handler()))

        authRoutes := router.Group("/")
        authRoutes.Use(middleware.AuthMiddleware(jwtSecret))
        {
                authRoutes.POST("/upload/presigned", uploadHandler.GetPresignedURL)
                authRoutes.POST("/stories", storiesHandler.CreateStory)
                authRoutes.GET("/stories/:id", storiesHandler.GetStory)
                authRoutes.GET("/feed", storiesHandler.GetFeed)
                authRoutes.POST("/stories/:id/view", storiesHandler.ViewStory)
                authRoutes.POST("/stories/:id/reactions", storiesHandler.AddReaction)
                authRoutes.GET("/me/stats", storiesHandler.GetStats)
                authRoutes.POST("/follow/:user_id", socialHandler.Follow)
                authRoutes.DELETE("/follow/:user_id", socialHandler.Unfollow)

                authRoutes.GET("/ws", func(c *gin.Context) {
                        userID, ok := middleware.GetUserID(c)
                        if !ok {
                                c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
                                return
                        }

                        conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
                        if err != nil {
                                logger.Error("websocket upgrade failed", zap.Error(err))
                                return
                        }

                        client := websocket.NewClient(userID, hub, conn)
                        hub.RegisterClient(client)

                        go client.WritePump()
                        go client.ReadPump()
                })
        }

        port := os.Getenv("PORT")
        if port == "" {
                port = "5000"
        }

        srv := &http.Server{
                Addr:    "0.0.0.0:" + port,
                Handler: router,
        }

        go func() {
                logger.Info("server starting", zap.String("port", port))
                if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                        logger.Fatal("server failed", zap.Error(err))
                }
        }()

        quit := make(chan os.Signal, 1)
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
        <-quit

        logger.Info("shutting down server...")

        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := srv.Shutdown(ctx); err != nil {
                logger.Fatal("server forced to shutdown", zap.Error(err))
        }

        logger.Info("server exited")
}
