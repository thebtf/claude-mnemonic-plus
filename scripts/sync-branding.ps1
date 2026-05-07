# sync-branding.ps1
# Copy canonical brand assets from assets/branding/ to all deployable locations.
# Run after editing any SVG in assets/branding/.

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$src = Join-Path $repoRoot "assets/branding"

$targets = @(
    @{ Path = Join-Path $repoRoot "ui/public/branding";   Description = "UI dashboard" }
    @{ Path = Join-Path $repoRoot "docs/public/branding"; Description = "Docs site" }
)

# Favicon at the root path of each Vite public dir (so /favicon.svg works).
$rootFavicons = @(
    Join-Path $repoRoot "ui/public/favicon.svg"
    Join-Path $repoRoot "docs/public/favicon.svg"
)

if (-not (Test-Path $src)) {
    Write-Error "Source directory not found: $src"
    exit 1
}

$svgFiles = Get-ChildItem $src -Filter *.svg
if ($svgFiles.Count -eq 0) {
    Write-Error "No SVG files in $src"
    exit 1
}
$pngFiles = Get-ChildItem $src -Filter *.png -ErrorAction SilentlyContinue
$assetFiles = @($svgFiles) + @($pngFiles)
$sourceNames = $assetFiles.Name

Write-Host "Source: $src ($($svgFiles.Count) SVGs, $($pngFiles.Count) PNGs)"
Write-Host ""

foreach ($target in $targets) {
    $dest = $target.Path
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    # Remove stale assets that no longer exist in the source directory.
    Get-ChildItem -Path $dest -File -Include *.svg,*.png -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -notin $sourceNames } |
        Remove-Item -Force
    foreach ($asset in $assetFiles) {
        Copy-Item $asset.FullName (Join-Path $dest $asset.Name) -Force
    }
    Write-Host "  -> $($target.Description): $dest"
}

$faviconSrc = Join-Path $src "favicon.svg"
foreach ($f in $rootFavicons) {
    Copy-Item $faviconSrc $f -Force
    Write-Host "  -> root favicon: $f"
}

Write-Host ""
Write-Host "Branding synced." -ForegroundColor Green
