$ErrorActionPreference = 'Continue'
$log = 'D:\Auto-nation\.work\winget-tf.txt'
$tf = 'C:\Program Files\Terraform\terraform.exe'

& winget install --id Hashicorp.Terraform -e -h --accept-package-agreements --accept-source-agreements 2>&1 |
    Out-File -FilePath $log -Encoding utf8

$sb = [System.Text.StringBuilder]::new()
if (Test-Path $tf) {
    [void]$sb.AppendLine('TF_INSTALLED')
    $v = & $tf version 2>&1
    [void]$sb.AppendLine($v)
} else {
    [void]$sb.AppendLine('TF_STILL_MISSING')
    $found = Get-ChildItem -Path 'C:\Program Files','C:\Users\monar\AppData\Local' -Filter 'terraform.exe' -Recurse -Depth 5 -ErrorAction SilentlyContinue
    if ($found) { foreach ($f in $found) { [void]$sb.AppendLine('FOUND: ' + $f.FullName) } }
}
[System.IO.File]::WriteAllText('D:\Auto-nation\.work\tfstatus.txt', $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Output 'DONE'
