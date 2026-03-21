#!/usr/bin/env pwsh
# deploy.ps1 — Build and deploy PokemonProfessor to LXC 131
#
# Usage:
#   .\deploy.ps1               # build + deploy via Ansible
#   .\deploy.ps1 -SkipBuild    # deploy existing binary (no rebuild)
#
# Requirements:
#   - Go 1.22+ in PATH
#   - SSH access to root@192.168.1.100 (key auth)
#   - D:\GitHub\proxmox\playbooks\update-pokemonprofessor.yml present

param(
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

$BINARY    = "pokemonprofessor-linux-amd64"
$PROXMOX   = "root@192.168.1.100"
$PLAYBOOK  = "D:\GitHub\proxmox\playbooks\update-pokemonprofessor.yml"
$HEALTH_URL = "http://192.168.1.131:8000/health"

Push-Location $PSScriptRoot

# ── Build ──────────────────────────────────────────────────────────────────────
if (-not $SkipBuild) {
    Write-Host "==> Building Linux amd64 binary..." -ForegroundColor Cyan
    $env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w -X main.Version=dev" -o $BINARY ./cmd/professor-arbortom
    if ($LASTEXITCODE -ne 0) { Write-Error "Build failed"; exit 1 }
    $env:GOOS = ""; $env:GOARCH = ""; $env:CGO_ENABLED = ""
    Write-Host "    Built: $((Get-Item $BINARY).LastWriteTime)  $([math]::Round((Get-Item $BINARY).Length/1MB,1)) MB" -ForegroundColor Green
} else {
    Write-Host "==> Skipping build (using existing binary)" -ForegroundColor Yellow
    if (-not (Test-Path $BINARY)) { Write-Error "Binary not found: $BINARY"; exit 1 }
}

# ── Copy to Proxmox ────────────────────────────────────────────────────────────
Write-Host "==> Copying binary to Proxmox host..." -ForegroundColor Cyan
scp $BINARY "${PROXMOX}:/tmp/${BINARY}"
if ($LASTEXITCODE -ne 0) { Write-Error "scp binary failed"; exit 1 }

Write-Host "==> Copying Ansible playbook to Proxmox host..." -ForegroundColor Cyan
if (-not (Test-Path $PLAYBOOK)) { Write-Error "Playbook not found: $PLAYBOOK"; exit 1 }
scp $PLAYBOOK "${PROXMOX}:/tmp/update-pokemonprofessor.yml"
if ($LASTEXITCODE -ne 0) { Write-Error "scp playbook failed"; exit 1 }

# ── Run Ansible update playbook ────────────────────────────────────────────────
Write-Host "==> Running Ansible update playbook on Proxmox..." -ForegroundColor Cyan
ssh $PROXMOX "ansible-playbook /tmp/update-pokemonprofessor.yml"
if ($LASTEXITCODE -ne 0) { Write-Error "Ansible playbook failed"; exit 1 }

# ── Health check (from Windows) ────────────────────────────────────────────────
Write-Host "==> Final health check from local machine..." -ForegroundColor Cyan
try {
    $health = (Invoke-WebRequest -Uri $HEALTH_URL -UseBasicParsing -TimeoutSec 10).Content
    Write-Host "    $health" -ForegroundColor Green
} catch {
    Write-Warning "Direct health check failed (Tailscale/VPN required). Check via Proxmox:"
    Write-Host "    ssh $PROXMOX `"curl -sf http://192.168.1.131:8000/health`""
}

Write-Host "==> Done." -ForegroundColor Green

Pop-Location
