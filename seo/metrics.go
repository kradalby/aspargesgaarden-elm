package seo

import "github.com/prometheus/client_golang/prometheus"

// register adds c to the default registry, which the service serves on
// /metrics, as the images package does.
func register[C prometheus.Collector](c C) C {
	prometheus.MustRegister(c)
	return c
}
