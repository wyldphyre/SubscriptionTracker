#Requires -Version 5.1
<#
.SYNOPSIS
    Loads a Subscription Tracker Docker image and starts/updates the container via docker-compose.

.PARAMETER ImageFile
    Path to the .tar.gz image file. Defaults to the newest
    subscription-tracker-*.tar.gz in the current directory.

.EXAMPLE
    .\deploy.ps1
    .\deploy.ps1 -ImageFile "subscription-tracker-20260302.tar.gz"
#>
param(
    [string]$ImageFile = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# -- Resolve image tarball ---------------------------------------------------
if ($ImageFile -eq "") {
    $tarballs = @(Get-ChildItem -Filter "subscription-tracker-*.tar.gz" -ErrorAction SilentlyContinue |
                  Sort-Object LastWriteTime -Descending)
    if ($tarballs.Count -eq 0) {
        Write-Error "No subscription-tracker-*.tar.gz found in the current directory. Specify -ImageFile."
        exit 1
    }
    $ImageFile = $tarballs[0].FullName
    Write-Host "Using image file: $ImageFile"
} else {
    if (-not (Test-Path $ImageFile)) {
        Write-Error "Image file not found: $ImageFile"
        exit 1
    }
}

# -- Load Docker image -------------------------------------------------------
Write-Host "==> Loading Docker image from $([System.IO.Path]::GetFileName($ImageFile))..."
docker load -i $ImageFile
if ($LASTEXITCODE -ne 0) { Write-Error "docker load failed"; exit 1 }

# -- Deploy via docker-compose -----------------------------------------------
Write-Host "==> Starting container via docker-compose..."
docker-compose up -d --force-recreate
if ($LASTEXITCODE -ne 0) { Write-Error "docker-compose up failed"; exit 1 }

Write-Host ""
Write-Host "Deployed successfully!"
Write-Host "Port and data path are configured in docker-compose.yml"
