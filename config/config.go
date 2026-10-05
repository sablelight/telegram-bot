// Package config provides a small registry of environment-variable-backed
// configuration options with typed accessors and a shared HTTP client that can
// be rebuilt at runtime when its settings change.
//
// # Design
//
// Options are registered once at package init with a dotted key, a
// description, and a default. The environment variable a key reads from is
// derived from the key itself rather than passed separately, so registering an
// option cannot get out of sync with the variable it claims to read:
//
//	RegisterOption("proxy.mode", "…", "direct")  ->  reads PROXY_MODE
//
// Values are read from the process environment at call time, not cached at
// registration, so a deployment can change behaviour without a restart and
// tests can simply use t.Setenv.
//
// # Thread safety
//
// The registry is safe for concurrent use. Reads of an option's value need no
// lock at all: the environment is consulted directly. Only registration and
// registry introspection take the mutex.
package config

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Option represents a single configuration value with a default.
type Option struct {
	key         string
	description string
	deflt       string
}

var (
	mu      sync.RWMutex
	options = map[string]*Option{}
)

// RegisterOption registers an option for key, or returns the existing one if
// the key is already registered. It is intended to be called from package
// initialisers, typically via a package-level var:
//
//	var Timeout = config.RegisterOption("http.timeout", "…", "15s")
//
// The environment variable is derived from the key: take the part after the
// last dot, uppercase it, and replace '-' with '_'. So "http.timeout" reads
// HTTP_TIMEOUT.
func RegisterOption(key, description, deflt string) *Option {
	mu.Lock()
	defer mu.Unlock()

	if o, ok := options[key]; ok {
		return o
	}
	o := &Option{key: key, description: description, deflt: deflt}
	options[key] = o
	return o
}

// Key returns the full dotted key this option was registered under.
func (o *Option) Key() string { return o.key }

// Description returns the human-readable description supplied at registration.
func (o *Option) Description() string { return o.description }

// Default returns the value used when the environment variable is unset or
// empty.
func (o *Option) Default() string { return o.deflt }

// EnvName returns the environment variable this option reads from.
func (o *Option) EnvName() string {
	short := o.key
	if i := strings.LastIndex(short, "."); i >= 0 {
		short = short[i+1:]
	}
	name := strings.ToUpper(strings.ReplaceAll(short, "-", "_"))
	return "PROXYHTTP_" + name
}

// GetString returns the current value: the environment variable if set and
// non-empty, otherwise the default.
func (o *Option) GetString() string {
	if v, ok := os.LookupEnv(o.EnvName()); ok && v != "" {
		return v
	}
	return o.deflt
}

// GetBool parses GetString() as a boolean. An unparseable value returns false.
func (o *Option) GetBool() bool {
	v, err := strconv.ParseBool(o.GetString())
	if err != nil {
		return false
	}
	return v
}

// GetInt parses GetString() as an integer, falling back to 0.
func (o *Option) GetInt() int {
	n, err := strconv.Atoi(o.GetString())
	if err != nil {
		return 0
	}
	return n
}

// Keys returns every registered key, sorted, for diagnostics and startup
// logging.
func Keys() []string {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]string, 0, len(options))
	for k := range options {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
