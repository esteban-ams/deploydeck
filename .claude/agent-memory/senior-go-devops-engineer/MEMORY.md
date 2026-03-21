# DeployDeck Agent Memory

## Project structure

- Entry point: `cmd/deploydeck/root.go` (runServer) + `cmd/deploydeck/main.go`
- Cobra CLI with subcommands: root (server), version, doctor, status
- Internal packages: config, deploy, docker, git, ipwhitelist, ratelimit, storage, webhook
- Echo v4 router; middleware registered at router level (Logger, Recover, CORS)
- Route groups: `/api` (all), `/api` sub-group (webhookGroup) for POST deploy/rollback only

## Key architectural decisions

- `golang.org/x/time/rate` (already a transitive dep via Echo) used for rate limiting — no new heavy deps
- Rate limiter is a per-IP token bucket in `internal/ratelimit/ratelimit.go`; cleanup goroutine evicts stale IPs every minute (5-min TTL)
- Rate limiting is scoped to a separate Echo sub-group (`webhookGroup`) so GET /health and GET /deployments are never affected
- `RateLimitConfig.Enabled` booleans-default-to-false issue: solved by checking `!Enabled && RequestsPerMinute == 0` in `applyDefaults` to detect an omitted section and set `Enabled = true`
- IP whitelisting (`internal/ipwhitelist`) is a separate package from ratelimit; both are applied only to `webhookGroup`. Empty whitelist = allow all (no-op middleware). Middleware returns HTTP 403 with JSON `{"error": "IP \"x.x.x.x\" is not allowed"}`. Plain IPs are stored as /32 or /128 host CIDRs internally. Config field: `ServerConfig.IPWhitelist []string yaml:"ip_whitelist"`. Validated in config.validate using `net.ParseCIDR` + `net.ParseIP`.

## Config conventions

- All new top-level sections go into `Config` struct with YAML tag
- `applyEnvOverrides` handles `DEPLOYDECK_*` env vars; add new vars there
- `applyDefaults` sets zero-value defaults for any new section
- `validate` runs after defaults — fail fast on invalid config
- `config.example.yaml` must be updated with every new config section

## Storage architecture

- `Deployment` and `Status` types live in `internal/storage/storage.go` (moved from deploy package)
- `Storage` interface: `Save`, `Update`, `Get`, `List`, `Close`
- `MemoryStorage` (memory.go): mutex-protected map, `Close` is no-op, `List` sorts by StartedAt desc
- `SQLiteStorage` (sqlite.go): `modernc.org/sqlite` driver (name "sqlite"), WAL+NORMAL+busy_timeout=5000 PRAGMAs, timestamps as Unix nanoseconds (INTEGER), completed_at nullable (sql.NullInt64), single mutex serialises all writes
- `NewEngine` now takes `store storage.Storage` as second parameter (dependency injection)
- In `executeDeploy`, PreviousImage+RollbackTag are persisted to store immediately via `Update()` before mode-specific steps — critical for SQLite restart recovery
- Config: `StorageConfig{DBPath string}` under `Config.Storage`; env override `DEPLOYDECK_DB_PATH`
- root.go wires storage before engine: non-empty DBPath → SQLiteStorage, empty → MemoryStorage

## go.mod notes

- Module path: `github.com/esteban-ams/deploydeck`
- `golang.org/x/time` promoted from indirect to direct when ratelimit package was added
- `modernc.org/sqlite v1.47.0` added as direct dep for storage package
- Running `go get <pkg>` may upgrade the `go` toolchain directive in go.mod — this is expected
- Run `go mod tidy` after any dependency changes

## Testing patterns

- Table-driven with `t.Run()` subtests; `t.Parallel()` where tests are independent
- Ratelimit tests use `httptest.NewRequest` + `httptest.NewRecorder` + `echo.New().NewContext`
- `entryCount()` and `cleanup()` are unexported helpers exposed for white-box testing within the same package
- Config tests use `writeConfigFile(t, content)` helper + `t.TempDir()`

## File paths (key files)

- `/Users/estebanmartinezsoto/Development/fastship/cmd/deploydeck/root.go` — server wiring
- `/Users/estebanmartinezsoto/Development/fastship/internal/config/config.go` — config types + Load
- `/Users/estebanmartinezsoto/Development/fastship/internal/deploy/deploy.go` — deployment engine (uses storage.Storage)
- `/Users/estebanmartinezsoto/Development/fastship/internal/storage/storage.go` — Storage interface + Deployment/Status types
- `/Users/estebanmartinezsoto/Development/fastship/internal/storage/memory.go` — in-memory Storage impl
- `/Users/estebanmartinezsoto/Development/fastship/internal/storage/sqlite.go` — SQLite Storage impl
- `/Users/estebanmartinezsoto/Development/fastship/internal/storage/storage_test.go` — shared contract tests for both impls
- `/Users/estebanmartinezsoto/Development/fastship/internal/ratelimit/ratelimit.go` — rate limiter
- `/Users/estebanmartinezsoto/Development/fastship/internal/ratelimit/ratelimit_test.go` — rate limiter tests
- `/Users/estebanmartinezsoto/Development/fastship/internal/ipwhitelist/ipwhitelist.go` — IP whitelist middleware
- `/Users/estebanmartinezsoto/Development/fastship/internal/ipwhitelist/ipwhitelist_test.go` — IP whitelist tests
- `/Users/estebanmartinezsoto/Development/fastship/config.example.yaml` — documented example config

## Error message conventions (established and consistent across codebase)

- **docker/** — `fmt.Errorf("docker <cmd> failed for service %q (compose file: %q): %w\nstderr: %s", ...)`; newline before "stderr:" is intentional
- **git/** — `fmt.Errorf("git clone of %q (branch: %q) into %q failed: %w\nstderr: %s", ...)`
- **deploy/health** — includes timeout duration, attempt count, and the URL in failure messages; suggests `docker compose logs` as next step
- **webhook/verify** — errors name the header that failed + tell user where to fix the secret (GitHub repo settings vs config.yaml); auth header renamed to `X-DeployDeck-Secret`; const renamed to `AuthMethodDeployDeck`
- **webhook/handler** — all HTTP error JSON bodies include service name using `%q`; auth failures pass the underlying verify error through to the client response
- **config/validate** — errors include field path (`service %q: 'compose_file' is required`) and valid values or examples
- **config/Load** — wraps all errors with the config file path; "cannot read" errors suggest `cp config.example.yaml config.yaml`
- **deploy/** — failure error stored in `deployment.ErrorMessage` follows `service %q: deployment failed at %s phase: %v`
