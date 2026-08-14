# ClipShare Server

A LAN clipboard-sharing daemon written in Go. It runs on a desktop (Linux or
Windows), watches the clipboard, and pushes changes to connected clients over
WebSocket — so a copy on your desktop appears on your phone (and vice versa).

The Android companion app lives in the separate repo
[`clipshare_client`](../clipshare_client) and auto-discovers the daemon, so no
IP configuration is needed.

## Features

- **WebSocket server** on port `40403` with a **localhost control API** on `40405`
- Watches the desktop clipboard and broadcasts changes to every connected client
- **Loop protection** — never rebroadcasts content it wrote itself
- **Zero-config discovery** — mDNS (`_clipshare._tcp`) + UDP beacons on `40404`
  so the phone connects automatically
- **One-shot `share`** — push text (or your current clipboard) without keeping a
  daemon running; it starts transiently, delivers, and exits
- Cross-platform: **Linux** (Wayland + X11) and **Windows**
- Optional shared **token** authentication
- Desktop-to-desktop peer relay via the `peers` config

## Requirements

- **Go 1.26+** to build from source
- **Linux**: `wl-copy`/`wl-paste` (Wayland) **or** `xclip`/`xsel` (X11)
- **Windows**: nothing extra

## Build

```sh
# Linux
make build                # -> ./clipshare

# Windows (cross-compile from Linux)
make build-windows        # -> ./clipshare.exe

# or with Go directly
go build -o clipshare ./cmd/clipshare          # current platform
GOOS=windows GOARCH=amd64 go build -o clipshare.exe ./cmd/clipshare
```

## Install to your PATH

### Linux

```sh
make install              # symlinks ~/.local/bin/clipshare -> repo binary
```

Make sure `~/.local/bin` is on your PATH (add to `~/.zshrc` / `~/.bashrc`):

```sh
export PATH=$HOME/.local/bin:$PATH
source ~/.zshrc
```

Alternatively use `go install` (puts the binary in `$(go env GOPATH)/bin`,
usually `~/go/bin`):

```sh
go install ./cmd/clipshare
```

### Windows

```powershell
# build (in the repo)
go build -o clipshare.exe ./cmd/clipshare

# move it somewhere permanent, e.g.
mkdir $HOME\bin
Move-Item .\clipshare.exe $HOME\bin\

# add that folder to PATH (persistent)
setx PATH "$env:PATH;$HOME\bin"

# restart your terminal, then verify
clipshare --help
```

> `go install` also works on Windows and drops the binary into
> `%USERPROFILE%\go\bin` — just add that folder to PATH instead.

## Usage

```
clipshare daemon [--no-watch]   run server + clipboard watcher (foreground)
  clipshare share [<text>]        push text (or your clipboard) to peers;
                                starts a transient daemon if none is running
clipshare send <text>           push text via a running daemon to connected peers
clipshare copy <text>           set the local clipboard only
clipshare watch [--timeout d]   temporarily listen (max 120s) and write the
                                first incoming push to the local clipboard,
                                then exit
clipshare status                show daemon status + connected clients
clipshare config --init         write default config to ~/.config/clipshare/config.toml
clipshare cert                  manage mTLS certificates (init/issue/export/list)
```

### Sending text

**One-shot (no daemon left running)** — the recommended way:

```sh
clipshare share "hello from the laptop"   # explicit text
clipshare share                           # sends your current clipboard
```

`share` checks for a running daemon; if none exists it starts a transient
one, waits up to 120s for a client (the phone's sync service auto-connects via
discovery), delivers the text, and exits ~5s later. Nothing stays running.

**Persistent daemon** — for always-on sync (clipboard watching + receiving):

```sh
clipshare daemon &          # or a systemd user unit, see below
clipshare send "hello"      # push from a second terminal / keybinding
clipshare status            # verify the daemon is up and who is connected
```

> `share` and `send` do the same push; `share` additionally falls back to a
> transient daemon and reads your clipboard when no text is given.

### Receiving on the laptop

- **Always-on:** run `clipshare daemon` — it watches for remote content and
  writes it straight to the local clipboard (no extra command needed).
