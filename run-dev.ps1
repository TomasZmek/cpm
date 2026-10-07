# CPM 로컬 개발 실행 스크립트
# 사용법: repo 루트에서  .\run-dev.ps1
$env:CADDY_CONFIG_PATH = "$PSScriptRoot\caddy-config"
$env:CADDY_DATA_PATH   = "$PSScriptRoot\caddy-data"
$env:PORT              = "8501"
# $env:CONTAINER_NAME  = "caddy"   # 필요시 Caddy 컨테이너 이름 변경
Write-Host "CADDY_CONFIG_PATH = $env:CADDY_CONFIG_PATH"
Write-Host "Starting CPM on http://localhost:$env:PORT ..."
go run ./cmd/cpm
