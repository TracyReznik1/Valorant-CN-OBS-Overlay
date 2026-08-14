$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path))
go test ./...
go build -trimpath -ldflags "-s -w" -o ValorantCNOverlay.exe ./src/cmd/valorant-overlay
$hash = (Get-FileHash "ValorantCNOverlay.exe" -Algorithm SHA256).Hash.ToLower()
"$hash  ValorantCNOverlay.exe" | Set-Content -Encoding ascii "CHECKSUMS.sha256"
Write-Host "Built: ValorantCNOverlay.exe"
Write-Host "SHA256: $hash"
