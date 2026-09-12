$ErrorActionPreference = 'Stop'
Add-Type @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class NativeUI {
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr FindWindow(IntPtr cls,string title);
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr parent,int id);
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr SendMessage(IntPtr hwnd,uint msg,IntPtr w,IntPtr l);
 [DllImport("user32.dll", CharSet=CharSet.Unicode, EntryPoint="SendMessageW")] public static extern IntPtr SetText(IntPtr hwnd,uint msg,IntPtr w,string text);
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr hwnd,StringBuilder name,int max);
}
'@
function Wait-Window($Title) {
 for ($i=0; $i -lt 300; $i++) {
  if ($App.HasExited) { throw "Desktop process exited: $($App.ExitCode)" }
  $Window = [NativeUI]::FindWindow([IntPtr]::Zero,$Title)
  if ($Window -ne [IntPtr]::Zero) { return $Window }
  Start-Sleep -Milliseconds 100
 }
 if (Test-Path $env:BAR_LAN_UI_TRACE) { Get-Content $env:BAR_LAN_UI_TRACE | Write-Host }
 $App.Refresh()
 Write-Host "App process $($App.Id), title $($App.MainWindowTitle), handle $($App.MainWindowHandle)"
 throw "Window not found: $Title"
}
function Assert-Control($Window,$Id,$Class) {
 $Control = [NativeUI]::GetDlgItem($Window,$Id)
 $Name = [Text.StringBuilder]::new(100)
 [void][NativeUI]::GetClassName($Control,$Name,100)
 if ($Name.ToString() -ine $Class) { throw "Control $Id should be $Class, got $Name" }
}
$env:BAR_LAN_UI_TRACE = Join-Path $env:RUNNER_TEMP "bar-lan-ui-trace.txt"
$App = Start-Process -FilePath ./dist/bar-lan.exe -PassThru
try {
 $HomeWindow = Wait-Window 'BAR LAN'
 Assert-Control $HomeWindow 101 'Edit'
 [void][NativeUI]::SetText([NativeUI]::GetDlgItem($HomeWindow,101),0xC,[IntPtr]::Zero,'Test Player')
 [void][NativeUI]::SendMessage($HomeWindow,0x111,[IntPtr]102,[IntPtr]::Zero)
 $HostWindow = Wait-Window 'BAR LAN — Host Game'
 Assert-Control $HostWindow 110 'ComboBox'
 Assert-Control $HostWindow 111 'ComboBox'
 Assert-Control $HostWindow 112 'Static'
 [void][NativeUI]::SendMessage($HostWindow,0x111,[IntPtr]108,[IntPtr]::Zero)
 [void][NativeUI]::SendMessage($HomeWindow,0x111,[IntPtr]103,[IntPtr]::Zero)
 $JoinWindow = Wait-Window 'BAR LAN — Join Game'
 Assert-Control $JoinWindow 118 'ListBox'
 [void][NativeUI]::SendMessage($JoinWindow,0x111,[IntPtr]108,[IntPtr]::Zero)
 [void][NativeUI]::SendMessage($HomeWindow,0x10,[IntPtr]::Zero,[IntPtr]::Zero)
 if (-not $App.WaitForExit(5000)) { throw 'Desktop did not exit normally' }
 Write-Host 'Native Windows home, host, join, dropdowns and navigation passed.'
} finally { if (-not $App.HasExited) { Stop-Process -Id $App.Id -Force } }
