package provider

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func clearProxyEnv(t *testing.T) {
	for _, env := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy"} {
		t.Setenv(env, "")
	}
}
func TestAllProxyRoutesHTTPClient(t *testing.T) {
	clearProxyEnv(t)
	var proxied bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied = r.URL.Host == "example.invalid"
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	t.Setenv("ALL_PROXY", proxy.URL)
	resp, err := HTTPClient(2e9).Do(mustRequest("http://example.invalid/missing"))
	if err != nil {
		t.Fatalf("request through ALL_PROXY failed: %v", err)
	}
	resp.Body.Close()
	if !proxied {
		t.Fatal("request bypassed ALL_PROXY")
	}
}
func TestSchemeProxyPrecedenceAndAllProxyFallback(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("ALL_PROXY", "http://all.example:8080")
	t.Setenv("HTTPS_PROXY", "http://https.example:8443")
	t.Setenv("HTTP_PROXY", "http://http.example:8081")
	for _, tc := range []struct{ url, want string }{{"https://prices.example", "https.example:8443"}, {"http://prices.example", "http.example:8081"}} {
		u, err := proxyFromEnv(mustRequest(tc.url))
		if err != nil || u == nil || u.Host != tc.want {
			t.Fatalf("%s: got %v err %v", tc.url, u, err)
		}
	}
	t.Setenv("HTTP_PROXY", "")
	u, err := proxyFromEnv(mustRequest("http://prices.example"))
	if err != nil || u == nil || u.Host != "all.example:8080" {
		t.Fatalf("ALL_PROXY fallback: %v %v", u, err)
	}
}
func TestLowercaseAndNoProxy(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("all_proxy", "proxy.local:3128")
	t.Setenv("no_proxy", ".example.com, 10.0.0.0/8, internal.test")
	checks := []struct{ host, want string }{{"api.example.com", ""}, {"sub.internal.test", ""}, {"10.1.2.3", ""}, {"market.test", "proxy.local:3128"}, {"localhost", ""}}
	for _, tc := range checks {
		u, _ := proxyFromEnv(&http.Request{URL: &url.URL{Scheme: "https", Host: tc.host}})
		got := ""
		if u != nil {
			got = u.Host
		}
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.host, got, tc.want)
		}
	}
}
func TestInvalidAllProxy(t *testing.T) {
	clearProxyEnv(t)
	t.Setenv("all_proxy", "%zz")
	if _, err := proxyFromEnv(mustRequest("http://example.invalid")); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("bad proxy accepted: %v", err)
	}
}
func mustRequest(raw string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, raw, nil)
	return r
}
