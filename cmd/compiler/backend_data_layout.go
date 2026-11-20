package main

import (
	"fmt"

	mlirtoolchain "sec/internal/mlir"
	"sec/internal/platform/target"
)

// verifyBackendDataLayout compares the data layout the native-track code
// generator applies for the target triple with the maintained layout the
// target's scalar alignment facts come from. A missing maintained layout or
// any difference fails closed.
// Rules: rules/platform/target_profiles.md — § 40(2)-(3); rules/memory/layout.md — §§ 47.2, 47.5;
// MD-141 correction § 4(3)-(4).
func verifyBackendDataLayout(toolchain mlirtoolchain.Toolchain, definition target.Definition) error {
	maintained, ok := definition.DataLayout()
	if !ok {
		return fmt.Errorf("target %s has no verified data layout, so its layouts cannot be materialized (MD-141)", definition.LLVMTriple)
	}
	applied, err := toolchain.TargetDataLayout(definition.LLVMTriple)
	if err != nil {
		return err
	}
	if applied != maintained {
		return fmt.Errorf("the code generator applies data layout %q for %s, but the target's layout facts come from %q (%s); refusing to emit objects with a different physical layout (MD-141)",
			applied, definition.LLVMTriple, maintained, target.DataLayoutProvenance)
	}
	return nil
}
