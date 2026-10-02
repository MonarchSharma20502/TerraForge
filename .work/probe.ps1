$ErrorActionPreference = 'Continue'
$out = 'D:\Auto-nation\.work\toolcheck.txt'
New-Item -ItemType Directory -Force -Path 'D:\Auto-nation\.work' | Out-Null
$sb = [System.Text.StringBuilder]::new()

$goExe = "$env:ProgramFiles\Go\bin\go.exe"
if (Test-Path $goExe) {
    [void]$sb.AppendLine('GO_INSTALLED')
    $gv = & $goExe version 2>&1
    [void]$sb.AppendLine($gv)
    $gp = & $goExe env GOPATH 2>&1
    [void]$sb.AppendLine('GOPATH=' + $gp)
} else {
    [void]$sb.AppendLine('GO_MISSING')
}

$tf = Get-ChildItem -Path 'C:\Program Files','C:\','C:\Users\monar\AppData\Local' -Filter 'terraform.exe' -Recurse -Depth 4 -ErrorAction SilentlyContinue
if ($tf) { foreach ($f in $tf) { [void]$sb.AppendLine('FOUND_TF: ' + $f.FullName) } } else { [void]$sb.AppendLine('TF_MISSING') }

[void]$sb.AppendLine('PATH_GO=' + (($env:PATH -split ';') -match 'Go'))
[System.IO.File]::WriteAllText($out, $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Output ('WROTE ' + $out)
