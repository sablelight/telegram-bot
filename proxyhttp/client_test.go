package proxyhttp

import (
	"net/http"
	"sync"
	"testing"
)

func TestProxyURLByMode(t *testing.T) {
	tests := []struct {
		mode string
		url  string
		want string
	}{
		{"tor", "", "socks5://127.0.0.1:9050"},
		{"tor", "socks5://10.0.0.1:9999", "socks5://10.0.0.1:9999"},
		{"custom", "http://proxy.internal:3128", "http://proxy.internal:3128"},
		{"custom", "", ""},
		{"direct", "socks5://ignored:9050", ""},
		{"nonsense", "socks5://ignored:9050", ""},
	}
	for _, tt := range tests {
		t.Setenv("PROXYHTTP_MODE", tt.mode)
		t.Setenv("PROXYHTTP_URL", tt.url)
		if got := ProxyURL(); got != tt.want {
			t.Errorf("mode=%q url=%q: ProxyURL() = %q, want %q", tt.mode, tt.url, got, tt.want)
		}
	}
}

func TestClientIsCachedWhileProxyUnchanged(t *testing.T) {
	Invalidate()
	t.Setenv("PROXYHTTP_MODE", "direct")

	first := Client()
	if first == nil {
		t.Fatal("Client() returned nil")
	}
	for i := 0; i < 5; i++ {
		if got := Client(); got != first {
			t.Fatal("Client() rebuilt the client even though the proxy URL did not change")
		}
	}
}

func TestClientRebuildsWhenProxyChanges(t *testing.T) {
	Invalidate()
	t.Setenv("PROXYHTTP_MODE", "direct")
	t.Setenv("PROXYHTTP_URL", "")

	direct := Client()

	// Switching mode changes the *effective* proxy URL from "" to the Tor
	// default, which is what the cache is keyed on.
	t.Setenv("PROXYHTTP_MODE", "tor")
	proxied := Client()

	if direct == proxied {
		t.Error("Client() did not rebuild after the effective proxy URL changed")
	}
}

// TestClientIgnoresIrrelevantConfigChange pins the reason the cache is keyed on
// the effective proxy rather than on raw config: a change that cannot alter the
// route must not throw away the connection pool.
func TestClientIgnoresIrrelevantConfigChange(t *testing.T) {
	Invalidate()
	t.Setenv("PROXYHTTP_MODE", "direct")
	t.Setenv("PROXYHTTP_URL", "")

	before := Client()

	// In "direct" mode the URL setting is ignored entirely, so this must not
	// trigger a rebuild despite the env var changing.
	t.Setenv("PROXYHTTP_URL", "socks5://127.0.0.1:9050")
	if after := Client(); after != before {
		t.Error("Client() rebuilt for a config change that cannot affect the route")
	}
}

func TestInvalidateForcesRebuild(t *testing.T) {
	Invalidate()
	t.Setenv("PROXYHTTP_MODE", "direct")

	before := Client()
	Invalidate()
	after := Client()

	if before == after {
		t.Error("Invalidate() did not force a rebuild")
	}
}

func TestClientTimeoutIsSet(t *testing.T) {
	Invalidate()
	c := Client()
	if c.Timeout != requestTimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, requestTimeout)
	}
}

// TestConcurrentClientAccess exercises the double-checked locking in Client().
// Run with -race to make it meaningful.
func TestConcurrentClientAccess(t *testing.T) {
	Invalidate()
	t.Setenv("PROXYHTTP_MODE", "direct")

	const goroutines = 32
	var wg sync.WaitGroup
	results := make([]*http.Client, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			results[i] = Client()
		}(i)
	}
	wg.Wait()

	for i := 1; i < goroutines; i++ {
		if results[i] != results[0] {
			t.Fatal("concurrent Client() calls returned different clients")
		}
	}
}