- **On demand:** run `clipshare watch` to temporarily listen (max 120s,
  `--timeout` to change). The first push from your phone is written to
  the local clipboard and printed, then it exits.
- Copying on the phone pushes while the ClipShare app is open on the phone
  (Android 10+ blocks background clipboard reads; enable background capture in
  the app's settings to sync from other apps).

## Config

`~/.config/clipshare/config.toml` — generated with `clipshare config --init`:

| key          | default    | meaning                                        |
|--------------|------------|------------------------------------------------|
| `device_name`| hostname   | name shown to clients                          |
| `mdns`       | `true`     | advertise via mDNS `_clipshare._tcp`           |
| `broadcast`  | `5`        | UDP beacon interval in seconds                 |
| `watch`      | `300`      | clipboard poll interval in ms                  |
| `token`      | `""`       | shared secret; if set, clients must send it    |
| `peers`      | `[]`       | desktop-to-desktop peer hosts (`ip[:port]`)    |
| `max_image_bytes` | `10485760` | drop images larger than this when broadcasting |
| `connection.mode` | `"discover"` | `discover` or `whitelist` (see docs)      |
| `connection.whitelist` | `[]`  | allowed devices by name/ip in whitelist mode |
| `[server] port` | `40403` | WebSocket port                              |
| `[api] port`    | `40405` | localhost control API                       |
| `[tls] enabled/ca/cert/key` | `false` | mutual TLS (see docs) |
| `[log] level`   | `"info"` | log verbosity: `debug` / `info` / `warn` / `error` |
| `[log] file`    | `""`     | log file path (empty = stderr)              |

Ports: WS/TCP `40403`, UDP beacon `40404`, localhost API `40405` (127.0.0.1 only).

> **Full configuration reference:** [docs/configuration.md](docs/configuration.md) —
> every key, both connection modes, and how to set up mutual TLS with
> `clipshare cert`.

## Run as a background service (Linux, systemd user unit)

`~/.config/systemd/user/clipshare.service`:

```ini
[Unit]
Description=ClipShare clipboard daemon
After=graphical-session.target

[Service]
Type=simple
ExecStart=/home/YOU/.local/bin/clipshare daemon
Restart=on-failure

[Install]
WantedBy=default.target
```

```sh
systemctl --user daemon-reload
systemctl --user enable --now clipshare
systemctl --user status clipshare
```

## Security

- **Mutual TLS (mTLS):** optional but recommended. `clipshare cert init`
  creates a private CA; `clipshare cert issue` signs a server cert and client
  certs; `clipshare cert export` produces a `.p12` to import on the phone. When
  `tls.enabled = true` the server requires a CA-signed client certificate and
  clients verify the server against the same CA + its SAN IPs (see
  [docs/configuration.md](docs/configuration.md)).
- **Token:** a shared secret passed as `?token=` (encrypted under `wss`).
  Without TLS it travels in plaintext on the URL.
- **Whitelist mode:** set `connection.mode = "whitelist"` to stop advertising
  and only accept (desktop) / connect to (phone) whitelisted devices.
- The control API listens on `127.0.0.1` only.

## Protocol

JSON over WebSocket (port `40403`):

| message     | payload                             |
|-------------|-------------------------------------|
| `hello`     | `{name, platform, version}`         |
| `clipboard` | `{type, text?, data?, mime?, ts, from}` — `type` is `"text"` (default) or `"image"` (base64 `data`) |
| `ping`/`pong` | —                                 |
| `error`     | `{code, msg}`                       |

## Project layout

```
cmd/clipshare/        entry point (thin main)
internal/cli/         command dispatch + App lifecycle (daemon, share, send, ...)
internal/clip/        clipboard backends (Linux/Wayland, Windows) + watcher
internal/config/      TOML config loading/saving
internal/certs/       private CA + certificate issuance (clipshare cert)
internal/consts/      non-configurable application constants
internal/discover/    mDNS advertising + UDP beacon broadcasting
internal/log/         leveled logging (level + file from config)
internal/protocol/    JSON-over-WebSocket wire types + codec
internal/websocket/   WebSocket server + outbound peers
internal/api/         localhost HTTP control API + client
internal/version/     release version
```
