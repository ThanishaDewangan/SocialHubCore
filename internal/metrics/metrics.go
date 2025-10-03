package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"route", "method", "code"},
	)

	StoriesCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "stories_created_total",
			Help: "Total number of stories created",
		},
	)

	StoryViewsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "story_views_total",
			Help: "Total number of story views",
		},
	)

	ReactionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "reactions_total",
			Help: "Total number of reactions",
		},
	)

	StoriesExpiredTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "stories_expired_total",
			Help: "Total number of stories expired by worker",
		},
	)

	WorkerLatencySeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "worker_latency_seconds",
			Help:    "Worker execution latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)
