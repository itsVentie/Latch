Write-Host "Running comprehensive checks..." -ForegroundColor Cyan

Write-Host "--- Checking Go Formatter (go fmt) ---" -ForegroundColor Yellow
$fmtFiles = go fmt ./...
if ($fmtFiles) {
    Write-Host "The following files were not properly formatted and have been fixed:" -ForegroundColor Yellow
    $fmtFiles | ForEach-Object { Write-Host "  $_" -ForegroundColor Match }
    Write-Host "Please commit the formatted files." -ForegroundColor Red
    exit 1
}

Write-Host "--- Checking Go Module Tidy (go mod tidy) ---" -ForegroundColor Yellow
git diff go.mod go.sum > $null
go mod tidy
$modDiff = git diff go.mod go.sum
if ($modDiff) {
    Write-Host "Go modules are not tidy. Run 'go mod tidy' and commit changes." -ForegroundColor Red
    git checkout go.mod go.sum > $null
    exit 1
}

Write-Host "--- Running go vet ---" -ForegroundColor Yellow
go vet ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "Code style/vet issues found." -ForegroundColor Red
    exit 1
}

if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    Write-Host "--- Running golangci-lint ---" -ForegroundColor Yellow
    golangci-lint run ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Linter issues found." -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "--- Skipping golangci-lint (tool not installed) ---" -ForegroundColor Gray
}

Write-Host "--- Running tests with race detector ---" -ForegroundColor Yellow
go test -v -race ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "Tests failed." -ForegroundColor Red
    exit 1
}

Write-Host "--- Building project ---" -ForegroundColor Green
go build -ldflags="-s -w" -v -o pqc-proxy.exe ./cmd/pqc-proxy/main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "Everything is OK. Build successful." -ForegroundColor Green
} else {
    Write-Host "Build failed." -ForegroundColor Red
    exit 1
}