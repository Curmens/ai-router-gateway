package telemetry

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "router_requests_total",
			Help: "Total number of chat completion requests processed",
		},
		[]string{"provider", "model", "status", "routing_type"},
	)

	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "router_request_duration_seconds",
			Help:    "Request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider", "model", "status", "routing_type"},
	)

	TokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "router_tokens_consumed_total",
			Help: "Total prompt and completion tokens consumed",
		},
		[]string{"provider", "model", "token_type"},
	)

	CostTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "router_cost_usd_total",
			Help: "Cumulative cost in USD",
		},
		[]string{"provider", "model", "api_key"},
	)

	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "router_cache_hits_total",
			Help: "Total number of response cache hits and misses",
		},
		[]string{"model", "status"},
	)

	FailoversTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "router_failovers_total",
			Help: "Total number of provider failover occurrences",
		},
		[]string{"from_provider", "to_provider", "model"},
	)

	Tracer trace.Tracer
)

func InitTelemetry(cfg *config.TelemetryConfig) {
	prometheus.MustRegister(RequestsTotal)
	prometheus.MustRegister(RequestDuration)
	prometheus.MustRegister(TokensTotal)
	prometheus.MustRegister(CostTotal)
	prometheus.MustRegister(CacheHitsTotal)
	prometheus.MustRegister(FailoversTotal)

	if cfg.OpenTelemetry.Enabled {
		tp, err := initTracerProvider(cfg.OpenTelemetry.CollectorURL)
		if err != nil {
			logger.Log.Error("Failed to initialize OpenTelemetry TracerProvider", zap.Error(err))
		} else {
			otel.SetTracerProvider(tp)
			Tracer = otel.Tracer("ai-router")
			logger.Log.Info("OpenTelemetry initialized", zap.String("collector_url", cfg.OpenTelemetry.CollectorURL))
			return
		}
	}

	Tracer = otel.Tracer("ai-router-noop")
}

func initTracerProvider(url string) (*sdktrace.TracerProvider, error) {
	r, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("ai-router"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(r),
	)
	return tp, nil
}

func HTTPHandler() http.Handler {
	return promhttp.Handler()
}
