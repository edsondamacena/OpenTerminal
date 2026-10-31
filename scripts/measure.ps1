[CmdletBinding()]
param(
    [string]$ExecutablePath,
    [int]$ProcessId = 0,
    [ValidateRange(0, 3600)]
    [int]$IdleSeconds = 120,
    [string]$OutputJson
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $repoRoot 'build\bin'

if (-not $ExecutablePath) {
    $installer = Get-ChildItem -LiteralPath $binDir -Filter '*installer*.exe' -File -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    $app = Get-ChildItem -LiteralPath $binDir -Filter '*.exe' -File -ErrorAction SilentlyContinue |
        Where-Object Name -NotLike '*installer*' |
        Sort-Object @{ Expression = { $_.Name -like '*e2e*' } }, LastWriteTime -Descending |
        Select-Object -First 1
    if (-not $app) {
        throw "Nenhum executável encontrado em $binDir. Faça o build antes da medição."
    }
    $ExecutablePath = $app.FullName
} else {
    $ExecutablePath = (Resolve-Path -LiteralPath $ExecutablePath).Path
    $installer = Get-ChildItem -LiteralPath $binDir -Filter '*installer*.exe' -File -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
}

$launched = $false
if ($ProcessId -eq 0) {
    Write-Host 'Abra exatamente uma aba local na janela iniciada; a contagem ociosa começa agora.'
    $process = Start-Process -FilePath $ExecutablePath -PassThru
    $ProcessId = $process.Id
    $launched = $true
}

Start-Sleep -Seconds $IdleSeconds

$processes = Get-CimInstance Win32_Process
$children = @{}
foreach ($item in $processes) {
    if (-not $children.ContainsKey([int]$item.ParentProcessId)) {
        $children[[int]$item.ParentProcessId] = [System.Collections.Generic.List[int]]::new()
    }
    $children[[int]$item.ParentProcessId].Add([int]$item.ProcessId)
}

$tree = [System.Collections.Generic.HashSet[int]]::new()
$queue = [System.Collections.Generic.Queue[int]]::new()
$queue.Enqueue($ProcessId)
while ($queue.Count -gt 0) {
    $current = $queue.Dequeue()
    if (-not $tree.Add($current)) { continue }
    if ($children.ContainsKey($current)) {
        foreach ($child in $children[$current]) { $queue.Enqueue($child) }
    }
}

$wanted = @($ProcessId)
foreach ($item in $processes) {
    if ($tree.Contains([int]$item.ProcessId) -and $item.Name -ieq 'msedgewebview2.exe') {
        $wanted += [int]$item.ProcessId
    }
}
$wanted = @($wanted | Sort-Object -Unique)
$perf = Get-CimInstance Win32_PerfFormattedData_PerfProc_Process |
    Where-Object { $wanted -contains [int]$_.IDProcess }
$privateWorkingSet = [int64](($perf | Measure-Object -Property WorkingSetPrivate -Sum).Sum)

$sizeFile = if ($installer) { $installer } else { Get-Item -LiteralPath $ExecutablePath }
$result = [pscustomobject]@{
    measuredAtUtc = [DateTime]::UtcNow.ToString('o')
    artifact = $sizeFile.FullName
    artifactKind = if ($installer) { 'installer' } else { 'executable' }
    artifactBytes = [int64]$sizeFile.Length
    artifactMiB = [math]::Round($sizeFile.Length / 1MB, 2)
    appProcessId = $ProcessId
    webViewProcessCount = @($wanted | Where-Object { $_ -ne $ProcessId }).Count
    privateWorkingSetBytes = $privateWorkingSet
    privateWorkingSetMiB = [math]::Round($privateWorkingSet / 1MB, 2)
    idleSeconds = $IdleSeconds
    launchedByScript = $launched
}

if ($OutputJson) {
    $jsonPath = [IO.Path]::GetFullPath($OutputJson, $repoRoot)
    $result | ConvertTo-Json | Set-Content -LiteralPath $jsonPath -Encoding utf8NoBOM
}
$result
