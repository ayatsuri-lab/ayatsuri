#
# Copyright (C) 2026 Yota Hamada
# SPDX-License-Identifier: GPL-3.0-or-later
#

[CmdletBinding()]
param(
    [string]$Version = "",
    [string]$InstallDir = "",
    [ValidateSet("yes", "no")]
    [string]$Service = "",
    [ValidateSet("user", "system")]
    [string]$ServiceScope = "",
    [string]$HostAddress = "",
    [string]$Port = "",
    [string[]]$SkillsDir = @(),
    [string]$AdminUsername = "",
    [string]$AdminPassword = "",
    [ValidateSet("yes", "no")]
    [string]$OpenBrowser = "",
    [switch]$Uninstall,
    [switch]$PurgeData,
    [switch]$RemoveSkill,
    [switch]$NoPrompt,
    [switch]$DryRun,
    [switch]$VerboseMode
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Script:InstallerSource = $MyInvocation.MyCommand.Definition
$Script:ReleaseBase = "https://github.com/ayatsuri-lab/ayatsuri/releases"
$Script:ReleaseApi = "https://api.github.com/repos/ayatsuri-lab/ayatsuri/releases/latest"
$Script:WinSWVersion = "v2.12.0"
$Script:WinSWBase = "https://github.com/winsw/winsw/releases/download/$($Script:WinSWVersion)"
$Script:ServiceName = "Ayatsuri"
$Script:ServiceWrapperExe = $null
$Script:ServiceConfigXml = $null
$Script:AyatsuriExe = $null
$Script:AyatsuriHome = $null
$Script:ServiceUrl = $null
$Script:DetectedSkillTargets = 0
$Script:RebootElevated = $false
$Script:SkillMode = ""
$Script:UninstallInstallPaths = @()
$Script:UninstallAyatsuriHomes = @()
$Script:UninstallSkillDirs = @()
$Script:UninstallCopilotFiles = @()
$Script:UninstallPathScopes = @()
$Script:UninstallServicePresent = $false
$Script:UninstallMultipleInstallsConfirmed = $false

function Write-Section {
    param([string]$Message)
    Write-Host ""
    Write-Host $Message -ForegroundColor Green
}

function Write-Info {
    param([string]$Message)
    Write-Host "· $Message" -ForegroundColor DarkGray
}

function Write-WarnMessage {
    param([string]$Message)
    Write-Host "! $Message" -ForegroundColor Yellow
}

function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Cyan
}

function Write-ErrorMessage {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

function Show-Banner {
    Write-Host "Ayatsuri Installer" -ForegroundColor Green
    Write-Host "Install Ayatsuri, set it up as a background app, and get you to the UI quickly." -ForegroundColor DarkGray
}

function Test-Interactive {
    if ($NoPrompt) { return $false }
    return ($Host.Name -ne "ServerRemoteHost")
}

function Confirm-Choice {
    param(
        [string]$Prompt,
        [bool]$Default = $true
    )

    if (-not (Test-Interactive)) {
        return $Default
    }

    $suffix = if ($Default) { "[Y/n]" } else { "[y/N]" }
    $answer = Read-Host "$Prompt $suffix"
    if ([string]::IsNullOrWhiteSpace($answer)) {
        return $Default
    }
    switch ($answer.Trim().ToLowerInvariant()) {
        "y" { return $true }
        "yes" { return $true }
        "n" { return $false }
        "no" { return $false }
        default { return $Default }
    }
}

function Add-UniqueItem {
    param(
        [ref]$Collection,
        [string]$Value
    )
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return
    }
    if ($Collection.Value -notcontains $Value) {
        $Collection.Value += $Value
    }
}

function Join-Values {
    param([string[]]$Values)
    $filtered = @($Values | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($filtered.Count -eq 0) {
        return ""
    }
    return ($filtered -join ", ")
}

function Choose-OperationMode {
    if ($Uninstall) {
        return
    }
    if (-not (Test-Interactive)) {
        return
    }
    Write-Section "Choose setup"
    if (Confirm-Choice "Install or repair Ayatsuri now?" $true) {
        $script:Uninstall = $false
    }
    else {
        $script:Uninstall = $true
    }
}

function Read-Default {
    param(
        [string]$Prompt,
        [string]$Default = ""
    )

    if (-not (Test-Interactive)) {
        return $Default
    }

    if ($Default) {
        $value = Read-Host "$Prompt [$Default]"
        if ([string]::IsNullOrWhiteSpace($value)) {
            return $Default
        }
        return $value.Trim()
    }

    return (Read-Host $Prompt).Trim()
}

function ConvertTo-PlainText {
    param([System.Security.SecureString]$SecureString)
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureString)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}

function Read-PasswordConfirm {
    param([string]$Prompt)
    if (-not (Test-Interactive)) {
        return ""
    }
    while ($true) {
        $first = ConvertTo-PlainText (Read-Host -AsSecureString $Prompt)
        $second = ConvertTo-PlainText (Read-Host -AsSecureString "Confirm $Prompt")
        if ($first -ne $second) {
            Write-WarnMessage "Passwords did not match. Try again."
            continue
        }
        return $first
    }
}

function Get-LatestVersion {
    if ($Version) {
        if ($Version -ieq "latest") {
            $script:Version = ""
        }
        elseif ($Version -notmatch '^v') {
            $script:Version = "v$Version"
            return
        }
        else {
            return
        }
    }
    if ($DryRun) {
        $script:Version = "latest"
        return
    }
    $release = Invoke-RestMethod -Uri $Script:ReleaseApi
    $script:Version = $release.tag_name
}

function Get-WindowsArch {
    if ([Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITEW6432 -eq "ARM64" -or $env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
            return "arm64"
        }
        return "amd64"
    }
    return "386"
}

