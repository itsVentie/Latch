package network_test

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	rustDir, err := filepath.Abs("../../../")
	if err != nil {
		log.Fatalf("Failed to resolve absolute path for Rust directory: %v", err)
	}

	buildCmd := exec.Command("cargo", "build")
	buildCmd.Dir = rustDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		log.Fatalf("Failed to build Rust dataplane for tests: %v", err)
	}

	var binaryName string
	if os.Getenv("OS") == "Windows_NT" {
		binaryName = "latch-dataplane.exe"
	} else {
		binaryName = "latch-dataplane"
	}

	binaryPath := filepath.Join(rustDir, "target", "debug", binaryName)
	rustCmd := exec.Command(binaryPath)

	if err := rustCmd.Start(); err != nil {
		log.Fatalf("Failed to start Rust dataplane binary: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	code := m.Run()

	if rustCmd.Process != nil {
		_ = rustCmd.Process.Kill()
	}

	os.Exit(code)
}
