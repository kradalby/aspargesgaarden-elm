package seo

import (
	"net/http"
	"net/url"
	"regexp"
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

// crawlers are matched as substrings of the User-Agent, case-insensitively,
// first match first: "Claude-User" before "ClaudeBot" and so on. Robots
// tokens that never appear in a User-Agent (Google-Extended,
// Applebot-Extended) cannot be counted; their owners fetch as Googlebot
// and Applebot.
var crawlers = []struct{ token, purpose string }{
	{"OAI-SearchBot", purposeAISearch},
	{"ChatGPT-User", purposeUserFetch},
	{"GPTBot", purposeAITraining},
	{"Claude-SearchBot", purposeAISearch},
	{"Claude-User", purposeUserFetch},
	{"ClaudeBot", purposeAITraining},
	{"PerplexityBot", purposeAISearch},
	{"Perplexity-User", purposeUserFetch},
	{"MistralAI-User", purposeUserFetch},
	{"DuckAssistBot", purposeAISearch},
	{"meta-externalagent", purposeAITraining},
	{"CCBot", purposeAITraining},
	{"Bytespider", purposeAITraining},
	{"Googlebot", purposeSearch},
	{"bingbot", purposeSearch},
	{"Applebot", purposeSearch},
	{"DuckDuckBot", purposeSearch},
	{"YandexBot", purposeSearch},
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
		if strings.Contains(ua, strings.ToLower(c.token)) {
			return strings.ToLower(c.token), c.purpose, true
		}
	}

	return "", "", false
}

// sources map a referring host (or a utm_source) to its label, first match
// first: Gemini lives under google.com.
var sources = []struct {
	re     *regexp.Regexp
	source string
}{
	{regexp.MustCompile(`^(chatgpt\.com|chat\.openai\.com|chatgpt)$`), "chatgpt"},
	{regexp.MustCompile(`^(perplexity\.ai|perplexity)$`), "perplexity"},
	{regexp.MustCompile(`^(claude\.ai|claude)$`), "claude"},
	{regexp.MustCompile(`^(gemini\.google\.com|gemini)$`), "gemini"},
	{regexp.MustCompile(`^(copilot\.microsoft\.com|copilot\.com|copilot)$`), "copilot"},
	{regexp.MustCompile(`^google(\.[a-z]{2,3}){1,2}$`), "google"},
	{regexp.MustCompile(`^bing\.com$`), "bing"},
	{regexp.MustCompile(`^duckduckgo\.com$`), "duckduckgo"},
}

// referral names where a visit came from: its utm_source, which assistants
// add to links, or else the referring site. ok is false for anywhere else.
func referral(r *http.Request) (source string, ok bool) {
	from := r.URL.Query().Get("utm_source")
	if from == "" {
		u, err := url.Parse(r.Referer())
		if err != nil {
			return "", false
		}

		from = u.Hostname()
	}

	from = strings.TrimPrefix(strings.ToLower(from), "www.")
	for _, s := range sources {
		if s.re.MatchString(from) {
			return s.source, true
		}
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
