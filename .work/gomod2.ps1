$ErrorActionPreference = 'Continue'
$go = "$env:ProgramFiles\Go\bin\go.exe"
$log = 'D:\Auto-nation\.work\init.txt'
$sb = [System.Text.StringBuilder]::new()

Set-Location 'D:\Auto-nation'

function Run($label, $cmd) {
    [void]$sb.AppendLine('=== ' + $label)
    $args = $cmd[1..($cmd.Length - 1)]
    & $cmd[0] @args 2>&1 | ForEach-Object { [void]$sb.AppendLine($_) }
    [void]$sb.AppendLine('=== exit')
}

if (-not (Test-Path 'D:\Auto-nation\go.mod')) {
    Run 'mod init' @($go, 'mod', 'init', 'github.com/autonation/autonation')
}
Run 'get hcl' @($go, 'get', 'github.com/hashicorp/hcl/v2@latest')
Run 'get hclwrite' @($go, 'get', 'github.com/hashicorp/hcl/v2/hclwrite@latest')
Run 'get yaml' @($go, 'get', 'gopkg.in/yaml.v3@latest')
Run 'get cobra' @($go, 'get', 'github.com/spf13/cobra@latest')
Run 'get jsonschema' @($go, 'get', 'github.com/invopop/jsonschema@latest')
Run 'mod tidy' @($go, 'mod', 'tidy')

[System.IO.File]::WriteAllText($log, $sb.ToString(), [System.Text.Encoding]::UTF8)
Write-Output 'DONE'
