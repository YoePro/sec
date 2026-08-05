package sema

import "testing"

func TestCompilerKnownFundamentalMembers(t *testing.T) {
	input := `
module main

fn Test(text: string, runes: rune[2], ptr: RawPtr[int]) void {
	let canonicalLength: uint := text.Len
	let migrationLength: uint := text.len
	let valueSize: uint := text.SizeOf()
	let typeSize: uint := int32.SizeOf()
	let minimum: int32 := int32.Min
	let maximum: int32 := int32.Max
	let bits: uint := int32.Bits
	let formatted: string := true.ToString()
	let bytes: byte[] := text.ToByteArray()
	let chars: char[] := text.ToCharArray()
	let decoded: rune[] := text.ToRuneArray()
	let joined: string := runes.ToString()
	let fromRunes: string := string.FromRuneArray(runes)
	let fromBytes: string := string.FromByteArray(bytes)
	unsafe {
		let address: RawPtr[byte] := text.Ptr
		let value: int := ptr.Read()
		ptr.Write(value)
	}
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestCompilerKnownArenaMembers(t *testing.T) {
	input := `
module main

fn Test(buffer: ref mut byte[]) void {
	let mut borrowed := Arena.FromBuffer(buffer)
	let one := borrowed.New[int]()
	let many := borrowed.Alloc[int](2u)
	borrowed.Reset()
	borrowed.Release()

	let fixed := Arena.WithCapacity(1024u)
	let growable := Arena.Growable(1024u)
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

func TestCompilerKnownMemberValidation(t *testing.T) {
	input := `
module main

fn Test(text: string, ptr: RawPtr[int], shared: ref byte[], number: int) void {
	let badLength := number.Len
	let badConversion := text.ToRuneArray(1)
	let unsafeRead := ptr.Read()
	let badArena := Arena.FromBuffer(shared)
}
`
	errors := analyzeSourceRaw(t, input)
	if len(errors) != 4 {
		t.Fatalf("wrong sema error count. got=%d want=4 errors=%v", len(errors), errors)
	}
}

func TestCompilerKnownRegistryHasStableRequiredIDs(t *testing.T) {
	stringMembers := CompilerKnownMembersForType(builtinTypes()["string"], false)
	want := map[string]bool{
		"CKM-PTR-VALUE":          false,
		"CKM-LEN-STRING":         false,
		"CKM-SIZEOF-VALUE":       false,
		"CKM-TOSTRING-STRING":    false,
		"CKM-STRING-TOBYTEARRAY": false,
		"CKM-STRING-TOCHARARRAY": false,
		"CKM-STRING-TORUNEARRAY": false,
	}
	for _, member := range stringMembers {
		if _, exists := want[member.ID]; exists {
			want[member.ID] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Fatalf("compiler-known registry is missing %s", id)
		}
	}
}
