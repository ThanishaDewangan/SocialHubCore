package cache

import (
        "context"
        "encoding/json"
        "fmt"
        "time"

        "github.com/google/uuid"
        "github.com/redis/go-redis/v9"
)

type Cache struct {
        client *redis.Client
}

func NewCache(addr, password string, db int) (*Cache, error) {
        client := redis.NewClient(&redis.Options{
                Addr:     addr,
                Password: password,
                DB:       db,
        })

        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := client.Ping(ctx).Err(); err != nil {
                return nil, fmt.Errorf("failed to ping redis: %w", err)
        }

        return &Cache{client: client}, nil
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
        if c == nil || c.client == nil {
                return fmt.Errorf("cache not available")
        }
        data, err := json.Marshal(value)
        if err != nil {
                return err
        }
        return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
        if c == nil || c.client == nil {
                return fmt.Errorf("cache not available")
        }
        data, err := c.client.Get(ctx, key).Bytes()
        if err != nil {
                return err
        }
        return json.Unmarshal(data, dest)
}

func (c *Cache) Delete(ctx context.Context, key string) error {
        if c == nil || c.client == nil {
                return fmt.Errorf("cache not available")
        }
        return c.client.Del(ctx, key).Err()
}

func (c *Cache) GetFollowees(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
        var followees []uuid.UUID
        key := fmt.Sprintf("followees:%s", userID.String())
        err := c.Get(ctx, key, &followees)
        return followees, err
}

func (c *Cache) SetFollowees(ctx context.Context, userID uuid.UUID, followees []uuid.UUID) error {
        key := fmt.Sprintf("followees:%s", userID.String())
        return c.Set(ctx, key, followees, 5*time.Minute)
}

func (c *Cache) CheckRateLimit(ctx context.Context, userID uuid.UUID, action string, limit int, window time.Duration) (bool, error) {
        if c == nil || c.client == nil {
                return true, nil
        }
        key := fmt.Sprintf("ratelimit:%s:%s", action, userID.String())
        
        count, err := c.client.Incr(ctx, key).Result()
        if err != nil {
                return true, nil
        }

        if count == 1 {
                c.client.Expire(ctx, key, window)
        }

        return count <= int64(limit), nil
}

func (c *Cache) Close() error {
        if c == nil || c.client == nil {
                return nil
        }
        return c.client.Close()
}
