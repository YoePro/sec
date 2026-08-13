package formatter

import (
	"strings"
	"testing"
)

func TestFormatPreservesDefaultClauseAndPartialStructLiteral(t *testing.T) {
	input := "module main\n\n" +
		"type User string in [\"Admin\", \"User\"] default \"User\"\n\n" +
		"fn main() int {\n" +
		"let mut user: User\n" +
		"let position := Position { line: 10 }\n" +
		"return 0\n" +
		"}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if !strings.Contains(got, `type User string in ["Admin", "User"] default "User"`) {
		t.Fatalf("formatter changed explicit default or membership order:\n%s", got)
	}
	if !strings.Contains(got, "Position { line: 10 }") {
		t.Fatalf("formatter expanded partial struct literal:\n%s", got)
	}
}

func TestFormatPlacesNoCopyAttributeOnOwnLine(t *testing.T) {
	input := "@noCopy type SessionID struct {\nvalue: uint64,\n}\n"
	want := "@noCopy\ntype SessionID struct {\n    value: uint64,\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong @noCopy formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatRemovesInitialByteOrderMark(t *testing.T) {
	got := Format(Source{Text: "\uFEFFmodule main\n"}, Options{}).Text
	if got != "module main\n" {
		t.Fatalf("formatter retained initial BOM: %q", got)
	}
}

func TestFormatImplExtension(t *testing.T) {
	input := "impl extends Vehicle {\nfn Stop() void {\n}\n}\n"
	want := "impl extends Vehicle {\n    fn Stop() void {\n    }\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong impl extension formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatInitAndNewLifecycleSyntax(t *testing.T) {
	input := "impl Buffer {\ninit( size: uint, alignment: uint, ) AllocationError {\n}\n}\n\nfn Make() Result[Buffer, AllocationError] {\nreturn try new Buffer(4096, 16)\n}\n"
	want := "impl Buffer {\n    init(size: uint, alignment: uint) AllocationError {\n    }\n}\n\nfn Make() Result[Buffer, AllocationError] {\n    return try new Buffer(4096, 16)\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("wrong lifecycle formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesRegisterLayoutModifiers(t *testing.T) {
	input := "type Header register[16] msb-first big-endian {\nVersion: bit[4],\nPayload: bit[12],\n}\n"
	want := "type Header register[16] msb-first big-endian {\n    Version: bit[4],\n    Payload: bit[12],\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong register modifier formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatPreservesUnionDefaultVariantMarker(t *testing.T) {
	input := "type State union {\nIdle default\nRunning\n}\n"
	want := "type State union {\n    Idle default\n    Running\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong union default formatting:\n%s\nwant:\n%s", got, want)
	}
}

func TestFixReversedTypeDeclarationOrder(t *testing.T) {
	input := "type struct User {\nname: string,\n}\n\n" +
		"type union State {\nReady\n}\n\n" +
		"type register Status[8] {\nReady: bit,\n_: bit[7],\n}\n"
	want := "type User struct {\n    name: string,\n}\n\n" +
		"type State union {\n    Ready\n}\n\n" +
		"type Status register[8] {\n    Ready: bit,\n    _: bit[7],\n}\n"

	if got := Format(Source{Text: input}, Options{}).Text; got == want || !strings.Contains(got, "type struct User") {
		t.Fatalf("ordinary formatting unexpectedly repaired declaration order:\n%s", got)
	}
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != want {
		t.Fatalf("wrong fixed declaration order:\n%s\nwant:\n%s", got, want)
	}
}

func TestFixDoesNotRewriteContextualRegisterIdentifierWithoutRegisterShape(t *testing.T) {
	input := "type register Word\n"
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != input {
		t.Fatalf("fix rewrote an unproven contextual register identifier: %q", got)
	}
}

func TestFormatPreservesCanonicalNumericFamilySuffixes(t *testing.T) {
	input := "fn Values() void {\nlet values := [8i, 8u, 8g, 8m, 65t, 65r, 0x41t, 0x10g, 0x10m]\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	for _, literal := range []string{"8i", "8u", "8g", "8m", "65t", "65r", "0x41t", "0x10g", "0x10m"} {
		if !strings.Contains(got, literal) {
			t.Fatalf("formatter lost canonical literal %s:\n%s", literal, got)
		}
	}
}

func TestFormatTryHandlerAfterStructLiteralCallArgument(t *testing.T) {
	input := `fn NextToken() Token {
if invalid {
let token := self.readOne(ILLEGAL)
try self.diagnostics.Append(Diagnostic {
ID: "L1002",
Message: $"unexpected byte-order mark at {line}:{column}",
Primary: token,
}) {
Err(error) => { return }
}
return token
}
return self.readOne(ILLEGAL)
}
`
	want := `fn NextToken() Token {
    if invalid {
        let token := self.readOne(ILLEGAL)
        try self.diagnostics.Append(Diagnostic {
            ID: "L1002",
            Message: $"unexpected byte-order mark at {line}:{column}",
            Primary: token,
        }) {
            Err(error) => { return }
        }
        return token
    }
    return self.readOne(ILLEGAL)
}
`

	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("wrong try-handler indentation:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("try-handler formatting is not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestFormatInterfaceReceiverSignatures(t *testing.T) {
	input := "interface Resource {\nmut fn Update( value: int, ) void\n-> fn Detach( ) int\nstatic fn Create( ) int\n}\n"
	want := "interface Resource {\n    mut fn Update(value: int) void\n    -> fn Detach() int\n    static fn Create() int\n}\n"
	if got := Format(Source{Text: input}, Options{}).Text; got != want {
		t.Fatalf("formatted interface signatures:\n%s\nwant:\n%s", got, want)
	}
}
