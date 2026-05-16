# Ghost-Silicon — Feature Roadmap to Chrome Parity

> Current state: single-tab browser with HTML chrome overlay, WebView2 rendering,
> identity profile polyfill, and a named-pipe IPC bridge.
> This document lists every capability Chrome ships that Ghost-Silicon does not yet have,
> grouped by priority.

---

## 1. Developer Tools

Chrome's DevTools is the most-requested missing piece.
WebView2 ships its own Chromium DevTools; we just need to wire the surface.

| Feature | What to build |
|---|---|
| **Open DevTools** | Call `ICoreWebView2.OpenDevToolsWindow()` via the go-webview2 COM sub-package. Wire to F12 and the ⋮ menu. |
| **Inspect Element** | Right-click context menu → "Inspect". Pass the clicked element's coordinates to `ExecuteScript` to call `inspect(document.elementFromPoint(x,y))`. |
| **Console tab** | Comes free with DevTools window. Also expose a `console.log` stream to the Go audit log via `AddScriptToExecuteOnDocumentCreated`. |
| **Network tab** | Wire `ICoreWebView2.add_WebResourceRequested` + `add_WebResourceResponseReceived` to capture request/response pairs. Display in an in-browser panel at `ghost://network`. |
| **Sources / Debugger** | Ships with the WebView2 DevTools window. No extra work needed once the window is opened. |
| **Performance tab** | Ships with DevTools window. |
| **Memory tab** | Ships with DevTools window. |
| **Application tab** | Ships with DevTools window. Shows cookies, localStorage, IndexedDB, Service Workers. |
| **Detach DevTools** | Allow DevTools window to be resized/moved independently. Already the default WebView2 behaviour. |

---

## 2. Multi-Tab Support

Currently pressing "+" navigates the current page. Real tabs need multiple WebView2 instances.

| Feature | What to build |
|---|---|
| **Multiple WebView2 instances** | Create one `webview2.WebView` per tab, each with its own `DataPath` (for isolated cookies/cache). Show/hide the active one by toggling the host HWND visibility (`ShowWindow`). |
| **Tab strip UI** | Extend the HTML chrome overlay: each tab is a `<div>` with its own title, favicon, and close button. A Go-bound function `__ghostSwitchTab(id)` swaps the active WebView2. |
| **Tab state** | Keep `url`, `title`, `favicon`, `canGoBack`, `canGoForward`, `isLoading` per tab. Persist across navigations so switching back restores the address bar. |
| **Tab reorder** | Drag-to-reorder via the HTML tab strip (Pointer Events API + `ondragstart`/`ondrop`). |
| **Pin tab** | Mark a tab as pinned; it shows only the favicon and ignores close clicks. |
| **Mute tab** | Call `ICoreWebView2Settings.put_IsScriptEnabled` per-tab, or use the `AudioContext` polyfill to silence a tab's audio. |
| **Duplicate tab** | Open a new WebView2 instance and navigate it to the current tab's URL. |
| **Move tab to new window** | Spawn a second `ghost-silicon.exe` process passing the URL as a flag. |
| **Tab search (Ctrl+Shift+A)** | Pop an overlay listing all open tabs with fuzzy search. |

---

## 3. Navigation & Address Bar

| Feature | What to build |
|---|---|
| **Omnibox suggestions** | As the user types, query the browser history SQLite table and the default search engine's suggest API (`/complete/search?q=…`). Show a dropdown below the address bar. |
| **Search engine choice** | Settings page: store a default search engine URL template (e.g. `https://www.google.com/search?q=%s`). Replace Brave Search with the user's choice. |
| **Per-profile search engines** | Each identity profile can override the search engine, matching the fingerprint's `navigator.language`. |
| **Keyboard shortcuts** | Ctrl+L / F6 focuses address bar. Alt+← / Alt+→ for back/forward. Ctrl+T new tab. Ctrl+W close tab. Ctrl+R reload. |
| **`chrome://` redirects** | Map `ghost://settings`, `ghost://history`, `ghost://downloads`, `ghost://extensions` to in-browser SPA pages. |
| **QR code for page** | Generate a QR code of the current URL using a Go QR library and display it in a popup. |

---

## 4. Browsing History

| Feature | What to build |
|---|---|
| **Persist history** | Write every navigation (URL, title, timestamp, profile ID) to a SQLite database at `%APPDATA%\ghost-silicon\history.db`. |
| **History page** | `ghost://history` — paginated list with search, grouped by day. |
| **Delete history** | Per-entry, per-day, all-time, or by time range. Also clear the WebView2 user data via `ICoreWebView2Profile.ClearBrowsingDataAsync`. |
| **Session restore** | On startup, reload the last set of open tabs and their scroll positions. |
| **Recently closed** | Keep the last 25 closed tab URLs in memory; show them in the ⋮ menu. |

