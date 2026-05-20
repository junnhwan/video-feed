param(
    [Parameter(Mandatory = $true)]
    [string[]]$Jtl,

    [string[]]$Name = @(),
    [string]$Out = ""
)

$ErrorActionPreference = "Stop"

function Get-Percentile([double[]]$Values, [double]$Percentile) {
    if ($Values.Count -eq 0) {
        return 0
    }
    $sorted = $Values | Sort-Object
    $index = [Math]::Ceiling($sorted.Count * $Percentile) - 1
    if ($index -lt 0) {
        $index = 0
    }
    if ($index -ge $sorted.Count) {
        $index = $sorted.Count - 1
    }
    return [double]$sorted[$index]
}

function Get-JtlStats([string]$Path, [string]$Label) {
    $rows = Import-Csv $Path
    $samples = @($rows)
    $elapsed = @($samples | ForEach-Object { [double]$_.elapsed })
    $timestamps = @($samples | ForEach-Object { [double]$_.timeStamp })
    $errors = @($samples | Where-Object { $_.success -ne "true" }).Count
    $durationSeconds = 0
    if ($timestamps.Count -gt 1) {
        $durationSeconds = (($timestamps | Measure-Object -Maximum).Maximum - ($timestamps | Measure-Object -Minimum).Minimum) / 1000
    }
    $throughput = 0
    if ($durationSeconds -gt 0) {
        $throughput = $samples.Count / $durationSeconds
    }

    [PSCustomObject]@{
        Label = $Label
        Samples = $samples.Count
        Errors = $errors
        ErrorRate = if ($samples.Count -gt 0) { $errors * 100.0 / $samples.Count } else { 0 }
        AvgMS = if ($elapsed.Count -gt 0) { ($elapsed | Measure-Object -Average).Average } else { 0 }
        P50MS = Get-Percentile $elapsed 0.50
        P95MS = Get-Percentile $elapsed 0.95
        P99MS = Get-Percentile $elapsed 0.99
        Throughput = $throughput
        DurationSeconds = $durationSeconds
    }
}

function Expand-ArgumentList([string[]]$Values) {
    $expanded = @()
    foreach ($value in $Values) {
        foreach ($part in ($value -split ",")) {
            $trimmed = $part.Trim()
            if ($trimmed) {
                $expanded += $trimmed
            }
        }
    }
    return $expanded
}

$Jtl = Expand-ArgumentList $Jtl
$Name = Expand-ArgumentList $Name

$stats = @()
for ($i = 0; $i -lt $Jtl.Count; $i++) {
    $path = (Resolve-Path $Jtl[$i]).Path
    $label = if ($i -lt $Name.Count -and $Name[$i]) {
        $Name[$i]
    } else {
        [System.IO.Path]::GetFileNameWithoutExtension($path)
    }
    $stats += Get-JtlStats -Path $path -Label $label
}

$lines = @()
$lines += "| scenario | samples | errors | error_rate | avg_ms | p50_ms | p95_ms | p99_ms | throughput_req_s | duration_s |"
$lines += "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|"
foreach ($item in $stats) {
    $lines += "| $($item.Label) | $($item.Samples) | $($item.Errors) | $([Math]::Round($item.ErrorRate, 2))% | $([Math]::Round($item.AvgMS, 2)) | $([Math]::Round($item.P50MS, 2)) | $([Math]::Round($item.P95MS, 2)) | $([Math]::Round($item.P99MS, 2)) | $([Math]::Round($item.Throughput, 2)) | $([Math]::Round($item.DurationSeconds, 2)) |"
}
$content = $lines -join [Environment]::NewLine

if ($Out) {
    $parent = Split-Path -Parent $Out
    if ($parent) {
        New-Item -ItemType Directory -Force $parent | Out-Null
    }
    Set-Content -Path $Out -Value $content -Encoding UTF8
}

$content
