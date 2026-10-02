$ErrorActionPreference = 'Continue'
$out = 'D:\Auto-nation\.work\tfcheck.txt'
$sb = [System.Text.StringBuilder]::new()

$tf = Get-ChildItem -Path 'C:\Program Files','C:\','C:\Users\monar\AppData\Local' -Filter 'terraform.exe' -Recurse -Depth 5 -ErrorAction SilentlyContinue
if ($tf) {
    foreach ($f in $tf) {
        [void]$sb.AppendLine('FOUND_TF: ' + $f.FullName)
        $v = & $f.FullName version 2>&1
        [void]$sb.AppendLine($v)
    }
} else {
    [void]$sb.AppendLine('TF_MISSING')
}

[System.IO.File]::WriteAllText($out, $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Output ('WROTE ' + $out)
