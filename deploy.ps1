#Requires -Version 5.1
<#
.SYNOPSIS
    Loads a Subscription Tracker Docker image and starts/updates the container via docker-compose.

.PARAMETER ImageFile
    Path to the .tar.gz image file. Defaults to subscription-tracker.tar.gz
    in the same directory as this script.

.EXAMPLE
    .\deploy.ps1
    .\deploy.ps1 -ImageFile "subscription-tracker.tar.gz"
#>
param(
    [string]$ImageFile = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# -- Resolve image tarball ---------------------------------------------------
if ($ImageFile -eq "") {
    $ImageFile = Join-Path $PSScriptRoot "subscription-tracker.tar.gz"
}
if (-not (Test-Path $ImageFile)) {
    Write-Error "Image file not found: $ImageFile"
    exit 1
}
Write-Host "Using image file: $ImageFile"

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
