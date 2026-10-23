package semantic

import (
	"strings"
	"testing"
)

const registerValueSource = `
module main

enum Mode bit[2] {
    Off = 0,
    On = 1,
}

type Inner register[4] {
    Low: bit[2],
    High: bit[2],
}

type Header register[32] msb-first {
    Flag: bit,
    Small: bit[2],
    Wide: bit[12] shadow,
    Mode: Mode,
    Nested: Inner,
    _: bit[11],
}

type Packet struct {
    Header: Header,
    Length: uint16,
}
`

// MD-077 §§ 5, 6, 9–10 and § 16: detached register field reads and writes
// are the pure register.value.extract / register.value.update operations
// over the module's register-definition table, carrying the field ordinal
// and the Sema write proof; no hardware or Place operation appears.
func TestRegisterValueOperations(t *testing.T) {
	source := registerValueSource + `
fn Read(h: Header) uint16 {
    let f: bool := h.Flag
    let s: uint8 := h.Small
    let m: Mode := h.Mode
    let low: uint8 := h.Nested.Low
    discard f
    discard s
    discard m
    discard low
    return h.Wide
}

fn Write(h: Header, other: Header, narrow: uint8) Header {
    let mut b := h
    b.Flag = 1
    b.Small = 3
    b.Wide = other.Wide
    b.Mode = Mode.On
    b.Nested.High = other.Small
    if narrow < 16 {
        b.Wide = narrow
    }
    b.Small = bit[2](narrow)
    return b
}

fn InStruct(p: Packet) Packet {
    let mut q := p
    q.Header.Flag = true
    return q
}

fn main() int {
    return 0
}
`
	module, err := buildPackage15Source(t, source, "")
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		`register "main::Header" width=32`,
		`#0 Flag:!`, `bits=[31+1] access=read-write`,
		`#5 _:!`, `reserved`,
		"register.value.extract", "access=read-write shadow",
		"proof=constant[1..1]", "proof=constant[3..3]", "proof=typed",
		"proof=interval[0..15]", "proof=interval[0..3]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"register.hw", "place.read", "place.write"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("detached register code must not contain %q:\n%s", forbidden, text)
		}
	}
	write := text[strings.Index(text, `func "main::Write(`):]
	write = write[:strings.Index(write, "\n  }")]
	// Nested write: extract Nested, update High, update Nested on Header.
	if got := strings.Count(write, "register.value.update"); got != 8 {
		t.Fatalf("expected 8 register.value.update operations in Write, got %d:\n%s", got, write)
	}
	if !strings.Contains(write, "numeric.convert-exact") {
		t.Fatalf("narrow uint8 into bit[12] must convert exactly to the uint16 carrier:\n%s", write)
	}
	inStruct := text[strings.Index(text, `func "main::InStruct(`):]
	inStruct = inStruct[:strings.Index(inStruct, "\n  }")]
	for _, want := range []string{"struct.extract-field", "register.value.update", "struct.replace-field"} {
		if !strings.Contains(inStruct, want) {
			t.Fatalf("missing %q in InStruct:\n%s", want, inStruct)
		}
	}
}

// MD-077 § 9(4), § 11(4) and MD-110: operations without an admitted
// Semantic IR representation fail closed with a specific diagnostic.
func TestRegisterValueBoundariesFailClosed(t *testing.T) {
	for name, body := range map[string]string{
		"conversion": `
fn Make(raw: uint32) bool {
    let h := Header(raw)
    return h.Flag
}
`,
		"hardware": `
impl Header {
    property IsSet: bool {
        get {
            return self.Flag
        }
    }
}

@address(0x40000000)
let status: Header

fn Check() bool {
    return status.IsSet
}
`,
		"read": `
impl Header {
    fn Snap() Header {
        return self.Read()
    }
}
`,
	} {
		want := map[string]string{"conversion": "integer-to-register conversion Header(...)", "hardware": "MD-077", "read": "hardware register operation read-register on Header"}
		t.Run(name, func(t *testing.T) {
			_, err := buildPackage15Source(t, registerValueSource+body+"\nfn main() int {\n    return 0\n}\n", "")
			if err == nil {
				t.Fatalf("%s must fail closed", name)
			}
			if !strings.Contains(err.Error(), want[name]) {
				t.Fatalf("%s: got %v, want %q", name, err, want[name])
			}
		})
	}
}

// MD-077 §§ 1, 5: on a software register `self.Field` in impl T is an
// ordinary field, so the member lowers to register.value.* operations.
func TestRegisterImplOnSoftwareReceiver(t *testing.T) {
	module, err := buildPackage15Source(t, registerValueSource+`
impl Header {
    fn IsWide() bool {
        return self.Flag && self.Wide > 100
    }

    fn Reset() void {
        self.Small = 0
        Nested.Low = 1
    }

    fn Implicit() bool {
        return Flag
    }
}

fn Check(h: Header) bool {
    let mut copy := h
    copy.Reset()
    return copy.IsWide()
}

fn main() int {
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for name, want := range map[string][]string{
		"Header.IsWide()":   {"place.read", "register.value.extract %2 field=#0", "field=#2"},
		"Header.Reset()":    {"proof=constant[0..0]", "field=#4", "proof=constant[1..1]", "proof=typed", "place.write"},
		"Header.Implicit()": {"register.value.extract %2 field=#0"},
	} {
		body := text[strings.Index(text, `func "main::`+name+`"`):]
		body = body[:strings.Index(body, "\n  }")]
		for _, operation := range want {
			if !strings.Contains(body, operation) {
				t.Fatalf("%s misses %q:\n%s", name, operation, body)
			}
		}
		if strings.Contains(body, "register.hw") {
			t.Fatalf("%s on a software receiver must not contain hardware operations:\n%s", name, body)
		}
	}
}
