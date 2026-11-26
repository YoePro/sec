package configedit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sec/internal/project"
)

// OpenProjectRoot retains an OS-confined directory handle for the explicitly
// named project. It never discovers parents or follows escaping symlinks.
// Rules: rules/projects/projects.md §5; harness design notes §§5(5)-(7).
func OpenProjectRoot(name string) (*os.Root, string, error) {
	if name == "" {
		return nil, "", fmt.Errorf("an explicit project root is required")
	}
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		return nil, "", fmt.Errorf("workspace root containment is unavailable on this host")
	}
	identity, err := filepath.Abs(name)
	if err != nil {
		return nil, "", err
	}
	identity, err = filepath.EvalSymlinks(identity)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(identity)
	if err != nil {
		return nil, "", err
	}
	data, err := root.ReadFile(filepath.FromSlash(".sec/sec.toml"))
	if err != nil {
		root.Close()
		return nil, "", fmt.Errorf("selected directory is not a Sec project root: %w", err)
	}
	if _, diagnostics := project.ParseManifest(filepath.Join(identity, ".sec", "sec.toml"), string(data)); len(diagnostics) > 0 {
		root.Close()
		return nil, "", fmt.Errorf("invalid selected project manifest: %v", diagnostics)
	}
	return root, identity, nil
}
