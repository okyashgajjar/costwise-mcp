package tests

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestMCPServerStarts(t *testing.T) {
	// Build the binary
	binName := "costaffective_test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binName, "../cmd/costaffective")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer func() { _ = os.Remove(binName) }()

	runCmd := exec.Command("./"+binName, "serve")

	stdin, err := runCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	if err := runCmd.Start(); err != nil {
		t.Fatalf("Failed to start serve command: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- runCmd.Wait()
	}()

	select {
	case err := <-done:
		t.Fatalf("Server exited unexpectedly: %v", err)
	case <-time.After(1 * time.Second):
		stdin.Close()
		<-done
	}
}