function Get-WrapperAssetName {
    param([string]$Arch)
    switch ($Arch) {
        "amd64" { return "WinSW-x64.exe" }
        "arm64" { return "WinSW-x64.exe" }
        "386" { return "WinSW-x86.exe" }
        default { return "WinSW-x64.exe" }
    }
}

function Has-AdminBootstrap {
    return (-not [string]::IsNullOrWhiteSpace($AdminUsername)) -and (-not [string]::IsNullOrWhiteSpace($AdminPassword))
}

function Escape-XmlValue {
    param([string]$Value)
    return [System.Security.SecurityElement]::Escape($Value)
}

function Validate-AdminBootstrap {
    if (-not [string]::IsNullOrWhiteSpace($AdminUsername) -and [string]::IsNullOrWhiteSpace($AdminPassword)) {
        throw "An admin password is required when -AdminUsername is provided."
    }
    if ([string]::IsNullOrWhiteSpace($AdminUsername) -and -not [string]::IsNullOrWhiteSpace($AdminPassword)) {
        throw "An admin username is required when -AdminPassword is provided."
    }
    if ((Has-AdminBootstrap) -and ($AdminPassword.Length -lt 8)) {
        throw "The admin password must be at least 8 characters long."
    }
}

function Validate-UninstallArgs {
    if (-not $Uninstall) {
        return
    }
    if ($PSBoundParameters.ContainsKey("Version")) {
        throw "-Version is only supported during install."
    }
    if ($PSBoundParameters.ContainsKey("HostAddress") -or $PSBoundParameters.ContainsKey("Port")) {
        throw "-HostAddress and -Port are only supported during install."
    }
    if ($PSBoundParameters.ContainsKey("AdminUsername") -or $PSBoundParameters.ContainsKey("AdminPassword")) {
        throw "Admin bootstrap flags are only supported during install."
    }
    if ($PSBoundParameters.ContainsKey("OpenBrowser")) {
        throw "-OpenBrowser is only supported during install."
    }
    if ($PSBoundParameters.ContainsKey("Service")) {
        throw "-Service is only supported during install. Use -ServiceScope to narrow service uninstall discovery."
    }
    if ($ServiceScope -and $ServiceScope -ne "system") {
        Write-WarnMessage "Windows uninstall ignores -ServiceScope user. The Ayatsuri service is machine-scoped when installed."
    }
}

function Resolve-Defaults {
    if (-not $Service) {
        $script:Service = if (Test-Interactive) { "yes" } else { "no" }
    }
    if (-not $HostAddress) {
        $script:HostAddress = "127.0.0.1"
    }
    if (-not $Port) {
        $script:Port = "8080"
    }
    if (-not $OpenBrowser) {
        $script:OpenBrowser = "yes"
    }
    if ($Service -eq "yes") {
        $script:ServiceScope = "system"
    }
    elseif (-not $ServiceScope) {
        $script:ServiceScope = "user"
    }
    if (-not $InstallDir) {
        if ($Service -eq "yes") {
            $script:InstallDir = Join-Path ${env:ProgramFiles} "Ayatsuri"
        } else {
            $script:InstallDir = Join-Path $env:LOCALAPPDATA "Programs\ayatsuri"
        }
    }
    if ($Service -eq "yes") {
        $script:AyatsuriHome = Join-Path $env:ProgramData "Ayatsuri"
    } else {
        $script:AyatsuriHome = Join-Path $env:LOCALAPPDATA "Ayatsuri"
    }
    $script:AyatsuriExe = Join-Path $InstallDir "ayatsuri.exe"
    $script:ServiceWrapperExe = Join-Path $InstallDir "ayatsuri-service.exe"
    $script:ServiceConfigXml = Join-Path $InstallDir "ayatsuri-service.xml"
    $script:ServiceUrl = "http://$HostAddress`:$Port"
    if ($SkillsDir.Count -gt 0) {
        $script:SkillMode = "explicit"
    }
    elseif (-not $Script:SkillMode) {
        if ($DetectedSkillTargets -gt 0) {
            $script:SkillMode = "auto"
        }
        else {
            $script:SkillMode = "skip"
        }
    }
}

function Detect-SkillTargets {
    $home = [Environment]::GetFolderPath("UserProfile")
    $count = 0
    $agentsHome = if ($env:AGENTS_HOME) { $env:AGENTS_HOME } else { Join-Path $home ".agents" }
    $codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $home ".codex" }
    if (Test-Path (Join-Path $home ".claude\.claude.json")) { $count++ }
    if (Test-Path $agentsHome) { $count++ }
    elseif (Test-Path $codexHome) { $count++ }
    if (Test-Path (Join-Path $home ".config\opencode")) { $count++ }
    if (Test-Path (Join-Path $home ".gemini\GEMINI.md")) { $count++ }
    $xdg = if ($env:XDG_CONFIG_HOME) { $env:XDG_CONFIG_HOME } else { $home }
    if ((Test-Path (Join-Path $xdg ".copilot\config.json")) -or (Test-Path (Join-Path $home ".copilot\config.json"))) { $count++ }
    $Script:DetectedSkillTargets = $count
}

function Get-ServiceWrapperPath {
    $service = Get-CimInstance Win32_Service -Filter "Name='$($Script:ServiceName)'" -ErrorAction SilentlyContinue
    if (-not $service -or [string]::IsNullOrWhiteSpace($service.PathName)) {
        return $null
    }
    $pathName = $service.PathName.Trim()
    if ($pathName -match '^"([^"]+)"') {
        return $matches[1]
    }
    return ($pathName -split '\s+', 2)[0]
}