---

## 5. Bookmarks

The data model (`BookmarkStore`) already exists. The UI is missing.

| Feature | What to build |
|---|---|
| **Bookmark bar** | Inject a second fixed row below the address bar in the HTML chrome when the user has bookmarks. Each bookmark is a clickable pill. |
| **Bookmark manager** | `ghost://bookmarks` — tree view with folders, drag-to-reorder, rename, delete. |
| **Folder support** | Extend `Bookmark.FolderID` to build a tree. Show folders as dropdowns in the bar. |
| **Import/export** | Parse Chrome/Firefox HTML bookmark export format; write the same on export. |
| **Bookmark search** | Ctrl+Shift+O opens a fuzzy-search popup over all bookmarks. |
| **Star button state** | Wire the existing ☆/★ toggle in the address bar to actually persist via `BookmarkStore.Add/Remove`. |

---

## 6. Downloads

The `DownloadManager` data model exists. Wire it to WebView2.

| Feature | What to build |
|---|---|
| **Intercept downloads** | Handle `ICoreWebView2.add_DownloadStarting`. Call `DownloadManager.Start(...)`. Show the download bar at the bottom of the window. |
| **Progress tracking** | Subscribe to `ICoreWebView2DownloadOperation.add_BytesReceivedChanged`. Call `DownloadManager.Update(...)`. |
| **Pause / resume / cancel** | Expose `ICoreWebView2DownloadOperation.Pause/Resume/Cancel`. Wire to buttons in the download shelf UI. |
| **Open file / open folder** | On completion, use `os.StartProcess("explorer.exe", ...)` to open the file or its containing folder. |
| **Download shelf** | A retractable bar at the bottom of the window showing in-progress and recently completed items. |
| **Downloads page** | `ghost://downloads` — full list with search, re-download, and delete from disk. |
| **Dangerous file warning** | Check the file extension against a known list (`.exe`, `.msi`, `.bat`, etc.) and show a confirmation dialog before saving. |

---

## 7. Extensions (WebExtensions API)

WebView2 supports Chrome extensions natively since version 108.

| Feature | What to build |
|---|---|
| **Load unpacked extension** | Call `ICoreWebView2Profile.AddBrowserExtension(path)`. Wire to Settings → Extensions → "Load unpacked". |
| **Extension management page** | `ghost://extensions` — list installed extensions, enable/disable, uninstall. |
| **Extension store** | Link to the Chrome Web Store (extensions installed there work in WebView2 without modification). |
| **Per-profile extension isolation** | Because each profile has its own `DataPath`, extensions are already isolated per profile. |
| **Policy engine integration** | Allow the existing `internal/policy/javascript.go` to block specific extension APIs (e.g. disable `webRequest` for hardened profiles). |

---

## 8. Privacy & Identity (Ghost-Silicon-Specific)

These are the features that differentiate Ghost-Silicon from Chrome.

| Feature | What to build |
|---|---|
| **Profile switcher UI** | Dropdown in the chrome overlay. Calls `bridge.UpdateProfile(newProfile)` which re-injects the polyfill without restarting. |
| **Profile editor** | `ghost://profile` — edit all hardware, screen, browser, and noise fields. Validate with `identity.Validate()`. |
| **Profile templates** | One-click buttons: "Desktop Windows 11", "Laptop Windows 11", "Mobile-like". Already in `identity/templates.go`; just need the UI. |
| **Profile rotation schedule** | A background goroutine that calls `bridge.Rotate()` on a configurable interval (daily, per-session, manual). |
| **Canvas noise visualiser** | Show a live preview of what `<canvas>` output looks like with the current noise seed. |
| **Fingerprint test page** | `ghost://fingerprint` — run a suite of fingerprint probes and show which values match the profile, which leak the real device. |
| **Proxy / VPN per profile** | Wire the existing `pkg/network` transport to the UI. Per-profile SOCKS5 or HTTP proxy. Show connection status in the address bar. |
| **DNS-over-HTTPS** | Configure the custom resolver in `pkg/network` via Settings. Show the current resolver in the Network Monitor. |
| **Audit log viewer** | `ghost://audit` — real-time stream of every IPC bridge event, rendered as a filterable table. |

---

## 9. Settings

