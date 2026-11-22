package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const buildInfoManifest = `[project]
name='Worker Project'
uuid='550e8400-e29b-41d4-a716-446655440000'
[version]
major=1
minor=2
revision=3
build=40
format='[major].[minor].[revision].[build]'
[build]
profile='debug'
[target.worker]
kind='command'
source='cmd/worker'
variants=['arm']
[target.worker.version]
build=10
[variant.arm]
os='linux'
arch='arm64'
`

func writeBuildInfoProject(t *testing.T, manifest string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{".sec/sec.toml": manifest, "cmd/worker/main.sec": "module main\n"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "cmd", "worker", "main.sec")
}

// MD-143 § 113B(1)-(4): the snapshot names the owning Target and its unique
// Variant, the Target's stored version, the selected (cross) target's OS and
// architecture and both profiles; an unresolvable Variant or an absent
// [build].profile yields no snapshot instead of invented defaults.
func TestCLIBuildInfoSnapshot(t *testing.T) {
	arm, _ := parseCompilerTarget("linux-arm64")
	source := writeBuildInfoProject(t, buildInfoManifest)
	snapshot := cliBuildInfo([]string{source}, arm)
	if snapshot == nil {
		t.Fatal("no snapshot")
	}
	if snapshot.ProjectName != "Worker Project" || snapshot.ProjectUUID != "550e8400-e29b-41d4-a716-446655440000" ||
		snapshot.TargetName != "worker" || snapshot.VariantName != "arm" || snapshot.VersionBuild != 10 ||
		snapshot.VersionRendered != "1.2.3.10" || snapshot.OperatingSystem != "linux" || snapshot.Architecture != "arm64" ||
		snapshot.BuildProfile != "debug" || snapshot.TargetProfile != "hosted" || snapshot.VersionUserDefined != nil {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	amd, _ := parseCompilerTarget("linux-amd64")
	if snapshot := cliBuildInfo([]string{source}, amd); snapshot != nil {
		t.Fatalf("variant-less platform produced %+v", snapshot)
	}
	noProfile := writeBuildInfoProject(t, strings.Replace(buildInfoManifest, "[build]\nprofile='debug'\n", "", 1))
	if snapshot := cliBuildInfo([]string{noProfile}, arm); snapshot != nil {
		t.Fatalf("profile-less manifest produced %+v", snapshot)
	}
}
