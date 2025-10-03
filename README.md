# Stories Service Backend

A production-grade backend service for ephemeral Stories - short text posts with optional media that expire after 24 hours.

## Architecture Overview

### Core Components

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Client    │────▶│   API Server │────▶│  PostgreSQL │
│             │     │   (Port 5000)│     │  Database   │
└─────────────┘     └──────────────┘     └─────────────┘
       │                    │                     │
       │                    ▼                     │
       │            ┌──────────────┐             │
       │            │  Redis Cache │             │
       │            │ Rate Limiting│             │
       │            └──────────────┘             │
       │                    │                     │
       │                    ▼                     │
       │            ┌──────────────┐             │
       ▼            │    MinIO     │             │
┌─────────────┐     │ Object Store │             │
│  WebSocket  │     └──────────────┘             │
│   Events    │                                   │
└─────────────┘             │                     │
                            ▼                     │
                    ┌──────────────┐             │
                    │    Worker    │─────────────┘
                    │  (Expiration)│
                    └──────────────┘
```

### Key Features

- **JWT Authentication**: Secure email/password signup and login with bcrypt password hashing
- **Story Management**: Create, view, and manage ephemeral stories with 24-hour expiration
- **Visibility Controls**: Public, friends-only, and private story visibility
- **Social Graph**: Follow/unfollow users with permission-based feed generation
- **Real-time Events**: WebSocket notifications for story views and reactions
- **Media Uploads**: Presigned S3/MinIO URLs for direct client-to-storage uploads
- **Background Worker**: Automatic story expiration after 24 hours with soft deletion
- **Observability**: Prometheus metrics, structured JSON logging, health checks
- **Graceful Degradation**: Continues operating without Redis cache or object storage
- **Rate Limiting**: Token bucket rate limiting for story creation and reactions

## Database Schema

### Tables

- **users**: User accounts with email and password hash
- **stories**: Story content with visibility, expiration, and soft deletion
- **follows**: Social graph for friend relationships
- **story_views**: Idempotent view tracking
- **reactions**: Emoji reactions (👍 ❤️ 😂 😮 😢 🔥)
- **story_audience**: Optional explicit audience for friends-only stories

### Indexes

- `stories(author_id, created_at DESC)` - Author's stories ordered by recency
- `stories(expires_at)` - Story expiration lookup
- `stories(expires_at) WHERE deleted_at IS NULL` - Active stories
- `follows(follower_id)` - Follow graph traversal
- `story_views(story_id)` - View counts
- `reactions(story_id)` - Reaction counts

## API Endpoints

### Authentication

- `POST /signup` - Create new user account
  ```json
  {
    "email": "user@example.com",
    "password": "password123"
  }
  ```

- `POST /login` - Authenticate and get JWT token
  ```json
  {
    "email": "user@example.com",
    "password": "password123"
  }
  ```

### Stories

- `POST /stories` - Create a new story (20/min rate limit)
  ```json
  {
    "text": "Hello world!",
    "media_key": "uploads/abc123.jpg",
    "visibility": "public|friends|private",
    "audience_user_ids": ["uuid1", "uuid2"]
  }
  ```

- `GET /stories/:id` - Get story by ID (permission check)
- `GET /feed` - Get paginated feed of visible stories
- `POST /stories/:id/view` - Record story view (idempotent)
- `POST /stories/:id/reactions` - Add emoji reaction (60/min rate limit)
  ```json
  {
    "emoji": "❤️"
  }
  ```

### Social

- `POST /follow/:user_id` - Follow a user
- `DELETE /follow/:user_id` - Unfollow a user

### User Stats

- `GET /me/stats` - Get stats for last 7 days
  ```json
  {
    "posted": 15,
    "views": 234,
    "unique_viewers": 67,
    "reactions": {
      "❤️": 45,
      "👍": 32,
      "😂": 12
    }
  }
  ```

### Media Upload

- `POST /upload/presigned` - Get presigned upload URL
  ```json
  {
    "content_type": "image/jpeg",
    "file_name": "photo.jpg"
  }
  ```

### System

- `GET /healthz` - Health check (DB, Redis, Storage)
- `GET /metrics` - Prometheus metrics
- `GET /ws` - WebSocket connection for real-time events

## Quick Start

### Prerequisites

- Go 1.23+
- PostgreSQL 15+
- Redis 7+ (optional, degrades gracefully)
- MinIO or S3 (optional, degrades gracefully)

### Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
DATABASE_URL=postgres://user:pass@localhost:5432/stories?sslmode=disable
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
JWT_SECRET=your-secret-key-change-in-production
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=stories
MINIO_USE_SSL=false
PORT=5000
```

### Run with Docker Compose

```bash
# Start all services (API, Worker, PostgreSQL, Redis, MinIO)
make docker-up

# Or manually
docker-compose up --build

# Stop and clean up
make docker-down
```

### Run Locally

```bash
# Install dependencies
go mod download

# Run database migrations (auto-runs on startup)
# Schema is created automatically

# Seed test data (optional)
make seed

# Run API server
make dev

# Run worker (in separate terminal)
make worker

# Run tests
make test

# Lint code
make lint
```

## Walkthrough

### 1. Sign Up & Login

```bash
# Sign up
curl -X POST http://localhost:5000/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"password123"}'

# Returns: {"token":"eyJ...", "user_id":"...", "email":"alice@example.com"}

# Login
curl -X POST http://localhost:5000/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"password123"}'
```

### 2. Get Presigned Upload URL

```bash
TOKEN="your-jwt-token"

curl -X POST http://localhost:5000/upload/presigned \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content_type":"image/jpeg","file_name":"photo.jpg"}'

# Returns: {"upload_url":"https://...", "media_key":"uploads/..."}
```

