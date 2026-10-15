//go:build linux

package runtimediag

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMemorySignalDoesNotTerminateProcess(t *testing.T) {
	if os.Getenv("SUB2_TEST_MEMORY_SIGNAL") == "1" {
		stop := StartMemoryDiagnostics()
		defer stop()
		fmt.Println("ready")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		return
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMemorySignalDoesNotTerminateProcess$")
	cmd.Env = append(os.Environ(), "SUB2_TEST_MEMORY_SIGNAL=1", "TMPDIR="+dir)
	stdout, _ := cmd.StdoutPipe()
	stdin, _ := cmd.StdinPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("startup: %q %v", line, err)
	}
	if err := cmd.Process.Signal(syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		files, _ := filepath.Glob(filepath.Join(dir, "sub2api-memory-*", "memory.json"))
		if len(files) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no completed capture")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, _ = fmt.Fprintln(stdin, "stop")
	if err := cmd.Wait(); err != nil {
		t.Fatalf("signal terminated process: %v", err)
	}
}
