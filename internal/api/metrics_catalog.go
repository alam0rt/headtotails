package api

import "slices"

// MetricDoc describes a custom Prometheus metric exposed by headtotails.
type MetricDoc struct {
	Name   string
	Type   string
	Help   string
	Labels []string
	Notes  string
}

// customMetricDocs is the source of truth for all custom headtotails metrics.
//
// metricdoc:start
var customMetricDocs = []MetricDoc{
	{
		Name: "headtotails_requests_total",
		Type: "counter",
		Help: "Total number of HTTP requests handled by headtotails.",
		Labels: []string{
			"method",
			"path",
			"status",
		},
		Notes: "path uses the chi route pattern when available to avoid high-cardinality labels from dynamic IDs.",
	},
	{
		Name: "headtotails_request_duration_seconds",
		Type: "histogram",
		Help: "Duration of HTTP requests handled by headtotails.",
		Labels: []string{
			"method",
			"path",
		},
		Notes: "uses default Prometheus histogram buckets (prometheus.DefBuckets).",
	},
	{
		Name: "headtotails_grpc_errors_total",
		Type: "counter",
		Help: "Total number of gRPC errors returned by headscale.",
		Labels: []string{
			"grpc_code",
		},
		Notes: "increments when handler paths call writeGRPCError() after upstream headscale failures.",
	},
}

// metricdoc:end

func metricDocByName(name string) MetricDoc {
	for _, doc := range customMetricDocs {
		if doc.Name == name {
			return doc
		}
	}
	panic("unknown metric doc: " + name)
}

// CustomMetricDocs returns the custom metrics metadata for docs generation.
func CustomMetricDocs() []MetricDoc {
	return slices.Clone(customMetricDocs)
}
