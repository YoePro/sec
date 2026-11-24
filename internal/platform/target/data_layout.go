package target

import (
	"fmt"

	"sec/internal/layout"
)

// DataLayoutProvenance names the toolchain that supplied every entry of
// targetDataLayouts. TestTargetDataLayoutsMatchClang compares each entry
// with that toolchain's output for its triple (MD-141 § 4(3)); a different
// toolchain version must re-verify the table rather than inherit it.
const DataLayoutProvenance = "clang 21.1.8 target datalayout (clang -target <triple> -S -emit-llvm)"

// targetDataLayouts is the compiler-maintained target layout description of
// every registered triple: the LLVM data-layout string whose scalar ABI
// alignments become the plan's typed facts. A triple without an entry has
// unresolved scalar alignment, so its layouts stay incomplete.
// Rules: rules/platform/target_profiles.md — § 40; rules/memory/layout.md — § 47.2;
// MD-141 correction §§ 3(4), 4.
var targetDataLayouts = map[string]string{
	"thumbv6m-none-eabi":              "e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"thumbv7m-none-eabi":              "e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"thumbv7em-none-eabi":             "e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"riscv32-unknown-elf":             "e-m:e-p:32:32-i64:64-n32-S128",
	"x86_64-pc-linux-gnu":             "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"aarch64-unknown-linux-gnu":       "e-m:e-p270:32:32-p271:32:32-p272:64:64-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128-Fn32",
	"armv6-unknown-linux-gnueabihf":   "e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"armv7-unknown-linux-gnueabihf":   "e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"riscv64-unknown-linux-gnu":       "e-m:e-p:64:64-i64:64-i128:128-n32:64-S128",
	"x86_64-apple-darwin":             "e-m:o-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"aarch64-apple-darwin":            "e-m:o-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n32:64-S128-Fn32",
	"x86_64-pc-windows-msvc":          "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"aarch64-pc-windows-msvc":         "e-m:w-p270:32:32-p271:32:32-p272:64:64-p:64:64-i32:32-i64:64-i128:128-n32:64-S128-Fn32",
	"x86_64-unknown-freebsd":          "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"aarch64-unknown-freebsd":         "e-m:e-p270:32:32-p271:32:32-p272:64:64-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128-Fn32",
	"armv7-unknown-freebsd-gnueabihf": "e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"x86_64-unknown-openbsd":          "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"aarch64-unknown-openbsd":         "e-m:e-p270:32:32-p271:32:32-p272:64:64-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128-Fn32",
	"x86_64-unknown-netbsd":           "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"aarch64-unknown-netbsd":          "e-m:e-p270:32:32-p271:32:32-p272:64:64-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128-Fn32",
	"wasm32-unknown-unknown":          "e-m:e-p:32:32-p10:8:8-p20:8:8-i64:64-i128:128-n32:64-S128-ni:1:10:20",
	"wasm64-unknown-unknown":          "e-m:e-p:64:64-p10:8:8-p20:8:8-i64:64-i128:128-n32:64-S128-ni:1:10:20",
}

// DataLayout returns the maintained LLVM data-layout string of the
// definition's triple and whether one is recorded.
// Rules: rules/platform/target_profiles.md — § 40(1).
func (definition Definition) DataLayout() (string, bool) {
	dataLayout, ok := targetDataLayouts[definition.LLVMTriple]
	return dataLayout, ok
}

// ScalarAlignments returns the typed scalar alignment facts of the
// definition (MD-141), normalized from its maintained data layout. A triple
// without a recorded layout returns nil facts and no error: its layouts are
// incomplete, never guessed (target_profiles.md § 40(3)).
// Rules: rules/platform/target_profiles.md — § 40; rules/memory/layout.md — § 47.2.
func (definition Definition) ScalarAlignments() (layout.ScalarAlignments, error) {
	dataLayout, ok := definition.DataLayout()
	if !ok {
		return nil, nil
	}
	facts, err := layout.ScalarAlignmentsFromLLVMDataLayout(dataLayout)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", definition.LLVMTriple, err)
	}
	return facts, nil
}

// LayoutResolver returns the canonical layout resolver of the definition's
// plan with its scalar alignment facts (layout.md § 47.5: one authority).
// Rules: rules/memory/layout.md — §§ 47.2, 47.5; rules/platform/target_profiles.md — § 40.
func (definition Definition) LayoutResolver() (*layout.Resolver, error) {
	plan, err := definition.ScalarPlan()
	if err != nil {
		return nil, err
	}
	facts, err := definition.ScalarAlignments()
	if err != nil {
		return nil, err
	}
	return layout.NewResolver(plan, facts)
}
