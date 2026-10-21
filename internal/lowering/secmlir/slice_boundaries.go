package secmlir

import (
	"fmt"

	"sec/internal/ir/semantic"
)

// containsSlice follows abstract type/declaration identity to retain the P16
// foreign/physical-layout exclusions, including nested aggregate carriers.
// Rules: rules/mlir/sec_mlir_lowering.md — v12.1,25,29;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§70–75,121–124.
func (e *emitter) containsSlice(id semantic.TypeID, active map[semantic.TypeID]bool) bool {
	if id == 0 || active[id] {
		return false
	}
	active[id] = true
	defer delete(active, id)
	typ, _ := e.module.Types.Lookup(id)
	if typ.Kind == semantic.TypeSharedSlice || typ.Kind == semantic.TypeMutableSlice {
		return true
	}
	children := []semantic.TypeID{typ.Base, typ.Element, typ.Success, typ.Error}
	children = append(children, typ.TypeArgs...)
	if d, ok := e.structDefinition(id); ok {
		for _, f := range d.Fields {
			children = append(children, f.Type)
		}
	}
	if d, ok := e.unionDefinition(id); ok {
		for _, v := range d.Variants {
			children = append(children, v.Payload)
			for _, f := range v.PayloadFields {
				children = append(children, f.Type)
			}
		}
	}
	for _, child := range children {
		if e.containsSlice(child, active) {
			return true
		}
	}
	return false
}

// validateSliceBoundaries rejects ABI/layout requests and mismatched target
// plans before emitting any bytes. Explicit schema selection is no ABI opt-in.
// Rules: rules/mlir/sec_mlir_lowering.md — v12.29–31;
// rules/compiler/semantic_ir.md — §§6(6),107,124.
func (e *emitter) validateSliceBoundaries() error {
	contains := func(id semantic.TypeID) bool { return e.containsSlice(id, map[semantic.TypeID]bool{}) }
	for _, f := range e.module.Functions {
		if !f.Extern {
			continue
		}
		if contains(f.ReturnType) {
			return &UnsupportedLoweringError{Feature: "foreign slice ABI return", Function: f.ID}
		}
		for _, p := range f.Parameters {
			if contains(p.Value.Type) {
				return &UnsupportedLoweringError{Feature: "foreign slice ABI parameter", Function: f.ID}
			}
		}
	}
	for _, d := range e.module.Structs {
		if d.LayoutRef != "" && contains(d.TypeID) {
			return &UnsupportedLoweringError{Feature: "physical slice layout request " + d.LayoutRef}
		}
	}
	for _, d := range e.module.Unions {
		if d.LayoutRef != "" && contains(d.TypeID) {
			return &UnsupportedLoweringError{Feature: "physical slice layout request " + d.LayoutRef}
		}
	}
	if e.module.PointerWidthBits != 0 && e.module.PointerWidthBits != e.plan.PointerWidthBits {
		for i := range e.module.Types.All() {
			if contains(semantic.TypeID(i + 1)) {
				return fmt.Errorf("slice module scalar plan %d differs from emission plan %d", e.module.PointerWidthBits, e.plan.PointerWidthBits)
			}
		}
	}
	return nil
}
