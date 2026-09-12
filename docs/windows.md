# Windows Installation

The Windows clipboard watcher must run in the logged-in user's desktop
session. Install ClipShare as a current-user application and use a Task
Scheduler **logon task**. Do not install it as a Windows service: service
sessions cannot reliably receive the interactive clipboard notifications.

## Maintainer builds

To generate an installer without publishing a release, open the repository's
GitHub Actions page, select the **Release** workflow, choose **Run workflow**,
and select the branch. The `Generate Windows installer` job uploads a versioned
installer artifact that can be downloaded from that workflow run.

When a `release/<version>` pull request is merged into `main`, the same job
runs automatically. The release job then downloads its installer artifact and
publishes it together with the Linux and Windows binaries.

## Recommended: installer

1. Download `clipshare-<version>-windows-installer.exe` from the release page.
2. Run the installer and approve the administrator prompt. Administrator
   access is required to configure the Windows Firewall rules.
3. It installs to `%LOCALAPPDATA%\ClipShare`.
4. The installer creates `%APPDATA%\clipshare\config.toml` if it does not
   already exist.
5. The installer creates and starts a `ClipShare` logon task for the current
   Windows user.

The installer preserves the config and certificates during upgrades. It keeps
the uninstaller in the install directory; uninstalling removes the scheduled
task and program files but intentionally keeps `%APPDATA%\clipshare` so that
settings and certificates are not lost.

Verify the task:

```powershell
Get-ScheduledTask -TaskName ClipShare
Get-ScheduledTaskInfo -TaskName ClipShare
```

Check that the daemon is running:

```powershell
Get-Process clipshare
```

## Manual installation

For a portable install, place `clipshare.exe` and `run-clipshare.vbs` from the
repository's `windows` directory in the same directory, for example
`%LOCALAPPDATA%\ClipShare`. Create the config once:

```powershell
& "$env:LOCALAPPDATA\ClipShare\clipshare.exe" config --init
```

Register the current user's interactive logon task:

```powershell
$launcher = "$env:LOCALAPPDATA\ClipShare\run-clipshare.vbs"
schtasks.exe /Create /TN ClipShare /TR "`"$env:WINDIR\System32\wscript.exe`" `"$launcher`"" /SC ONLOGON /RL LIMITED /F
schtasks.exe /Run /TN ClipShare
```

The launcher starts `clipshare.exe daemon` hidden. The CLI automatically uses
`%APPDATA%\clipshare\config.toml` on Windows. It resolves the executable
relative to the VBS file, so the install directory can contain spaces and does
not need to be hard-coded.

## LAN firewall

The installer creates these inbound rules for the `Private` network profile:

- `ClipShare WebSocket TCP 40403`
- `ClipShare UDP Beacon 40404`

If you installed manually, or the rules were removed, create them from an
elevated PowerShell window:

```powershell
New-NetFirewallRule -DisplayName "ClipShare WebSocket TCP 40403" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 40403 -Profile Private
New-NetFirewallRule -DisplayName "ClipShare UDP Beacon 40404" -Direction Inbound -Action Allow -Protocol UDP -LocalPort 40404 -Profile Private
```

Use the `Private` profile on a trusted home or office LAN. Do not expose these
ports on public networks. Remove the rules later with:

```powershell
Remove-NetFirewallRule -DisplayName "ClipShare WebSocket TCP 40403"
Remove-NetFirewallRule -DisplayName "ClipShare UDP Beacon 40404"
```

## Task control

```powershell
schtasks.exe /Query /TN ClipShare /V /FO LIST
schtasks.exe /Run /TN ClipShare
schtasks.exe /End /TN ClipShare
schtasks.exe /Delete /TN ClipShare /F
```

After deleting the task, start the daemon manually with:

```powershell
& "$env:LOCALAPPDATA\ClipShare\clipshare.exe" daemon
```

## Troubleshooting

- **The task exists but no process is running:** run `schtasks.exe /Run /TN
  ClipShare`, then inspect `Get-ScheduledTaskInfo -TaskName ClipShare`.
- **The task fails after moving the install directory:** recreate it with the
  manual command above; the task stores the VBS path.
- **The daemon runs but the phone cannot connect:** verify the daemon process,
  allow TCP `40403` and UDP `40404` on the Private firewall profile, and check
  that both devices are on the same LAN.
- **The clipboard is not captured:** ensure the task uses `/SC ONLOGON` and
  `/RL LIMITED`, and run it from the same Windows account that owns the desktop.
- **A previous administrator install exists:** uninstall that copy first, or
  remove its old `ClipShare` task before creating the current-user task.
