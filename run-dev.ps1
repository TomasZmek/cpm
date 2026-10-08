# Local development runner for CPM on Windows (PowerShell)
# Usage (from the repository root):  .\run-dev.ps1
# Requires caddy-config/ and caddy-data/ directories in the repository root.
$env:CADDY_CONFIG_PATH = "$PSScriptRoot\caddy-config"
$env:CADDY_DATA_PATH   = "$PSScriptRoot\caddy-data"
$env:PORT              = "8501"
# $env:CONTAINER_NAME  = "caddy"   # change if your Caddy container has a different name
Write-Host "CADDY_CONFIG_PATH = $env:CADDY_CONFIG_PATH"
Write-Host "Starting CPM on http://localhost:$env:PORT ..."
go run ./cmd/cpm
