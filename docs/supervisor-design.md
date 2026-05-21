# Browser Window & Startup Design

## Startup Sequence (`cmd/ghost-silicon/main.go`)

```
run()
  │
  ├─ 1. Parse CLI flags
  │       --config   path to ghost-silicon.yaml (auto-discovered if empty)
  │       --headless supervisor-only mode, no GUI
  │       --version  print and exit
  │
  ├─ 2. Config discovery  (exe dir → parent dir → %APPDATA%)
  │
  ├─ 3. bootstrap.Run()
  │       — structured logger
  │       — config load + validation
  │       — audit trail
  │
  ├─ 4. storage.Migrate()  (schema migrations)
  │
  ├─ 5. Profile store open
  │
  ├─ 6. resolveProfile()
  │       1. Load default_profile ID from store
  │       2. Load first profile in store
  │       3. Auto-generate from windows11-desktop template
  │       4. Error (startup aborts)
  │
  ├─ 7. newSessionID()  — crypto/rand 16-byte hex
  │
  ├─ 8. filesystem.CreateSessionLayout()
  │       %APPDATA%\ghost-silicon\sessions\<session-id>\
  │
  ├─ 9. [GUI] browser.RunSetupWizard()
  │       — go-webview2 window, blocks until Launch or close
  │       — returns *SetupResult{Profile, SearchEngineURL}
  │       — nil result = user cancelled, process exits cleanly
  │
  ├─ 10. IPC bridge  (named pipe + JSON-RPC)
  │
  ├─ 11. [if engine.executable set] Renderer adapter + runtime
  │
  ├─ 12. lc.Start(ctx)  — ordered hook start
  │
  ├─ 13. [headless] signal wait → lc.Stop() → return
  │
  └─ 14. [GUI] browser.NewWindow() → win.Open()  ← blocks (Walk message loop)
              → win closed → cancel() → lc.Stop()
```

## Setup Wizard (`pkg/browser/setup.go`)

Shown on every launch in GUI mode. Uses a standalone go-webview2 window
with `wv.Run()` as its own message loop (independent of Walk).

The wizard collects five choices and converts them into a `*SetupResult`:

| Choice | Profile field(s) set |
|---|---|
| OS preset | `Hardware.Platform`, `Hardware.CPUCores`, `Hardware.RAMMb`, `Screen.*` |
| GPU preset | `Hardware.GPUVendor`, `Hardware.GPURenderer` |
| Browser identity | `Browser.UserAgent`, `Browser.AppVersion`, `Browser.Vendor`, `Browser.ProductSub` |
| Language / Timezone | `Browser.Languages`, `Network.Timezone` |
| Search engine | `SetupResult.SearchEngineURL` (passed to `WindowOptions`) |

User-Agent strings are OS-aware: picking "macOS" with "Google Chrome 125"
produces a macOS UA; picking "Windows 11" with the same browser produces a
Windows UA. The `uaForPlatform` helper picks the right variant.

Bindings used:
- `__setupDone(jsonStr)` — called on Launch; unmarshals choice JSON, then `PostMessage(WM_CLOSE)`
- `__setupMinimize`, `__setupClose`, `__setupDrag` — window controls

## Browser Window (`pkg/browser/window.go`)

`Window` owns the Walk `MainWindow` (hidden, message-loop only) and the
`WebViewPanel` which holds the real go-webview2 window.

Construction (`NewWindow`) sets defaults; `Open()` does the real work:

1. Create Walk `MainWindow`, remove `WS_CAPTION`, restore DWM shadow
2. Hide the Walk HWND — only the WebView2 window is visible to the user
3. Build menu bar + `Actions`
4. Open `BookmarkStore` + `BrowsingHistoryStore` (JSON files in session dir)
5. Create `DownloadManager`
6. Create `WebViewPanel` (passes `SearchEngineURL` from `WindowOptions`)
7. Wire action callbacks (`wireActions`) and WebView event callbacks
8. Navigate to `ghost://newtab`
9. `mw.Run()` — blocks until window closed

`Window.Close()` is safe to call from any goroutine (posts to UI thread via
`mw.Synchronize`).

## WebViewPanel (`pkg/browser/webview.go`)

Holds the go-webview2 instance. Injections happen via
`AddScriptToExecuteOnDocumentCreated` so they run before any page script:

| Injection | Function | Purpose |
|---|---|---|
| Polyfill | `injectPolyfill()` | Override fingerprinting APIs |
| Chrome overlay | `injectChromeOverlay()` | Tab strip, toolbar, window chrome |
| Resize edges | `injectResizeEdges()` | JS edge detection → `__ghostStartResize` |

### Tab State

Tab state is a JSON string (`tabsJSON`) stored in Go and synced to JS:

```
Go tabsJSON  ←──  __ghostSetTabs(json)  ←── JS saves on every change
Go tabsJSON  ──►  __ghostGetTabs()       ──► JS callback via mainWindow.Synchronize
```

For `ghost://` navigations, tab state is embedded directly into the page HTML
(`window.__ghostInitTabs`) so the tab strip renders synchronously on load.

### ghost:// Navigation

All `ghost://` URLs are converted to `data:text/html;base64,...` by
`ghostPageDataURL()`. JS code must use the `__ghostGoTo` binding instead of
`location.href` — setting `location.href='ghost://...'` silently fails in WebView2.

### Search Engine

`_searchURL` is injected as a `%q`-quoted JS variable in `injectChromeOverlay`.
The value comes from `WebViewPanel.searchEngineURL`, which is set from
`WindowOptions.SearchEngineURL`, which comes from `SetupResult.SearchEngineURL`.

Both the address bar handler and the context-menu "Search for..." item use
`_searchURL` at runtime, so the search engine is never hardcoded.

## Lifecycle Manager (`internal/app/lifecycle`)

Ordered start / stop hooks:

| Name | Registered when | Start | Stop |
|---|---|---|---|
| `ipc-bridge` | always | Open named pipe, begin serving conns | Close pipe listener |
| `renderer` | `engine.executable != ""` | Start external renderer via adapter | Terminate + Wait |

Stop hooks run LIFO (last registered = first stopped).

## Shutdown

### GUI mode
1. Walk message loop exits (window closed or `mw.Close()` called)
2. `cancel()` propagates to all goroutines
3. `lc.Stop(shutCtx)` — LIFO hook order

### Headless mode
1. `SIGINT` or `SIGTERM` received
2. `cancel()` propagates
3. `lc.Stop(shutCtx)` — LIFO hook order

Both paths respect `app.shutdown_timeout` (default 15 s). If the timeout
is exceeded the process returns an error.