| Feature | What to build |
|---|---|
| **Settings SPA** | `ghost://settings` — React-like SPA served from an embedded Go HTTP server (loopback only, matching the existing `pkg/api` pattern). Sections: General, Appearance, Privacy, Network, Profiles, Extensions, Advanced. |
| **Dark / light / system theme** | Toggle between dark and light chrome overlay CSS. Respect `prefers-color-scheme` for the default. |
| **Font size / zoom default** | Persist `window.devicePixelRatio` override and `document.body.style.zoom` default per profile. |
| **Cookie policy** | Per-profile: accept all / block third-party / block all. Wire to `ICoreWebView2Settings.put_AreCookiesEnabled` + the Storage policy. |
| **JavaScript toggle** | Per-profile: wire to `ICoreWebView2Settings.put_IsScriptEnabled`. |
| **Startup behaviour** | Open new tab / restore last session / open specific URLs. |
| **Default browser** | Register `ghost-silicon.exe` as the default HTTP/HTTPS handler via the Windows registry (`HKCU\Software\Classes\ghost-silicon`). |

---

## 10. Media & Permissions

| Feature | What to build |
|---|---|
| **Permission prompts** | Handle `ICoreWebView2.add_PermissionRequested` for camera, microphone, notifications, geolocation. Show a native Walk dialog. Log decisions to the audit trail. |
| **Geolocation spoofing** | Override the Geolocation API in the polyfill to return profile-specified lat/lon. Already planned in the identity schema. |
| **Notification support** | Allow/deny Web Push via the permission system. Show allowed notifications as Windows toast notifications. |
| **Media autoplay policy** | `ICoreWebView2Settings` → configure autoplay per profile (block by default on hardened profiles). |
| **Picture-in-Picture** | Comes free with WebView2; no extra work. |
| **Screen sharing** | Handle `ICoreWebView2.add_ScreenCaptureStarting` and apply the sandbox policy. |

---

## 11. Security

| Feature | What to build |
|---|---|
| **HTTPS-only mode** | Intercept `ICoreWebView2.add_NavigationStarting`; if URL is `http://` and an HTTPS equivalent exists, redirect. Show a warning page otherwise. |
| **Certificate errors** | Handle `ICoreWebView2.add_ServerCertificateErrorDetected`. Show a custom error page with the certificate details. |
| **Site isolation** | Already handled by WebView2's underlying Chromium process model. |
| **Sandbox status indicator** | Display Job Object and restricted token status in `ghost://audit`. Already implemented in `pkg/sandbox`. |
| **Content Security Policy override** | Allow the policy engine to inject or strengthen CSP headers per profile via `ICoreWebView2.add_WebResourceResponseReceived`. |
| **Mixed content blocking** | Already enforced by WebView2; surface override capability in Settings. |

---

## 12. Performance

| Feature | What to build |
|---|---|
| **Memory usage per tab** | Query `ICoreWebView2.get_BrowserProcessId`, then read working set from `GetProcessMemoryInfo`. Display in the tab strip tooltip. |
| **Tab hibernation** | After a configurable idle period, serialize the tab's URL and discard its WebView2 instance. Restore on click. |
| **Preload / prefetch** | On hover over a link, speculatively navigate a hidden WebView2 to warm up the connection. Swap it in on click. |
| **Hardware acceleration status** | `ICoreWebView2Settings` → GPU info. Surface in `ghost://settings`. |

---

## Implementation Priority Order

```
Phase 2 (next sprint)
  1. Multi-tab (multiple WebView2 instances)          — biggest UX gap
  2. DevTools window (F12 / Inspect Element)          — developer essential
  3. Omnibox suggestions + history persistence        — daily usability
  4. Bookmark bar + manager                           — daily usability
  5. Download intercept + shelf                       — baseline browser feature

Phase 3
  6. Settings SPA (ghost://settings)
  7. Profile switcher UI + profile editor
  8. Permission prompts (camera, mic, notifications)
  9. Extensions support (WebView2 native)
 10. Search engine choice

Phase 4
  11. Fingerprint test page (ghost://fingerprint)
  12. Proxy / VPN per profile UI
  13. Audit log viewer (ghost://audit)
  14. Session restore
  15. HTTPS-only mode + certificate error page

Phase 5 (parity stretch)
  16. Tab hibernation
  17. Geolocation spoofing UI
  18. Content Security Policy override
  19. Tab memory display
  20. Default browser registration
```

---

## Notes on WebView2 COM APIs needed

Most of the above features rely on accessing the WebView2 COM interface
directly, which go-webview2's public API does not yet fully expose.
The recommended approach is to use the `github.com/jchv/go-webview2/pkg/edge`
sub-package which provides raw COM access, or to call `QueryInterface` on the
`ICoreWebView2` pointer via `golang.org/x/sys/windows`.

Key interfaces:
- `ICoreWebView2` — navigation, script, resource interception
- `ICoreWebView2Controller` — bounds, focus, zoom, visibility
- `ICoreWebView2Settings` — JS, cookies, autoplay, images
- `ICoreWebView2Profile` — user data, extensions, browsing data clear
- `ICoreWebView2Environment` — create new instances, set user data folder
- `ICoreWebView2DownloadOperation` — download lifecycle
- `ICoreWebView2Frame` — iframe isolation