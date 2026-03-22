package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc/codes"
)

var (
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "headtotails_requests_total",
		Help: "Total number of HTTP requests handled by headtotails.",
	}, []string{"method", "path", "status"})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "headtotails_request_duration_seconds",
		Help:    "Duration of HTTP requests handled by headtotails.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	grpcErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "headtotails_grpc_errors_total",
		Help: "Total number of gRPC errors returned by headscale.",
	}, []string{"grpc_code"})
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration, grpcErrorsTotal)
}

// MetricsHandler returns the Prometheus metrics HTTP handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// prometheusMiddleware records per-request Prometheus metrics.
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		// Use the chi route pattern (not the raw path) so high-cardinality
		// path parameters like device IDs don't explode label cardinality.
		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = r.URL.Path
		}

		status := strconv.Itoa(ww.Status())
		requestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
		requestDuration.WithLabelValues(r.Method, routePattern).Observe(time.Since(start).Seconds())
	})
}

func observeGRPCError(code codes.Code) {
	grpcErrorsTotal.WithLabelValues(code.String()).Inc()
}
