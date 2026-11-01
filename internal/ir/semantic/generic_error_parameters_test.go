package semantic

import (
	"strings"
	"testing"
)

// MD-120 §§ 10(6), 11(5): each error argument selects its own specialization
// with the exact channel, and an enum first reached through a generic argument
// keeps its case values.
func TestGenericErrorChannelSpecializations(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
enum IOError error { Closed, Busy }
enum NetError error { Down }
enum Color { Red, Blue }
fn Id[T](x: T) T {
    return <-x
}
fn Forward[E: error](r: Result[int, E]) Result[int, E] {
    let v := try r
    return Ok(v + 1)
}
fn SourceA() Result[int, IOError] {
    return Err(IOError.Busy)
}
fn SourceB() Result[int, NetError] {
    return Ok(1)
}
fn main() int {
    let a := Forward(SourceA())
    let b := Forward(SourceB())
    let c := Id(Color.Red)
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"Forward[E=main::IOError]", "Forward[E=main::NetError]", "Id[T=main::Color]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

// A generic error union instance is an ordinary concrete closed union error.
func TestGenericErrorUnionInstance(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
enum IOError error { Closed }
type W[E: error] union error {
    Format(int)
    Sink(E)
}
fn Make(e: IOError) Result[int, W[IOError]] {
    return Err(W[IOError].Sink(e))
}
fn main() int {
    let r := Make(IOError.Closed)
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}

// A bit-backed enum first reached through a generic argument keeps its
// representation; the same snapshot must stay independent of the target width.
func TestBitBackedEnumThroughGenericArgument(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
enum Mode bit[2] { Off, On }
fn Id[T](x: T) T {
    return <-x
}
fn main() int {
    let m := Id(Mode.On)
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}
