package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func tempBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, binaryNameForTest())

	cmd := exec.Command("go", "build", "-o", out, "../../cmd/costaffective/")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build temp binary: %v", err)
	}
	return out
}

func binaryNameForTest() string {
	if runtime.GOOS == "windows" {
		return "costaffective_test.exe"
	}
	return "costaffective_test"
}

func TestInstallBinary(t *testing.T) {
	binPath := tempBinary(t)

	// Simulate what InstallBinary does: copy to a target path
	targetDir := filepath.Join(t.TempDir(), "bin")
	targetPath := filepath.Join(targetDir, "costaffective")
	os.MkdirAll(targetDir, 0755)

	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, data, 0755); err != nil {
		t.Fatal(err)
	}

	if !Exists(targetPath) {
		t.Fatal("binary should exist at target path")
	}

	SetBinaryPath(targetPath)
	if BinaryPath() != targetPath {
		t.Fatalf("BinaryPath() = %q, want %q", BinaryPath(), targetPath)
	}
}

func TestBinaryExecutable(t *testing.T) {
	binPath := tempBinary(t)

	fi, err := os.Stat(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0111 == 0 {
		t.Fatal("binary should be executable")
	}

	// Verify it runs
	cmd := exec.Command(binPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("binary should launch: %v", err)
	}
	if !strings.Contains(string(out), "1.0.0") {
		t.Fatalf("version output = %q, want 1.0.0", string(out))
	}
}

func TestCheckBinary(t *testing.T) {
	binPath := tempBinary(t)
	SetBinaryPath(binPath)

	result := CheckBinary()
	if !result.Exists {
		t.Fatal("CheckBinary should find the binary")
	}
	if !result.Executable {
		t.Fatal("binary should be executable")
	}
	if result.Version == "" {
		t.Fatal("binary version should not be empty")
	}
}

func TestVerifyBinary(t *testing.T) {
	binPath := tempBinary(t)

	if err := VerifyBinary(binPath); err != nil {
		t.Fatalf("VerifyBinary should pass: %v", err)
	}

	// Test with non-existent path
	err := VerifyBinary("/nonexistent/costaffective")
	if err == nil {
		t.Fatal("VerifyBinary should fail for nonexistent path")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention 'not found': %v", err)
	}
}

func TestDefaultBinaryPath(t *testing.T) {
	path := DefaultBinaryPath()
	if !strings.Contains(path, "costaffective") {
		t.Fatalf("DefaultBinaryPath should contain 'costaffective': %s", path)
	}
	wantSuffix := "costaffective"
	if runtime.GOOS == "windows" {
		wantSuffix = "costaffective.exe"
	}
	if !strings.HasSuffix(path, wantSuffix) {
		t.Fatalf("DefaultBinaryPath should end with %q: %s", wantSuffix, path)
	}
}

func TestGetMcpServerConfig(t *testing.T) {
	testPath := "/home/testuser/.local/bin/costaffective"
	SetBinaryPath(testPath)
	defer SetBinaryPath("")

	cfg := GetMcpServerConfig()
	cmd, ok := cfg["command"].(string)
	if !ok {
		t.Fatalf("command should be string, got %T", cfg["command"])
	}
	if cmd != testPath {
		t.Fatalf("command = %q, want %q", cmd, testPath)
	}

	args, ok := cfg["args"].([]string)
	if !ok {
		// JSON unmarshal produces []interface{}
		if argsIface, ok := cfg["args"].([]interface{}); ok {
			args = make([]string, len(argsIface))
			for i, v := range argsIface {
				args[i] = fmt.Sprint(v)
			}
		}
	}
	if len(args) != 1 || args[0] != "serve" {
		t.Fatalf("args = %v, want [serve]", args)
	}
}

func TestTargetUsesAbsolutePath(t *testing.T) {
	testPath := "/home/testuser/.local/bin/costaffective"
	SetBinaryPath(testPath)
	defer SetBinaryPath("")

	// Test that GetMcpServerConfig includes the absolute path
	cfg := GetMcpServerConfig()
	cmd := cfg["command"].(string)
	if cmd != testPath {
		t.Errorf("GetMcpServerConfig command = %q, want %q", cmd, testPath)
	}
}

func TestTildify(t *testing.T) {
	home, _ := os.UserHomeDir()
	tilded := Tildify(filepath.Join(home, "test", "file"))
	if !strings.HasPrefix(tilded, "~/") {
		t.Fatalf("Tildify should produce ~/ prefix: %s", tilded)
	}
}

func TestIsDirWritable(t *testing.T) {
	dir := t.TempDir()
	if !IsDirWritable(dir) {
		t.Fatal("temp dir should be writable")
	}

	if IsDirWritable("/nonexistent") {
		t.Fatal("/nonexistent should not be writable")
	}
}

func TestActionableError(t *testing.T) {
	err := ActionableError{
		Message: "something went wrong",
		Action:  "run this command to fix",
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "something went wrong") {
		t.Fatalf("error should contain message: %s", errStr)
	}
	if !strings.Contains(errStr, "run this command") {
		t.Fatalf("error should contain action: %s", errStr)
	}
}

func TestBinaryPathDefault(t *testing.T) {
	SetBinaryPath("")
	want := "costaffective"
	if runtime.GOOS == "windows" {
		want = "costaffective.exe"
	}
	if BinaryPath() != want {
		t.Fatalf("default BinaryPath should be %q, got %q", want, BinaryPath())
	}
}

func TestGetBinaryCandidates(t *testing.T) {
	candidates := GetBinaryCandidates()
	if len(candidates) == 0 {
		t.Fatal("should return at least one candidate")
	}
	for _, c := range candidates {
		if !strings.Contains(c, "costaffective") {
			t.Errorf("candidate %q should contain 'costaffective'", c)
		}
	}
}

func TestWriteJSONFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.json")

	data := map[string]interface{}{
		"name": "test",
		"nested": map[string]interface{}{
			"value": 42,
		},
	}

	if err := WriteJSONFile(file, data); err != nil {
		t.Fatalf("WriteJSONFile: %v", err)
	}

	if !Exists(file) {
		t.Fatal("file should exist after WriteJSONFile")
	}

	// Read back
	read := ReadJSONFile(file)
	if read["name"] != "test" {
		t.Fatalf("read name = %v, want 'test'", read["name"])
	}
}

func TestReadJSONFile(t *testing.T) {
	// Non-existent file returns empty
	empty := ReadJSONFile("/nonexistent/file.json")
	if len(empty) != 0 {
		t.Fatal("ReadJSONFile should return empty for nonexistent file")
	}

	// Invalid JSON returns empty
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.json")
	os.WriteFile(badFile, []byte("not json"), 0644)
	bad := ReadJSONFile(badFile)
	if len(bad) != 0 {
		t.Fatal("ReadJSONFile should return empty for invalid JSON")
	}
}

func TestDeepEqual(t *testing.T) {
	a := map[string]interface{}{"a": 1, "b": "hello"}
	b := map[string]interface{}{"a": 1, "b": "hello"}
	c := map[string]interface{}{"a": 2, "b": "hello"}

	if !DeepEqual(a, b) {
		t.Fatal("DeepEqual should return true for equal maps")
	}
	if DeepEqual(a, c) {
		t.Fatal("DeepEqual should return false for different maps")
	}
}
