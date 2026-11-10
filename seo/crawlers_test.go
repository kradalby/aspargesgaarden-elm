package seo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestCrawler(t *testing.T) {
	tests := []struct {
		ua, agent, purpose string
	}{
		{"Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "googlebot", "search"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm) Chrome/116.0.1938.76 Safari/537.36", "bingbot", "search"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)", "applebot", "search"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot)", "oai-searchbot", "ai_search"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; ChatGPT-User/1.0; +https://openai.com/bot", "chatgpt-user", "user_fetch"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.1; +https://openai.com/gptbot)", "gptbot", "ai_training"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Claude-SearchBot/1.0; +https://www.anthropic.com)", "claude-searchbot", "ai_search"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Claude-User/1.0; +Claude-User@anthropic.com)", "claude-user", "user_fetch"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ClaudeBot/1.0; +claudebot@anthropic.com)", "claudebot", "ai_training"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)", "perplexitybot", "ai_search"},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Perplexity-User/1.0; +https://perplexity.ai/perplexity-user)", "perplexity-user", "user_fetch"},
		{"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)", "facebookexternalhit", "user_fetch"},
	}

	for _, tt := range tests {
		agent, purpose, ok := crawler(tt.ua)
		require.True(t, ok, tt.ua)
		require.Equal(t, tt.agent, agent, tt.ua)
		require.Equal(t, tt.purpose, purpose, tt.ua)
	}

	for _, ua := range []string{"", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36", "curl/8.10.1"} {
		_, _, ok := crawler(ua)
		require.False(t, ok, ua)
	}
}

func TestReferral(t *testing.T) {
	tests := []struct {
		target, referer, want string
	}{
		{"/lokaler?utm_source=chatgpt.com", "", "chatgpt"},
		{"/", "https://chatgpt.com/", "chatgpt"},
		{"/", "https://www.perplexity.ai/search/abc", "perplexity"},
		{"/", "https://claude.ai/", "claude"},
		{"/", "https://gemini.google.com/", "gemini"},
		{"/?utm_source=copilot.com", "https://copilot.microsoft.com/", "copilot"},
		{"/", "https://www.google.com/", "google"},
		{"/", "https://www.google.no/", "google"},
		{"/", "https://www.google.co.uk/", "google"},
		{"/", "https://www.bing.com/", "bing"},
		{"/", "https://duckduckgo.com/", "duckduckgo"},
		{"/", "https://aspargesgaarden.no/kontakt", ""},
		{"/", "https://google.evil.example/", ""},
		{"/", "https://google.co.uk.example/", ""},
		{"/", "https://google.n0/", ""},
		{"/", "https://google./", ""},
		{"/", "https://www.GOOGLE.de/", "google"},
		{"/?utm_source=Claude.ai", "", "claude"},
		{"/", "https://notchatgpt.com/", ""},
		{"/?utm_source=newsletter", "", ""},
		{"/", "", ""},
	}

	for _, tt := range tests {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil)
		r.Header.Set("Referer", tt.referer)

		got, ok := referral(r)
		require.Equal(t, tt.want, got, "%s from %s", tt.target, tt.referer)
		require.Equal(t, tt.want != "", ok)
	}
}

func TestCountVisit(t *testing.T) {
	bots := testutil.ToFloat64(metricBots.WithLabelValues("gptbot", "ai_training"))
	chatgpt := testutil.ToFloat64(metricReferrals.WithLabelValues("chatgpt"))

	page := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/lokaler?utm_source=chatgpt.com", nil)
	CountVisit(page)

	// A page's own photos carry the page as referrer: not another visit.
	photo := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/img/v1/x.jpg?utm_source=chatgpt.com", nil)
	photo.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GPTBot/1.1; +https://openai.com/gptbot)")
	CountVisit(photo)

	require.InDelta(t, chatgpt+1, testutil.ToFloat64(metricReferrals.WithLabelValues("chatgpt")), 0)
	require.InDelta(t, bots+1, testutil.ToFloat64(metricBots.WithLabelValues("gptbot", "ai_training")), 0)
}

// CountVisit runs on every request, photos and files included.
func BenchmarkCountVisit(b *testing.B) {
	browser := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/img/v1/x.jpg", nil)
	browser.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1")
	page := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/lokaler", nil)
	page.Header.Set("User-Agent", browser.UserAgent())
	page.Header.Set("Referer", "https://www.google.no/")

	b.ReportAllocs()

	for b.Loop() {
		CountVisit(browser)
		CountVisit(page)
	}
}
