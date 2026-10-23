$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path))
$version = (Get-Content VERSION -Raw).Trim()
go vet ./...
go test ./...
go build -trimpath -ldflags "-s -w -X main.version=$version" -o ValorantCNOverlay.exe ./src/cmd/valorant-overlay
$hash = (Get-FileHash "ValorantCNOverlay.exe" -Algorithm SHA256).Hash.ToLower()
"$hash  ValorantCNOverlay.exe" | Set-Content -Encoding ascii "CHECKSUMS.sha256"
Write-Host "Built: ValorantCNOverlay.exe (v$version)"
Write-Host "SHA256: $hash"
