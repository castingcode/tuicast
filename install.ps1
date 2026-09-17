[CmdletBinding()]
param(
    [Alias("b")]
    [string]$BinDir = (Join-Path -Path (Get-Location).Path -ChildPath "bin"),

    [Alias("c")]
    [ValidateSet("driver", "mcp", "reference-tui", "all")]
    [string]$Component = $(if ($env:TUICAST_COMPONENT) { $env:TUICAST_COMPONENT } else { "driver" }),

    [Alias("v")]
    [Parameter(Position = 0)]
    [string]$Version = $(if ($env:TUICAST_VERSION) { $env:TUICAST_VERSION } else { "latest" })
)

$ErrorActionPreference = "Stop"
$Repository = "castingcode/tuicast"

$Architecture = if ($env:PROCESSOR_ARCHITEW6432) {
    $env:PROCESSOR_ARCHITEW6432
} else {
    $env:PROCESSOR_ARCHITECTURE
}

$Arch = switch ($Architecture.ToUpperInvariant()) {
    "AMD64" { "amd64" }
    "X86_64" { "amd64" }
    "ARM64" { "arm64" }
    default { throw "Unsupported architecture: $Architecture" }
}

$Binaries = switch ($Component) {
    "driver" { @("tuicast-driver.exe") }
    "mcp" { @("tuicast-mcp.exe") }
    "reference-tui" { @("reference-tui.exe") }
    "all" { @("tuicast-driver.exe", "tuicast-mcp.exe", "reference-tui.exe") }
}

if ($Version -eq "latest") {
    Write-Host "Finding the latest TUICast release..."
    $Headers = @{ "User-Agent" = "tuicast-installer" }
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest" -Headers $Headers
    $Tag = $Release.tag_name
    if (-not $Tag) {
        throw "Could not determine the latest TUICast release"
    }
} elseif ($Version.StartsWith("v")) {
    $Tag = $Version
} else {
    $Tag = "v$Version"
}

$VersionNumber = $Tag.TrimStart([char]"v")
$ArchiveBase = "tuicast_${VersionNumber}_windows_${Arch}"
$ArchiveName = "$ArchiveBase.zip"
$ReleaseUrl = "https://github.com/$Repository/releases/download/$Tag"
$TempDir = Join-Path -Path ([System.IO.Path]::GetTempPath()) -ChildPath ("tuicast-" + [guid]::NewGuid())

New-Item -ItemType Directory -Path $TempDir | Out-Null
try {
    $ArchivePath = Join-Path $TempDir $ArchiveName
    $ChecksumsPath = Join-Path $TempDir "checksums.txt"

    Write-Host "Downloading TUICast $Tag for windows/$Arch..."
    Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseUrl/$ArchiveName" -OutFile $ArchivePath
    Invoke-WebRequest -UseBasicParsing -Uri "$ReleaseUrl/checksums.txt" -OutFile $ChecksumsPath

    $Pattern = '^([0-9a-fA-F]{64})\s+\*?' + [regex]::Escape($ArchiveName) + '$'
    $ChecksumLine = Get-Content $ChecksumsPath | Where-Object { $_ -match $Pattern } | Select-Object -First 1
    if (-not $ChecksumLine) {
        throw "Checksum not found for $ArchiveName"
    }
    $null = $ChecksumLine -match $Pattern
    $ExpectedChecksum = $Matches[1]
    $ActualChecksum = (Get-FileHash -Algorithm SHA256 -Path $ArchivePath).Hash
    if ($ActualChecksum -ne $ExpectedChecksum) {
        throw "Checksum verification failed for $ArchiveName"
    }

    Expand-Archive -Path $ArchivePath -DestinationPath $TempDir
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    foreach ($Binary in $Binaries) {
        $SourcePath = Join-Path $TempDir "$ArchiveBase/$Binary"
        if (-not (Test-Path -PathType Leaf $SourcePath)) {
            throw "$Binary was not found in $ArchiveName"
        }
        $DestinationPath = Join-Path $BinDir $Binary
        Copy-Item -Force -Path $SourcePath -Destination $DestinationPath
        Write-Host "Installed $DestinationPath"
    }
} finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $TempDir
}
