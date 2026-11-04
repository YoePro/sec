package semantic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

// SerializationVersion versions the JSON transport independently of the
// Semantic IR operation schema (Version) and Sec MLIR's dialect schema.
// This internal format has no long-term external compatibility guarantee.
// Rules: rules/compiler/semantic_ir.md — §106(1–5).
const SerializationVersion uint32 = 1

const serializationFormat = "sec.semantic-ir"

type serializedEnvelope struct {
	Format  string          `json:"format"`
	Version uint32          `json:"version"`
	Module  json.RawMessage `json:"module"`
}

// The alias includes every module fact, including future exported fields;
// only the private TypeTable representation needs a transport projection.
type moduleFields Module
type serializedModule struct {
	*moduleFields
	Types []Type
}

// Serialize verifies and encodes a complete module deterministically. Type IDs
// are the one-based positions in Types; no identity, target fact, exact integer,
// source location or operation metadata is reconstructed from printed IR.
// Rules: rules/compiler/semantic_ir.md — §§97(1–2),106(1–5),107.
func Serialize(module *Module) ([]byte, error) {
	if err := Verify(module); err != nil {
		return nil, fmt.Errorf("serialize semantic IR: %w", err)
	}
	payload, err := json.Marshal(serializedModule{
		moduleFields: (*moduleFields)(module),
		Types:        append([]Type{}, module.Types.All()...),
	})
	if err != nil {
		return nil, fmt.Errorf("serialize semantic IR module: %w", err)
	}
	data, err := json.MarshalIndent(serializedEnvelope{serializationFormat, SerializationVersion, payload}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize semantic IR envelope: %w", err)
	}
	return append(data, '\n'), nil
}

// Deserialize admits only this transport and operation schema, reconstructs the
// type interning index without renumbering IDs, and verifies the complete module
// before publishing it. Any failure returns no module. This is not cache-key or
// source/CompilationPlan freshness validation; callers still own that policy.
// Rules: rules/compiler/semantic_ir.md — §§97(1–2),106(1–5).
func Deserialize(data []byte) (*Module, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("deserialize semantic IR: invalid UTF-8")
	}
	if err := validateSerializedJSON(data); err != nil {
		return nil, fmt.Errorf("deserialize semantic IR: %w", err)
	}
	var envelope serializedEnvelope
	if err := decodeSerializedJSON(data, &envelope); err != nil {
		return nil, fmt.Errorf("deserialize semantic IR envelope: %w", err)
	}
	if envelope.Format != serializationFormat || envelope.Version != SerializationVersion {
		return nil, fmt.Errorf("unsupported semantic IR serialization %q version %d", envelope.Format, envelope.Version)
	}
	payload := serializedModule{moduleFields: &moduleFields{}}
	if err := decodeSerializedJSON(envelope.Module, &payload); err != nil {
		return nil, fmt.Errorf("deserialize semantic IR module: %w", err)
	}
	if payload.moduleFields == nil || payload.Types == nil {
		return nil, fmt.Errorf("deserialize semantic IR: missing module or type table")
	}
	module := (*Module)(payload.moduleFields)
	if module.Version != Version {
		return nil, fmt.Errorf("unsupported semantic IR version %d", module.Version)
	}
	module.Types = NewTypeTable()
	for index, typ := range payload.Types {
		if id := module.Types.Intern(typ); id != TypeID(index+1) {
			return nil, fmt.Errorf("deserialize semantic IR: duplicate type at ID %d", index+1)
		}
	}
	if err := Verify(module); err != nil {
		return nil, fmt.Errorf("deserialize semantic IR verification: %w", err)
	}
	return module, nil
}

// decodeSerializedJSON rejects unknown fields and extra top-level values rather
// than silently discarding metadata belonging to a different internal schema.
// Rules: rules/compiler/semantic_ir.md — §106(2–3).
func decodeSerializedJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return nil
}

// validateSerializedJSON rejects ambiguous repeated object keys, including
// case variants accepted by encoding/json, before decoding semantic facts.
// UseNumber keeps wide integer constants exact during this structural pass.
// Rules: rules/compiler/semantic_ir.md — §§97,106(2–3),107.
func validateSerializedJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value func() error
	value = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token {
		case json.Delim('{'):
			seen := map[string]bool{}
			for decoder.More() {
				token, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if !ok {
					return fmt.Errorf("expected object key")
				}
				folded := foldedSerializationField(key)
				if seen[folded] {
					return fmt.Errorf("duplicate JSON field %q", key)
				}
				seen[folded] = true
				if err := value(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
		case json.Delim('['):
			for decoder.More() {
				if err := value(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
		}
		return err
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return nil
}

// foldedSerializationField uses the same Unicode simple-fold equivalence as
// encoding/json's field matching, so spelling variants cannot bypass duplicate
// metadata rejection. String values are never folded or otherwise normalized.
// Rules: rules/compiler/semantic_ir.md — §106(2–3).
func foldedSerializationField(key string) string {
	runes := []rune(key)
	for index, r := range runes {
		minimum := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < minimum {
				minimum = next
			}
		}
		runes[index] = minimum
	}
	return string(runes)
}
