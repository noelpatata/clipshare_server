package clip

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeToolScript creates an executable shell script simulating a clipboard
// writer like wl-copy: the parent exits immediately while a forked child
// inherits stderr and outlives it.
func writeToolScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-clip-tool")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// Regression test: runStdin used to capture stderr with a pipe. Daemonizing
// writers (wl-copy/xclip) fork a selection owner that inherits that pipe, so
// cmd.Run() blocked until the child exited — wedging every clipboard write.
// With a temp file for stderr, Run() must return as soon as the parent exits.
func TestRunStdinReturnsDespiteDaemonizedChild(t *testing.T) {
	script := writeToolScript(t, `sleep 30 &
echo parent-diagnostic >&2
exit 0
`)

	start := time.Now()
	err := runStdin(script, nil, "payload")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("runStdin: %v", err)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("runStdin blocked %v behind daemonized child", elapsed)
	}
}

// The temp-file stderr must still surface the parent's diagnostics on failure.
func TestRunStdinCapturesStderrDiagnostics(t *testing.T) {
	script := writeToolScript(t, `echo boom-reason >&2
exit 3
`)

	err := runStdin(script, nil, "payload")
	if err == nil {
		t.Fatal("expected error from failing tool")
	}
	if !strings.Contains(err.Error(), "boom-reason") {
		t.Errorf("error missing stderr diagnostics: %v", err)
	}
}

func TestRunStdinMissingToolFails(t *testing.T) {
	if _, err := exec.LookPath("definitely-not-installed-clipshare"); err == nil {
		t.Skip("unexpectedly found fake tool name")
	}
	if err := runStdin("definitely-not-installed-clipshare", nil, "x"); err == nil {
		t.Fatal("expected error for missing binary")
	}
}
