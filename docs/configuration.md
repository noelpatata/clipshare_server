# ClipShare Server Configuration

This document is the reference for every configuration option in the ClipShare
server. There is only one config file; it lives at:

```
$HOME/.config/clipshare/config.toml
```

or wherever `CLIPSHARE_CONFIG` points (recommended on Windows, where `HOME`
may be unset).

- Generate the default file with `clipshare config --init`.
- Re-save the current effective config with `clipshare config`.
- A missing file silently falls back to defaults; the file and its directory
  are created with `0700` permissions.

## Full reference

| key | type | default | description |
|-----|------|---------|-------------|
| `device_name` | string | hostname | The name this daemon announces. Shown in `hello`, `status`, and mDNS/beacon discovery. The Android app's "device name" is the analogous setting. |
| `mdns` | bool | `true` | Advertise via mDNS as `_clipshare._tcp`. Set `false` to stop mDNS (the UDP beacon still runs). |
| `broadcast` | int (seconds) | `5` | Interval of the UDP discovery beacon. `<= 0` falls back to 5s. |
| `watch` | int (milliseconds) | `300` | Clipboard poll interval. Values `< 50` are clamped back to `300`. |
| `token` | string | `""` | Optional shared secret. When set, clients must pass `?token=` on the WebSocket URL (the Android app and desktop peers do this automatically). Empty = no token check. **Security note:** without TLS the token travels in plaintext (URL and never again in the beacon). Prefer mTLS (`[tls]`). |
| `peers` | []string | `[]` | Desktop-to-desktop relay targets: `host[:port]`. Each is dialed out and retried with backoff (1s → 30s). Used to relay clipboard data between two desktops. |
| `max_image_bytes` | int | `10485760` | Images larger than this are dropped at broadcast time instead of being sent. 0 or negative falls back to 10 MiB. |
| `server.port` | int | `40403` | WebSocket listen port (all interfaces). |
| `api.port` | int | `40405` | Localhost control API port (bound to `127.0.0.1` only): `GET /status`, `POST /send`. |
| `connection.mode` | string | `"discover"` | How this daemon finds and trusts peers. `"discover"` or `"whitelist"` (below). |
| `connection.whitelist` | table array | `[]` | Allowed devices in whitelist mode: `{ name = "...", ip = "..." }`. |
| `tls.enabled` | bool | `false` | Enable mutual TLS on the WebSocket server (and `wss://` outbound peer dials). |
| `tls.ca` | string | `<config-dir>/certs/ca.pem` | The private CA (trust root) both sides must share. |
| `tls.cert` | string | `<config-dir>/certs/server.pem` | This device's certificate (CN = your device name, SAN IPs = its LAN IPs). |
| `tls.key` | string | `<config-dir>/certs/server.key` | This device's private key (permissions `0600`). |

### Fixed values

- WebSocket port `40403`, UDP beacon port `40404`, localhost API `40405`.
- The beacon advertises `name`, `port`, and `tls` only — the shared token is
  deliberately **not** broadcast (plaintext leak on the LAN).

## Connection mode

`connection.mode` has the same two options on the desktop server and the
Android app, so the semantics match on both ends. Configure the same mode and
whitelist on both apps.

### `discover` (default)

- Desktop: advertises over mDNS and UDP beacons; accepts any client that
  passes the token (if set) and TLS (if enabled).
- Android: scans the LAN and auto-connects to the first discovered device.

### `whitelist`

- Desktop: **stops advertising** (invisible on the LAN) and rejects any
  client whose source IP **and** `hello` name do not match an entry. An entry
  matches on **either** `name` **or** `ip`.
- Android: stops scanning; connects directly to each whitelisted IP and
  verifies the server's `hello` name against the entry.

The whitelist is the per-device allowlist for both ends — configure the same
entries (device names must match each device's `device_name`) on both apps.

```toml
[connection]
mode = "whitelist"
whitelist = [
  { name = "desktop", ip = "192.168.1.10" },
  { name = "phone1",  ip = "192.168.1.50" },
]
```

## Mutual TLS (mTLS)

When `tls.enabled = true`:

- The server presents its certificate (`tls.cert`/`tls.key`) and **requires**
  a client certificate signed by `tls.ca` — clients without one are rejected
  at the TLS handshake.
- The Android app (and desktop peers) present their own CA-signed client
  certificate and verify the server against the same CA plus the server's SAN
  IPs.
- The UDP beacon advertises `tls=true` so clients automatically use `wss://`.

Generate the CA and certificates with the `clipshare cert` subcommands:

```sh
clipshare cert init                                   # create the private CA
clipshare cert issue --name desktop --type server     # server cert (auto SAN IPs)
clipshare cert issue --name phone1  --type client     # phone client cert
clipshare cert export --name phone1 --type client     # -> phone1-client.p12
```

Import the `.p12` on the phone (Settings → certificate), and set the same
values for `tls.ca`/`tls.cert`/`tls.key` here. The `.p12` password is
`clipshare` (exported in the legacy 3DES PKCS#12 format for Android
compatibility — treat the file like a secret).

**DHCP note:** the server certificate's SANs carry the desktop's LAN IPs. If
the IP changes (DHCP), reissue the server certificate (`clipshare cert issue
--name desktop --type server`) and re-import if the phone pinned it. Prefer a
DHCP reservation for the desktop.

## Environment variables

| variable | description |
|----------|-------------|
| `CLIPSHARE_CONFIG` | Full path to the config file, overriding the default location. |

## Example config

```toml
device_name = "desktop"
mdns = true
broadcast = 5
watch = 300
token = ""
peers = []
max_image_bytes = 10485760

[connection]
mode = "discover"
whitelist = []

[server]
port = 40403

[api]
port = 40405

[tls]
enabled = false
ca = "/home/you/.config/clipshare/certs/ca.pem"
cert = "/home/you/.config/clipshare/certs/server.pem"
key = "/home/you/.config/clipshare/certs/server.key"
```

## Behavior notes

- **Loop protection:** the daemon never rebroadcasts content it wrote to the
  clipboard itself (received images/text are not echoed back).
- `clipshare daemon --no-watch` disables the outgoing clipboard watcher: the
  daemon still receives and writes remote content, but never broadcasts local
  changes.
- The clipboard backend requires `wl-copy`/`wl-paste` (Wayland) or
  `xclip`/`xsel` (X11) on Linux, and supports images via `image/png` targets.
