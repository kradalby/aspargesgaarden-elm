package server

import "github.com/prometheus/client_golang/prometheus"

// Public requests as the service answered them, for the dashboard's error
// and latency graphs.
var (
	metricRequests = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_http_requests_total",
		Help: "Public requests answered, by status code.",
	}, []string{"code"}))
	metricRequestSeconds = register(prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "asparges_http_request_duration_seconds",
		Help:    "Time to answer a public request.",
		Buckets: prometheus.DefBuckets,
	}, nil))
)

// register adds c to the default registry, which the service serves on
// /metrics, as the images package does.
func register[C prometheus.Collector](c C) C {
	prometheus.MustRegister(c)
	return c
}
