package images

import "github.com/prometheus/client_golang/prometheus"

var (
	metricBuilt = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_images_variants_built_total",
		Help: "Variant files written.",
	}, []string{"kind", "format"}))
	metricBuildErrors = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_images_build_errors_total",
		Help: "Variant group builds that left at least one file unbuilt.",
	}, []string{"kind"}))
	metricBuildSeconds = register(prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "asparges_images_build_seconds",
		Help:    "Time to build one group: one photo, one kind, its missing files.",
		Buckets: prometheus.ExponentialBuckets(0.25, 2, 10),
	}, []string{"kind"}))
	metricQueue = register(prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_images_queue_length",
		Help: "Photos waiting for their variants to be built.",
	}))
)

// register adds c to the default registry, which the service serves on
// /metrics.
func register[C prometheus.Collector](c C) C {
	prometheus.MustRegister(c)
	return c
}
