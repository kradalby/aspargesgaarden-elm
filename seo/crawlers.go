package seo

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// Why a crawler fetches a page. The labels are this closed set, never the
// raw user agent, so the metric cannot grow without bound.
const (
	purposeSearch     = "search"      // a search engine's index
	purposeAISearch   = "ai_search"   // an assistant's search index
	purposeAITraining = "ai_training" // data for training models
	purposeUserFetch  = "user_fetch"  // a person asked an assistant, or shared a link
)

// crawlers are matched as substrings of the lower-cased User-Agent, so the
// tokens are written in lower case; each token is also its label. First
// match first: "claude-user" before "claudebot" and so on. Robots tokens
// that never appear in a User-Agent (Google-Extended, Applebot-Extended)
// cannot be counted; their owners fetch as Googlebot and Applebot.
var crawlers = []struct{ token, purpose string }{
	{"oai-searchbot", purposeAISearch},
	{"chatgpt-user", purposeUserFetch},
	{"gptbot", purposeAITraining},
	{"claude-searchbot", purposeAISearch},
	{"claude-user", purposeUserFetch},
	{"claudebot", purposeAITraining},
	{"perplexitybot", purposeAISearch},
	{"perplexity-user", purposeUserFetch},
	{"mistralai-user", purposeUserFetch},
	{"duckassistbot", purposeAISearch},
	{"meta-externalagent", purposeAITraining},
	{"ccbot", purposeAITraining},
	{"bytespider", purposeAITraining},
	{"googlebot", purposeSearch},
	{"bingbot", purposeSearch},
	{"applebot", purposeSearch},
	{"duckduckbot", purposeSearch},
	{"yandexbot", purposeSearch},
	{"facebookexternalhit", purposeUserFetch},
}

var (
	metricBots = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_bot_requests_total",
		Help: "Requests from known search and AI crawlers, by crawler and what it fetches for.",
	}, []string{"agent", "purpose"}))
	metricReferrals = register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "asparges_referrals_total",
		Help: "Page visits arriving from an AI assistant or a search engine, by source.",
	}, []string{"source"}))
)

// crawler names the known crawler in a User-Agent; ok is false for anyone
// else.
func crawler(userAgent string) (agent, purpose string, ok bool) {
	ua := strings.ToLower(userAgent)
	for _, c := range crawlers {
		if strings.Contains(ua, c.token) {
			return c.token, c.purpose, true
		}
	}

	return "", "", false
}

// sources map a referring host (or a utm_source) to its label. Gemini lives
// under google.com, so it is named before isGoogle is asked.
var sources = map[string]string{
	"chatgpt.com":           "chatgpt",
	"chat.openai.com":       "chatgpt",
	"chatgpt":               "chatgpt",
	"perplexity.ai":         "perplexity",
	"perplexity":            "perplexity",
	"claude.ai":             "claude",
	"claude":                "claude",
	"gemini.google.com":     "gemini",
	"gemini":                "gemini",
	"copilot.microsoft.com": "copilot",
	"copilot.com":           "copilot",
	"copilot":               "copilot",
	"bing.com":              "bing",
	"duckduckgo.com":        "duckduckgo",
}

// isGoogle reports whether host is Google search in some country: google
// followed by one or two labels of two or three letters (google.no,
// google.co.uk).
func isGoogle(host string) bool {
	rest, ok := strings.CutPrefix(host, "google.")
	if !ok {
		return false
	}

	labels := strings.Split(rest, ".")
	if len(labels) > 2 {
		return false
	}

	for _, l := range labels {
		if len(l) < 2 || len(l) > 3 || strings.Trim(l, "abcdefghijklmnopqrstuvwxyz") != "" {
			return false
		}
	}

	return true
}

// referral names where a visit came from: its utm_source, which assistants
// add to links, or else the referring site. ok is false for anywhere else.
func referral(r *http.Request) (source string, ok bool) {
	var from string
	if strings.Contains(r.URL.RawQuery, "utm_source") {
		from = r.URL.Query().Get("utm_source")
	}

	if from == "" {
		ref := r.Referer()
		if ref == "" {
			return "", false
		}

		u, err := url.Parse(ref)
		if err != nil {
			return "", false
		}

		from = u.Hostname()
	}

	from = strings.TrimPrefix(strings.ToLower(from), "www.")
	if s, ok := sources[from]; ok {
		return s, true
	}

	if isGoogle(from) {
		return "google", true
	}

	return "", false
}

// CountVisit counts r in the crawler and referral metrics. Photos and
// files are not visits: a page's own requests carry it as referrer.
func CountVisit(r *http.Request) {
	if agent, purpose, ok := crawler(r.UserAgent()); ok {
		metricBots.WithLabelValues(agent, purpose).Inc()
	}

	if strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/img/") {
		return
	}

	if source, ok := referral(r); ok {
		metricReferrals.WithLabelValues(source).Inc()
	}
}
