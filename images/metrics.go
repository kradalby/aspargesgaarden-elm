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
	metricServeSeconds = register(prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "asparges_images_serve_seconds",
		Help:    "Time to serve one variant: hit = file on disk, built = built for this request.",
		Buckets: prometheus.ExponentialBuckets(0.0005, 4, 10),
	}, []string{"result"}))
	metricFiles = register(prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "asparges_images_files",
		Help: "Files per image folder at the last reconcile.",
	}, []string{"dir"}))
	metricBytes = register(prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "asparges_images_bytes",
		Help: "Bytes per image folder at the last reconcile.",
	}, []string{"dir"}))
	metricOrphansRemoved = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_images_orphans_removed_total",
		Help: "Files and folders reconcile deleted because nothing references them.",
	}, []string{"dir"}))
	metricMissingOriginals = register(prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_images_missing_originals",
		Help: "Photos in the database whose original file is gone. Needs a restore.",
	}))
	metricQueue = register(prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "asparges_images_queue_length",
		Help: "Photos waiting for their variants to be built.",
	}))
)

// register adds c to the default registry, which the skeleton serves on
// /metrics. Not promauto: that package is not in the vendor tree yet.
func register[C prometheus.Collector](c C) C {
	prometheus.MustRegister(c)
	return c
}
