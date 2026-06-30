Write-Host "Running tests..." -ForegroundColor Cyan
go test -v -race ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host "Building project..." -ForegroundColor Green
    go build ./cmd/pqc-proxy/...
    Write-Host "Everything is OK!" -ForegroundColor Green
} else {
    Write-Host "Tests failed!" -ForegroundColor Red
}