package secmlir

import (
	"fmt"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
)

// EmitOptions selects a high-level schema within the single compilation
// track. Zero retains schema 11 until P16 source production is usable.
// Explicit schema 12 admits represented slice operations, never physical layout.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §87;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — §1.
type EmitOptions struct {
	DialectVersion uint32
}

// emissionSchema admits the current emitter and explicit P16 schema only.
// Older schemas remain readable regression inputs, not output downgrade modes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §87;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — §1.
// rules/mlir/packages/sec-mlir-dialect_package17.md — §86 (unimplemented schema 13).
func emissionSchema(version uint32) (uint32, error) {
	if version == 0 {
		return dialectSchemaVersion, nil
	}
	if version == 13 {
		return 0, &UnsupportedLoweringError{Feature: "P17 ownership/cleanup emission (schema 13)", SchemaVersion: version}
	}
	if version != 11 && version != 12 {
		return 0, fmt.Errorf("unsupported Sec MLIR emission schema version %d; supported output schemas are 11 and 12", version)
	}
	return version, nil
}

// EmitWithOptions verifies the same Semantic IR and uses the same emitter as
// Emit, with explicit high-level schema selection. Unsupported constructs
// still return no output; schema selection never chooses another backend.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §87;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — §1;
// rules/compiler/compiler_pipeline.md — §§32–34.
func EmitWithOptions(module *semantic.Module, plan layout.ResolvedScalarPlan, options EmitOptions) ([]byte, error) {
	schema, err := emissionSchema(options.DialectVersion)
	if err != nil {
		return nil, err
	}
	if err := semantic.Verify(module); err != nil {
		return nil, fmt.Errorf("verify Semantic IR: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("validate scalar plan: %w", err)
	}
	if module.Version != semantic.Version {
		return nil, fmt.Errorf("Semantic IR version %d is not supported", module.Version)
	}
	e := &emitter{
		schemaVersion: schema, module: module, types: map[semantic.TypeID]string{},
		visiting: map[semantic.TypeID]bool{}, functions: map[semantic.FunctionID]string{}, plan: plan,
	}
	if err := e.validatePackage14ArrayBoundaries(); err != nil {
		return nil, err
	}
	if err := validateArrayOwnershipSchema(module); err != nil {
		return nil, err
	}
	if err := e.validateSliceBoundaries(); err != nil {
		return nil, err
	}
	if err := validateTryHandlerSchema(module); err != nil {
		return nil, err
	}
	for index, function := range module.Functions {
		e.functions[function.ID] = fmt.Sprintf("sec_fn_%d", index)
	}
	if err := e.emitModule(); err != nil {
		return nil, err
	}
	return []byte(e.out.String()), nil
}