function Get-XmlEnvValue {
    param(
        [string]$XmlPath,
        [string]$Name
    )
    if (-not (Test-Path $XmlPath)) {
        return ""
    }
    [xml]$xml = Get-Content -Raw -Path $XmlPath
    $envNode = @($xml.service.env | Where-Object { $_.name -eq $Name } | Select-Object -First 1)
    if ($envNode.Count -eq 0) {
        return ""
    }
    return [string]$envNode[0].value
}

function Discover-SkillRemovals {
    $home = [Environment]::GetFolderPath("UserProfile")
    $agentsHome = if ($env:AGENTS_HOME) { $env:AGENTS_HOME } else { Join-Path $home ".agents" }
    $codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $home ".codex" }
    $xdg = if ($env:XDG_CONFIG_HOME) { $env:XDG_CONFIG_HOME } else { $home }
    $claudeSkill = Join-Path $home ".claude\skills\ayatsuri"
    $agentsSkill = Join-Path $agentsHome "skills\ayatsuri"
    $codexSkill = Join-Path $codexHome "skills\ayatsuri"
    $openCodeSkill = Join-Path $home ".config\opencode\skills\ayatsuri"
    $geminiSkill = Join-Path $home ".gemini\skills\ayatsuri"
    $xdgCopilot = Join-Path $xdg ".copilot\copilot-instructions.md"
    $homeCopilot = Join-Path $home ".copilot\copilot-instructions.md"
    foreach ($dir in @($claudeSkill, $agentsSkill, $codexSkill, $openCodeSkill, $geminiSkill)) {
        if (Test-Path $dir) {
            Add-UniqueItem ([ref]$Script:UninstallSkillDirs) $dir
        }
    }
    foreach ($file in @($xdgCopilot, $homeCopilot)) {
        if (Test-Path $file) {
            Add-UniqueItem ([ref]$Script:UninstallCopilotFiles) $file
        }
    }
    foreach ($dir in $SkillsDir) {
        Add-UniqueItem ([ref]$Script:UninstallSkillDirs) (Join-Path $dir "ayatsuri")
    }
}

function Discover-UninstallArtifacts {
    $script:UninstallInstallPaths = @()
    $script:UninstallAyatsuriHomes = @()
    $script:UninstallSkillDirs = @()
    $script:UninstallCopilotFiles = @()
    $script:UninstallPathScopes = @()
    $script:UninstallServicePresent = $false

    $serviceWrapper = Get-ServiceWrapperPath
    if ($serviceWrapper) {
        $serviceInstallDir = Split-Path -Parent $serviceWrapper
        $script:UninstallServicePresent = $true
        $script:ServiceWrapperExe = $serviceWrapper
        $script:ServiceConfigXml = Join-Path $serviceInstallDir "ayatsuri-service.xml"
        $script:AyatsuriExe = Join-Path $serviceInstallDir "ayatsuri.exe"
        Add-UniqueItem ([ref]$Script:UninstallInstallPaths) $Script:AyatsuriExe
        Add-UniqueItem ([ref]$Script:UninstallPathScopes) $serviceInstallDir
        Add-UniqueItem ([ref]$Script:UninstallAyatsuriHomes) (Get-XmlEnvValue -XmlPath $Script:ServiceConfigXml -Name "DAGU_HOME")
    }

    $cmd = Get-Command ayatsuri.exe -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($cmd -and $cmd.Source) {
        Add-UniqueItem ([ref]$Script:UninstallInstallPaths) $cmd.Source
        Add-UniqueItem ([ref]$Script:UninstallPathScopes) (Split-Path -Parent $cmd.Source)
    }

    $userInstallDir = Join-Path $env:LOCALAPPDATA "Programs\ayatsuri"
    $systemInstallDir = Join-Path ${env:ProgramFiles} "Ayatsuri"
    foreach ($path in @((Join-Path $userInstallDir "ayatsuri.exe"), (Join-Path $systemInstallDir "ayatsuri.exe"))) {
        if (Test-Path $path) {
            Add-UniqueItem ([ref]$Script:UninstallInstallPaths) $path
            Add-UniqueItem ([ref]$Script:UninstallPathScopes) (Split-Path -Parent $path)
        }
    }

    foreach ($homePath in @((Join-Path $env:LOCALAPPDATA "Ayatsuri"), (Join-Path $env:ProgramData "Ayatsuri"))) {
        if (Test-Path $homePath) {
            Add-UniqueItem ([ref]$Script:UninstallAyatsuriHomes) $homePath
        }
    }

    Discover-SkillRemovals

    if ($InstallDir) {
        $explicitExe = Join-Path $InstallDir "ayatsuri.exe"
        $script:UninstallInstallPaths = @($Script:UninstallInstallPaths | Where-Object { $_ -eq $explicitExe })
        if ($Script:UninstallInstallPaths.Count -eq 0) {
            $script:UninstallInstallPaths = @($explicitExe)
        }
        $script:UninstallPathScopes = @($Script:UninstallPathScopes | Where-Object { $_ -eq $InstallDir })
        if ($Script:UninstallPathScopes.Count -eq 0) {
            $script:UninstallPathScopes = @($InstallDir)
        }
        if ($Script:UninstallServicePresent -and ((Split-Path -Parent $Script:ServiceWrapperExe) -ne $InstallDir)) {
            $script:UninstallServicePresent = $false
        }
        if (-not $Script:UninstallServicePresent) {
            $script:ServiceWrapperExe = Join-Path $InstallDir "ayatsuri-service.exe"
            $script:ServiceConfigXml = Join-Path $InstallDir "ayatsuri-service.xml"
            $script:AyatsuriExe = $explicitExe
        }
    }
}

