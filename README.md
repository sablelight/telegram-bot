# proxyhttp + config

Two small Go packages extracted from a larger project: an environment-backed configuration registry, and a shared `*http.Client` that can be re-pointed at a different proxy while the process is running.

**This is a library, not an application.** There is no `main`. It was extracted from a service I built and is published here because the two pieces are self-contained and generally useful.

## Install

```bash
go get github.com/sablelight/telegram-bot
```

## `config`

Register an option once; the environment variable it reads is derived from the key, so an option cannot get out of sync with the variable it claims to read.

```go
var Timeout = config.RegisterOption("http.timeout", "Request timeout.", "15s")

fmt.Println(Timeout.EnvName())  // HTTP_TIMEOUT
fmt.Println(Timeout.GetString()) // $HTTP_TIMEOUT, else "15s"
```

`GetBool` and `GetInt` are provided. Unparseable values degrade to `false` / `0` rather than panicking, since a malformed environment variable should not take a service down.

Two details worth knowing:

- Values are read from the environment **at call time**, not cached at registration. A deployment can change behaviour without a restart, and tests can use `t.Setenv`.
- `RegisterOption` is idempotent. A duplicate key returns the existing option and does not overwrite its default, so two packages can register the same key without clobbering each other.

## `proxyhttp`

```go
resp, err := proxyhttp.Client().Get("https://example.org")
```

Three modes, selected by `PROXYHTTP_MODE`:

| Mode | Behaviour |
|---|---|
| `tor` | `PROXYHTTP_URL`, or `socks5://127.0.0.1:9050` if unset. **Default.** |
| `custom` | `PROXYHTTP_URL` only. |
| `direct` | No proxy. |

### Why the client is cached

Rebuilding an `*http.Client` per request is cheap in isolation but expensive in aggregate: it discards the connection pool and forces a fresh TLS handshake against every origin. So the client is built once and rebuilt **only when the effective proxy URL changes**. A config reload that does not touch the proxy costs nothing at all.

The cache is keyed on the *effective* proxy (`ProxyURL()`), not on raw config. Changing `PROXYHTTP_URL` while in `direct` mode therefore does not rebuild the client — that value cannot affect the route, and churning the pool for it would be a self-inflicted outage. There is a test pinning this.

Rebuilding uses double-checked locking, so concurrent callers racing on the same change produce exactly one new client rather than one each. Verified under `go test -race`.

### Failure behaviour

A malformed or unsupported proxy URL degrades to a direct connection rather than returning an error, and an unrecognised mode falls back to direct. This is deliberate: it is transport plumbing, and a typo in a config value should not make a service unreachable. If you would rather fail closed, that is a one-line change in `ProxyURL()`.

## Tests

```bash
go test -race ./...
go test -cover ./...
```

92% statement coverage on both packages. The concurrency test is only meaningful under `-race`.

## Licence

MIT — see [LICENSE](LICENSE).