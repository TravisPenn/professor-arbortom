#!/usr/bin/env pwsh
# deploy.ps1 — Build and deploy PokemonProfessor to LXC 131
#
# Usage:
#   .\deploy.ps1               # build + deploy
#   .\deploy.ps1 -SkipBuild    # deploy existing binary (no rebuild)
#
# Requirements:
#   - Go 1.22+ in PATH
#   - SSH access to root@192.168.1.100 (key auth)

param(
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

$BINARY    = "pokemonprofessor-linux-amd64"
$PROXMOX   = "root@192.168.1.100"
$LXC_ID    = 131
$DEST_BIN  = "/usr/local/bin/pokemonprofessor"
$HEALTH_URL = "http://192.168.1.131:8000/health"

Push-Location $PSScriptRoot

# ── Build ──────────────────────────────────────────────────────────────────────
if (-not $SkipBuild) {
    Write-Host "==> Building Linux amd64 binary..." -ForegroundColor Cyan
    $env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w -X main.Version=dev" -o $BINARY ./cmd/professor-arbortom
    if ($LASTEXITCODE -ne 0) { Write-Error "Build failed"; exit 1 }
    Write-Host "    Built: $((Get-Item $BINARY).LastWriteTime)  $([math]::Round((Get-Item $BINARY).Length/1MB,1)) MB" -ForegroundColor Green
} else {
    Write-Host "==> Skipping build (using existing binary)" -ForegroundColor Yellow
    if (-not (Test-Path $BINARY)) { Write-Error "Binary not found: $BINARY"; exit 1 }
}

# ── Deploy ─────────────────────────────────────────────────────────────────────
Write-Host "==> Copying to Proxmox host..." -ForegroundColor Cyan
scp $BINARY "${PROXMOX}:/tmp/pokemonprofessor"
if ($LASTEXITCODE -ne 0) { Write-Error "scp failed"; exit 1 }

Write-Host "==> Deploying to LXC $LXC_ID..." -ForegroundColor Cyan
ssh $PROXMOX "pct exec $LXC_ID -- systemctl stop pokemonprofessor"
ssh $PROXMOX "pct push $LXC_ID /tmp/pokemonprofessor $DEST_BIN --perms 0755"
ssh $PROXMOX "rm /tmp/pokemonprofessor"
ssh $PROXMOX "pct exec $LXC_ID -- systemctl start pokemonprofessor"
ssh $PROXMOX "sleep 2"
if ($LASTEXITCODE -ne 0) { Write-Error "Deployment failed"; exit 1 }

# ── Health check ───────────────────────────────────────────────────────────────
Write-Host "==> Health check..." -ForegroundColor Cyan
$health = ssh $PROXMOX "curl -sf $HEALTH_URL"
if ($LASTEXITCODE -ne 0) { Write-Error "Health check failed - service may not be running"; exit 1 }
Write-Host "    $health" -ForegroundColor Green
Write-Host "==> Done." -ForegroundColor Green

Pop-Location
