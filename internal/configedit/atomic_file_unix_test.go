//go:build linux || darwin

package configedit

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// A failed staged write (not just rejected validation) leaves original bytes
// intact and removes its temporary file. The limit is applied only in a child.
func TestInterruptedStagedWrite(t *testing.T) {
	if os.Getenv("SEC_CONFIGEDIT_LIMIT_TEST") == "1" {
		directory := os.Getenv("SEC_CONFIGEDIT_TEST_DIRECTORY")
		file := filepath.Join(directory, "sec.toml")
		before, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		signal.Ignore(syscall.SIGXFSZ)
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 1, Max: 1}); err != nil {
			t.Fatal(err)
		}
		hash := SHA256(before)
		if err := ReplaceFileAtomically(file, []byte(strings.Repeat("replacement", 100)), &hash); err == nil {
			t.Fatal("write unexpectedly succeeded")
		}
		data, err := os.ReadFile(file)
		if err != nil || string(data) != string(before) {
			t.Fatal("original changed", err)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 1 {
			t.Fatal("temporary leaked", entries, err)
		}
		return
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "sec.toml"), []byte("original manifest bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestInterruptedStagedWrite$")
	command.Env = append(os.Environ(), "SEC_CONFIGEDIT_LIMIT_TEST=1", "SEC_CONFIGEDIT_TEST_DIRECTORY="+directory)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}
