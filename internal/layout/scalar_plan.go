// Package layout contains target-resolved representation facts shared by
// frontend-neutral IR emission and later compiler stages.
package layout

import "fmt"

type Endianness string

const (
	LittleEndian Endianness = "little"
	BigEndian    Endianness = "big"
)

// ResolvedScalarPlan is the authoritative scalar subset of a CompilationPlan.
// Consumers must not derive these facts again from architecture spellings.
type ResolvedScalarPlan struct {
	TargetOS         string
	TargetArch       string
	LLVMTriple       string
	ABI              string
	Profile          string
	PointerWidthBits uint16
	Endianness       Endianness
}

func (p ResolvedScalarPlan) Validate() error {
	if p.TargetOS == "" || p.TargetArch == "" {
		return fmt.Errorf("scalar plan requires target OS and architecture")
	}
	if p.PointerWidthBits != 32 && p.PointerWidthBits != 64 {
		return fmt.Errorf("scalar plan pointer width must be 32 or 64, got %d", p.PointerWidthBits)
	}
	if p.Endianness != LittleEndian && p.Endianness != BigEndian {
		return fmt.Errorf("scalar plan endianness must be little or big, got %q", p.Endianness)
	}
	return nil
}
