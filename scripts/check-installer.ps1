[CmdletBinding()]
param(
    [string]$InstallerPath,
    [ValidateRange(5, 60)]
    [int]$AliveSeconds = 10,
    [string]$OutputJson
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0

$repoRoot = Split-Path -Parent $PSScriptRoot
$uninstallKeyName = 'OpenTerminalOpenTerminal'
$desktopShortcut = Join-Path $env:PUBLIC 'Desktop\OpenTerminal.lnk'
$startMenuShortcut = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\OpenTerminal.lnk'
$realDataDir = Join-Path $env:APPDATA 'OpenTerminal'
$testDataDir = Join-Path ([IO.Path]::GetFullPath($env:TEMP)) ('openterminal-installer-check-' + [guid]::NewGuid().ToString('N'))

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

function Get-OpenTerminalRegistration {
    foreach ($root in @(
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall',
        'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall'
    )) {
        $path = Join-Path $root $uninstallKeyName
        if (Test-Path -LiteralPath $path) {
            $value = Get-ItemProperty -LiteralPath $path
            return [pscustomobject]@{ Path = $path; Value = $value }
        }
    }
    return $null
}

function Get-ListingHash {
    param([string]$Path)
    $root = [IO.Path]::GetFullPath($Path).TrimEnd('\')
    $lines = foreach ($item in Get-ChildItem -LiteralPath $root -Force -Recurse | Sort-Object FullName) {
        $relative = $item.FullName.Substring($root.Length).TrimStart('\')
        $kind = if ($item.PSIsContainer) { 'D' } else { 'F' }
        $length = if ($item.PSIsContainer) { 0 } else { $item.Length }
        '{0}|{1}|{2}' -f $kind, $relative, $length
    }
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes(($lines -join "`n"))
        return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

function Wait-Until {
    param([scriptblock]$Condition, [int]$TimeoutSeconds, [string]$Failure)
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        if (& $Condition) { return }
        Start-Sleep -Milliseconds 250
    } while ([DateTime]::UtcNow -lt $deadline)
    throw $Failure
}

function Wait-NsisChildren {
    $deadline = [DateTime]::UtcNow.AddSeconds(60)
    do {
        $children = @(Get-Process -ErrorAction SilentlyContinue | Where-Object Name -Like 'Au_*')
        if ($children.Count -eq 0) { return }
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'O processo auxiliar Au_.exe do desinstalador não terminou em 60 segundos.'
}

function Stop-TestProcess {
    param($Process)
    if ($null -eq $Process -or $Process.HasExited) { return }
    $null = $Process.CloseMainWindow()
    if (-not $Process.WaitForExit(10000)) {
        Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
        $Process.WaitForExit(5000) | Out-Null
    }
}

function Invoke-OpenTerminalUninstall {
    $registration = Get-OpenTerminalRegistration
    if ($null -eq $registration) { return }

    $displayIcon = [string]$registration.Value.DisplayIcon
    $installDir = if ($displayIcon) { Split-Path -Parent ($displayIcon.Trim('"') -replace ',\d+$', '') } else { $null }
    $uninstaller = if ($installDir) { Join-Path $installDir 'uninstall.exe' } else { $null }
    if (-not $uninstaller -or -not (Test-Path -LiteralPath $uninstaller -PathType Leaf)) {
        $command = [string]$registration.Value.UninstallString
        if ($command -match '^"([^"]+)' -or $command -match '^(\S+)') { $uninstaller = $Matches[1] }
    }
    Assert-True (Test-Path -LiteralPath $uninstaller -PathType Leaf) 'Desinstalador do OpenTerminal não encontrado.'

    Write-Host 'Desinstalando a versão existente do OpenTerminal...'
    $process = Start-Process -FilePath $uninstaller -ArgumentList '/S' -PassThru
    Assert-True ($process.WaitForExit(120000)) 'O desinstalador não terminou em 120 segundos.'
    Wait-NsisChildren
    Wait-Until { $null -eq (Get-OpenTerminalRegistration) } 30 'A chave de desinstalação permaneceu no registro.'
    if ($installDir) {
        Wait-Until { -not (Test-Path -LiteralPath $installDir) } 30 "O diretório instalado permaneceu em $installDir."
    }
}

function Get-Descendants {
    param([int]$RootProcessId)
    $all = @(Get-CimInstance Win32_Process)
    $children = @{}
    foreach ($item in $all) {
        $parent = [int]$item.ParentProcessId
        if (-not $children.ContainsKey($parent)) {
            $children[$parent] = [Collections.Generic.List[int]]::new()
        }
        $children[$parent].Add([int]$item.ProcessId)
    }
    $seen = [Collections.Generic.HashSet[int]]::new()
    $queue = [Collections.Generic.Queue[int]]::new()
    $queue.Enqueue($RootProcessId)
    while ($queue.Count -gt 0) {
        $current = $queue.Dequeue()
        if (-not $seen.Add($current)) { continue }
        if ($children.ContainsKey($current)) {
            foreach ($child in $children[$current]) { $queue.Enqueue($child) }
        }
    }
    return @($all | Where-Object { $seen.Contains([int]$_.ProcessId) })
}

if ($PSVersionTable.PSVersion.Major -ge 6 -and -not $IsWindows) {
    throw 'Este script só pode ser executado no Windows.'
}
$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
Assert-True ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) 'Execute este script em um PowerShell elevado (Administrador).'

if (-not $InstallerPath) {
    $InstallerPath = Join-Path $repoRoot 'build\bin\OpenTerminal-amd64-installer.exe'
}
$InstallerPath = (Resolve-Path -LiteralPath $InstallerPath).Path
Assert-True (Test-Path -LiteralPath $InstallerPath -PathType Leaf) "Instalador não encontrado: $InstallerPath"

$createdSentinel = -not (Test-Path -LiteralPath $realDataDir)
if ($createdSentinel) {
    New-Item -ItemType Directory -Path $realDataDir | Out-Null
    [IO.File]::WriteAllText((Join-Path $realDataDir 'sentinela.txt'), 'preservar', [Text.UTF8Encoding]::new($false))
}
$dataHashBefore = Get-ListingHash $realDataDir

$first = $null
$installedByScript = $false
try {
    Invoke-OpenTerminalUninstall
    foreach ($shortcut in @($desktopShortcut, $startMenuShortcut)) {
        Assert-True (-not (Test-Path -LiteralPath $shortcut)) "Atalho antigo permaneceu antes da instalação: $shortcut"
    }

    Write-Host 'Instalando o OpenTerminal em modo silencioso...'
    $installer = Start-Process -FilePath $InstallerPath -ArgumentList '/S' -PassThru
    Assert-True ($installer.WaitForExit(120000)) 'O instalador não terminou em 120 segundos.'
    Assert-True ($installer.ExitCode -eq 0) "O instalador terminou com código $($installer.ExitCode)."
    $installedByScript = $true

    Wait-Until { $null -ne (Get-OpenTerminalRegistration) } 30 'A chave de desinstalação não foi criada.'
    $registration = Get-OpenTerminalRegistration
    Assert-True ([string]$registration.Value.DisplayName -eq 'OpenTerminal') 'DisplayName do desinstalador não é OpenTerminal.'
    Assert-True ([string]$registration.Value.DisplayVersion -match '^0\.2\.\d+$') 'DisplayVersion não está no formato 0.2.x.'
    Assert-True (-not [string]::IsNullOrWhiteSpace([string]$registration.Value.DisplayIcon)) 'DisplayIcon não foi registrado.'

    $installedExe = ([string]$registration.Value.DisplayIcon).Trim('"') -replace ',\d+$', ''
    Assert-True (Test-Path -LiteralPath $installedExe -PathType Leaf) "Executável instalado não encontrado: $installedExe"
    $installDir = Split-Path -Parent $installedExe
    Assert-True (Test-Path -LiteralPath (Join-Path $installDir 'uninstall.exe') -PathType Leaf) 'uninstall.exe não foi instalado.'
    Assert-True (Test-Path -LiteralPath $desktopShortcut -PathType Leaf) 'Atalho da Área de Trabalho não foi criado.'
    Assert-True (Test-Path -LiteralPath $startMenuShortcut -PathType Leaf) 'Atalho do Menu Iniciar não foi criado.'

    New-Item -ItemType Directory -Path $testDataDir | Out-Null
    $hadOverride = Test-Path Env:OPENTERMINAL_DATA_DIR
    $previousOverride = $env:OPENTERMINAL_DATA_DIR
    try {
        $env:OPENTERMINAL_DATA_DIR = $testDataDir
        $first = Start-Process -FilePath $installedExe -PassThru
    } finally {
        if ($hadOverride) { $env:OPENTERMINAL_DATA_DIR = $previousOverride } else { Remove-Item Env:OPENTERMINAL_DATA_DIR -ErrorAction SilentlyContinue }
    }

    Start-Sleep -Seconds $AliveSeconds
    Assert-True (-not $first.HasExited) 'O app instalado encerrou antes do período mínimo.'
    $descendants = Get-Descendants $first.Id
    Assert-True (@($descendants | Where-Object Name -IEq 'msedgewebview2.exe').Count -gt 0) 'Nenhum processo WebView2 filho foi encontrado.'

    try {
        $env:OPENTERMINAL_DATA_DIR = $testDataDir
        $second = Start-Process -FilePath $installedExe -PassThru
    } finally {
        if ($hadOverride) { $env:OPENTERMINAL_DATA_DIR = $previousOverride } else { Remove-Item Env:OPENTERMINAL_DATA_DIR -ErrorAction SilentlyContinue }
    }
    if (-not $second.WaitForExit(10000)) {
        Stop-Process -Id $second.Id -Force -ErrorAction SilentlyContinue
        throw 'A segunda instância não encerrou em 10 segundos.'
    }
    Assert-True ($second.ExitCode -eq 0) "A segunda instância terminou com código $($second.ExitCode)."
    Assert-True (-not $first.HasExited) 'A primeira instância encerrou ao abrir a segunda.'

    Stop-TestProcess $first
    $first = $null
    Invoke-OpenTerminalUninstall
    $installedByScript = $false

    Assert-True ($null -eq (Get-OpenTerminalRegistration)) 'A chave de desinstalação não foi removida.'
    Assert-True (-not (Test-Path -LiteralPath $installDir)) 'O diretório de instalação não foi removido.'
    Assert-True (-not (Test-Path -LiteralPath $desktopShortcut)) 'O atalho da Área de Trabalho não foi removido.'
    Assert-True (-not (Test-Path -LiteralPath $startMenuShortcut)) 'O atalho do Menu Iniciar não foi removido.'

    $dataHashAfter = Get-ListingHash $realDataDir
    Assert-True ($dataHashAfter -eq $dataHashBefore) 'A árvore de dados reais do usuário foi alterada.'
    if ($createdSentinel) {
        Assert-True (Test-Path -LiteralPath (Join-Path $realDataDir 'sentinela.txt')) 'A sentinela dos dados do usuário foi removida.'
    }

    $result = [pscustomobject]@{
        installer = $InstallerPath
        installedExecutable = $installedExe
        displayVersion = [string]$registration.Value.DisplayVersion
        firstInstanceAliveSeconds = $AliveSeconds
        webViewChildren = @($descendants | Where-Object Name -IEq 'msedgewebview2.exe').Count
        secondInstanceExitCode = $second.ExitCode
        uninstallValidated = $true
        userDataPreserved = $true
        windows10Fallback = 'pendente de validação manual'
    }
    if ($OutputJson) {
        $jsonPath = [IO.Path]::GetFullPath($OutputJson)
        [IO.File]::WriteAllText($jsonPath, ($result | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
    }
    $result
} catch {
    if ($OutputJson) {
        $jsonPath = [IO.Path]::GetFullPath($OutputJson)
        $failure = [pscustomobject]@{
            error = $_.Exception.Message
            location = $_.InvocationInfo.PositionMessage
        }
        [IO.File]::WriteAllText($jsonPath, ($failure | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
    }
    throw
} finally {
    Stop-TestProcess $first
    if ($installedByScript) {
        try { Invoke-OpenTerminalUninstall } catch { Write-Warning "Falha ao limpar instalação de teste: $_" }
    }
    $tempRoot = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
    $resolvedTestData = [IO.Path]::GetFullPath($testDataDir)
    if ($resolvedTestData.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTestData)) {
        Remove-Item -LiteralPath $resolvedTestData -Recurse -Force -ErrorAction SilentlyContinue
    }
}
