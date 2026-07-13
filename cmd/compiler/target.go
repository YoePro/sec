package main

import (
	"fmt"
	"runtime"
	"strings"
)

type TargetStatus string

const (
	TargetImplemented  TargetStatus = "implemented"
	TargetExperimental TargetStatus = "experimental"
	TargetPlanned      TargetStatus = "planned"
)

type TargetDefinition struct {
	OS          string
	Arch        string
	LLVMTriple  string
	Status      TargetStatus
	CanParse    bool
	CanCheck    bool
	CanEmitLLVM bool
	CanLink     bool
	CanRun      bool
}

var targets = []TargetDefinition{
	{
		OS:          "linux",
		Arch:        "amd64",
		LLVMTriple:  "x86_64-pc-linux-gnu",
		Status:      TargetImplemented,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: true,
		CanLink:     true,
		CanRun:      true,
	},
	{
		OS:          "linux",
		Arch:        "arm64",
		LLVMTriple:  "aarch64-unknown-linux-gnu",
		Status:      TargetExperimental,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: true,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "linux",
		Arch:        "armv6",
		LLVMTriple:  "armv6-unknown-linux-gnueabihf",
		Status:      TargetExperimental,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: true,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "linux",
		Arch:        "armv7",
		LLVMTriple:  "armv7-unknown-linux-gnueabihf",
		Status:      TargetExperimental,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: true,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "macos",
		Arch:        "amd64",
		LLVMTriple:  "x86_64-apple-darwin",
		Status:      TargetExperimental,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: true,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "macos",
		Arch:        "arm64",
		LLVMTriple:  "aarch64-apple-darwin",
		Status:      TargetExperimental,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: true,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "windows",
		Arch:        "amd64",
		LLVMTriple:  "x86_64-pc-windows-msvc",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "windows",
		Arch:        "arm64",
		LLVMTriple:  "aarch64-pc-windows-msvc",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "freebsd",
		Arch:        "amd64",
		LLVMTriple:  "x86_64-unknown-freebsd",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "freebsd",
		Arch:        "arm64",
		LLVMTriple:  "aarch64-unknown-freebsd",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "openbsd",
		Arch:        "amd64",
		LLVMTriple:  "x86_64-unknown-openbsd",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "openbsd",
		Arch:        "arm64",
		LLVMTriple:  "aarch64-unknown-openbsd",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "netbsd",
		Arch:        "amd64",
		LLVMTriple:  "x86_64-unknown-netbsd",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "netbsd",
		Arch:        "arm64",
		LLVMTriple:  "aarch64-unknown-netbsd",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "baremetal",
		Arch:        "cortex-m0",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "baremetal",
		Arch:        "cortex-m3",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "baremetal",
		Arch:        "cortex-m4",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "baremetal",
		Arch:        "cortex-m7",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "baremetal",
		Arch:        "riscv32",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "freertos",
		Arch:        "cortex-m4",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "freertos",
		Arch:        "cortex-m7",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "rtems",
		Arch:        "any",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
	{
		OS:          "zephyr",
		Arch:        "any",
		Status:      TargetPlanned,
		CanParse:    true,
		CanCheck:    true,
		CanEmitLLVM: false,
		CanLink:     false,
		CanRun:      false,
	},
}

type CompilerTarget struct {
	OS   string
	Arch string
}

func (t CompilerTarget) String() string {
	if t.OS == "" || t.Arch == "" {
		return ""
	}
	return t.OS + "-" + t.Arch
}

func hostCompilerTarget() CompilerTarget {
	return CompilerTarget{
		OS:   normalizeTargetOS(runtime.GOOS),
		Arch: normalizeTargetArch(runtime.GOARCH),
	}
}

func normalizeTargetOS(osName string) string {
	switch osName {
	case "darwin":
		return "macos"
	default:
		return osName
	}
}

func normalizeTargetArch(arch string) string {
	switch arch {
	case "arm":
		return "arm32"
	default:
		return arch
	}
}

func parseCompilerTarget(value string) (CompilerTarget, bool) {
	separator := strings.LastIndex(value, "-")
	if separator <= 0 || separator == len(value)-1 {
		return CompilerTarget{}, false
	}
	target := CompilerTarget{
		OS:   normalizeTargetOS(value[:separator]),
		Arch: normalizeTargetArch(value[separator+1:]),
	}
	if target.OS == "" || target.Arch == "" {
		return CompilerTarget{}, false
	}
	return target, true
}

func findTargetDefinition(target CompilerTarget) (TargetDefinition, bool) {
	for _, definition := range targets {
		if definition.OS == target.OS && definition.Arch == target.Arch {
			return definition, true
		}
	}
	return TargetDefinition{}, false
}

func requireTargetCanEmitLLVM(target CompilerTarget) (TargetDefinition, error) {
	definition, ok := findTargetDefinition(target)
	if !ok {
		return TargetDefinition{}, fmt.Errorf("unsupported target %s", target.String())
	}
	if !definition.CanEmitLLVM || definition.LLVMTriple == "" {
		return TargetDefinition{}, fmt.Errorf("target %s cannot emit LLVM yet", target.String())
	}
	return definition, nil
}

func requireTargetCanLink(target CompilerTarget) (TargetDefinition, error) {
	definition, err := requireTargetCanEmitLLVM(target)
	if err != nil {
		return TargetDefinition{}, err
	}
	if !definition.CanLink {
		return TargetDefinition{}, fmt.Errorf("target %s cannot link yet", target.String())
	}
	return definition, nil
}
