#!/usr/bin/env bash

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m'

echo -e "${CYAN}Running comprehensive checks...${NC}"

export GOTMPDIR="${TMPDIR:-/tmp}"

echo -e "${YELLOW}--- Checking Go Formatter (go fmt) ---${NC}"
fmt_files=$(go fmt ./...)
if [ -n "$fmt_files" ]; then
    echo -e "${YELLOW}The following files were not properly formatted and have been fixed:${NC}"
    echo "$fmt_files" | while read -r file; do
        echo -e "   $file"
    done
    echo -e "${RED}Please commit the formatted files.${NC}"
    exit 1
fi

echo -e "${YELLOW}--- Checking Go Module Tidy (go mod tidy) ---${NC}"
git diff go.mod go.sum > /dev/null 2>&1
go mod tidy
mod_diff=$(git diff go.mod go.sum)
if [ -n "$mod_diff" ]; then
    echo -e "${RED}Go modules are not tidy. Run 'go mod tidy' and commit changes.${NC}"
    git checkout go.mod go.sum > /dev/null 2>&1
    exit 1
fi

echo -e "${YELLOW}--- Running go vet ---${NC}"
if ! go vet ./...; then
    echo -e "${RED}Code style/vet issues found.${NC}"
    exit 1
fi

if command -v golangci-lint &> /dev/null; then
    echo -e "${YELLOW}--- Running golangci-lint ---${NC}"
    if ! golangci-lint run ./...; then
        echo -e "${RED}Linter issues found.${NC}"
        exit 1
    fi
else
    echo -e "${GRAY}--- Skipping golangci-lint (tool not installed) ---${NC}"
fi

echo -e "${YELLOW}--- Running fast parser fuzzing test (3s) ---${NC}"
if ! go test -fuzz=FuzzSecureConnRead -fuzztime=3s ./internal/crypto; then
    echo -e "${RED}Fuzzing test failed or found a vulnerability/panic.${NC}"
    exit 1
fi

echo -e "${YELLOW}--- Running tests with race detector ---${NC}"
if ! go test -v -race ./...; then
    echo -e "${RED}Tests failed.${NC}"
    exit 1
fi

echo -e "${GREEN}--- Building project for Windows and Linux ---${NC}"

PACKAGE_PATH="./cmd/pqc-proxy"
DIST_DIR="./dist"

mkdir -p "$DIST_DIR"

echo -e "${YELLOW}Building Windows binary (dist/latch.exe)...${NC}"
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/latch.exe" "$PACKAGE_PATH"
if [ $? -ne 0 ]; then
    echo -e "${RED}Windows build failed.${NC}"
    exit 1
fi

echo -e "${YELLOW}Building Linux binary (dist/latch-linux)...${NC}"
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$DIST_DIR/latch-linux" "$PACKAGE_PATH"
if [ $? -ne 0 ]; then
    echo -e "${RED}Linux build failed.${NC}"
    exit 1
fi

echo -e "${GREEN}Everything is OK. Both builds saved to /dist successfully${NC}"