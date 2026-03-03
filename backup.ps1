#Requires -Version 5.1
<#
.SYNOPSIS
    Backup script for Subscription Tracker - subscriptions.json data file.

.DESCRIPTION
    Creates compressed backups with daily/weekly/monthly rotation.
    The data is a single JSON file (bind-mounted from ./data/subscriptions.json)
    so no database dump or container interaction is required.

.PARAMETER BackupDir
    Destination directory for backups (e.g., a NAS mount). Defaults to .\backups

.PARAMETER DataDir
    Path to the data directory containing subscriptions.json. Defaults to .\data

.PARAMETER DailyKeep
    Number of daily backups to retain. Default: 7

.PARAMETER WeeklyKeep
    Number of weekly backups to retain (created on Sundays). Default: 4

.PARAMETER MonthlyKeep
    Number of monthly backups to retain (created on 1st of month). Default: 12

.EXAMPLE
    .\backup.ps1 -BackupDir "Z:\Backups\SubscriptionTracker"

.EXAMPLE
    .\backup.ps1 -BackupDir "\\nas\backups\subscriptiontracker" -DailyKeep 14
#>

param(
    [string]$BackupDir = "",
    [string]$DataDir = "",
    [int]$DailyKeep = 7,
    [int]$WeeklyKeep = 4,
    [int]$MonthlyKeep = 12
)

$ErrorActionPreference = "Stop"
$timestamp = Get-Date -Format "yyyy-MM-dd_HHmmss"
$date = Get-Date

# Default paths relative to where the script lives, not the working directory
$scriptDir = $PSScriptRoot
if (-not $DataDir)   { $DataDir   = Join-Path $scriptDir "data" }
if (-not $BackupDir) { $BackupDir = Join-Path $scriptDir "backups" }

# Resolve to absolute paths
$BackupDir = [System.IO.Path]::GetFullPath($BackupDir)
$DataDir   = [System.IO.Path]::GetFullPath($DataDir)

Write-Host "Subscription Tracker Backup" -ForegroundColor Cyan
Write-Host "  Backup destination: $BackupDir"
Write-Host "  Data directory:     $DataDir"
Write-Host ""

# Validate data file exists
$dataFile = Join-Path $DataDir "subscriptions.json"
if (-not (Test-Path $dataFile)) {
    Write-Host "ERROR: Data file not found: $dataFile" -ForegroundColor Red
    Write-Host "Make sure you're running this from the subscription-tracker directory, or specify -DataDir" -ForegroundColor Yellow
    exit 1
}

# Create backup subdirectories
$dailyDir   = Join-Path $BackupDir "daily"
$weeklyDir  = Join-Path $BackupDir "weekly"
$monthlyDir = Join-Path $BackupDir "monthly"
foreach ($dir in @($dailyDir, $weeklyDir, $monthlyDir)) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
}

# Create a temp directory for this backup
$tempDir = Join-Path $env:TEMP "subscription-tracker-backup-$timestamp"
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    # Copy subscriptions.json
    Write-Host "Backing up subscriptions.json..." -ForegroundColor Yellow
    Copy-Item $dataFile (Join-Path $tempDir "subscriptions.json")
    $subCount = (Get-Content $dataFile | ConvertFrom-Json).subscriptions.Count
    Write-Host "  Copied $subCount subscriptions" -ForegroundColor Gray

    # Create compressed archive
    Write-Host "Compressing backup..." -ForegroundColor Yellow
    $zipName = "subscription-tracker-$timestamp.zip"
    $dailyZipPath = Join-Path $dailyDir $zipName
    Compress-Archive -Path "$tempDir\*" -DestinationPath $dailyZipPath -CompressionLevel Optimal
    $zipSize = (Get-Item $dailyZipPath).Length / 1KB
    Write-Host "  Created: $dailyZipPath ($([math]::Round($zipSize, 1)) KB)" -ForegroundColor Green

    # Copy to weekly (Sundays)
    if ($date.DayOfWeek -eq [DayOfWeek]::Sunday) {
        $weeklyZipPath = Join-Path $weeklyDir $zipName
        Copy-Item $dailyZipPath $weeklyZipPath
        Write-Host "  Weekly backup: $weeklyZipPath" -ForegroundColor Green
    }

    # Copy to monthly (1st of month)
    if ($date.Day -eq 1) {
        $monthlyZipPath = Join-Path $monthlyDir $zipName
        Copy-Item $dailyZipPath $monthlyZipPath
        Write-Host "  Monthly backup: $monthlyZipPath" -ForegroundColor Green
    }

    # Prune old backups
    Write-Host "Pruning old backups..." -ForegroundColor Yellow

    function Remove-OldBackups {
        param([string]$Dir, [int]$Keep)
        $files = @(Get-ChildItem $Dir -Filter "subscription-tracker-*.zip" | Sort-Object Name -Descending)
        if ($files.Count -gt $Keep) {
            $toRemove = $files | Select-Object -Skip $Keep
            foreach ($file in $toRemove) {
                Remove-Item $file.FullName
                Write-Host "  Removed: $($file.Name)" -ForegroundColor Gray
            }
        }
    }

    Remove-OldBackups -Dir $dailyDir   -Keep $DailyKeep
    Remove-OldBackups -Dir $weeklyDir  -Keep $WeeklyKeep
    Remove-OldBackups -Dir $monthlyDir -Keep $MonthlyKeep

    # Summary
    Write-Host ""
    Write-Host "Backup complete!" -ForegroundColor Green
    $dailyCount   = (Get-ChildItem $dailyDir   -Filter "subscription-tracker-*.zip" | Measure-Object).Count
    $weeklyCount  = (Get-ChildItem $weeklyDir  -Filter "subscription-tracker-*.zip" | Measure-Object).Count
    $monthlyCount = (Get-ChildItem $monthlyDir -Filter "subscription-tracker-*.zip" | Measure-Object).Count
    Write-Host "  Daily:   $dailyCount / $DailyKeep"
    Write-Host "  Weekly:  $weeklyCount / $WeeklyKeep"
    Write-Host "  Monthly: $monthlyCount / $MonthlyKeep"

} finally {
    # Clean up temp directory
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force
    }
}
