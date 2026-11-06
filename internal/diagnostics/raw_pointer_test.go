package diagnostics

import "testing"

// Raw-pointer authorization and contradicted validity stay distinct mandatory
// errors, with stable registry identities for both editor and compiler output.
// Rules: rules/memory/raw_pointers.md §34(1–4).
func TestRawPointerMentorDefinitions(t *testing.T) {
	for id, name := range map[string]string{
		RawPointerUnsafeRequired:   "unsafe.raw-pointer-context",
		RawPointerInvalidOperation: "unsafe.raw-pointer-operation",
		RawPointerDanglingOrigin:   "unsafe.raw-pointer-dangling",
		RawPointerNullDereference:  "unsafe.raw-pointer-null",
	} {
		d, ok := Lookup(id)
		if !ok || d.Name != name || d.Family != "unsafe" || !d.Mandatory || d.DefaultSeverity != SeverityError {
			t.Fatal(d)
		}
	}
}
