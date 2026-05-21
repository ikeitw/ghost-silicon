# Development Roadmap — Phase Tracking

## Phase 1 — Core Infrastructure ✓

- [x] CLI entry point, config flag, version flag, headless flag
- [x] YAML config loader with env overrides and built-in defaults
- [x] Config validation
- [x] Profile model, validation, consistency checks
- [x] Profile store (atomic JSON file writes)
- [x] Profile generator (realistic randomised values)
- [x] Built-in profile templates (`windows11-desktop`)
- [x] Profile import / export
- [x] Profile rotation policies (never / on_session / on_interval / on_request_count)
- [x] Structured logging via `log/slog` (text or JSON)
- [x] Append-only NDJSON audit trail
- [x] In-process metrics and span tracing
- [x] Bootstrap + lifecycle manager
- [x] Graceful shutdown with SIGINT / SIGTERM handling
- [x] Windows MessageBox on fatal error + crash.log to `%APPDATA%`

## Phase 2 — Windows Process Isolation ✓

- [x] Windows Job Object (kill-on-close, optional memory/CPU limits)
- [x] Restricted process token (privilege removal)
- [x] Integrity level configuration (low / medium / high)
- [x] Per-session directory tree creation
- [x] Session directory ACL (owner-only)
- [x] Renderer process launcher (`CREATE_SUSPENDED` → assign job → resume)
- [x] stdout/stderr capture via anonymous pipes
- [x] Crash restart loop with exponential backoff
- [x] Windows Firewall rule management (netsh, optional)

## Phase 3 — IPC Bridge ✓

- [x] Length-prefixed frame protocol (4-byte big-endian)
- [x] JSON-RPC 2.0 codec
- [x] JSON-RPC server with method dispatcher
- [x] Named pipe server and client
- [x] Bridge handlers for all profile subsystems
- [x] Renderer event notifications
- [x] Permission-level method access control
- [x] Profile hot-swap on rotation
- [x] Optional WebSocket transport

## Phase 4 — Network Layer ✓

- [x] Custom `http.RoundTripper`
- [x] Custom TCP dialer with configurable timeouts
- [x] Custom DNS resolver with server override
- [x] Proxy support (HTTP, HTTPS, SOCKS5)
- [x] Host ACL (allowlist / blocklist with wildcard support)
- [x] Header policy (strip, inject, set-if-missing)
- [x] TLS policy (minimum version enforcement)
- [x] Connection pool configuration
- [x] Windows system proxy reader (registry)
- [x] Windows system DNS reader (registry)

## Phase 5 — Storage ✓

- [x] Profile store (atomic JSON file writes)
- [x] Session store (metadata persistence)
- [x] Cache store with size reporting
- [x] Cookie store with quota checking
- [x] Permission store (per-origin decisions)
- [x] AES-256-GCM encrypted key-value store
- [x] Windows AppData path resolution
- [x] Storage schema migrations

## Phase 6 — Security Utilities ✓

- [x] DPAPI secret store (Windows)
- [x] In-memory keyring fallback
- [x] Isolation probe (Job Object, token availability)
- [x] Safe defaults checker
- [x] Loopback address enforcement
- [x] Security audit event helpers

## Phase 7 — Developer Tooling ✓

- [x] Profile inspector (validate, diff, export)
- [x] Sandbox checker (isolation prerequisites)
- [x] Network debugger (transport test with ACL)
- [x] Renderer mock (pipe client sending bridge requests)
- [x] Windows environment checker

## Phase 8 — Local HTTP Control API ✓

- [x] HTTP control API server (loopback only)
- [x] `GET /health`
- [x] `GET/POST /profiles`, `GET/DELETE /profiles/{id}`
- [x] `GET /sessions`, `GET/POST /sessions/{id}/stop`
- [x] `GET /network/status`
- [x] Logging + recovery middleware
- [x] Bearer token authentication

## Phase 9 — Embedded Browser Window ✓

- [x] go-webview2 embedded WebView2 (no separate process by default)
- [x] Frameless window (WS_CAPTION removed, DWM shadow + rounded corners)
- [x] WndProc subclass (WM_NCHITTEST / WM_NCCALCSIZE)
- [x] HTML chrome overlay (position:fixed, z-index max)
  - [x] Tab strip with JS state persisted in Go
  - [x] Address bar with search engine URL (chosen at setup)
  - [x] Navigation buttons (back, forward, reload, stop, home)
  - [x] Window controls (minimize, maximize/restore, close)
  - [x] Drag region + JS resize edges
  - [x] Bookmark star toggle
  - [x] Context menu
- [x] Identity polyfill injected on every document
- [x] ghost:// pages (`newtab`, `settings` placeholder)
- [x] Content blocker (`Blocker` + `blocklist.txt`)
- [x] BookmarkStore (JSON, per-session)
- [x] BrowsingHistoryStore (JSON, per-session)
- [x] DownloadManager (in-memory)
- [x] DevToolsPanel (F12)
- [x] Zoom control

## Phase 10 — Setup Wizard ✓

- [x] First-run identity setup wizard (`RunSetupWizard`)
- [x] OS presets (Windows 11, Windows 10, macOS spoof, Linux spoof)
- [x] GPU presets (NVIDIA, AMD, Intel)
- [x] Browser presets with per-OS UA strings (Chrome, Edge, Brave, Firefox, DDG, Yandex, Opera, Safari)
- [x] Language + timezone selection
- [x] Search engine selection (Google, DuckDuckGo, Bing, Yandex, Brave, Ecosia, Yahoo)
- [x] Auto-selects matching search engine when browser changes
- [x] SearchEngineURL wired through WindowOptions → WebViewPanel → JS `_searchURL`

## Phase 11 — Browser UI (planned)

- [ ] Omnibox suggestions (history + search suggest API)
- [ ] Bookmark bar (second fixed row in HTML chrome)
- [ ] Bookmark manager (`ghost://bookmarks`)
- [ ] History page (`ghost://history`) with search and delete
- [ ] Session restore (reload last tabs on startup)
- [ ] Download shelf (retractable bar, progress, pause/cancel)
- [ ] Downloads page (`ghost://downloads`)
- [ ] Settings SPA (`ghost://settings`)

## Phase 12 — Privacy Features (planned)

- [ ] Profile editor (`ghost://profile`)
- [ ] Fingerprint test page (`ghost://fingerprint`)
- [ ] Geolocation spoofing (lat/lon in polyfill)
- [ ] Proxy / VPN UI wired to `pkg/network`
- [ ] Audit log viewer (`ghost://audit`)
- [ ] Profile rotation UI

## Phase 13 — Security (planned)

- [ ] HTTPS-only mode
- [ ] Certificate error page
- [ ] Permission prompts (camera, mic, geo, notifications)
- [ ] Content Security Policy override per profile

## Phase 14 — Extensions (planned)

- [ ] Load unpacked extension (`ICoreWebView2Profile.AddBrowserExtension`)
- [ ] Extensions management page (`ghost://extensions`)

## Phase 15 — Platform (planned)

- [ ] Default browser registration (Windows registry)
- [ ] Linux: namespaces (`clone(2)`), cgroup v2, seccomp-BPF
- [ ] macOS: `sandbox-exec` Seatbelt integration
- [ ] AppContainer (`CreateAppContainerProfile` + capability SIDs)
