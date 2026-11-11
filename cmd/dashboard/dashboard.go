package main

import (
	"github.com/grafana/grafana-foundation-sdk/go/common"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"
	"github.com/grafana/grafana-foundation-sdk/go/units"
)

// The service's metrics are selected by $job and the host's HTTPS probe by
// $site, so one dashboard serves the beta and production nodes.
const (
	job   = `job="$job"`
	probe = `instance="$site"`
)

// Photo builds and crawler visits come in bursts, so a step-sized window is
// empty most of the time; an hour matches the build-error alert's window.
const hour = `[1h]`

func promDS() common.DataSourceRef {
	return common.DataSourceRef{Type: new("prometheus"), Uid: new("${datasource}")}
}

func query(expr, legend string) *prometheus.DataqueryBuilder {
	return prometheus.NewDataqueryBuilder().Expr(expr).LegendFormat(legend).Range()
}

func panel(title, desc, unit string, targets ...*prometheus.DataqueryBuilder) *timeseries.PanelBuilder {
	p := timeseries.NewPanelBuilder().
		Title(title).
		Description(desc).
		Unit(unit).
		Datasource(promDS()).
		Span(12).
		Height(8).
		Legend(common.NewVizLegendOptionsBuilder().
			Placement(common.LegendPlacementBottom).
			ShowLegend(true).
			DisplayMode(common.LegendDisplayModeTable).
			Calcs([]string{"lastNotNull", "max"}))
	for _, t := range targets {
		p = p.WithTarget(t)
	}

	return p
}

func variable(name, label, def string) *dashboard.QueryVariableBuilder {
	return dashboard.NewQueryVariableBuilder(name).
		Label(label).
		Query(dashboard.StringOrMap{String: new(def)}).
		Definition(def).
		Datasource(promDS()).
		Refresh(dashboard.VariableRefreshOnDashboardLoad).
		Sort(dashboard.VariableSortAlphabeticalAsc)
}

