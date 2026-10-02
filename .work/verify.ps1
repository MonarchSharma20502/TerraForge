$ErrorActionPreference = 'Continue'
$go = "$env:ProgramFiles\Go\bin\go.exe"
$tf = 'C:\Users\monar\AppData\Local\Microsoft\WinGet\Packages\Hashicorp.Terraform_Microsoft.Winget.Source_8wekyb3d8bbwe\terraform.exe'
$sb = [System.Text.StringBuilder]::new()

[void]$sb.AppendLine('GO: ' + (& $go version 2>&1))
[void]$sb.AppendLine('TF: ' + (& $tf version 2>&1))
[void]$sb.AppendLine('GO_NET: ' + (& $go env GOPROXY 2>&1))

[System.IO.File]::WriteAllText('D:\Auto-nation\.work\verify.txt', $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Output 'DONE'
