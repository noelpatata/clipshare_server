!define APP_NAME "ClipShare"
!define COMPANY_NAME "ClipShare"
!define APP_VERSION "__VERSION__"

Name "${APP_NAME}"
OutFile "..\dist\clipshare-${APP_VERSION}-windows-installer.exe"
InstallDir "$LOCALAPPDATA\ClipShare"
RequestExecutionLevel admin
ShowInstDetails show
ShowUninstDetails show

!include "MUI2.nsh"

VIProductVersion "${APP_VERSION}.0"
VIAddVersionKey /LANG=1033 "ProductName" "${APP_NAME}"
VIAddVersionKey /LANG=1033 "CompanyName" "${COMPANY_NAME}"
VIAddVersionKey /LANG=1033 "FileDescription" "ClipShare clipboard sharing daemon"
VIAddVersionKey /LANG=1033 "FileVersion" "${APP_VERSION}"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "Install"
  SetOutPath "$INSTDIR"

  ; Install the Windows release binary next to the launcher.
  File "..\dist\clipshare-${APP_VERSION}-windows-amd64.exe"
  Rename "$INSTDIR\clipshare-${APP_VERSION}-windows-amd64.exe" "$INSTDIR\clipshare.exe"

  ; Keep the user's config when the installer is run again for an upgrade.
  CreateDirectory "$APPDATA\clipshare"
  IfFileExists "$APPDATA\clipshare\config.toml" config_exists
    SetOutPath "$APPDATA\clipshare"
    File "..\config.toml"
  config_exists:

  ; The launcher derives the executable path from its own location, so the
  ; installer can use any per-user directory safely.
  SetOutPath "$INSTDIR"
  File "..\windows\run-clipshare.vbs"

  ; LAN access requires administrator rights. Limit these rules to Private
  ; networks; the daemon should not be exposed on public networks.
  ExecWait '"$SYSDIR\netsh.exe" advfirewall firewall delete rule name="ClipShare WebSocket TCP 40403"'
  ExecWait '"$SYSDIR\netsh.exe" advfirewall firewall add rule name="ClipShare WebSocket TCP 40403" dir=in action=allow protocol=TCP localport=40403 profile=private'
  ExecWait '"$SYSDIR\netsh.exe" advfirewall firewall delete rule name="ClipShare UDP Beacon 40404"'
  ExecWait '"$SYSDIR\netsh.exe" advfirewall firewall add rule name="ClipShare UDP Beacon 40404" dir=in action=allow protocol=UDP localport=40404 profile=private'

  ; Omitting /RU makes schtasks use the account running this installer without
  ; requiring a password. ONLOGON runs in that user's desktop session, and
  ; /RL LIMITED prevents the elevated installer token becoming the daemon's
  ; task privilege.
  ExecWait '"$SYSDIR\schtasks.exe" /Create /TN "ClipShare" /TR "$SYSDIR\wscript.exe $\"$INSTDIR\run-clipshare.vbs$\"" /SC ONLOGON /RL LIMITED /F' $0
  StrCmp $0 0 task_created
  Goto task_failed

  task_created:
  ; Start it now so the user does not need to log out and back in.
  ExecWait '"$SYSDIR\schtasks.exe" /Run /TN "ClipShare"'
  Goto done

  task_failed:
    MessageBox MB_ICONEXCLAMATION "ClipShare was installed, but the automatic login task could not be created. See the Windows installation guide for the manual command."
  done:
SectionEnd

Section "Uninstall"
  ExecWait '"$SYSDIR\schtasks.exe" /End /TN "ClipShare"'
  ExecWait '"$SYSDIR\schtasks.exe" /Delete /TN "ClipShare" /F'
  ExecWait '"$SYSDIR\netsh.exe" advfirewall firewall delete rule name="ClipShare WebSocket TCP 40403"'
  ExecWait '"$SYSDIR\netsh.exe" advfirewall firewall delete rule name="ClipShare UDP Beacon 40404"'
  Delete "$INSTDIR\clipshare.exe"
  Delete "$INSTDIR\run-clipshare.vbs"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  ; Configuration and certificates are user data and are intentionally kept.
SectionEnd

Function .onInstSuccess
  WriteUninstaller "$INSTDIR\uninstall.exe"
FunctionEnd
