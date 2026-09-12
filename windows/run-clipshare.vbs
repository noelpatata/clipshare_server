Option Explicit

Dim shell, fso, executable, configPath, command
Set shell = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")

executable = fso.BuildPath(fso.GetParentFolderName(WScript.ScriptFullName), "clipshare.exe")
configPath = shell.ExpandEnvironmentStrings("%APPDATA%\clipshare\config.toml")

If Not fso.FileExists(executable) Then
  WScript.Quit 2
End If

shell.Environment("Process")("CLIPSHARE_CONFIG") = configPath
command = """" & executable & """ daemon"
shell.Run command, 0, False
