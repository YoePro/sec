package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§42,43(2–4),50(1).
func TestInitTransactionRollback(t *testing.T) {
	for _, preserve := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_project", true: "existing_user_file"}[preserve], func(t *testing.T) {
			root := t.TempDir()
			destination := filepath.Join(root, "project")
			plan := &initPlan{}
			if preserve {
				if err := os.Mkdir(destination, 0755); err != nil {
					t.Fatal(err)
				}
				user := filepath.Join(destination, "user.txt")
				if err := os.WriteFile(user, []byte("user-owned"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := plan.addFile(user, []byte("ignored"), true); err != nil {
					t.Fatal(err)
				}
			}
			first := filepath.Join(destination, "cmd", "example", "main.sec")
			manifest := filepath.Join(destination, ".sec", "sec.toml")
			if err := plan.addFile(first, []byte("first"), false); err != nil {
				t.Fatal(err)
			}
			if err := plan.addFile(manifest, []byte("second"), false); err != nil {
				t.Fatal(err)
			}
			injected := errors.New("simulated disk failure")
			count := 0
			err := plan.apply(func(path string) (initWritable, error) {
				count++
				if count == 2 {
					return nil, injected
				}
				return createInitFile(path)
			})
			if !errors.Is(err, injected) {
				t.Fatal(err)
			}
			if fileExists(first) || fileExists(manifest) || fileExists(filepath.Join(destination, "cmd")) || fileExists(filepath.Join(destination, ".sec")) {
				t.Fatal("partial transaction survived rollback")
			}
			if preserve {
				got, err := os.ReadFile(filepath.Join(destination, "user.txt"))
				if err != nil || string(got) != "user-owned" {
					t.Fatal("rollback changed user data", err)
				}
			} else if fileExists(destination) {
				t.Fatal("rollback left newly-created project directory")
			}
		})
	}
}

type failingInitWrite struct{ *os.File }

func (file failingInitWrite) Write(data []byte) (int, error) {
	count, _ := file.File.Write(data[:len(data)/2])
	return count, errors.New("simulated partial write")
}

// Rules: rules/projects/projects.md — §§43(4),50(1).
func TestInitTransactionPartialWriteRollback(t *testing.T) {
	root := t.TempDir()
	plan := &initPlan{}
	target := filepath.Join(root, "new", "starter.sec")
	if err := plan.addFile(target, []byte("partial contents"), false); err != nil {
		t.Fatal(err)
	}
	err := plan.apply(func(path string) (initWritable, error) {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return nil, err
		}
		return failingInitWrite{file}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "simulated partial write") {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(root, "new")) {
		t.Fatal("partial file and new directory survived rollback")
	}
}

// Rules: rules/projects/projects.md — §§42,43(4).
func TestInitTransactionPreservesConcurrentFile(t *testing.T) {
	root := t.TempDir()
	plan := &initPlan{}
	first := filepath.Join(root, "new", "starter.sec")
	second := filepath.Join(root, "manifest.toml")
	if err := plan.addFile(first, []byte("starter"), false); err != nil {
		t.Fatal(err)
	}
	if err := plan.addFile(second, []byte("manifest"), false); err != nil {
		t.Fatal(err)
	}
	err := plan.apply(func(path string) (initWritable, error) {
		if path == second {
			if err := os.WriteFile(second, []byte("concurrent user contents"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		return createInitFile(path)
	})
	if err == nil {
		t.Fatal("overwrote a concurrently-created file")
	}
	got, err := os.ReadFile(second)
	if err != nil || string(got) != "concurrent user contents" {
		t.Fatal(got, err)
	}
	if fileExists(filepath.Join(root, "new")) {
		t.Fatal("earlier create survived conflict rollback")
	}
}

// Rules: rules/projects/projects.md — §§42,43(1–3).
func TestInitTransactionRejectsChangedPreservedFile(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "user.txt")
	if err := os.WriteFile(user, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	plan := &initPlan{}
	if err := plan.addFile(user, []byte("ignored"), true); err != nil {
		t.Fatal(err)
	}
	if err := plan.addFile(filepath.Join(root, "new", "generated.txt"), []byte("new"), false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte("user edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(createInitFile); err == nil {
		t.Fatal("accepted changed preserved file")
	}
	if fileExists(filepath.Join(root, "new")) {
		t.Fatal("conflict applied early creates")
	}
}
