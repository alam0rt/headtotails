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
	requestsTotalDoc   = metricDocByName("headtotails_requests_total")
	requestDurationDoc = metricDocByName("headtotails_request_duration_seconds")
	grpcErrorsTotalDoc = metricDocByName("headtotails_grpc_errors_total")

	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: requestsTotalDoc.Name,
		Help: requestsTotalDoc.Help,
	}, requestsTotalDoc.Labels)

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    requestDurationDoc.Name,
		Help:    requestDurationDoc.Help,
		Buckets: prometheus.DefBuckets,
	}, requestDurationDoc.Labels)

	grpcErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: grpcErrorsTotalDoc.Name,
		Help: grpcErrorsTotalDoc.Help,
	}, grpcErrorsTotalDoc.Labels)
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
