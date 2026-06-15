#!/usr/bin/env pwsh
# deploy.ps1 — Build and deploy PokemonProfessor to LXC
#
# Usage:
#   .\deploy.ps1               # build + deploy via Ansible
#   .\deploy.ps1 -SkipBuild    # deploy existing binary (no rebuild)
#
# Requirements:
#   - Go 1.22+ in PATH
#   - SSH access to your Proxmox host (key auth)
#   - Ansible playbook path set in deploy.config.ps1
#   - Copy deploy.config.ps1.example -> deploy.config.ps1 and fill in your values

param(
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

# ── Local config (gitignored) ──────────────────────────────────────────────────
# Defaults — override by creating deploy.config.ps1 from deploy.config.ps1.example
$BINARY     = "pokemonprofessor-linux-amd64"
$PROXMOX    = "root@<proxmox-host>"
$PLAYBOOK   = "<path-to-playbook>/update-pokemonprofessor.yml"
$HEALTH_URL = "http://<lxc-ip>:8000/health"

$configFile = Join-Path $PSScriptRoot "deploy.config.ps1"
if (Test-Path $configFile) {
    . $configFile
} else {
    Write-Error "deploy.config.ps1 not found. Copy deploy.config.ps1.example and fill in your values."
    exit 1
}

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
    Write-Host "    ssh $PROXMOX `"curl -sf $HEALTH_URL`""
}

Write-Host "==> Done." -ForegroundColor Green

Pop-Location
