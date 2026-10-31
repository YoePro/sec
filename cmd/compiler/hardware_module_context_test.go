package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHardwareProjectFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

const hardwareDriverSource = `module gpio
hardware region PeripheralSpace {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: true,
}
hardware resource OutputState { width: 32, }
`

const hardwareEndpointSource = `module consumer
import board "drivers/gpio"
hardware endpoint Output {
    kind: HardwareEndpointKind.MemoryMapped,
    region: board.PeripheralSpace,
    resource: board.%s,
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

// TestCLIHardwareModuleContext checks that `sec check` supplies resolver-owned
// module identities: a Project source references hardware declared in an
// imported Project module through its actual import qualifier, a source
// outside every Project uses the anonymous root, and an unknown imported
// declaration is still rejected rather than guessed.
// Rules: MD-111 §§2,8(3),12; rules/projects/modules.md — §§2.2,9;
// rules/projects/projects.md — §13(9).
func TestCLIHardwareModuleContext(t *testing.T) {
	root := t.TempDir()
	if _, stderr, code := runCLIForDiagnostics(t, "init", root, "--minimal", "--name", "Board"); code != 0 {
		t.Fatal(code, stderr)
	}
	writeHardwareProjectFile(t, filepath.Join(root, "drivers", "gpio", "gpio.sec"), hardwareDriverSource)
	consumer := filepath.Join(root, "app", "consumer", "consumer.sec")
	writeHardwareProjectFile(t, consumer, strings.Replace(hardwareEndpointSource, "%s", "OutputState", 1))
	if _, stderr, code := runCLIForDiagnostics(t, "check", consumer); code != 0 || strings.Contains(stderr, "S1187") {
		t.Fatalf("cross-module hardware check: code=%d\n%s", code, stderr)
	}
	writeHardwareProjectFile(t, consumer, strings.Replace(hardwareEndpointSource, "%s", "Missing", 1))
	if _, stderr, code := runCLIForDiagnostics(t, "check", consumer); code == 0 || strings.Contains(stderr, "S1187") || !strings.Contains(stderr, "S1183") {
		t.Fatalf("unknown imported resource: code=%d\n%s", code, stderr)
	}

	loose := filepath.Join(t.TempDir(), "board.sec")
	writeHardwareProjectFile(t, loose, hardwareDriverSource)
	if _, stderr, code := runCLIForDiagnostics(t, "check", loose); code != 0 || strings.Contains(stderr, "S1187") {
		t.Fatalf("project-less hardware check: code=%d\n%s", code, stderr)
	}
}
