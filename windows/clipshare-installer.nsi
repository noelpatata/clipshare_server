!define APP_NAME "ClipShare"
!define COMPANY_NAME "ClipShare"
!define APP_VERSION "${VERSION}"

Name "${APP_NAME}"
OutFile "..\dist\clipshare-${APP_VERSION}-windows-installer.exe"
InstallDir "$LOCALAPPDATA\ClipShare"
RequestExecutionLevel user
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

  ; Run as the current interactive user. /IT is intentional: clipboard
  ; notifications are tied to the logged-in desktop and do not work reliably
  ; from a Windows service session.
  ExecWait '"$SYSDIR\schtasks.exe" /Create /TN "ClipShare" /TR "$SYSDIR\wscript.exe $\"$INSTDIR\run-clipshare.vbs$\"" /SC ONLOGON /IT /RL LIMITED /F' $0
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
  Delete "$INSTDIR\clipshare.exe"
  Delete "$INSTDIR\run-clipshare.vbs"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  ; Configuration and certificates are user data and are intentionally kept.
SectionEnd

Function .onInstSuccess
  WriteUninstaller "$INSTDIR\uninstall.exe"
FunctionEnd
