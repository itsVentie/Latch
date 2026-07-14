Write-Host "Running comprehensive checks..." -ForegroundColor Cyan

$env:GOTMPDIR = $env:TEMP

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

Write-Host "--- Running fast parser fuzzing test (3s) ---" -ForegroundColor Yellow
go test -fuzz=FuzzSecureConnRead -fuzztime=3s ./internal/crypto
if ($LASTEXITCODE -ne 0) {
    Write-Host "Fuzzing test failed or found a vulnerability/panic." -ForegroundColor Red
    exit 1
}

Write-Host "--- Running tests with race detector ---" -ForegroundColor Yellow
go test -v -race ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "Tests failed." -ForegroundColor Red
    exit 1
}

Write-Host "--- Building project for Windows and Linux ---" -ForegroundColor Green

$packagePath = "./cmd/pqc-proxy"
$distDir = "./dist"

if (-not (Test-Path $distDir)) {
    New-Item -ItemType Directory -Path $distDir > $null
}

Write-Host "Building Windows binary (dist/latch.exe)..." -ForegroundColor Yellow
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o "$distDir/latch.exe" $packagePath
if ($LASTEXITCODE -ne 0) {
    Write-Host "Windows build failed." -ForegroundColor Red
    exit 1
}

Write-Host "Building Linux binary (dist/latch-linux)..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o "$distDir/latch-linux" $packagePath
if ($LASTEXITCODE -ne 0) {
    Write-Host "Linux build failed." -ForegroundColor Red
    $env:GOOS = "windows"
    exit 1
}

$env:GOOS = "windows"

Write-Host "Everything is OK. Both builds saved to /dist successfully" -ForegroundColor Green