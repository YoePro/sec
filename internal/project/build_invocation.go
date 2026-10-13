package project

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// BuildInvocation owns one resolved version and optional automatic reservation
// shared by every requested Variant of a logical Target build.
// Rules: rules/projects/projects.md — §§3.8,29(3–10),30,31.
type BuildInvocation struct {
	manifest *Manifest
	target   string
	version  Version
	rendered string
	edit     *BuildNumberEdit
	lock     *flock.Flock
	finished bool
}

// BeginBuildInvocation reads current storage under a project-wide OS lock for
// automatic builds. Conservatively serializing all project counters also keeps
// independent Target edits to the one manifest from overwriting each other.
// Explicit numbers bypass allocation and locks entirely, including at overflow.
// Rules: rules/projects/projects.md — §§27(8),29(1–10),30(2–4).
func BeginBuildInvocation(ctx context.Context, path, target string, explicit *uint64) (*BuildInvocation, error) {
	manifest, err := LoadManifest(path)
	if err != nil {
		return nil, err
	}
	counter, found := manifest.BuildCounter(target)
	if !found {
		return nil, fmt.Errorf("unknown Target %q", target)
	}
	invocation := &BuildInvocation{manifest: manifest, target: target}
	if explicit != nil {
		if *explicit > math.MaxInt64 {
			return nil, fmt.Errorf("build number exceeds 9223372036854775807")
		}
		invocation.version, _ = manifest.TargetVersion(target)
		invocation.version.Build = *explicit
	} else {
		if counter.Enabled {
			state := filepath.Join(filepath.Dir(manifest.Path()), "state")
			if err := os.MkdirAll(state, 0700); err != nil {
				return nil, err
			}
			lock := flock.New(filepath.Join(state, "build-number.lock"), flock.SetPermissions(0600))
			locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
			if err != nil || !locked {
				_ = lock.Close()
				if err == nil {
					err = ctx.Err()
				}
				return nil, fmt.Errorf("cannot acquire build counter lock: %w", err)
			}
			invocation.lock = lock
			// A parallel build or manual edit may have changed the baseline while
			// this invocation waited. Never use the pre-lock candidate.
			manifest, err = LoadManifest(path)
			if err != nil {
				_ = invocation.Close()
				return nil, err
			}
			invocation.manifest = manifest
		}
		invocation.version, invocation.edit, err = manifest.BuildNumberCandidate(target)
		if err != nil {
			_ = invocation.Close()
			return nil, err
		}
	}
	invocation.rendered, err = invocation.version.Render()
	if err != nil {
		_ = invocation.Close()
		return nil, err
	}
	return invocation, nil
}

func (invocation *BuildInvocation) Version() Version    { return invocation.version }
func (invocation *BuildInvocation) Rendered() string    { return invocation.rendered }
func (invocation *BuildInvocation) Manifest() *Manifest { return invocation.manifest }

// ArtifactIdentity includes typed version fields, stable project identity and
// Target/Variant identity, without ambient time or the human display name.
// This identity accompanies retained LLVM artifacts; it does not invent a
// source-language build metadata API or a cache reuse policy.
// Rules: rules/projects/projects.md — §§30(2),33,34(2–5),49(1–3).
func (invocation *BuildInvocation) ArtifactIdentity(variant string) string {
	data, _ := json.Marshal(struct {
		UUID            string
		Target, Variant string
		Version         Version
	}{invocation.manifest.Facts().ProjectUUID.String(), invocation.target, variant, invocation.version})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// Commit is called once only after every requested Variant succeeds. A failed
// or aborted build calls Close without Commit. Concurrent source-snapshot edits
// fail the commit rather than publishing the candidate as committed state.
// Rules: rules/projects/projects.md — §29(6–12),§30(2–4).
func (invocation *BuildInvocation) Commit() error {
	if invocation.finished {
		return fmt.Errorf("build invocation is already finished")
	}
	invocation.finished = true
	if invocation.edit != nil {
		return invocation.edit.Commit()
	}
	return nil
}

// Close releases the OS lock without advancing storage; OS process teardown
// also releases it if an existing compiler error path exits immediately.
// The lock inode remains on disk so waiting processes always lock the same file.
// Rules: rules/projects/projects.md — §29(7–10).
func (invocation *BuildInvocation) Close() error {
	invocation.finished = true
	if invocation.lock == nil {
		return nil
	}
	lock := invocation.lock
	invocation.lock = nil
	return lock.Close()
}
