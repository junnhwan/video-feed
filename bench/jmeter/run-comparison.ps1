param(
    [Parameter(Mandatory = $true)]
    [string]$Manifest,

    [string]$Config = "configs/config.yaml",
    [string]$BaseHost = "127.0.0.1",
    [string]$BasePort = "8080",
    [string]$BaseProtocol = "http",
    [int]$Threads = 20,
    [int]$Duration = 60,
    [int]$RampUp = 10,
    [int]$Limit = 20,
    [int]$CommentDelayMS = 7000,
    [string]$OutDir = "bench/results"
)

$ErrorActionPreference = "Stop"

function Resolve-RepoPath([string]$Path) {
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $Path
    }
    return (Resolve-Path $Path).Path
}

function Reset-ReportPath([string]$Path) {
    if (Test-Path $Path) {
        Remove-Item -LiteralPath $Path -Recurse -Force
    }
}

function Invoke-JMeterScenario(
    [string]$Label,
    [string]$Scenario,
    [string]$StateMode
) {
    if ($StateMode) {
        go run ./cmd/benchstate -config $Config -manifest $Manifest -mode $StateMode
    }

    $jtl = Join-Path $OutDir "$Label.jtl"
    $html = Join-Path $OutDir "$Label-html"
    $log = Join-Path $OutDir "$Label-jmeter.log"
    Reset-ReportPath $jtl
    Reset-ReportPath $html
    Reset-ReportPath $log

    jmeter -n -t bench/jmeter/video-feed-benchmark.jmx `
        "-Jscenario=$Scenario" `
        "-Jbase_protocol=$BaseProtocol" `
        "-Jbase_host=$BaseHost" `
        "-Jbase_port=$BasePort" `
        "-Jusers_csv=$usersCsv" `
        "-Jvideos_csv=$videosCsv" `
        "-Jhot_as_of=$($manifestData.hot_as_of)" `
        "-Jpassword=$($manifestData.password)" `
        "-Jthreads=$Threads" `
        "-Jduration=$Duration" `
        "-Jrampup=$RampUp" `
        "-Jlimit=$Limit" `
        "-Jcomment_delay_ms=$CommentDelayMS" `
        -l $jtl -j $log -e -o $html
}

New-Item -ItemType Directory -Force $OutDir | Out-Null
$Manifest = Resolve-RepoPath $Manifest
$manifestData = Get-Content $Manifest | ConvertFrom-Json
$usersCsv = Resolve-RepoPath $manifestData.users_csv
$videosCsv = Resolve-RepoPath $manifestData.videos_csv

Invoke-JMeterScenario -Label "hot-db" -Scenario "hot" -StateMode "db"
Invoke-JMeterScenario -Label "hot-redis" -Scenario "hot" -StateMode "hot"
Invoke-JMeterScenario -Label "detail-cold" -Scenario "detail" -StateMode "detail-cold"
Invoke-JMeterScenario -Label "detail-hot" -Scenario "detail" -StateMode ""
Invoke-JMeterScenario -Label "latest" -Scenario "latest" -StateMode ""
Invoke-JMeterScenario -Label "comment" -Scenario "comment" -StateMode ""

Write-Host "JMeter reports written to $OutDir"
