# ClipShare — LAN Clipboard Sharing

Share your clipboard between an **Arch (or any Linux) desktop** and an
**Android phone**, with desktop-to-desktop support coming to Windows.

- **Desktop = server, phone = client.** A Go daemon (`clipshare`) watches the
  desktop clipboard and broadcasts changes to connected peers over WebSocket.
- **Native Kotlin/Compose Android app** with a Quick Settings tile, discovery,
  and recent history.
- Loop protection: the daemon tracks the last content it wrote itself and
  never rebroadcasts it, so no ping-pong loops.

## How it works

```
┌────────────────────────────┐         WebSocket (port 40403)          ┌────────────────────────┐
│  Desktop (Arch) Go daemon  │  ◄──────────────────────────────────────►  Android app          │
│  - watches clipboard       │        JSON: hello / clipboard / ping   │  - foreground service  │
│  - broadcasts changes      │                                          │  - QS tile to toggle  │
│  - writes received text    │                                          │  - clipboard sync     │
└────────────────────────────┘                                          └────────────────────────┘
        Discovery: mDNS (_clipshare._tcp) + UDP beacons (40404) + manual IP
        Local API: 127.0.0.1:40405 (status / send)
```

## Desktop (daemon)

### Build

```sh
cd daemon
go build -o clipshare ./cmd/clipshare          # Linux/Arch
GOOS=windows GOARCH=amd64 go build -o clipshare.exe ./cmd/clipshare   # Windows
```

### Commands

```sh
clipshare daemon           # run server + clipboard watcher (foreground)
clipshare daemon --no-watch   # receive + write remote content, never broadcast local clipboard
clipshare share [text]     # push text (or your current selection) to peers;
                           # starts a transient daemon if none is running
clipshare send <text>      # push text via a running daemon to connected peers
clipshare copy <text>      # set the local clipboard only
clipshare watch            # debug: print clipboard changes
clipshare status           # daemon status + connected clients
clipshare config --init    # write default config to ~/.config/clipshare/config.toml
```

### Push selected text with a keyboard shortcut

If you don't want the daemon watching (and auto-broadcasting) your clipboard,
use the on-demand `share` command: it reads your **primary selection** and
sends it, starting a short-lived daemon if needed, then exits.

```sh
# no daemon running, text is selected in any app:
clipshare share            # reads the primary selection and delivers it
clipshare share "hello"    # or pass text explicitly
```

Bind it to a key in your window manager. Examples:

```ini
# Sway/Hyprland (wlroots compositors)
bindsym $mod+Shift+C exec clipshare share

# X11 with xbindkeys (~/.xbindkeysrc)
"clipshare share"
  Mod4+c
```

> On Wayland, the primary selection (`wl-paste --primary`) works on wlroots
> compositors (Sway, Hyprland). On GNOME/KDE it is unavailable, so `share`
> falls back to your clipboard selection (last Ctrl+C) instead.

`share` does **not** touch your clipboard or your selection, and it never runs
a watcher. For a persistent daemon that only receives (phone → laptop) without
ever broadcasting, use `clipshare daemon --no-watch`.

> **Tip:** with autodiscovery on the phone, `clipshare share` is the only
> command you need on the laptop. It briefly starts a daemon, the phone
> auto-detects and connects, receives the text into its clipboard, and the
> daemon exits ~5s later. Nothing stays running.

### Run under systemd (user unit)

```ini
# ~/.config/systemd/user/clipshare.service
[Unit]
Description=ClipShare clipboard daemon
After=graphical-session.target

[Service]
Type=simple
ExecStart=/home/you/bin/clipshare daemon
Restart=on-failure

[Install]
WantedBy=default.target
```

```sh
systemctl --user enable --now clipshare
```

### Config (`~/.config/clipshare/config.toml`)

| key         | default   | meaning                                    |
|-------------|-----------|--------------------------------------------|
| `device_name` | hostname | name shown to clients                      |
| `mdns`      | `true`    | advertise via mDNS `_clipshare._tcp`       |
| `broadcast` | `5`       | UDP beacon interval in seconds             |
| `watch`     | `300`     | clipboard poll interval in ms              |
| `token`     | `""`      | shared secret; if set, clients must send it |
| `peers`     | `[]`      | desktop-to-desktop peer hosts (`ip[:port]`)|
| `[server] port` | `40403` | WebSocket port                         |
| `[api] port`    | `40405` | localhost control API                  |

Ports: WS/TCP `40403`, UDP beacon `40404`, localhost API `40405` (127.0.0.1 only).

## Android app

- `android/` — Gradle Kotlin DSL + Jetpack Compose Material 3 (minSdk 26, targetSdk 36).
- `SyncService`: foreground service owning discovery + the WebSocket connection
  with exponential-backoff reconnect, clipboard writer, and history.
- **Auto-connect:** the service browses for daemons (mDNS `_clipshare._tcp` +
  UDP beacons on 40404) and connects to the first one it finds. **No server
  address is needed** — flip the switch and it just works. A discovered daemon
  that appears briefly (like the `share` one-shot) is picked up automatically.
- Quick Settings tile: toggle sync on/off.
- Manual `ip:port` in Settings still works for overriding auto-connect.

### Build

```sh
cd android
./gradlew :app:assembleDebug   # requires Android SDK with platforms;android-36
```

### Install

```sh
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

Add the Quick Settings tile: Settings → Quick Settings → ClipShare.

## Important Android platform constraint

**Android 10+ blocks background apps from reading the clipboard.**

- **Phone → laptop** push works **only while the app is in the foreground**
  (the activity watches the clipboard in `onResume`). This is a platform
  restriction; a full background read would require an Accessibility Service
  workaround (out of scope for MVP).
- **Laptop → phone** works 24/7 while the sync service is running: writing to
  the clipboard is allowed in the background.
- Android 15+ caps the `dataSync` foreground service at 6 hours. For now the
  service is started with `START_STICKY`; the tile re-enables it. A future
  version can switch to the `specialUse` type.

## Security

No TLS for the MVP (LAN, self-signed cert friction on Android). Set `token`
in the daemon config and in the app's Settings to require a shared secret on
untrusted networks. Future: WSS/mTLS.

## Protocol

JSON over WebSocket (port 40403):

| message     | payload                      |
|-------------|------------------------------|
| `hello`     | `{name, platform, version}`  |
| `clipboard` | `{text, ts, from}`           |
| `ping`/`pong` | —                          |
| `error`     | `{code, msg}`                |
