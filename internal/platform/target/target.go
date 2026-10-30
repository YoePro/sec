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
	CABI layout.CABIModel
	// Atomics is frozen against this definition's resolved scalar/profile plan.
	// An unspecified table leaves concrete capability unknown.
	// Rules: rules/concurrency/atomics.md — §37(1–5).
	Atomics layout.AtomicCapabilities
	// AddressDomains are the hardware address domains this target supplies
	// (MD-111 §§ 1(6)-1(7)); see AddressDomains for the defaults.
	AddressDomains []layout.AddressDomain
	Status         Status
	CanParse       bool
	CanCheck       bool
	CanEmitLLVM    bool
	CanLink        bool
	CanRun         bool
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
	// WebAssembly modules without an operating system interface (the LLVM
	// wasm32/wasm64-unknown-unknown environments); the embedder provides
	// imports. Registered as planned: parse and check only (MD-075).
	{
		OS:               "unknown",
		Arch:             "wasm32",
		LLVMTriple:       "wasm32-unknown-unknown",
		ABI:              "wasm-basic-c",
		Profile:          "freestanding",
		PointerWidthBits: 32,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelWasm32,
		Status:           Planned,
		CanParse:         true,
		CanCheck:         true,
		CanEmitLLVM:      false,
		CanLink:          false,
		CanRun:           false,
	},
	{
		OS:               "unknown",
		Arch:             "wasm64",
		LLVMTriple:       "wasm64-unknown-unknown",
		ABI:              "wasm-basic-c",
		Profile:          "freestanding",
		PointerWidthBits: 64,
		Endianness:       layout.LittleEndian,
		CABI:             layout.CModelWasm64,
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

// ScalarPlan projects canonical representation and frozen atomic capability
// facts without reconstructing support from host or architecture spellings.
// Rules: rules/platform/platform_model.md — §9 "Architecture capability";
// rules/concurrency/atomics.md — §37(1–5).
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
		Atomics:          definition.Atomics,
		AddressDomains:   definition.addressDomains(),
	}
	if err := plan.Validate(); err != nil {
		return layout.ResolvedScalarPlan{}, fmt.Errorf("target %s-%s has no resolved scalar plan: %w", definition.OS, definition.Arch, err)
	}
	return plan, nil
}

// addressDomains returns the target's hardware address domains. A CPU target
// supplies the linear, byte-addressed CPU physical domain `cpu-physical` with
// the CPU's address width; WebAssembly has no physical address domain. Whether
// a binding in a domain is usable on a profile is decided by the region and
// plan checks, never by the domain alone.
// Rules: rules/platform/platform_model.md — § 10 (Amendment MD-111).
func (definition Definition) addressDomains() []layout.AddressDomain {
	if definition.AddressDomains != nil {
		return append([]layout.AddressDomain(nil), definition.AddressDomains...)
	}
	if strings.HasPrefix(definition.Arch, "wasm") || definition.PointerWidthBits == 0 {
		return nil
	}
	return []layout.AddressDomain{{Identity: "cpu-physical", AddressBits: definition.PointerWidthBits, UnitBits: 8, Linear: true}}
}

// Definitions returns a copy of the compiler-owned target registry.
func Definitions() []Definition {
	return append([]Definition(nil), definitions...)
}