function Validate-UninstallDiscovery {
    if (-not $Uninstall) {
        return
    }
    if ($Script:UninstallInstallPaths.Count -gt 1 -and -not $InstallDir) {
        if (-not (Test-Interactive)) {
            throw "Multiple Ayatsuri installations were detected. Rerun with -InstallDir to choose which one to remove."
        }
        Write-WarnMessage ("Multiple Ayatsuri binaries were detected: " + (Join-Values $Script:UninstallInstallPaths))
        if (-not (Confirm-Choice "Remove all detected Ayatsuri binaries?" $true)) {
            throw "Rerun with -InstallDir to choose which installation to remove."
        }
        $script:UninstallMultipleInstallsConfirmed = $true
    }
}

function Invoke-UninstallWizard {
    if (-not (Test-Interactive)) {
        return
    }
    Write-Section "Uninstall options"
    if (($Script:UninstallSkillDirs.Count -gt 0 -or $Script:UninstallCopilotFiles.Count -gt 0) -and (Confirm-Choice "Remove the Ayatsuri AI skill from detected AI tools too?" $false)) {
        $script:RemoveSkill = $true
    }
    if ($Script:UninstallAyatsuriHomes.Count -gt 0 -and (Confirm-Choice "Delete the detected Ayatsuri data directory too?" $false)) {
        $script:PurgeData = $true
    }
}

function Show-UninstallPlan {
    Write-Section "Uninstall plan"
    Write-Host ("Binary paths".PadRight(20) + $(if ($Script:UninstallInstallPaths.Count -gt 0) { Join-Values $Script:UninstallInstallPaths } else { "none" }))
    Write-Host ("Background service".PadRight(20) + $(if ($Script:UninstallServicePresent) { $Script:ServiceName } else { "none" }))
    $dataAction = if ($PurgeData) { "remove" } else { "keep" }
    Write-Host ("Data directory".PadRight(20) + "$dataAction: $(if ($Script:UninstallAyatsuriHomes.Count -gt 0) { Join-Values $Script:UninstallAyatsuriHomes } else { 'none detected' })")
    Write-Host ("PATH cleanup".PadRight(20) + $(if ($Script:UninstallPathScopes.Count -gt 0) { Join-Values $Script:UninstallPathScopes } else { "none detected" }))
    if ($RemoveSkill) {
        $skillTargets = @($Script:UninstallSkillDirs + $Script:UninstallCopilotFiles)
        Write-Host ("AI skill removal".PadRight(20) + $(if ($skillTargets.Count -gt 0) { Join-Values $skillTargets } else { "requested, but nothing detected" }))
    }
    else {
        Write-Host ("AI skill removal".PadRight(20) + "keep")
    }
    if ($DryRun) {
        Write-Host ("Dry run".PadRight(20) + "yes")
    }
}

function Test-UninstallHasAnything {
    return ($Script:UninstallInstallPaths.Count -gt 0) -or
        $Script:UninstallServicePresent -or
        ($Script:UninstallPathScopes.Count -gt 0) -or
        ($Script:UninstallAyatsuriHomes.Count -gt 0) -or
        ($Script:UninstallSkillDirs.Count -gt 0) -or
        ($Script:UninstallCopilotFiles.Count -gt 0)
}

function Normalize-InstallerPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    try {
        return [IO.Path]::GetFullPath($Path).TrimEnd('\')
    }
    catch {
        return $Path.TrimEnd('\')
    }
}

function Test-UnsafeDeleteTarget {
    param([string]$Path)
    $normalized = Normalize-InstallerPath $Path
    $protected = @(
        [Environment]::GetFolderPath("UserProfile"),
        $env:LOCALAPPDATA,
        $env:APPDATA,
        $env:ProgramData,
        ${env:ProgramFiles},
        ${env:ProgramFiles(x86)}
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | ForEach-Object { Normalize-InstallerPath $_ }
    return $protected -contains $normalized
}

function Remove-ExactPathEntry {
    param(
        [string]$PathEntry,
        [ValidateSet("User", "Machine")]
        [string]$Scope
    )
    if ([string]::IsNullOrWhiteSpace($PathEntry)) {
        return
    }
    $current = [Environment]::GetEnvironmentVariable("Path", $Scope)
    if ([string]::IsNullOrWhiteSpace($current)) {
        return
    }
    $parts = @($current -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($parts -notcontains $PathEntry) {
        return
    }
    if ($DryRun) {
        Write-Info "Would remove $PathEntry from the $Scope PATH"
        return
    }
    $updated = @($parts | Where-Object { $_ -ne $PathEntry }) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $updated, $Scope)
}

function Remove-SkillDirectory {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }
    if ((Split-Path -Leaf $Path) -ne "ayatsuri") {
        Write-WarnMessage "Skipping unexpected skill path: $Path"
        return
    }
    if ($DryRun) {
        Write-Info "Would remove $Path"
        return
    }
    Remove-Item -Recurse -Force -Path $Path -ErrorAction SilentlyContinue
}

function Remove-CopilotMarkers {
    param([string]$Path)
    if (-not (Test-Path $Path)) {
        return
    }
    $lines = Get-Content -Path $Path
    $beginCount = @($lines | Where-Object { $_ -eq "<!-- BEGIN AYATSURI -->" }).Count
    $endCount = @($lines | Where-Object { $_ -eq "<!-- END AYATSURI -->" }).Count
    if (($beginCount -eq 0) -and ($endCount -eq 0)) {
        return
    }
    if ($beginCount -ne 1 -or $endCount -ne 1) {
        Write-WarnMessage "Skipping malformed Copilot instructions file: $Path"
        return
    }
    if ($DryRun) {
        Write-Info "Would remove the Ayatsuri section from $Path"
        return
    }
    $result = New-Object System.Collections.Generic.List[string]
    $skip = $false
    foreach ($line in $lines) {
        if ($line -eq "<!-- BEGIN AYATSURI -->") {
            $skip = $true
            continue
        }
        if ($line -eq "<!-- END AYATSURI -->") {
            $skip = $false
            continue
        }
        if (-not $skip) {
            $result.Add($line)
        }
    }
    if ($result.Count -eq 0) {
        Remove-Item -Force -Path $Path -ErrorAction SilentlyContinue
        return
    }
    Set-Content -Path $Path -Value $result -Encoding UTF8
}

function Remove-UninstallDataDirectory {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }
    if (Test-UnsafeDeleteTarget $Path) {
        Write-WarnMessage "Skipping unsafe data directory removal: $Path"
        return
    }
    if ($DryRun) {
        Write-Info "Would remove $Path"
        return
    }
    if (Test-Path $Path) {
        Remove-Item -Recurse -Force -Path $Path -ErrorAction SilentlyContinue
    }
}

