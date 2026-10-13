package project

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func invocationFixture(t *testing.T, target string) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".sec", "sec.toml")
	if err := os.Mkdir(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	text := manifestFixture(t) + "\n[target." + target + ".build_counter]\nenabled=true\n"
	if target == "library" {
		text += "scope='project'\n"
	}
	if err := os.WriteFile(path, []byte(text), 0640); err != nil {
		t.Fatal(err)
	}
	return path, text
}

// Rules: rules/projects/projects.md — §29(3–12).
func TestBuildInvocationCommitAbortAndConflict(t *testing.T) {
	for _, target := range []string{"sec", "library"} {
		t.Run(target, func(t *testing.T) {
			path, original := invocationFixture(t, target)
			begin := func() *BuildInvocation {
				t.Helper()
				build, err := BeginBuildInvocation(context.Background(), path, target, nil)
				if err != nil {
					t.Fatal(err)
				}
				return build
			}
			read := func() string {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			build := begin()
			candidate := build.Version().Build
			for _, variant := range []string{"first", "second"} {
				if build.Version().Build != candidate || build.ArtifactIdentity(variant) == "" {
					t.Fatal("different candidate")
				}
				if read() != original {
					t.Fatal("premature commit")
				}
			}
			_ = build.Close() // simulated failed Variant, candidate not consumed
			if read() != original || build.Commit() == nil {
				t.Fatal("aborted invocation committed")
			}
			build = begin()
			if build.Version().Build != candidate {
				t.Fatal("failure consumed candidate")
			}
			if err := build.Commit(); err != nil {
				t.Fatal(err)
			}
			if err := build.Commit(); err == nil {
				t.Fatal("double commit")
			}
			_ = build.Close()
			manifest, err := LoadManifest(path)
			if err != nil {
				t.Fatal(err)
			}
			counter, _ := manifest.BuildCounter(target)
			if counter.StoredBuild != candidate {
				t.Fatal(counter)
			}
			build = begin()
			changed := read() + "\n# external editor\n"
			if err := os.WriteFile(path, []byte(changed), 0640); err != nil {
				t.Fatal(err)
			}
			if err := build.Commit(); err == nil || read() != changed {
				t.Fatal("overwrote external edit", err)
			}
			_ = build.Close()
		})
	}
}

// Rules: rules/projects/projects.md — §§29(8–10),30(2–5),33,34(2–5).
func TestExplicitBuildNumberBypassesAllocation(t *testing.T) {
	path, text := invocationFixture(t, "sec")
	text = strings.Replace(text, "build = 421", fmt.Sprintf("build = %d", int64(math.MaxInt64)), 1)
	if err := os.WriteFile(path, []byte(text), 0640); err != nil {
		t.Fatal(err)
	}
	for _, number := range []uint64{0, 19, math.MaxInt64} {
		build, err := BeginBuildInvocation(context.Background(), path, "sec", &number)
		if err != nil || build.Version().Build != number || build.edit != nil || build.lock != nil {
			t.Fatal(build, err)
		}
		identity := build.ArtifactIdentity("native")
		repeated, err := BeginBuildInvocation(context.Background(), path, "sec", &number)
		if err != nil || repeated.ArtifactIdentity("native") != identity {
			t.Fatal("non-reproducible explicit identity", err)
		}
		if repeated.ArtifactIdentity("another") == identity {
			t.Fatal("Variant identity omitted")
		}
		_ = repeated.Close()
		if err := build.Commit(); err != nil {
			t.Fatal(err)
		}
		_ = build.Close()
		data, _ := os.ReadFile(path)
		if string(data) != text {
			t.Fatal("explicit number mutated storage")
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "state")); !os.IsNotExist(err) {
		t.Fatal("explicit build created lock/counter metadata", err)
	}
	tooLarge := uint64(math.MaxInt64) + 1
	if _, err := BeginBuildInvocation(context.Background(), path, "sec", &tooLarge); err == nil {
		t.Fatal("overflow accepted")
	}
}

// Rules: rules/projects/projects.md — §29(8–10),§30(3–4).
func TestBuildInvocationParallelSerialization(t *testing.T) {
	path, _ := invocationFixture(t, "sec")
	first, err := BeginBuildInvocation(context.Background(), path, "sec", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if build, err := BeginBuildInvocation(ctx, path, "sec", nil); err == nil || build != nil {
		t.Fatal("parallel automatic build ignored lock")
	}
	// Explicit invocation does not wait for the active automatic reservation.
	number := uint64(9)
	explicit, err := BeginBuildInvocation(ctx, path, "sec", &number)
	if err != nil {
		t.Fatal(err)
	}
	_ = explicit.Close()
	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}
	_ = first.Close()
	const workers = 6
	var group sync.WaitGroup
	results := make(chan uint64, workers)
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			build, err := BeginBuildInvocation(context.Background(), path, "sec", nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer build.Close()
			value := build.Version().Build
			if err := build.Commit(); err != nil {
				t.Error(err)
				return
			}
			results <- value
		}()
	}
	group.Wait()
	close(results)
	seen := map[uint64]bool{}
	for number := range results {
		if seen[number] {
			t.Fatal("duplicate committed candidate", number)
		}
		seen[number] = true
	}
	if len(seen) != workers {
		t.Fatal(seen)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	counter, _ := manifest.BuildCounter("sec")
	if counter.StoredBuild != 422+workers {
		t.Fatal(counter)
	}
}
