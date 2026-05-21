# Ghost-Silicon — Feature Roadmap

> Current state: multi-tab browser with HTML chrome overlay, embedded WebView2,
> identity profile polyfill (OS / GPU / UA / timezone / search engine chosen at
> launch via setup wizard), bookmarks, browsing history, download manager,
> content blocker, DevTools, and a named-pipe IPC bridge.

---

## Done

| Feature | Notes |
|---|---|
| Frameless window (no OS title bar) | WS_CAPTION removed, DWM shadow restored |
| HTML chrome overlay | Tab strip, address bar, nav buttons, window controls — all HTML |
| Multi-tab support | JS state persisted in Go; same-URL switch skips navigation |
| Setup wizard | OS, GPU, browser UA, language, timezone, search engine |
| Identity polyfill | navigator.*, screen.*, WebGL, canvas/audio/font noise |
| Search engine choice | Address bar uses chosen engine; context-menu "Search for…" too |
| Content blocking | Host blocklist (`blocker.go` + `blocklist.txt`) |
| Bookmarks | Star button, BookmarkStore (JSON), per-session |
| Browsing history | BrowsingHistoryStore (JSON), per-session |
| Download manager | DownloadManager wired to `__ghostDownloadStarted` |
| DevTools | F12 / Ctrl+Shift+I via `ICoreWebView2.OpenDevToolsWindow` |
| Zoom | Ctrl+=/-, zoom reset (0.25×–5×) |
| Keyboard shortcuts | Ctrl+T, Ctrl+W, Ctrl+R, Alt+←/→, F5, F11, Ctrl+L |
| ghost:// pages | newtab, settings (placeholder) |
| Named-pipe IPC bridge | JSON-RPC 2.0, available for external adapter use |
| External renderer mode | engine.executable in config → separate process + crash restart |

---

## Near-Term (next improvements)

### Address Bar

| Feature | What to build |
|---|---|
| **Omnibox suggestions** | Query history + search suggest API as user types; show dropdown |
| **Ctrl+L focus** | Focus address bar from keyboard |
| **ghost:// autocomplete** | Suggest `ghost://settings`, `ghost://history`, etc. |

### Bookmarks

The `BookmarkStore` data model exists. UI is minimal.

| Feature | What to build |
|---|---|
| **Bookmark bar** | Second fixed row in HTML chrome when bookmarks exist |
| **Bookmark manager** | `ghost://bookmarks` — tree view, folders, drag-to-reorder |
| **Import / export** | Parse Chrome/Firefox HTML bookmark export; write same on export |

### History

| Feature | What to build |
|---|---|
| **History page** | `ghost://history` — paginated, searchable, grouped by day |
| **Delete entries** | Per-entry, per-day, all-time |
| **Session restore** | Reload last open tabs on startup |

### Downloads

`DownloadManager` is wired. UI is missing.

| Feature | What to build |
|---|---|
| **Download shelf** | Retractable bar at window bottom, in-progress + recent |
| **Progress tracking** | `ICoreWebView2DownloadOperation.add_BytesReceivedChanged` |
| **Pause / resume / cancel** | Wire to `ICoreWebView2DownloadOperation` controls |
| **Downloads page** | `ghost://downloads` |

---

## Medium-Term

### Settings

| Feature | What to build |
|---|---|
| **Settings SPA** | `ghost://settings` — General, Privacy, Network, Profiles, Advanced |
| **Dark / light / system theme** | Toggle chrome overlay CSS; respect `prefers-color-scheme` |
| **Cookie policy** | Per-profile: accept all / block third-party / block all |
| **JavaScript toggle** | Per-profile: `ICoreWebView2Settings.put_IsScriptEnabled` |
| **Startup behaviour** | New tab / restore last session / specific URL |

### Privacy & Identity

| Feature | What to build |
|---|---|
| **Profile editor** | `ghost://profile` — edit all hardware, screen, browser, noise fields |
| **Profile rotation** | Background goroutine; re-runs wizard or picks next preset |
| **Fingerprint test page** | `ghost://fingerprint` — probe suite showing what sites see |
| **Proxy / VPN UI** | Wire `pkg/network` transport to Settings; show status in address bar |
| **Audit log viewer** | `ghost://audit` — real-time filterable table of IPC events |

### Security

| Feature | What to build |
|---|---|
| **HTTPS-only mode** | Intercept `NavigationStarting`; redirect or show warning page |
| **Certificate error page** | Custom page on `ServerCertificateErrorDetected` |
| **Permission prompts** | Handle `add_PermissionRequested` for camera/mic/geo/notifications |
| **Geolocation spoofing** | Override Geolocation API in polyfill; return profile lat/lon |

---

## Long-Term

### Extensions

WebView2 supports Chrome extensions since version 108.

| Feature | What to build |
|---|---|
| **Load unpacked** | `ICoreWebView2Profile.AddBrowserExtension(path)` |
| **Extensions page** | `ghost://extensions` — list, enable/disable, uninstall |
| **Per-profile isolation** | Already implicit — each profile has its own `DataPath` |

### Performance

| Feature | What to build |
|---|---|
| **Tab hibernation** | Discard WebView2 after idle period; restore on click |
| **Memory per tab** | `GetProcessMemoryInfo` on `get_BrowserProcessId`; show in tooltip |

### Platform

| Feature | What to build |
|---|---|
| **Default browser registration** | `HKCU\Software\Classes\ghost-silicon` HTTP/HTTPS handler |
| **Linux namespaces** | `clone(2)` UTS + net + mnt + user + PID, cgroup v2, seccomp-BPF |
| **macOS Seatbelt** | `sandbox-exec` with deny-default profile |
| **AppContainer** | `CreateAppContainerProfile` via COM + capability SIDs |

---

## WebView2 COM APIs needed for future features

Most medium/long-term features require direct COM access not fully exposed
by go-webview2's public API. Use `github.com/jchv/go-webview2/pkg/edge`
or `QueryInterface` via `golang.org/x/sys/windows`.

Key interfaces:
- `ICoreWebView2` — navigation, script, resource interception
- `ICoreWebView2Controller` — bounds, focus, zoom, visibility
- `ICoreWebView2Settings` — JS, cookies, autoplay, images
- `ICoreWebView2Profile` — user data, extensions, browsing data clear
- `ICoreWebView2Environment` — create new instances, user data folder
- `ICoreWebView2DownloadOperation` — download lifecycle
- `ICoreWebView2Frame` — iframe isolation
