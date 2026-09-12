$ErrorActionPreference = 'Stop'
Add-Type @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class NativeUI {
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr FindWindow(string cls,string title);
 [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr parent,int id);
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr SendMessage(IntPtr hwnd,uint msg,IntPtr w,IntPtr l);
 [DllImport("user32.dll", CharSet=CharSet.Unicode, EntryPoint="SendMessageW")] public static extern IntPtr SetText(IntPtr hwnd,uint msg,IntPtr w,string text);
 [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr hwnd,StringBuilder name,int max);
}
'@
function Wait-Window($Title) {
 for ($i=0; $i -lt 60; $i++) {
  if ($App.HasExited) { throw "Desktop process exited: $($App.ExitCode)" }
  $Window = [NativeUI]::FindWindow($null,$Title)
  if ($Window -ne [IntPtr]::Zero) { return $Window }
  Start-Sleep -Milliseconds 100
 }
 throw "Window not found: $Title"
}
function Assert-Control($Window,$Id,$Class) {
 $Control = [NativeUI]::GetDlgItem($Window,$Id)
 $Name = [Text.StringBuilder]::new(100)
 [void][NativeUI]::GetClassName($Control,$Name,100)
 if ($Name.ToString() -ine $Class) { throw "Control $Id should be $Class, got $Name" }
}
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
