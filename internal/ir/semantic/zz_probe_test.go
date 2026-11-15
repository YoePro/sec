package semantic

import "testing"

func TestZZProbe(t *testing.T) {
	prelude := "type Box struct {\n    value: int,\n}\n\nfn Use(b: ref Box) void {\n}\n\nfn Bump(b: ref mut Box) void {\n}\n\n"
	src := "fn F(b: ref Box, m: ref mut Box) void {\n    Use(b)\n    Bump(m)\n}\n"
	m, err := analyzedModule(t, "module main\n\n"+prelude+src+"\nfn main() int {\n    return 0\n}\n", 17)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(Format(m))
}
