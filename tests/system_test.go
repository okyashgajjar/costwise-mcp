package tests

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestMCPServerStarts(t *testing.T) {
	// Build the binary
	cmd := exec.Command("go", "build", "-o", "mycli_test", "../cmd/mycli")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mycli_test")

	runCmd := exec.Command("./mycli_test", "serve")
	
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
