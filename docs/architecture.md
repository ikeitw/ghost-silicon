# Ghost-Silicon Architecture

## Overview

Ghost-Silicon is a Windows 11 privacy browser written in Go. It embeds the
system WebView2 runtime (Chromium) and spoofs every fingerprinting API the
engine exposes — hardware, screen geometry, GPU, fonts, canvas noise, audio
noise — so websites see a controlled profile instead of real device values.

```
┌──────────────────────────────────────────────────────────────────┐
│  ghost-silicon.exe                                               │
│                                                                  │
│  ┌──────────────┐   first run    ┌───────────────────────────┐  │
│  │  Setup       │ ─────────────▶ │  Identity Profile         │  │
│  │  Wizard      │                │  (OS / GPU / UA / Search) │  │
│  └──────────────┘                └──────────┬────────────────┘  │
│                                             │                   │
│  ┌──────────────────────────────────────────▼───────────────┐   │
│  │  Browser Window  (Walk MainWindow + go-webview2)         │   │
│  │                                                          │   │
│  │  ┌────────────────────────────────────────────────────┐  │   │
│  │  │  WebView2 controller (fills entire client area)    │  │   │
│  │  │                                                    │  │   │
│  │  │  ┌──────────────────────────────────────────────┐  │  │   │
│  │  │  │  HTML Chrome Overlay  (position:fixed)       │  │  │   │
│  │  │  │  — tab strip, address bar, nav buttons       │  │  │   │
│  │  │  │  — window controls (min / max / close)       │  │  │   │
│  │  │  └──────────────────────────────────────────────┘  │  │   │
│  │  │                                                    │  │   │
│  │  │  ┌──────────────────────────────────────────────┐  │  │   │
│  │  │  │  Identity Polyfill  (injected on doc create) │  │  │   │
│  │  │  │  — overrides navigator.*, screen.*, WebGL    │  │  │   │
│  │  │  │  — canvas / audio / font noise seeds         │  │  │   │
│  │  │  └──────────────────────────────────────────────┘  │  │   │
│  │  │                                                    │  │   │
│  │  │  Web content (any site / ghost:// pages)           │  │   │
│  │  └────────────────────────────────────────────────────┘  │   │
│  │                                                          │   │
│  │  Bookmarks │ History │ Downloads │ Content Blocker       │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  IPC bridge  (named pipe + JSON-RPC, for external adapters)     │
│  Lifecycle manager  (ordered start / stop hooks)                │
└──────────────────────────────────────────────────────────────────┘
```

## Startup Sequence

```
main()
  │
  ├─ CLI flags (--config, --headless, --version)
  ├─ Config discovery (exe dir → parent dir → %APPDATA%)
  ├─ bootstrap.Run()  — logging, config, audit trail
  ├─ storage.Migrate()
  ├─ profile store open
  ├─ session ID + filesystem layout created
  │
  ├─ [GUI mode] RunSetupWizard()
  │     └─ User picks OS, GPU, browser UA, timezone, search engine
  │        Returns *SetupResult{Profile, SearchEngineURL}
  │
  ├─ IPC bridge start  (named pipe listener + JSON-RPC server)
  │
  ├─ browser.NewWindow() + win.Open()
  │     ├─ Walk MainWindow (hidden, message-loop only)
  │     ├─ go-webview2 instance (fills screen)
  │     ├─ Identity polyfill injected (AddScriptToExecuteOnDocumentCreated)
  │     ├─ HTML chrome injected (AddScriptToExecuteOnDocumentCreated)
  │     ├─ ghost:// bindings registered
  │     └─ Navigate("ghost://newtab")
  │
  └─ mw.Run()  ← Walk message loop, blocks until window closed
```

## Core Components

### Setup Wizard (`pkg/browser/setup.go`)

A go-webview2 window shown on every launch. The user selects:
- **OS preset** — sets `navigator.platform`, screen dimensions, CPU cores, RAM
- **GPU** — sets WebGL `UNMASKED_VENDOR_WEBGL` / `UNMASKED_RENDERER_WEBGL`
- **Browser identity** — sets the complete User-Agent string (OS-appropriate variant)
- **Language / Timezone** — sets `navigator.languages` and `Intl.DateTimeFormat`
- **Search engine** — URL prefix used when typing text in the address bar

Returns a `*SetupResult{Profile *identity.Profile, SearchEngineURL string}`.

### Identity Profile (`pkg/identity`)

Every value exposed to the browser engine flows through a `Profile` struct.
The profile is created by the setup wizard and never changes during a session.

Subsections:
- `Hardware` — platform, CPU cores, RAM, GPU vendor + renderer
- `Browser` — UserAgent, AppVersion, Vendor, ProductSub, Languages
- `Screen` — width, height, availHeight, colorDepth, devicePixelRatio, orientation
- `Network` — timezone
- `Noise` — canvas, audio, WebGL, font noise seeds
- `Storage` — API enablement flags
- `Permissions` — per-API permission states

### Identity Polyfill (`pkg/browser/webview.go` → `injectPolyfill`)

