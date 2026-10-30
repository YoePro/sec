package formatter

import "testing"

// MD-111: hardware fact declarations format canonically, one member per line
// with a trailing comma and exact value spelling, idempotently and at every
// indentation width; a declaration containing a comment keeps its comment.
// Rules: rules/foundations/grammar.md — HardwareDeclaration (Amendment MD-111).
func TestFormatHardwareDeclarations(t *testing.T) {
	input := `module main

hardware   region   Space{
  domain:"cpu-physical",
      kind :HardwareRegionKind.MMIO,
  base:0x40000000,size: 0x10000,
  readable : true,
  writable: true
}

hardware endpoint Out {
    read_widths: [ 32 , 16 ],
    write_widths:[],
}
`
	want := `module main

hardware region Space {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: true,
}

hardware endpoint Out {
    read_widths: [32, 16],
    write_widths: [],
}
`
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("formatted:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("not idempotent:\n%s", again)
	}
	two := Format(Source{Text: input}, Options{IndentationWidth: 2}).Text
	if two == got || Format(Source{Text: two}, Options{IndentationWidth: 2}).Text != two {
		t.Fatalf("indentation width 2:\n%s", two)
	}
	commented := "module main\n\nhardware resource State {\n    // modeled logical state\n    width: 32,\n}\n"
	if got := Format(Source{Text: commented}, Options{}).Text; got != commented {
		t.Fatalf("comment lost:\n%s", got)
	}
}
