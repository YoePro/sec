package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// headCountSnapshot reads only the committed YAML subtree, including the HEAD
// index/library scope. It never checks out files or changes the working tree;
// additions/deletions in the current ledger are compared normally.
// Contract: cmd/yamlstatus/IDEAS.md — §7.
func headCountSnapshot(dir string) (countSnapshot, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return countSnapshot{}, err
	}
	rootData, err := exec.Command("git", "-C", absolute, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return countSnapshot{}, fmt.Errorf("-diff HEAD requires a governance directory inside a Git working tree: %w", err)
	}
	root := strings.TrimSpace(string(rootData))
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return countSnapshot{}, fmt.Errorf("governance directory is outside the Git working tree")
	}
	relative = filepath.ToSlash(relative)
	listing, err := exec.Command("git", "-C", root, "ls-tree", "-rz", "--name-only", "HEAD", "--", relative).Output()
	if err != nil {
		return countSnapshot{}, fmt.Errorf("cannot list HEAD governance: %w", err)
	}
	temporary, err := os.MkdirTemp("", "yamlstatus-head-")
	if err != nil {
		return countSnapshot{}, err
	}
	defer os.RemoveAll(temporary)
	files := 0
	for _, raw := range bytes.Split(listing, []byte{0}) {
		name := string(raw)
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		local := name
		if relative != "." {
			local = strings.TrimPrefix(name, relative+"/")
		}
		if local == name && relative != "." || filepath.IsAbs(local) || local == ".." || strings.HasPrefix(local, "../") {
			return countSnapshot{}, fmt.Errorf("invalid HEAD governance fragment path")
		}
		data, err := exec.Command("git", "-C", root, "show", "HEAD:"+name).Output()
		if err != nil {
			return countSnapshot{}, fmt.Errorf("cannot read HEAD governance fragment %s: %w", local, err)
		}
		target := filepath.Join(temporary, filepath.FromSlash(local))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return countSnapshot{}, err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return countSnapshot{}, err
		}
		files++
	}
	if files == 0 {
		return countSnapshot{}, fmt.Errorf("HEAD contains no YAML fragments in the governance directory")
	}
	stats, err := collectStats(temporary)
	if err != nil {
		return countSnapshot{}, fmt.Errorf("cannot read HEAD governance: %w", err)
	}
	return snapshotCounts(temporary, stats)
}
