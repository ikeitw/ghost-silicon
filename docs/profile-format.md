# Profile Format

Profiles are JSON files that define every controlled value the supervisor
exposes to the renderer. This document describes every field.

## Top-level fields

| Field            | Type     | Description                                      |
|------------------|----------|--------------------------------------------------|
| `id`             | string   | Unique profile identifier (UUID recommended)     |
| `name`           | string   | Human-readable label                             |
| `description`    | string   | Optional description                             |
| `schema_version` | int      | Schema version (currently `1`)                   |
| `created_at`     | RFC3339  | Creation timestamp                               |
| `updated_at`     | RFC3339  | Last modification timestamp                      |
| `tags`           | []string | Optional labels for filtering                    |

## `hardware` section

Controls navigator hardware values.

| Field              | JS API                        | Valid values                        |
|--------------------|-------------------------------|-------------------------------------|
| `cpu_cores`        | `navigator.hardwareConcurrency` | 1, 2, 4, 6, 8, 10, 12, 16, 20, 24, 32 |
| `gpu_vendor`       | WebGL `UNMASKED_VENDOR_WEBGL` | Any non-empty string               |
| `gpu_renderer`     | WebGL `UNMASKED_RENDERER_WEBGL` | Any non-empty string             |
| `ram_mb`           | `navigator.deviceMemory`      | 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536 |
| `platform`         | `navigator.platform`          | `Win32`, `Win64`, `Linux x86_64`, `MacIntel` |
| `max_touch_points` | `navigator.maxTouchPoints`    | 0–10                                |

`ram_mb` is stored in MB but exposed as a bucketed GB value per the
`navigator.deviceMemory` spec (0.25, 0.5, 1, 2, 4, 8).

## `browser` section

Controls navigator identity strings.

| Field           | JS API                   | Notes                              |
|-----------------|--------------------------|------------------------------------|
| `user_agent`    | `navigator.userAgent`    | Must start with `Mozilla/`         |
| `app_version`   | `navigator.appVersion`   | Everything after `Mozilla/`        |
| `vendor`        | `navigator.vendor`       | e.g. `Google Inc.`                 |
| `vendor_sub`    | `navigator.vendorSub`    | Usually empty                      |
| `product`       | `navigator.product`      | Always `Gecko` per spec            |
| `product_sub`   | `navigator.productSub`   | e.g. `20030107`                    |
| `languages`     | `navigator.languages`    | At least one entry required        |
| `do_not_track`  | `navigator.doNotTrack`   | `""`, `"0"`, or `"1"`              |
| `cookie_enabled`| `navigator.cookieEnabled`| bool                               |

## `screen` section

Controls `window.screen.*` and `window.devicePixelRatio`.

| Field                | JS API                     | Valid values                     |
|----------------------|----------------------------|----------------------------------|
| `width`              | `screen.width`             | 320–7680                         |
| `height`             | `screen.height`            | 240–4320                         |
| `avail_width`        | `screen.availWidth`        | ≤ width                          |
| `avail_height`       | `screen.availHeight`       | ≤ height                         |
| `color_depth`        | `screen.colorDepth`        | 24, 30, 32                       |
| `pixel_depth`        | `screen.pixelDepth`        | Must equal `color_depth`         |
| `device_pixel_ratio` | `window.devicePixelRatio`  | 0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0 |
| `orientation`        | `screen.orientation.type`  | See orientation values below     |

Orientation values: `landscape-primary`, `landscape-secondary`,
`portrait-primary`, `portrait-secondary`.

## `network` section

| Field        | Purpose                                      |
|--------------|----------------------------------------------|
| `timezone`   | IANA timezone string e.g. `Europe/Amsterdam` |
| `proxy_url`  | Upstream proxy (`http://`, `https://`, `socks5://`) |
| `dns_servers`| Override DNS servers e.g. `["1.1.1.1:53"]`  |

## `noise` section

Seeds for deterministic noise injected into fingerprinting APIs.
A seed of `0` disables noise for that API.

| Field         | API affected                          |
|---------------|---------------------------------------|
| `canvas_seed` | `Canvas2D` getImageData / toDataURL   |
| `audio_seed`  | `AudioContext` getChannelData         |
| `webgl_seed`  | WebGL `getParameter` readback         |
| `font_seed`   | Canvas `measureText`                  |

## `storage` section

| Field                  | Controls                        | Default |
|------------------------|---------------------------------|---------|
| `enable_cookies`       | Cookie persistence              | true    |
| `enable_local_storage` | localStorage API                | true    |
| `enable_session_storage` | sessionStorage API            | true    |
| `enable_indexed_db`    | IndexedDB API                   | true    |
| `enable_cache_storage` | Cache Storage API               | true    |
| `enable_service_worker`| Service Worker registration     | true    |
| `max_cookie_jar_mb`    | Cookie jar size cap (0=no limit)| 0       |
| `max_storage_mb`       | Total web storage cap (0=no limit) | 0    |

## `permissions` section

Each permission is one of: `"deny"`, `"prompt"`, `"grant"`.

| Field            | Browser permission        |
|------------------|---------------------------|
| `geolocation`    | Geolocation API           |
| `notifications`  | Web Notifications         |
| `microphone`     | MediaDevices microphone   |
| `camera`         | MediaDevices camera       |
| `clipboard`      | Clipboard API             |
| `full_screen`    | Fullscreen API            |
| `payment_handler`| Payment Request API       |
| `midi`           | Web MIDI API              |
| `usb`            | WebUSB API                |
| `bluetooth`      | Web Bluetooth API         |

## Schema versioning

The `schema_version` field enables forward-compatible migrations. When a
profile with an older schema version is loaded, the `identity.Migrate`
function upgrades it to the current version in memory before use.

Current version: **1**