function Remove-UninstallFile {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }
    if ($DryRun) {
        Write-Info "Would remove $Path"
        return
    }
    if (Test-Path $Path) {
        Remove-Item -Force -Path $Path -ErrorAction SilentlyContinue
    }
}

function Invoke-Uninstall {
    if (-not (Test-UninstallHasAnything)) {
        Write-Section "Uninstall"
        Write-Info "Nothing to uninstall. No Ayatsuri install, service, PATH entry, data directory, or skill install was detected."
        return
    }
    if ($DryRun) {
        Write-Success "Dry run complete. No changes were made."
        return
    }
    if ($Script:UninstallServicePresent) {
        if (Get-Service -Name $Script:ServiceName -ErrorAction SilentlyContinue) {
            if ($Script:ServiceWrapperExe -and (Test-Path $Script:ServiceWrapperExe)) {
                try { Invoke-WinSW stop } catch {}
                try { Invoke-WinSW uninstall } catch {}
            }
            else {
                try { Stop-Service -Name $Script:ServiceName -Force -ErrorAction SilentlyContinue } catch {}
                & sc.exe delete $Script:ServiceName | Out-Null
            }
        }
    }
    foreach ($path in $Script:UninstallInstallPaths) {
        Remove-UninstallFile -Path $path
    }
    foreach ($path in @($Script:ServiceWrapperExe, $Script:ServiceConfigXml)) {
        Remove-UninstallFile -Path $path
    }
    foreach ($pathEntry in $Script:UninstallPathScopes) {
        Remove-ExactPathEntry -PathEntry $pathEntry -Scope User
        Remove-ExactPathEntry -PathEntry $pathEntry -Scope Machine
        if (Test-Path $pathEntry) {
            $children = @(Get-ChildItem -Force -LiteralPath $pathEntry -ErrorAction SilentlyContinue)
            if ($children.Count -eq 0) {
                Remove-Item -Force -LiteralPath $pathEntry -ErrorAction SilentlyContinue
            }
        }
    }
    if ($RemoveSkill) {
        foreach ($dir in $Script:UninstallSkillDirs) {
            Remove-SkillDirectory -Path $dir
        }
        foreach ($file in $Script:UninstallCopilotFiles) {
            Remove-CopilotMarkers -Path $file
        }
    }
    if ($PurgeData) {
        foreach ($dir in $Script:UninstallAyatsuriHomes) {
            Remove-UninstallDataDirectory -Path $dir
        }
    }
}

function Show-UninstallSummary {
    Write-Section "Uninstall complete"
    Write-Host ("Removed binaries".PadRight(20) + $(if ($Script:UninstallInstallPaths.Count -gt 0) { Join-Values $Script:UninstallInstallPaths } else { "none" }))
    Write-Host ("Removed service".PadRight(20) + $(if ($Script:UninstallServicePresent) { $Script:ServiceName } else { "none" }))
    Write-Host ("PATH cleanup".PadRight(20) + $(if ($Script:UninstallPathScopes.Count -gt 0) { Join-Values $Script:UninstallPathScopes } else { "none" }))
    Write-Host ("Data directory".PadRight(20) + $(if ($PurgeData) { "removed" } else { "kept" }))
    Write-Host ("AI skill".PadRight(20) + $(if ($RemoveSkill) { "removed where found" } else { "kept" }))
}

function Show-Plan {
    Write-Section "Install plan"
    Write-Host ("Version".PadRight(20) + $Version)
    Write-Host ("Install directory".PadRight(20) + $InstallDir)
    Write-Host ("Background service".PadRight(20) + $Service)
    if ($Service -eq "yes") {
        Write-Host ("Service scope".PadRight(20) + $ServiceScope)
        Write-Host ("Ayatsuri home".PadRight(20) + $AyatsuriHome)
        Write-Host ("Web URL".PadRight(20) + $ServiceUrl)
        Write-Host ("Admin bootstrap".PadRight(20) + $(if ($AdminUsername) { $AdminUsername } else { "disabled" }))
    }
    Write-Host ("Skill install".PadRight(20) + $(if ($SkillMode -eq "explicit") { "custom" } elseif ($SkillMode -eq "auto") { "detected tools" } else { "skip" }))
    if ($DryRun) {
        Write-Host ("Dry run".PadRight(20) + "yes")
    }
}

function Invoke-InstallerWizard {
    if (-not (Test-Interactive)) {
        return
    }

    Write-Section "Recommended setup"
    $script:Service = if (Confirm-Choice "Install Ayatsuri as a background service?" $true) { "yes" } else { "no" }
    if ($script:Service -eq "yes") {
        $script:ServiceScope = "system"
    } else {
        $script:ServiceScope = "user"
    }
    $script:HostAddress = if (Confirm-Choice "Open Ayatsuri only on this computer?" $true) { "127.0.0.1" } else { "0.0.0.0" }
    $script:Port = Read-Default "Web UI port" $Port
    $script:InstallDir = Read-Default "Install directory" $InstallDir
    if ($script:Service -eq "yes") {
        $script:AyatsuriHome = Read-Default "Ayatsuri data directory" $AyatsuriHome
        $script:AdminUsername = Read-Default "Initial admin username" $(if ($AdminUsername) { $AdminUsername } else { "admin" })
        if (-not $AdminPassword) {
            $script:AdminPassword = Read-PasswordConfirm "Initial admin password"
        }
    }
    if ($DetectedSkillTargets -gt 0 -and $SkillsDir.Count -eq 0) {
        if (-not (Confirm-Choice "Install the Ayatsuri AI skill into detected AI tools?" $true)) {
            $script:SkillMode = "skip"
        }
        else {
            $script:SkillMode = "auto"
        }
    }
    elseif ($SkillsDir.Count -gt 0) {
        $script:SkillMode = "explicit"
    }
    Resolve-Defaults
}

