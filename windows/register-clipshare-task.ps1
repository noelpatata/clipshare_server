$ErrorActionPreference = 'Stop'

$taskName = 'ClipShare'
$userId = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
$launcher = Join-Path $PSScriptRoot 'run-clipshare.vbs'
$wscript = Join-Path $env:WINDIR 'System32\wscript.exe'

$action = New-ScheduledTaskAction `
    -Execute $wscript `
    -Argument ('"{0}"' -f $launcher)
$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal `
    -UserId $userId `
    -LogonType Interactive `
    -RunLevel Limited

Register-ScheduledTask `
    -TaskName $taskName `
    -Action $action `
    -Trigger $trigger `
    -Principal $principal `
    -Force | Out-Null

Start-ScheduledTask -TaskName $taskName
