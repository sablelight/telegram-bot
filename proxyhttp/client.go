// Package proxyhttp maintains a shared *http.Client whose transport honours a
// proxy setting that can change while the process is running.
//
// # Why this exists
//
// A long-lived service often needs its outbound proxy to be reconfigurable —
// because it was unavailable at boot, because traffic had to be drained onto a
// different route, or because an operator changed it. Rebuilding an
// *http.Client is cheap but not free: doing it per request throws away the
// connection pool and causes a TLS handshake storm against every origin.
//
// So the client is built once and cached. It is only rebuilt when the effective
// proxy URL actually changes, which means a config reload that does not touch
// the proxy costs nothing.
//
// # Usage
//
//	import "github.com/sablelight/telegram-bot/proxyhttp"
//
//	resp, err := proxyhttp.Client().Get("https://example.org")
//
// # Modes
//
// Three modes are supported, selected by config:
//
//   - "tor"    — the configured proxy URL, or socks5://127.0.0.1:9050 if unset.
//                The default, and the reason this package exists.
//   - "custom" — only the configured proxy URL; an error if it is unset.
//   - "direct" — no proxy.
//
// An unrecognised mode falls back to direct rather than failing closed. That is
// a deliberate trade-off: this is transport plumbing, and a typo in a mode name
// should not take a service offline.
package proxyhttp

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/sablelight/telegram-bot/config"
)

const (
	defaultTorSOCKS = "socks5://127.0.0.1:9050"
	requestTimeout  = 15 * time.Second
	idleTimeout     = 30 * time.Second
	maxIdleConns    = 10
)

var (
	confMode = config.RegisterOption(
		"proxy.mode",
		`Outbound proxy mode: "tor" (SOCKS5, defaulting to 127.0.0.1:9050), "custom" (requires proxy.url), or "direct" (no proxy).`,
		"tor",
	)
	confURL = config.RegisterOption(
		"proxy.url",
		"Proxy URL. Defaults to the local Tor SOCKS5 port when mode is \"tor\".",
		"",
	)

	clientMu       sync.RWMutex
	sharedClient   *http.Client
	cachedProxyURL string
)

// Client returns the shared HTTP client, rebuilding it only if the effective
// proxy URL has changed since the last call.
//
// The common case is a read-locked fast path with no allocation and no lock
// contention. A changed proxy takes the write lock, and the check is repeated
// after acquiring it so that concurrent callers racing on the same change only
// build one client.
func Client() *http.Client {
	current := ProxyURL()

	clientMu.RLock()
	if sharedClient != nil && cachedProxyURL == current {
		clientMu.RUnlock()
		return sharedClient
	}
	clientMu.RUnlock()

	clientMu.Lock()
	defer clientMu.Unlock()

	// Re-check: another goroutine may have rebuilt while we waited.
	if sharedClient != nil && cachedProxyURL == current {
		return sharedClient
	}

	transport := &http.Transport{
		MaxIdleConns:    maxIdleConns,
		IdleConnTimeout: idleTimeout,
	}

	if current != "" {
		// A malformed or unsupported proxy URL degrades to a direct connection
		// rather than returning an error, so a bad value cannot wedge the caller.
		if u, err := url.Parse(current); err == nil {
			if d, err := proxy.FromURL(u, proxy.Direct); err == nil {
				transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return d.Dial(network, addr)
				}
			}
		}
	}

	sharedClient = &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
	}
	cachedProxyURL = current

	return sharedClient
}

// Invalidate drops the cached client so the next Client() call rebuilds it.
// Use this after changing proxy settings if you need the change to take effect
// immediately rather than on the next request.
func Invalidate() {
	clientMu.Lock()
	defer clientMu.Unlock()
	sharedClient = nil
	cachedProxyURL = ""
}

// ProxyURL returns the effective proxy URL for the current mode: "" means a
// direct connection.
func ProxyURL() string {
	switch confMode.GetString() {
	case "tor":
		if u := confURL.GetString(); u != "" {
			return u
		}
		return defaultTorSOCKS
	case "custom":
		return confURL.GetString()
	default:
		return ""
	}
}

// Mode returns the configured proxy mode.
func Mode() string { return confMode.GetString() }

// ConfiguredURL returns the raw configured proxy URL, ignoring the mode.
func ConfiguredURL() string { return confURL.GetString() }
