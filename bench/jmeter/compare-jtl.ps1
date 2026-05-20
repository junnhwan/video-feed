param(
    [Parameter(Mandatory = $true)]
    [string]$Baseline,

    [Parameter(Mandatory = $true)]
    [string]$Candidate,

    [string]$BaselineName = "baseline",
    [string]$CandidateName = "candidate",
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

function Get-JtlStats([string]$Path, [string]$Name) {
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
        Name = $Name
        Samples = $samples.Count
        Errors = $errors
        ErrorRate = if ($samples.Count -gt 0) { $errors * 100.0 / $samples.Count } else { 0 }
        AvgMS = if ($elapsed.Count -gt 0) { ($elapsed | Measure-Object -Average).Average } else { 0 }
        P50MS = Get-Percentile $elapsed 0.50
        P90MS = Get-Percentile $elapsed 0.90
        P95MS = Get-Percentile $elapsed 0.95
        P99MS = Get-Percentile $elapsed 0.99
        Throughput = $throughput
    }
}

function Get-ChangePct([double]$BaselineValue, [double]$CandidateValue) {
    if ($BaselineValue -eq 0) {
        return 0
    }
    return (($CandidateValue - $BaselineValue) / $BaselineValue) * 100
}

$baselineStats = Get-JtlStats $Baseline $BaselineName
$candidateStats = Get-JtlStats $Candidate $CandidateName

$lines = @()
$lines += "# JMeter Comparison"
$lines += ""
$lines += "| item | $BaselineName | $CandidateName | change |"
$lines += "|---|---:|---:|---:|"
$lines += "| samples | $($baselineStats.Samples) | $($candidateStats.Samples) | - |"
$lines += "| errors | $($baselineStats.Errors) | $($candidateStats.Errors) | - |"
$lines += "| error_rate | $([Math]::Round($baselineStats.ErrorRate, 2))% | $([Math]::Round($candidateStats.ErrorRate, 2))% | - |"
$lines += "| avg_ms | $([Math]::Round($baselineStats.AvgMS, 2)) | $([Math]::Round($candidateStats.AvgMS, 2)) | $([Math]::Round((Get-ChangePct $baselineStats.AvgMS $candidateStats.AvgMS), 2))% |"
$lines += "| p50_ms | $([Math]::Round($baselineStats.P50MS, 2)) | $([Math]::Round($candidateStats.P50MS, 2)) | $([Math]::Round((Get-ChangePct $baselineStats.P50MS $candidateStats.P50MS), 2))% |"
$lines += "| p95_ms | $([Math]::Round($baselineStats.P95MS, 2)) | $([Math]::Round($candidateStats.P95MS, 2)) | $([Math]::Round((Get-ChangePct $baselineStats.P95MS $candidateStats.P95MS), 2))% |"
$lines += "| p99_ms | $([Math]::Round($baselineStats.P99MS, 2)) | $([Math]::Round($candidateStats.P99MS, 2)) | $([Math]::Round((Get-ChangePct $baselineStats.P99MS $candidateStats.P99MS), 2))% |"
$lines += "| throughput_req_s | $([Math]::Round($baselineStats.Throughput, 2)) | $([Math]::Round($candidateStats.Throughput, 2)) | $([Math]::Round((Get-ChangePct $baselineStats.Throughput $candidateStats.Throughput), 2))% |"
$content = $lines -join [Environment]::NewLine

if ($Out) {
    $parent = Split-Path -Parent $Out
    if ($parent) {
        New-Item -ItemType Directory -Force $parent | Out-Null
    }
    Set-Content -Path $Out -Value $content -Encoding UTF8
}

$content