// buildDashboard emits the v1 model: it is what Grafana's file provisioning
// reads, hence the deprecation waiver. Build() validates.
//
//nolint:staticcheck // v1 model required for file-based provisioning
func buildDashboard() (dashboard.Dashboard, error) {
	return dashboard.NewDashboardBuilder("asparges").
		Uid("asparges").
		Tags([]string{"asparges", "generated"}).
		Refresh("1m").
		Time("now-24h", "now").
		Timezone(common.TimeZoneBrowser).
		WithVariable(dashboard.NewDatasourceVariableBuilder("datasource").
			Label("Data source").Type("prometheus")).
		WithVariable(variable("job", "Node", `label_values(asparges_images_queue_length, job)`)).
		WithVariable(variable("site", "Site", `label_values(probe_success{instance=~"https://.*aspargesgaarden\\.no.*"}, instance)`)).
		WithRow(dashboard.NewRowBuilder("Site — is it up and answering?")).
		WithPanel(panel("Up",
			"1 is up. probe: the public HTTPS address answers from outside. "+
				"scrape: Prometheus reaches the service over the tailnet. Probe down "+
				"with scrape up points at nginx or DNS, not the service.",
			units.NoUnit,
			query(`probe_success{`+probe+`}`, "probe"),
			query(`up{`+job+`}`, "scrape")).
			Min(0).Max(1)).
		WithPanel(panel("Requests by status",
			"Public requests per second the service answered, by HTTP status code. "+
				"5xx is the service failing; a burst of 404 is usually a scanner.",
			units.RequestsPerSecond,
			query(`sum by (code) (rate(asparges_http_requests_total{`+job+`}[$__rate_interval]))`, "{{code}}"))).
		WithPanel(panel("Response time",
			"p50 and p95 time the service took to answer a public request, and the "+
				"external probe's fetch of the front page through nginx and TLS. "+
				"Probe slow with the service fast points at nginx or the network. "+
				"Service gaps are windows with no requests.",
			units.Seconds,
			query(`histogram_quantile(0.5, sum by (le) (rate(asparges_http_request_duration_seconds_bucket{`+job+`}[$__rate_interval])))`, "service p50"),
			query(`histogram_quantile(0.95, sum by (le) (rate(asparges_http_request_duration_seconds_bucket{`+job+`}[$__rate_interval])))`, "service p95"),
			query(`probe_duration_seconds{`+probe+`}`, "probe"))).
		WithRow(dashboard.NewRowBuilder("Photos — is the worker keeping up?")).
		WithPanel(panel("Queue",
			"Photos waiting for their sizes. A rebuild keeps it up for a while; "+
				"waiting while nothing is built means the worker is stuck.",
			units.Short,
			query(`asparges_images_queue_length{`+job+`}`, "waiting"))).
		WithPanel(panel("Sizes built (1h)",
			"Variant files written in the last hour, by photo kind and format.",
			units.Short,
			query(`sum by (kind, format) (increase(asparges_images_variants_built_total{`+job+`}`+hour+`))`, "{{kind}} {{format}}"))).
		WithPanel(panel("Build errors (1h)",
			"Photo groups left with a size unbuilt in the last hour; the page shows "+
				"that picture as missing. The cause is in journalctl -u asparges.",
			units.Short,
			query(`sum by (kind) (increase(asparges_images_build_errors_total{`+job+`}`+hour+`))`, "{{kind}}"))).
		WithPanel(panel("Build time p95 (1h)",
			"95th percentile time to build one photo's missing sizes for one kind. "+
				"Gaps are hours with no builds.",
			units.Seconds,
			query(`histogram_quantile(0.95, sum by (le, kind) (rate(asparges_images_build_seconds_bucket{`+job+`}`+hour+`)))`, "{{kind}}"))).
		WithRow(dashboard.NewRowBuilder("Visitors — who finds the site?")).
		WithPanel(panel("Crawler requests (1h)",
			"Requests from known search and AI crawlers in the last hour, by crawler and what it fetches for.",
			units.Short,
			query(`sum by (agent, purpose) (increase(asparges_bot_requests_total{`+job+`}`+hour+`))`, "{{agent}} ({{purpose}})"))).
		WithPanel(panel("Referred visits (1h)",
			"Page visits in the last hour that arrived from an AI assistant or a search engine, by source.",
			units.Short,
			query(`sum by (source) (increase(asparges_referrals_total{`+job+`}`+hour+`))`, "{{source}}"))).
		WithPanel(panel("IndexNow pings (1h)",
			"Changed addresses announced to search engines in the last hour. "+
				"failed means rejected, or still failing after retries.",
			units.Short,
			query(`sum by (result) (increase(asparges_indexnow_submissions_total{`+job+`}`+hour+`))`, "{{result}}"))).
		WithRow(dashboard.NewRowBuilder("Process")).
		WithPanel(panel("Memory",
			"Resident memory of the service and the Go heap in use. "+
				"vips runs as a child process and is not counted here.",
			units.BytesIEC,
			query(`process_resident_memory_bytes{`+job+`}`, "resident"),
			query(`go_memstats_heap_inuse_bytes{`+job+`}`, "heap in use"))).
		WithPanel(panel("CPU",
			"CPU time of the service process, as a share of one core. vips is not counted.",
			units.PercentUnit,
			query(`rate(process_cpu_seconds_total{`+job+`}[$__rate_interval])`, "cpu"))).
		WithPanel(panel("Goroutines",
			"Live goroutines. A steady climb means something leaks.",
			units.Short,
			query(`go_goroutines{`+job+`}`, "goroutines"))).
		WithPanel(panel("Uptime",
			"Time since the service started; a drop to zero is a restart.",
			units.Seconds,
			query(`time() - process_start_time_seconds{`+job+`}`, "uptime"))).
		Build()
}