Injected via `AddScriptToExecuteOnDocumentCreated`. It overrides:
- `navigator.userAgent`, `appVersion`, `vendor`, `platform`, `hardwareConcurrency`,
  `deviceMemory`, `languages`, `language`, `cookieEnabled`
- `screen.width/height/availWidth/availHeight/colorDepth/pixelDepth/orientation`
- `window.devicePixelRatio`
- `Intl.DateTimeFormat` prototype — timezone
- WebGL `getParameter(UNMASKED_VENDOR/RENDERER_WEBGL)`
- `HTMLCanvasElement.toDataURL/toBlob/getImageData` — deterministic noise
- `AudioContext.createAnalyser` / `getChannelData` — deterministic noise

### HTML Chrome Overlay (`pkg/browser/webview.go` → `injectChromeOverlay`)

A `position:fixed` DOM subtree injected into every page at `z-index:2147483647`.
It implements the entire browser chrome with no Win32 child-window conflicts:

- **Tab strip** — tabs stored as JSON in Go (`WebViewPanel.tabsJSON`), synced to JS
  via `__ghostGetTabs` / `__ghostSetTabs` bindings
- **Address bar** — converts typed text to search queries using `_searchURL`
  (set from the wizard's search engine choice)
- **Navigation buttons** — back, forward, reload, stop, home
- **Window controls** — minimize, maximize/restore, close
- **Drag region** — `WM_NCLBUTTONDOWN` via `__ghostStartDrag`
- **Resize edges** — JS edge-proximity detection → `__ghostStartResize`
- **Bookmarks bar** — ☆/★ toggle wired to `BookmarkStore`
- **Context menu** — right-click menu with search, copy, inspect, etc.

### ghost:// Protocol

Internal pages are served as `data:text/html;base64,...` URLs.
`WebViewPanel.ghostPageDataURL(url)` generates the HTML and injects
`window.__ghostInitTabs` so the tab strip renders synchronously on load.

| URL | Purpose |
|---|---|
| `ghost://newtab` | New tab / home page |
| `ghost://settings` | Settings (placeholder) |

### Browser Features

| Feature | Implementation |
|---|---|
| Bookmarks | `BookmarkStore` (JSON file at `<session>/bookmarks.json`) |
| History | `BrowsingHistoryStore` (JSON file at `<session>/history.json`) |
| Downloads | `DownloadManager` (in-memory, wired to `__ghostDownloadStarted`) |
| Content blocking | `Blocker` (host blocklist from `pkg/browser/blocklist.txt`) |
| DevTools | `DevToolsPanel` (F12 via `ICoreWebView2.OpenDevToolsWindow`) |
| Zoom | `WebViewPanel.SetZoom()` → `ICoreWebView2Controller.put_ZoomFactor` |

### IPC Bridge (`pkg/bridge`, `internal/ipc`)

A named-pipe JSON-RPC server available for external renderer adapters.
In normal embedded-WebView2 mode it starts but receives no traffic —
the polyfill reads profile values directly from the injected script.

The bridge is used when `engine.executable` is set in config, which launches
a separate Chromium-compatible renderer process that connects via the pipe.

### Lifecycle Manager (`internal/app/lifecycle`)

Ordered start/stop hooks. Start hooks run in registration order; stop hooks
run LIFO. Current hooks:

| Name | Start | Stop |
|---|---|---|
| `ipc-bridge` | Open named pipe, begin serving | Close pipe listener |
| `renderer` | Start external renderer process | Terminate + Wait |

The `renderer` hook is registered only when `engine.executable` is non-empty.

## Package Map

```
cmd/ghost-silicon/           — entry point, startup sequence
pkg/
  browser/                   — browser window, WebView2, chrome overlay,
  │                            setup wizard, bookmarks, history, downloads
  bridge/                    — profile → IPC handler wiring
  identity/                  — Profile struct, templates, store, validation
  renderer/                  — StartOptions, Adapter interface
  storage/                   — migrations, profile store, session store
  version/                   — version string
internal/
  app/
    bootstrap/               — logging + config init
    lifecycle/               — ordered hook manager
    shutdown/                — signal handler
  config/                    — YAML loader + validation
  engine/
    adapter/                 — adapter registry + mock
    runtime/                 — crash-restart supervise loop
  ipc/
    jsonrpc/                 — JSON-RPC 2.0 codec + server
    namedpipe/               — Windows named pipe listener
  platform/windows/
    filesystem/              — per-session directory layout
    process/                 — process launch, job object, token restriction
  telemetry/
    logging/                 — structured logger (log/slog)
    audit/                   — NDJSON audit trail
```

## Thread Safety

| Component | Mechanism |
|---|---|
| `WebViewPanel` tab state | Single goroutine (UI thread via `mainWindow.Synchronize`) |
| `bridge.Bridge` profile | Atomic pointer swap (`UpdateProfile`) |
| `jsonrpc.Server` handlers | Stateless; called concurrently |
| `BookmarkStore` | `sync.RWMutex` |
| `BrowsingHistoryStore` | `sync.Mutex` |
| `DownloadManager` | `sync.Mutex` |
| `audit.Logger` | `sync.Mutex` |
