package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLSPHardwareModuleContextMatchesCLI checks that the editor supplies the
// same resolver-owned module identities as `sec check`: a Project source that
// references hardware declared in an imported Project module has no S1187,
// and an unknown imported resource is still rejected.
// Rules: MD-111 §§2,12,13(3); rules/projects/modules.md — §§2.2,23.
func TestLSPHardwareModuleContextMatchesCLI(t *testing.T) {
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, ".sec", "sec.toml"), `[project]
name = "board"
uuid = "72e6e978-b92e-42e0-a25e-39e07fca9177"
imports = []

[version]
major = 0
minor = 1
revision = 0
build = 0
format = "[major].[minor].[revision].[build]"

[build]
backend = "mlir"
profile = "server"
diagnostics = "default"

[profile.server]
optimization = "none"
debug = "line"
strip = false

[variant.linux-amd64]
os = "linux"
arch = "amd64"
llvm_triple = "x86_64-pc-linux-gnu"

[target.board]
kind = "command"
source = "app/consumer"
artifact = "board"
variants = ["linux-amd64"]
output = "bin/board/linux-amd64/server/board"
`)
	write(filepath.Join(root, "drivers", "gpio", "gpio.sec"), `module gpio
hardware region PeripheralSpace {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: true,
}
hardware resource OutputState { width: 32, }
`)
	consumer := filepath.Join(root, "app", "consumer", "consumer.sec")
	source := func(resource string) string {
		return `module consumer
import board "drivers/gpio"
hardware endpoint Output {
    kind: HardwareEndpointKind.MemoryMapped,
    region: board.PeripheralSpace,
    resource: board.` + resource + `,
    offset: 0,
    size: 4,
    alignment: 4,
    read_widths: [],
    write_widths: [32],
    write_footprint: HardwareFootprintKind.Whole,
    write_operation: HardwareWriteOperation.Replace,
    ordering: HardwareOrdering.Volatile,
    completion: HardwareCompletion.Issued,
    atomicity: HardwareAtomicity.Unspecified,
    context: HardwareAccessContext.Current,
    fault: HardwareFaultContract.Platform,
}
`
	}
	codes := func(text string) []string {
		write(consumer, text)
		var found []string
		for _, item := range analyze(uriFromPath(consumer), text) {
			if item.Severity == 1 {
				found = append(found, item.Code+" "+item.Message)
			}
		}
		return found
	}
	if errors := codes(source("OutputState")); len(errors) != 0 {
		t.Fatalf("cross-module hardware endpoint: %v", errors)
	}
	errors := codes(source("Missing"))
	if joined := strings.Join(errors, "\n"); !strings.Contains(joined, "S1183") || strings.Contains(joined, "S1187") {
		t.Fatalf("unknown imported resource: %v", errors)
	}
}