### 3. Upload Media (Direct to Storage)

```bash
# Upload file directly to presigned URL
curl -X PUT "presigned-url" \
  --data-binary @photo.jpg \
  -H "Content-Type: image/jpeg"
```

### 4. Create a Story

```bash
# Public story
curl -X POST http://localhost:5000/stories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Beautiful sunset! 🌅",
    "media_key": "uploads/abc123.jpg",
    "visibility": "public"
  }'

# Friends-only story
curl -X POST http://localhost:5000/stories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Private moment",
    "visibility": "friends"
  }'
```

### 5. Follow a User & View Feed

```bash
# Follow user
curl -X POST http://localhost:5000/follow/USER_ID \
  -H "Authorization: Bearer $TOKEN"

# Get feed (shows public stories + friends' stories)
curl -X GET http://localhost:5000/feed \
  -H "Authorization: Bearer $TOKEN"
```

### 6. View & React to Stories

```bash
# View story (idempotent)
curl -X POST http://localhost:5000/stories/STORY_ID/view \
  -H "Authorization: Bearer $TOKEN"

# Add reaction
curl -X POST http://localhost:5000/stories/STORY_ID/reactions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"emoji":"❤️"}'
```

### 7. Real-time Events (WebSocket)

```javascript
const ws = new WebSocket('ws://localhost:5000/ws', {
  headers: { 'Authorization': `Bearer ${token}` }
});

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  if (data.type === 'story.viewed') {
    console.log(`Story ${data.payload.story_id} viewed by ${data.payload.viewer_id}`);
  }
  
  if (data.type === 'story.reacted') {
    console.log(`Story ${data.payload.story_id} reacted with ${data.payload.emoji}`);
  }
};
```

### 8. Worker Expiration

The worker runs every minute and soft-deletes expired stories:

```bash
# Worker logs
{"level":"info","msg":"stories expired","count":5,"duration":"12.3ms"}
```

### 9. Observability

```bash
# Health check
curl http://localhost:5000/healthz

# Prometheus metrics
curl http://localhost:5000/metrics

# Key metrics:
# - http_requests_total{route,method,code}
# - stories_created_total
# - story_views_total
# - reactions_total
# - stories_expired_total
# - worker_latency_seconds
```

## Production Deployment

### Docker Build

```bash
# Build API
docker build --target api -t stories-api .

# Build Worker
docker build --target worker -t stories-worker .
```

### Environment Configuration

Set the following environment variables in production:

- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_ADDR` - Redis address (optional, degrades gracefully)
- `JWT_SECRET` - Strong secret key for JWT signing
- `MINIO_ENDPOINT` - S3/MinIO endpoint (optional, degrades gracefully)
- `MINIO_BUCKET` - Storage bucket name
- `PORT` - API server port (default: 5000)

### Graceful Degradation

The service continues operating even if:
- **Redis is unavailable**: Rate limiting disabled, no caching
- **MinIO/S3 is unavailable**: Presigned upload URLs disabled
- Both services can be added later without code changes

### Security Considerations

1. **JWT Secret**: Use strong, randomly generated secret in production
2. **Password Hashing**: bcrypt with default cost (10 rounds)
3. **CORS**: Configure allowed origins in production
4. **Rate Limiting**: Adjust limits based on your use case
5. **HTTPS**: Always use HTTPS in production
6. **Database**: Use connection pooling and prepared statements

## Monitoring

### Prometheus Metrics

```
# Request metrics
http_requests_total{route="/stories",method="POST",code="201"} 1234

# Business metrics
stories_created_total 5678
story_views_total 12345
reactions_total 3456

# Worker metrics
stories_expired_total 234
worker_latency_seconds_bucket{le="0.1"} 56
```

### Structured Logging

All logs are JSON formatted with fields:
- `level`: info, warn, error
- `timestamp`: ISO 8601 format
- `msg`: Log message
- `user_id`, `story_id`, etc.: Context fields

## Testing

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...

# Integration tests (requires database)
go test -tags=integration ./...
```

## Architecture Decisions

### Visibility Model
- **Public**: Visible to all users
- **Friends**: Visible only to followers (one-way follow model)
- **Private**: Visible only to the author

### Expiration Model
- Stories expire 24 hours after creation
- Soft deletion (sets `deleted_at` timestamp)
- Worker runs every minute to clean up
- Configurable expiration window

### Caching Strategy
- Feed results cached for 30 seconds
- Followee lists cached for 5 minutes
- Cache misses don't block requests
- Rate limits bypass when cache unavailable

### Real-time Events
- WebSocket hub with per-user channels
- Events sent only to story author
- Automatic reconnection handling
- Ping/pong keep-alive

## Performance

### Database Optimization
- Indexed queries for feeds and views
- Partial indexes for active stories
- Connection pooling (max 25 connections)
- Query result limits (50 stories per feed)

### Caching
- Redis for frequently accessed data
- TTL-based cache invalidation
- Cache warming on startup (optional)

### Rate Limiting
- Token bucket algorithm
- Per-user limits (not global)
- 20 stories/min per user
- 60 reactions/min per user

## Future Enhancements

- [ ] Full-text search with Elasticsearch
- [ ] Pagination cursors for large feeds
- [ ] Story replies/threads
- [ ] User blocking/muting
- [ ] Analytics dashboard
- [ ] CDN integration for media
- [ ] Multi-region deployment
- [ ] GraphQL API
- [ ] Mobile push notifications
- [ ] Story highlights (persist beyond 24h)

## License

MIT License - see LICENSE file for details
