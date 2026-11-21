package luart

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// HTTPClient returns a client whose proxy is resolved per request. HTTP_PROXY,
// HTTPS_PROXY, ALL_PROXY and their lowercase forms are accepted. Go's
// ProxyFromEnvironment ignores ALL_PROXY, so selection is implemented here.
func HTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyFromEnv
	return &http.Client{Timeout: timeout, Transport: transport}
}

func proxyFromEnv(req *http.Request) (*url.URL, error) {
	host := req.URL.Hostname()
	if host == "" || localHost(host) || noProxyMatch(host, req.URL.Port()) {
		return nil, nil
	}
	raw := env("HTTPS_PROXY")
	if req.URL.Scheme != "https" {
		raw = env("HTTP_PROXY")
	}
	if raw == "" {
		raw = env("ALL_PROXY")
	}
	if raw == "" {
		return nil, nil
	}
	return parseProxy(raw)
}

func env(key string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv(strings.ToLower(key)))
}

func parseProxy(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	return url.Parse(raw)
}

func localHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func noProxyMatch(host, port string) bool {
	target := strings.ToLower(strings.Trim(host, "[]"))
	address := target
	if port != "" {
		address = net.JoinHostPort(target, port)
	}
	for _, entry := range strings.Split(env("NO_PROXY"), ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if entry == "*" {
			return true
		}
		if entry == address || entry == target {
			return true
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			if ip := net.ParseIP(target); ip != nil && network.Contains(ip) {
				return true
			}
		}
		if strings.HasPrefix(entry, ".") {
			domain := strings.TrimPrefix(entry, ".")
			if target == domain || strings.HasSuffix(target, "."+domain) {
				return true
			}
		}
		if !strings.Contains(entry, ":") && !strings.Contains(entry, "/") {
			if target == entry || strings.HasSuffix(target, "."+entry) {
				return true
			}
		}
	}
	return false
}
