// Package target owns the compiler's canonical target registry and resolves
// its representation facts into layout plans shared by compiler clients.
//
// Rules:
//   - rules/projects/projects.md — "Compilation plans and target lowering"
//   - rules/platform/target_profiles.md — target and profile identity
//   - rules/memory/layout.md — resolved scalar representation facts
package target

import (
	"fmt"
	"runtime"
	"strings"

	"sec/internal/layout"
)

type Status string

const (
	Implemented  Status = "implemented"
	Experimental Status = "experimental"
	Planned      Status = "planned"
)

type Definition struct {
	OS               string
	Arch             string
	LLVMTriple       string
	ABI              string
	Profile          string
	PointerWidthBits uint16
	Endianness       layout.Endianness
	// CABI is the target's C ABI data model (rules/platform/abi.md § 19).
	CABI        layout.CABIModel
	Status      Status
	CanParse    bool
	CanCheck    bool
	CanEmitLLVM bool
	CanLink     bool
	CanRun      bool
}

// Supported platforms
var definitions = []Definition{
	{
		OS:               "linux",
		Arch:             "amd64",
		LLVMTriple:       "x86_64-pc-linux-gnu",
		ABI:              "gnu",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelSysVX8664,
		Status:           Implemented,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      true,
		CanLink:          true,
		CanRun:           true,
	},
	{
		OS:               "linux",
		Arch:             "arm64",
		LLVMTriple:       "aarch64-unknown-linux-gnu",
		ABI:              "gnu",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCS64,
		Status:           Experimental,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      true,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "linux",
		Arch:             "armv6",
		LLVMTriple:       "armv6-unknown-linux-gnueabihf",
		ABI:              "gnueabihf",
		Profile:          "hosted",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSLinuxHF,
		Status:           Experimental,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      true,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "linux",
		Arch:             "armv7",
		LLVMTriple:       "armv7-unknown-linux-gnueabihf",
		ABI:              "gnueabihf",
		Profile:          "hosted",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSLinuxHF,
		Status:           Experimental,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      true,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "linux",
		Arch:             "riscv64",
		LLVMTriple:       "riscv64-unknown-linux-gnu",
		ABI:              "lp64d",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelRISCVLP64D,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "macos",
		Arch:             "amd64",
		LLVMTriple:       "x86_64-apple-darwin",
		ABI:              "darwin",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelDarwinX8664,
		Status:           Experimental,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      true,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "macos",
		Arch:             "arm64",
		LLVMTriple:       "aarch64-apple-darwin",
		ABI:              "darwin",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelDarwinARM64,
		Status:           Experimental,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      true,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "windows",
		Arch:             "amd64",
		LLVMTriple:       "x86_64-pc-windows-msvc",
		ABI:              "msvc",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelWindowsX64,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "windows",
		Arch:             "arm64",
		LLVMTriple:       "aarch64-pc-windows-msvc",
		ABI:              "msvc",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelWindowsARM64,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "freebsd",
		Arch:             "amd64",
		LLVMTriple:       "x86_64-unknown-freebsd",
		ABI:              "system-v",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelSysVX8664,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "freebsd",
		Arch:             "arm64",
		LLVMTriple:       "aarch64-unknown-freebsd",
		ABI:              "system-v",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCS64,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "freebsd",
		Arch:             "armv7",
		LLVMTriple:       "armv7-unknown-freebsd-gnueabihf",
		ABI:              "gnueabihf",
		Profile:          "hosted",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSFreeBSDHF,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "openbsd",
		Arch:             "amd64",
		LLVMTriple:       "x86_64-unknown-openbsd",
		ABI:              "system-v",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelSysVX8664,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "openbsd",
		Arch:             "arm64",
		LLVMTriple:       "aarch64-unknown-openbsd",
		ABI:              "system-v",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCS64,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "netbsd",
		Arch:             "amd64",
		LLVMTriple:       "x86_64-unknown-netbsd",
		ABI:              "system-v",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelSysVX8664,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "netbsd",
		Arch:             "arm64",
		LLVMTriple:       "aarch64-unknown-netbsd",
		ABI:              "system-v",
		Profile:          "hosted",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCS64,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "baremetal",
		Arch:             "cortex-m0",
		ABI:              "aapcs",
		Profile:          "freestanding",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSBareMetal,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "baremetal",
		Arch:             "cortex-m3",
		ABI:              "aapcs",
		Profile:          "freestanding",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSBareMetal,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "baremetal",
		Arch:             "cortex-m4",
		ABI:              "aapcs",
		Profile:          "freestanding",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSBareMetal,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "baremetal",
		Arch:             "cortex-m7",
		ABI:              "aapcs",
		Profile:          "freestanding",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSBareMetal,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "baremetal",
		Arch:             "riscv32",
		ABI:              "ilp32",
		Profile:          "freestanding",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelRISCVILP32,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "freertos",
		Arch:             "cortex-m4",
		ABI:              "aapcs",
		Profile:          "rtos",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSBareMetal,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "freertos",
		Arch:             "cortex-m7",
		ABI:              "aapcs",
		Profile:          "rtos",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelAAPCSBareMetal,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:          "rtems",
		Arch:        "any",
		Status:      Planned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "zephyr",
		Arch:        "any",
		Status:      Planned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
}

type Target struct {
	OS   string
	Arch string
}

func (t Target) String() string {
	if t.OS == "" || t.Arch == "" {
		return ""
	}
	return t.OS + "-" + t.Arch
}

func Host() Target {
	return Target{
		OS:   NormalizeOS(runtime.GOOS),
		Arch: NormalizeArch(runtime.GOARCH),
	}
}

func NormalizeOS(osName string) string {
	switch osName {
	case "darwin":
		return "macos"
	default:
		return osName
	}
}

func NormalizeArch(arch string) string {
	switch arch {
	case "arm":
		return "arm32"
	default:
		return arch
	}
}

// Parse resolves registered OS/architecture spellings before the generic
// separator fallback, preserving hyphens inside architectures such as cortex-m3.
// Rules: rules/platform/platform_model.md — target registry and target selection;
// rules/memory/allocation.md — §§22,29(1).
func Parse(value string) (Target, bool) {
	for _, definition := range definitions {
		if osName, found := strings.CutSuffix(value, "-"+definition.Arch); found && NormalizeOS(osName) == definition.OS {
			return Target{OS: definition.OS, Arch: definition.Arch}, true
		}
	}
	separator := strings.LastIndex(value, "-")
	if separator <= 0 || separator == len(value)-1 {
		return Target{}, false
	}
	target := Target{
		OS:   NormalizeOS(value[:separator]),
		Arch: NormalizeArch(value[separator+1:]),
	}
	if target.OS == "" || target.Arch == "" {
		return Target{}, false
	}
	return target, true
}

func Find(target Target) (Definition, bool) {
	for _, definition := range definitions {
		if definition.OS == target.OS && definition.Arch == target.Arch {
			return definition, true
		}
	}
	return Definition{}, false
}

func (definition Definition) ScalarPlan() (layout.ResolvedScalarPlan, error) {
	plan := layout.ResolvedScalarPlan{
		TargetOS:         definition.OS,
		TargetArch:       definition.Arch,
		LLVMTriple:       definition.LLVMTriple,
		ABI:              definition.ABI,
		Profile:          definition.Profile,
		PointerWidthBits: definition.PointerWidthBits,
		Endianness:       definition.Endianness,
		CABI:             definition.CABI,
	}
	if err := plan.Validate(); err != nil {
		return layout.ResolvedScalarPlan{}, fmt.Errorf("target %s-%s has no resolved scalar plan: %w", definition.OS, definition.Arch, err)
	}
	return plan, nil
}

// Definitions returns a copy of the compiler-owned target registry.
func Definitions() []Definition {
	return append([]Definition(nil), definitions...)
}
