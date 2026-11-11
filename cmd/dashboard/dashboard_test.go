package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// model is the part of the emitted JSON Grafana reads that these tests check.
type model struct {
	UID        string `json:"uid"`
	Templating struct {
		List []struct {
			Name       string `json:"name"`
			Definition string `json:"definition"`
		} `json:"list"`
	} `json:"templating"`
	Panels []struct {
		Type        string `json:"type"`
		Title       string `json:"title"`
		FieldConfig struct {
			Defaults struct {
				Unit string `json:"unit"`
			} `json:"defaults"`
		} `json:"fieldConfig"`
		Options struct {
			Legend struct {
				ShowLegend bool `json:"showLegend"`
			} `json:"legend"`
		} `json:"options"`
		Targets []struct {
			Expr         string `json:"expr"`
			LegendFormat string `json:"legendFormat"`
		} `json:"targets"`
	} `json:"panels"`
}

func emitted(t *testing.T) ([]byte, model) {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, run(&out))

	var m model
	require.NoError(t, json.Unmarshal(out.Bytes(), &m))

	return out.Bytes(), m
}

// Provisioning files the dashboard by uid, and a rebuild must not churn it.
func TestDashboardIsStable(t *testing.T) {
	first, m := emitted(t)
	second, _ := emitted(t)

	require.Equal(t, "asparges", m.UID)
	require.Equal(t, string(first), string(second))
}

func TestPanelsHaveLegendAndUnit(t *testing.T) {
	_, m := emitted(t)

	for _, p := range m.Panels {
		if p.Type == "row" {
			continue
		}

		require.True(t, p.Options.Legend.ShowLegend, p.Title)
		require.NotEmpty(t, p.FieldConfig.Defaults.Unit, p.Title)
		require.NotEmpty(t, p.Targets, p.Title)

		for _, target := range p.Targets {
			require.NotEmpty(t, target.LegendFormat, "%s: %s", p.Title, target.Expr)
		}
	}
}

// external are charted but come from Prometheus and the host's HTTPS probe
// rather than this binary, so no source here declares them.
var external = map[string]bool{
	"up":                     true,
	"probe_success":          true,
	"probe_duration_seconds": true,
}

// A panel on a metric nobody exports stays empty without an error, so every
// name the dashboard queries must be declared by the service.
func TestQueriesNameExportedMetrics(t *testing.T) {
	declared := declaredMetrics(t)
	_, m := emitted(t)

	check := func(where, metric string) {
		if external[metric] {
			return
		}
		// The service runs on Linux; elsewhere the process collector exports less.
		if runtime.GOOS != "linux" && strings.HasPrefix(metric, "process_") {
			return
		}

		require.True(t, declared[metric] || declared[histogramBase(metric)],
			"%s queries %q, which asparges does not export", where, metric)
	}

	for _, p := range m.Panels {
		for _, target := range p.Targets {
			metrics := metricsIn(target.Expr)
			require.NotEmpty(t, metrics, "no metric found in %s: %s", p.Title, target.Expr)

			for _, metric := range metrics {
				check(p.Title, metric)
			}
		}
	}

	for _, v := range m.Templating.List {
		for _, sm := range labelValuesRe.FindAllStringSubmatch(v.Definition, -1) {
			check("variable "+v.Name, sm[1])
		}
	}
}

var metricLiteralRe = regexp.MustCompile(`"asparges_[a-z0-9_]+"`)

// declaredMetrics is what /metrics serves: the asparges_ names spelled in the
// service's sources, which grepping keeps from going stale, plus what the
// default registry's runtime and process collectors gather.
func declaredMetrics(t *testing.T) map[string]bool {
	t.Helper()

	out := map[string]bool{}

	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Not this package: its own queries would vouch for themselves.
			if path != "../.." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || path == "../../cmd/dashboard") {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		src, err := os.ReadFile(path) //nolint:gosec // this repo's own sources
		if err != nil {
			return err
		}

		for _, lit := range metricLiteralRe.FindAllString(string(src), -1) {
			out[strings.Trim(lit, `"`)] = true
		}

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, out, "no asparges_ metric found in the sources: the scan is broken")

	mfs, err := prometheus.DefaultGatherer.Gather()
	if runtime.GOOS == "linux" {
		require.NoError(t, err)
	}

	for _, mf := range mfs {
		out[mf.GetName()] = true
	}

	return out
}

var (
	labelValuesRe = regexp.MustCompile(`label_values\(([a-zA-Z_:][a-zA-Z0-9_:]*)`)

	// Label lists, matchers, ranges and strings hold no metric names.
	promGroupingRe = regexp.MustCompile(`\b(?:by|without|on|ignoring)\s*\([^)]*\)`)
	promNoiseRe    = regexp.MustCompile(`\[[^\]]*\]|\{[^}]*\}|"[^"]*"`)
	promIdentRe    = regexp.MustCompile(`[a-zA-Z_:][a-zA-Z0-9_:]*`)
	promKeywords   = map[string]bool{
		"and": true, "or": true, "unless": true, "offset": true, "bool": true,
		"sum": true, "min": true, "max": true, "avg": true, "count": true,
	}
)

// metricsIn returns the metric names a PromQL expression selects: the
// identifiers left once label lists are gone that are neither keywords nor
// function calls.
func metricsIn(expr string) []string {
	clean := promNoiseRe.ReplaceAllString(promGroupingRe.ReplaceAllString(expr, " "), " ")

	var out []string

	for _, loc := range promIdentRe.FindAllStringIndex(clean, -1) {
		name := clean[loc[0]:loc[1]]
		if promKeywords[name] || (loc[1] < len(clean) && clean[loc[1]] == '(') {
			continue
		}

		out = append(out, name)
	}

	return out
}

// histogramBase strips the series suffixes Prometheus derives from a
// histogram, which are queried but never declared.
func histogramBase(metric string) string {
	for _, suffix := range []string{"_bucket", "_sum", "_count"} {
		if base, ok := strings.CutSuffix(metric, suffix); ok {
			return base
		}
	}

	return metric
}

func TestMetricsIn(t *testing.T) {
	require.Equal(t,
		[]string{"asparges_images_build_seconds_bucket"},
		metricsIn(`histogram_quantile(0.95, sum by (le, kind) (rate(asparges_images_build_seconds_bucket{job="$job"}[1h])))`))
	require.Equal(t,
		[]string{"process_start_time_seconds"},
		metricsIn(`time() - process_start_time_seconds{job="$job"}`))
}