function Test-IsAdmin {
    $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object System.Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-ForwardArgs {
    $argsList = @()
    if ($Version) { $argsList += @("-Version", $Version) }
    if ($InstallDir) { $argsList += @("-InstallDir", $InstallDir) }
    if ($Service) { $argsList += @("-Service", $Service) }
    if ($ServiceScope) { $argsList += @("-ServiceScope", $ServiceScope) }
    if ($HostAddress) { $argsList += @("-HostAddress", $HostAddress) }
    if ($Port) { $argsList += @("-Port", $Port) }
    foreach ($dir in $SkillsDir) { $argsList += @("-SkillsDir", $dir) }
    if ($AdminUsername) { $argsList += @("-AdminUsername", $AdminUsername) }
    if ($AdminPassword) { $argsList += @("-AdminPassword", $AdminPassword) }
    if ($OpenBrowser) { $argsList += @("-OpenBrowser", $OpenBrowser) }
    if ($Uninstall) { $argsList += "-Uninstall" }
    if ($PurgeData) { $argsList += "-PurgeData" }
    if ($RemoveSkill) { $argsList += "-RemoveSkill" }
    if ($NoPrompt) { $argsList += "-NoPrompt" }
    if ($DryRun) { $argsList += "-DryRun" }
    if ($VerboseMode) { $argsList += "-VerboseMode" }
    return $argsList
}

function Ensure-ElevatedForService {
    if ($Service -ne "yes" -or $DryRun) {
        return
    }
    if (Test-IsAdmin) {
        return
    }
    if (-not (Test-Interactive)) {
        throw "Service installation on Windows requires running PowerShell as Administrator."
    }
    if (-not (Confirm-Choice "Windows service installation needs Administrator rights. Elevate now?" $true)) {
        $script:Service = "no"
        Resolve-Defaults
        return
    }

    $argsList = Get-ForwardArgs
    $tmpScript = Join-Path $env:TEMP ("ayatsuri-installer-" + [guid]::NewGuid().ToString("N") + ".ps1")
    [IO.File]::WriteAllText($tmpScript, $Script:InstallerSource, [Text.Encoding]::UTF8)
    Start-Process -Verb RunAs powershell -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $tmpScript) + $argsList | Out-Null
    exit 0
}

function Ensure-ElevatedForUninstall {
    if (-not $Uninstall -or $DryRun) {
        return
    }
    if (Test-IsAdmin) {
        return
    }
    $needsElevation = $Script:UninstallServicePresent
    if (-not $needsElevation) {
        $protectedRoots = @(
            Normalize-InstallerPath ${env:ProgramFiles},
            Normalize-InstallerPath ${env:ProgramFiles(x86)},
            Normalize-InstallerPath $env:ProgramData
        ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
        foreach ($path in @($Script:UninstallInstallPaths + $Script:UninstallPathScopes + $Script:UninstallAyatsuriHomes)) {
            $normalized = Normalize-InstallerPath $path
            if ([string]::IsNullOrWhiteSpace($normalized)) {
                continue
            }
            if ($protectedRoots | Where-Object { $normalized.StartsWith($_, [System.StringComparison]::OrdinalIgnoreCase) }) {
                $needsElevation = $true
                break
            }
        }
    }
    if (-not $needsElevation) {
        return
    }
    if (-not (Test-Interactive)) {
        throw "Windows uninstall requires running PowerShell as Administrator for service or machine-scoped removals."
    }
    if (-not (Confirm-Choice "Windows uninstall needs Administrator rights. Elevate now?" $true)) {
        throw "Windows uninstall requires elevation for the detected service or machine-scoped install."
    }

    $argsList = Get-ForwardArgs
    $tmpScript = Join-Path $env:TEMP ("ayatsuri-installer-" + [guid]::NewGuid().ToString("N") + ".ps1")
    [IO.File]::WriteAllText($tmpScript, $Script:InstallerSource, [Text.Encoding]::UTF8)
    Start-Process -Verb RunAs powershell -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $tmpScript) + $argsList | Out-Null
    exit 0
}

function Invoke-Download {
    param(
        [string]$Url,
        [string]$OutFile
    )
    Invoke-WebRequest -Uri $Url -OutFile $OutFile
}

function Verify-AyatsuriArchive {
    param(
        [string]$ArchiveFile,
        [string]$AssetName,
        [string]$TempDir
    )
    $checksums = Join-Path $TempDir "checksums.txt"
    Invoke-Download -Url "$($Script:ReleaseBase)/download/$Version/checksums.txt" -OutFile $checksums
    $matching = Select-String -Path $checksums -Pattern ([regex]::Escape($AssetName) + '$') | Select-Object -First 1
    if (-not $matching) {
        throw "Checksum entry for $AssetName was not found."
    }
    $expected = ($matching.Line -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -Path $ArchiveFile -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expected -ne $actual) {
        throw "Checksum verification failed for $AssetName."
    }
}

