$ErrorActionPreference = 'Stop'
$go = "$env:ProgramFiles\Go\bin\go.exe"
$log = 'D:\Auto-nation\.work\init.txt'
$sb = [System.Text.StringBuilder]::new()

try {
    Set-Location 'D:\Auto-nation'
    & $go mod init github.com/autonation/autonation 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    & $go get github.com/hashicorp/hcl/v2@latest 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    & $go get github.com/hashicorp/hcl/v2/hclwrite@latest 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    & $go get gopkg.in/yaml.v3@latest 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    & $go get github.com/spf13/cobra@latest 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    & $go get github.com/invopop/jsonschema@latest 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    & $go mod tidy 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    [void]$sb.AppendLine('INIT_OK')
} catch {
    [void]$sb.AppendLine('INIT_FAIL: ' + $_.Exception.Message)
}

[System.IO.File]::WriteAllText($log, $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Output 'DONE'