function New-TempDir {
    $path = Join-Path $env:TEMP ("ayatsuri-installer-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $path | Out-Null
    return $path
}

function Install-AyatsuriBinary {
    $arch = Get-WindowsArch
    $tmpDir = New-TempDir
    try {
        $asset = "ayatsuri_$($Version.TrimStart('v'))_windows_$arch.tar.gz"
        $archive = Join-Path $tmpDir $asset
        $extractDir = Join-Path $tmpDir "extract"
        New-Item -ItemType Directory -Force -Path $extractDir | Out-Null
        Write-Info "Downloading Ayatsuri $Version"
        Invoke-Download -Url "$($Script:ReleaseBase)/download/$Version/$asset" -OutFile $archive
        Verify-AyatsuriArchive -ArchiveFile $archive -AssetName $asset -TempDir $tmpDir
        tar -xzf $archive -C $extractDir
        $sourceExe = Join-Path $extractDir "ayatsuri.exe"
        if (-not (Test-Path $sourceExe)) {
            throw "ayatsuri.exe was not found in the downloaded archive."
        }
        if ($DryRun) {
            Write-Info "Would install $sourceExe to $AyatsuriExe"
            return
        }
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        Move-Item -Path $sourceExe -Destination $AyatsuriExe -Force
        Write-Success "Installed Ayatsuri to $AyatsuriExe"
    }
    finally {
        Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    }
}

function Ensure-PathEntry {
    if ($DryRun) {
        Write-Info "Would update PATH to include $InstallDir"
        return
    }
    if ($Service -eq "yes" -and (Test-IsAdmin)) {
        $scope = "Machine"
    } else {
        $scope = "User"
    }
    $current = [Environment]::GetEnvironmentVariable("Path", $scope)
    if ($current -and ($current -split ';' | Where-Object { $_ -eq $InstallDir })) {
        return
    }
    $updated = if ([string]::IsNullOrWhiteSpace($current)) { $InstallDir } else { "$current;$InstallDir" }
    [Environment]::SetEnvironmentVariable("Path", $updated, $scope)
    Write-Success "Updated the $scope PATH to include $InstallDir"
}

function Get-WinSWDownloadUrl {
    $arch = Get-WindowsArch
    $asset = Get-WrapperAssetName -Arch $arch
    return "$($Script:WinSWBase)/$asset"
}

function Install-WinSWWrapper {
    if ($DryRun) {
        Write-Info "Would download the Windows service wrapper to $ServiceWrapperExe"
        return
    }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Invoke-Download -Url (Get-WinSWDownloadUrl) -OutFile $ServiceWrapperExe
    Write-Success "Downloaded the Windows service wrapper"
}

function Write-ServiceXml {
    param([switch]$IncludeBootstrap)

    $logDir = Join-Path $AyatsuriHome "logs"
    $xml = @"
<service>
  <id>$($Script:ServiceName)</id>
  <name>Ayatsuri</name>
  <description>Ayatsuri Workflow Engine</description>
  <executable>$(Escape-XmlValue $Script:AyatsuriExe)</executable>
  <arguments>start-all</arguments>
  <workingdirectory>$(Escape-XmlValue $Script:AyatsuriHome)</workingdirectory>
  <startmode>Automatic</startmode>
  <onfailure action="restart" delay="10 sec"/>
  <resetfailure>1 hour</resetfailure>
  <stoptimeout>30 sec</stoptimeout>
  <logpath>$(Escape-XmlValue $logDir)</logpath>
  <log mode="append" />
  <env name="DAGU_HOME" value="$(Escape-XmlValue $Script:AyatsuriHome)" />
  <env name="AYATSURI_HOST" value="$(Escape-XmlValue $Script:HostAddress)" />
  <env name="AYATSURI_PORT" value="$(Escape-XmlValue $Script:Port)" />
"@
    if ($IncludeBootstrap -and (Has-AdminBootstrap)) {
        $xml += @"
  <env name="AYATSURI_AUTH_BUILTIN_INITIAL_ADMIN_USERNAME" value="$(Escape-XmlValue $AdminUsername)" />
  <env name="AYATSURI_AUTH_BUILTIN_INITIAL_ADMIN_PASSWORD" value="$(Escape-XmlValue $AdminPassword)" />
"@
    }
    $xml += @"
</service>
"@
    if ($DryRun) {
        Write-Info "Would write $ServiceConfigXml"
        return
    }
    New-Item -ItemType Directory -Force -Path $AyatsuriHome | Out-Null
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    if (Test-Path $ServiceConfigXml) {
        Copy-Item $ServiceConfigXml "$ServiceConfigXml.$((Get-Date).ToString('yyyyMMddHHmmss')).bak"
    }
    Set-Content -Path $ServiceConfigXml -Value $xml -Encoding UTF8
}

function Invoke-WinSW {
    param([string]$Command)
    if ($DryRun) {
        Write-Info "Would run $ServiceWrapperExe $Command"
        return
    }
    & $ServiceWrapperExe $Command | Out-Null
}

function Install-WindowsService {
    if ($Service -ne "yes") {
        return
    }
    Install-WinSWWrapper
    Write-ServiceXml -IncludeBootstrap
    if ($DryRun) {
        Write-Info "Would install and start the Ayatsuri Windows service"
        return
    }
    try { Invoke-WinSW stop } catch {}
    try { Invoke-WinSW uninstall } catch {}
    Invoke-WinSW install
    Invoke-WinSW start
    Write-Success "Installed the Ayatsuri Windows service"
}

function Wait-ForHealth {
    param([int]$Attempts = 60)
    if ($DryRun -or $Service -ne "yes") {
        return $true
    }
    for ($i = 0; $i -lt $Attempts; $i++) {
        try {
            Invoke-WebRequest -Uri "$ServiceUrl/api/v1/health" -UseBasicParsing | Out-Null
            return $true
        }
        catch {
            Start-Sleep -Seconds 1
        }
    }
    return $false
}

function Test-AdminLogin {
    if ($DryRun -or $Service -ne "yes") {
        return $true
    }
    $body = @{ username = $AdminUsername; password = $AdminPassword } | ConvertTo-Json
    try {
        $response = Invoke-RestMethod -Method Post -Uri "$ServiceUrl/api/v1/auth/login" -ContentType "application/json" -Body $body
        return [bool]$response.token
    }
    catch {
        return $false
    }
}

function Verify-Bootstrap {
    if ($Service -ne "yes") {
        return
    }
    if (-not (Has-AdminBootstrap)) {
        Write-WarnMessage "No initial admin credentials were provided. Open $ServiceUrl/setup to finish the first-time setup."
        return
    }
    if (-not (Wait-ForHealth)) {
        throw "Ayatsuri did not become healthy after the Windows service started."
    }
    if (-not (Test-AdminLogin)) {
        throw "Ayatsuri started, but the initial admin login did not verify."
    }
    $backupPath = $null
    $backupDir = $null
    if (Test-Path $ServiceConfigXml) {
        $backupDir = New-TempDir
        $backupPath = Join-Path $backupDir "ayatsuri-service.xml"
        Copy-Item $ServiceConfigXml $backupPath -Force
    }
    Write-ServiceXml
    try {
        if (-not $DryRun) {
            Invoke-WinSW stop
            Invoke-WinSW start
        }
        if (-not (Wait-ForHealth)) {
            throw "Ayatsuri did not return after removing the bootstrap credentials."
        }
        if (-not (Test-AdminLogin)) {
            throw "The admin login no longer works after the bootstrap cleanup."
        }
    }
    catch {
        if ($backupPath -and (Test-Path $backupPath)) {
            Copy-Item $backupPath $ServiceConfigXml -Force
            if (-not $DryRun) {
                try { Invoke-WinSW stop } catch {}
                try { Invoke-WinSW start } catch {}
            }
            Write-WarnMessage "Restored the bootstrap configuration so you can retry safely."
        }
        throw
    }
    finally {
        if ($backupDir) {
            Remove-Item -Recurse -Force $backupDir -ErrorAction SilentlyContinue
        }
    }
    Write-Success "Verified the admin bootstrap and removed the bootstrap credentials from the service config"
}

function Install-AISkill {
    if ($SkillMode -eq "skip") {
        return
    }
    if ($DryRun) {
        if ($SkillMode -eq "explicit" -or $SkillMode -eq "auto") {
            Write-Info "Would install the Ayatsuri AI skill"
        }
        return
    }
    if ($SkillMode -eq "explicit" -and $SkillsDir.Count -gt 0) {
        foreach ($dir in $SkillsDir) {
            & $AyatsuriExe ai install --skills-dir $dir
        }
        return
    }
    if ($SkillMode -eq "auto" -and $DetectedSkillTargets -gt 0) {
        $output = & $AyatsuriExe ai install --yes 2>&1 | Out-String
        Write-Host $output.TrimEnd()
        if ($output -match "No AI coding tools detected") {
            Write-WarnMessage "No supported AI tool was detected."
        }
        return
    }
    if ((Test-Interactive) -and (Confirm-Choice "Install the Ayatsuri AI skill into a custom skills directory?" $false)) {
        $dir = Read-Default "Skills directory" (Join-Path ([Environment]::GetFolderPath("UserProfile")) ".agents\skills")
        if ($dir) {
            & $AyatsuriExe ai install --skills-dir $dir
            return
        }
    }
    if ((Test-Interactive) -and (Get-Command npx -ErrorAction SilentlyContinue) -and (Confirm-Choice "Use the shared skills installer instead?" $false)) {
        & npx skills add https://github.com/ayatsuri-lab/ayatsuri --skill ayatsuri
    }
}

function Open-BrowserIfRequested {
    if ($OpenBrowser -ne "yes" -or $DryRun -or -not (Test-Interactive)) {
        return
    }
    if (-not (Confirm-Choice "Open Ayatsuri in your browser now?" $true)) {
        return
    }
    Start-Process $ServiceUrl | Out-Null
}

function Show-Summary {
    Write-Section "Success"
    Write-Host ("Installed".PadRight(20) + $AyatsuriExe)
    if ($Service -eq "yes") {
        Write-Host ("Service URL".PadRight(20) + $ServiceUrl)
        Write-Host ("Service".PadRight(20) + $Script:ServiceName)
        Write-Host ("Logs".PadRight(20) + (Join-Path $AyatsuriHome "logs"))
        Write-Host ("Status".PadRight(20) + "Get-Service $($Script:ServiceName)")
    }
    if ($AdminUsername) {
        Write-Host ("Admin username".PadRight(20) + $AdminUsername)
    }
    elseif ($Service -eq "yes") {
        Write-Host ("First-time setup".PadRight(20) + "$ServiceUrl/setup")
    }
}

Show-Banner
Choose-OperationMode

if ($Uninstall) {
    Validate-UninstallArgs
    Discover-UninstallArtifacts
    Validate-UninstallDiscovery
    Invoke-UninstallWizard
    Ensure-ElevatedForUninstall
    Show-UninstallPlan
    Invoke-Uninstall
    if (Test-UninstallHasAnything) {
        Show-UninstallSummary
    }
    return
}

Get-LatestVersion
Detect-SkillTargets
Resolve-Defaults
Invoke-InstallerWizard
Resolve-Defaults
Validate-AdminBootstrap
Ensure-ElevatedForService
Show-Plan

if ($DryRun) {
    Write-Success "Dry run complete. No changes were made."
    return
}

Install-AyatsuriBinary
Ensure-PathEntry
Install-WindowsService
Verify-Bootstrap
Install-AISkill
Show-Summary
Open-BrowserIfRequested
