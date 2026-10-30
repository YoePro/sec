package hardware

import (
	"math/big"
	"reflect"
	"sync"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/modules"
)

func testModule(root, path string) modules.ModuleIdentity {
	return modules.ModuleIdentity{ImportRootIdentity: root, CanonicalImportPath: path}
}
func integer(text string) Value      { return Value{Kind: "integer", Text: text} }
func text(value string) Value        { return Value{Kind: "string", Text: value} }
func boolean(b bool) Value           { return Value{Kind: "bool", Bool: b} }
func selector(parts ...string) Value { return Value{Kind: "path", Path: parts} }
func widths(values ...string) Value {
	v := Value{Kind: "list"}
	for _, s := range values {
		v.Elements = append(v.Elements, integer(s))
	}
	return v
}
func testDeclaration(category Category, name string, properties map[string]Value) Declaration {
	module := testModule("project", "board")
	d := Declaration{Identity: Identity{Module: module, Category: category, Name: name}, Scope: Scope{Module: module, File: "board.sec"}, Origin: Origin{Token: lexer.Token{File: "board.sec", Line: 1, Column: 1}}}
	// Match the schema's stable presentation order, independent of Go map order.
	for _, member := range Schema(category) {
		if v, ok := properties[member.Name]; ok {
			v.Origin = Origin{Token: lexer.Token{File: "board.sec", Line: len(d.Properties) + 2, Column: 4}}
			d.Properties = append(d.Properties, Property{Name: member.Name, Value: v, Origin: v.Origin})
		}
	}
	return d
}
func testDeclarations() []Declaration {
	return []Declaration{
		testDeclaration(RegionCategory, "Space", map[string]Value{"domain": text("physical"), "kind": selector("HardwareRegionKind", "MMIO"), "base": integer("16"), "size": integer("256"), "readable": boolean(true), "writable": boolean(true)}),
		testDeclaration(ResourceCategory, "State", map[string]Value{"width": integer("32")}),
		testDeclaration(EndpointCategory, "Port", map[string]Value{
			"kind": selector("HardwareEndpointKind", "MemoryMapped"), "region": selector("Space"), "resource": selector("State"), "offset": integer("4"), "size": integer("4"), "alignment": integer("4"), "read_widths": widths("32", "8", "16"), "write_widths": widths("32"),
			"read_footprint": selector("HardwareFootprintKind", "Whole"), "write_footprint": selector("HardwareFootprintKind", "Whole"), "read_effect": selector("HardwareReadEffect", "NonDestructive"), "write_operation": selector("HardwareWriteOperation", "Replace"), "ordering": selector("HardwareOrdering", "Volatile"), "completion": selector("HardwareCompletion", "Issued"), "atomicity": selector("HardwareAtomicity", "Unspecified"), "context": selector("HardwareAccessContext", "Current"), "fault": selector("HardwareFaultContract", "Platform")}),
	}
}
func testTarget() TargetFacts {
	return TargetFacts{UintBits: 32, Domains: []layout.AddressDomain{{Identity: "physical", AddressBits: 128, UnitBits: 8, Linear: true}}}
}
func setProperty(d *Declaration, name string, v Value) {
	for i := range d.Properties {
		if d.Properties[i].Name == name {
			v.Origin = d.Properties[i].Value.Origin
			d.Properties[i].Value = v
			return
		}
	}
	d.Properties = append(d.Properties, Property{Name: name, Value: v})
}
func removeProperty(d *Declaration, name string) {
	for i, p := range d.Properties {
		if p.Name == name {
			d.Properties = append(d.Properties[:i], d.Properties[i+1:]...)
			return
		}
	}
}
func hasIssue(issues []Issue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
func requireFailure(t *testing.T, target TargetFacts, decls []Declaration, code string) []Issue {
	t.Helper()
	facts, issues := Normalize(target, decls)
	if !hasIssue(issues, code) {
		t.Fatalf("want %s issue, got %+v", code, issues)
	}
	if len(facts.Regions)+len(facts.Resources)+len(facts.Endpoints) != 0 {
		t.Fatal("invalid declaration published usable facts")
	}
	return issues
}

func TestClosedSchemasAndEveryMemberType(t *testing.T) {
	sizes := map[Category]int{RegionCategory: 6, ResourceCategory: 1, EndpointCategory: 17}
	for category, size := range sizes {
		if len(Schema(category)) != size {
			t.Fatalf("%s schema size", category)
		}
	}
	if Schema("other") != nil || Enum("Other") != nil {
		t.Fatal("schema fabricated a category")
	}
	for index, decl := range testDeclarations() {
		for _, member := range Schema(decl.Identity.Category) {
			t.Run(string(decl.Identity.Category)+"/"+member.Name, func(t *testing.T) {
				decls := testDeclarations()
				value := boolean(false)
				if member.Type == "bool" {
					value = text("false")
				}
				setProperty(&decls[index], member.Name, value)
				requireFailure(t, testTarget(), decls, "type")
				decls = testDeclarations()
				removeProperty(&decls[index], member.Name)
				requireFailure(t, testTarget(), decls, "capability")
			})
		}
	}
	schema := Schema(EndpointCategory)
	schema[0].Type = "fake"
	if Schema(EndpointCategory)[0].Type == "fake" {
		t.Fatal("mutable authoritative schema")
	}
}
func TestEveryClosedMetadataEnum(t *testing.T) {
	counts := map[string]int{"HardwareRegionKind": 4, "HardwareEndpointKind": 5, "HardwareFootprintKind": 1, "HardwareReadEffect": 3, "HardwareWriteOperation": 4, "HardwareOrdering": 1, "HardwareCompletion": 1, "HardwareAtomicity": 1, "HardwareAccessContext": 1, "HardwareFaultContract": 1}
	for typ, count := range counts {
		cases := Enum(typ)
		if len(cases) != count {
			t.Fatalf("%s cases", typ)
		}
		encodings := map[string]bool{}
		for _, c := range cases {
			n := normalizer{target: testTarget()}
			v, ok := n.value(typ, selector(typ, c.Name))
			if !ok || v.Text != c.Name || c.Encoding == "" || encodings[c.Encoding] {
				t.Fatalf("%s.%s %+v", typ, c.Name, n.issues)
			}
			encodings[c.Encoding] = true
		}
		for _, bad := range []Value{selector(typ, "Unknown"), selector("Other", cases[0].Name), selector(cases[0].Name)} {
			n := normalizer{}
			if _, ok := n.value(typ, bad); ok || !hasIssue(n.issues, "enum") {
				t.Fatalf("accepted bad %s: %+v", typ, bad)
			}
		}
		n := normalizer{}
		if _, ok := n.value(typ, text(cases[0].Encoding)); ok || !hasIssue(n.issues, "type") {
			t.Fatal("source accepted manifest enum string")
		}
		cases[0].Name = "Fake"
		if Enum(typ)[0].Name == "Fake" {
			t.Fatal("mutable enum table")
		}
	}
}
func TestExactIntegersAndDomainBoundaries(t *testing.T) {
	for input, want := range map[string]string{"0010": "10", "0x10": "16", "0Xf_f": "255", "1_024": "1024", "0": "0"} {
		got, ok := ParseInteger(input)
		if !ok || got.String() != want {
			t.Fatalf("%s -> %v", input, got)
		}
	}
	for _, bad := range []string{"-1", "+1", "0b10", "0o10", " 1", "1 ", "1e9", "0x", "1__0", "_1", "1_", "12u32"} {
		if _, ok := ParseInteger(bad); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
	for _, bits := range []uint16{32, 64, 128, 256} {
		t.Run(big.NewInt(int64(bits)).String(), func(t *testing.T) {
			target := testTarget()
			target.Domains[0].AddressBits = bits
			limit := target.Domains[0].Limit()
			decls := testDeclarations()
			setProperty(&decls[0], "base", integer("0"))
			setProperty(&decls[0], "size", integer(limit.String()))
			if _, issues := Normalize(target, decls); len(issues) > 0 {
				t.Fatal(issues)
			}
			setProperty(&decls[0], "base", integer("1"))
			requireFailure(t, target, decls, "range")
			setProperty(&decls[0], "base", integer(limit.String()))
			setProperty(&decls[0], "size", integer("1"))
			requireFailure(t, target, decls, "range")
			// Last address plus one ends exactly at the cardinality, independent of uint.
			decls = decls[:1]
			setProperty(&decls[0], "base", integer(new(big.Int).Sub(limit, big.NewInt(1)).String()))
			if _, issues := Normalize(target, decls); len(issues) > 0 {
				t.Fatal(issues)
			}
		})
	}
	// Domain values exceed CPU uint; ordinary uints may not use that wider domain.
	decls := testDeclarations()
	setProperty(&decls[0], "base", integer("18446744073709551616"))
	facts, issues := Normalize(testTarget(), decls)
	if len(issues) > 0 || facts.Endpoints[0].Address.String() != "18446744073709551620" {
		t.Fatalf("wide domain: %+v %+v", facts, issues)
	}
	setProperty(&decls[1], "width", integer("4294967296"))
	requireFailure(t, testTarget(), decls, "range")
	target := testTarget()
	target.UintBits = 64
	if _, issues = Normalize(target, decls); len(issues) > 0 {
		t.Fatal(issues)
	}
}
func TestSchemaAndEndpointFailures(t *testing.T) {
	cases := []struct {
		name, code string
		change     func([]Declaration, *TargetFacts)
	}{
		{"unknown-member", "member", func(d []Declaration, _ *TargetFacts) {
			d[0].Properties = append(d[0].Properties, Property{Name: "name", Value: text("bad")})
		}},
		{"duplicate-member", "member", func(d []Declaration, _ *TargetFacts) { d[0].Properties = append(d[0].Properties, d[0].Properties[0]) }},
		{"unknown-domain", "domain", func(d []Declaration, _ *TargetFacts) { setProperty(&d[0], "domain", text("invented")) }},
		{"non-linear", "unsupported", func(_ []Declaration, p *TargetFacts) { p.Domains[0].Linear = false }},
		{"zero-region", "range", func(d []Declaration, _ *TargetFacts) { setProperty(&d[0], "size", integer("0")) }},
		{"zero-resource", "capability", func(d []Declaration, _ *TargetFacts) { setProperty(&d[1], "width", integer("0")) }},
		{"offset-overflow", "range", func(d []Declaration, _ *TargetFacts) {
			setProperty(&d[2], "offset", integer("340282366920938463463374607431768211455"))
		}},
		{"region-overrun", "range", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "offset", integer("256")) }},
		{"zero-extent", "range", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "size", integer("0")) }},
		{"zero-alignment", "range", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "alignment", integer("0")) }},
		{"misalignment", "range", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "offset", integer("1")) }},
		{"width-units", "range", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "size", integer("1")) }},
		{"duplicate-width", "capability", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "read_widths", widths("32", "32")) }},
		{"zero-width", "capability", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "read_widths", widths("0")) }},
		{"disabled-read-property", "capability", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "read_widths", widths()) }},
		{"disabled-write-property", "capability", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "write_widths", widths()) }},
		{"no-direction", "capability", func(d []Declaration, _ *TargetFacts) {
			setProperty(&d[2], "read_widths", widths())
			setProperty(&d[2], "write_widths", widths())
		}},
		{"region-read-permission", "capability", func(d []Declaration, _ *TargetFacts) { setProperty(&d[0], "readable", boolean(false)) }},
		{"region-write-permission", "capability", func(d []Declaration, _ *TargetFacts) { setProperty(&d[0], "writable", boolean(false)) }},
		{"reserved", "conflict", func(d []Declaration, _ *TargetFacts) {
			setProperty(&d[0], "kind", selector("HardwareRegionKind", "Reserved"))
		}},
		{"rom-write", "conflict", func(d []Declaration, _ *TargetFacts) {
			setProperty(&d[0], "kind", selector("HardwareRegionKind", "ROM"))
		}},
		{"unknown-region", "symbol", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "region", selector("Missing")) }},
		{"unknown-resource", "symbol", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "resource", selector("Missing")) }},
		{"wrong-category", "symbol", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "resource", selector("Space")) }},
		{"invented-qualifier", "symbol", func(d []Declaration, _ *TargetFacts) { setProperty(&d[2], "region", selector("Platform", "Space")) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := testDeclarations()
			p := testTarget()
			c.change(d, &p)
			requireFailure(t, p, d, c.code)
		})
	}
	for _, kind := range Enum("HardwareEndpointKind")[1:] {
		d := testDeclarations()
		setProperty(&d[2], "kind", selector("HardwareEndpointKind", kind.Name))
		issues := requireFailure(t, testTarget(), d, "unsupported")
		if hasIssue(issues, "range") {
			t.Fatal("invented scalar location interpretation for unsupported endpoint")
		}
	}
}
func TestTargetCompatibilityAndRegionOverlap(t *testing.T) {
	d := testDeclarations()
	baseFacts, issues := Normalize(testTarget(), d)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	p := testTarget()
	known := baseFacts.Regions[0]
	known.Kind = "Reserved"
	p.EstablishedRegions = []Region{known}
	requireFailure(t, p, d, "conflict")
	second := testDeclarations()[0]
	second.Identity.Name = "OtherSpace"
	d = append(d, second)
	facts, issues := Normalize(testTarget(), d)
	if len(facts.Regions) != 2 || len(issues) != 1 || !issues[0].Warning || issues[0].Related.Token.File == "" {
		t.Fatalf("compatible overlap %+v", issues)
	}
	setProperty(&d[3], "kind", selector("HardwareRegionKind", "RAM"))
	requireFailure(t, testTarget(), d, "conflict")
}
func TestCanonicalResolutionVisibilityAndIdentity(t *testing.T) {
	d := testDeclarations()
	owner := d[0].Identity.Module
	consumer := testModule("dependency", "client")
	d[2].Identity.Module = consumer
	d[2].Scope = Scope{Module: consumer, File: "client.sec", Imports: map[string][]modules.ModuleIdentity{"board": {owner}}}
	setProperty(&d[2], "region", selector("board", "Space"))
	setProperty(&d[2], "resource", selector("board", "State"))
	facts, issues := Normalize(testTarget(), d)
	if len(issues) > 0 || Identity(facts.Endpoints[0].Region).Module != owner {
		t.Fatal(issues)
	}
	// Same short module spelling in another import root is a distinct owner.
	other := testDeclarations()[1]
	other.Identity.Module = testModule("other-root", "board")
	other.Scope.Module = other.Identity.Module
	d = append(d, other)
	if _, issues = Normalize(testTarget(), d); len(issues) > 0 {
		t.Fatal(issues)
	}
	d[2].Scope.Imports["board"] = []modules.ModuleIdentity{owner, other.Identity.Module}
	requireFailure(t, testTarget(), d, "symbol")
	d[2].Scope.Imports["board"] = []modules.ModuleIdentity{owner}
	for _, name := range []string{"_State", "__State"} {
		d[1].Identity.Name = name
		setProperty(&d[2], "resource", selector("board", name))
		requireFailure(t, testTarget(), d, "symbol")
	}
	// Module-internal access works in the same canonical module, private needs same file.
	d = testDeclarations()
	d[1].Identity.Name = "_State"
	setProperty(&d[2], "resource", selector("_State"))
	d[2].Scope.File = "second.sec"
	if _, issues = Normalize(testTarget(), d); len(issues) > 0 {
		t.Fatal(issues)
	}
	d[1].Identity.Name = "__State"
	setProperty(&d[2], "resource", selector("__State"))
	requireFailure(t, testTarget(), d, "symbol")
	d[2].Scope.File = d[1].Scope.File
	if _, issues = Normalize(testTarget(), d); len(issues) > 0 {
		t.Fatal(issues)
	}
	d[0].Identity.Module = modules.ModuleIdentity{}
	requireFailure(t, testTarget(), d, "context")
}
func TestFactsIsolationDeterminismAndProvenance(t *testing.T) {
	d := testDeclarations()
	p := testTarget()
	facts, issues := Normalize(p, d)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	endpoint := facts.Endpoints[0]
	rv := endpoint.Properties["read_widths"]
	if rv.Widths[0].String() != "8" || rv.Source.Elements[0].Text != "32" || rv.Origin.Token.Line == 0 || rv.Source.Origin.Token.Column == 0 {
		t.Fatal("lost source order/provenance or set normalization")
	}
	snapshot := facts.Clone()
	facts.Regions[0].Base.SetInt64(99)
	facts.Endpoints[0].Properties["alignment"].Integer.SetInt64(99)
	facts.Endpoints[0].Properties["read_widths"].Source.Elements[0].Text = "99"
	facts.Domains[0].Identity = "changed"
	if snapshot.Regions[0].Base.Int64() != 16 || snapshot.Endpoints[0].Properties["alignment"].Integer.Int64() != 4 || snapshot.Endpoints[0].Properties["read_widths"].Source.Elements[0].Text != "32" || snapshot.Domains[0].Identity != "physical" {
		t.Fatal("mutable snapshot")
	}
	// Reverse enumeration order. Locations remain original; values and identities must agree.
	d[0], d[2] = d[2], d[0]
	reordered, issues := Normalize(p, d)
	if len(issues) > 0 || !reflect.DeepEqual(snapshot, reordered) {
		t.Fatalf("enumeration-dependent facts: %v", issues)
	}
	for index := range d {
		for i, j := 0, len(d[index].Properties)-1; i < j; i, j = i+1, j-1 {
			d[index].Properties[i], d[index].Properties[j] = d[index].Properties[j], d[index].Properties[i]
		}
	}
	reordered, issues = Normalize(p, d)
	if len(issues) > 0 || !reflect.DeepEqual(snapshot, reordered) {
		t.Fatal("property-order-dependent facts", issues)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, errs := Normalize(p, d)
			if len(errs) > 0 || !reflect.DeepEqual(snapshot, got) {
				t.Error("parallel nondeterminism")
			}
		}()
	}
	wg.Wait()
	// Two endpoint declarations may share a resource without a binding alias warning.
	d = testDeclarations()
	other := testDeclarations()[2]
	other.Identity.Name = "OtherPort"
	d = append(d, other)
	facts, issues = Normalize(p, d)
	if len(issues) > 0 || len(facts.Endpoints) != 2 || facts.Endpoints[0].Resource != facts.Endpoints[1].Resource || facts.Endpoints[0].Identity == facts.Endpoints[1].Identity {
		t.Fatal(issues)
	}
}

func TestInvalidSelectedHardwareContextFailsClosed(t *testing.T) {
	for _, change := range []func(*TargetFacts){
		func(p *TargetFacts) { p.UintBits = 0 },
		func(p *TargetFacts) { p.Domains = append(p.Domains, p.Domains[0]) },
		func(p *TargetFacts) { p.Domains[0].UnitBits = 0 },
		func(p *TargetFacts) { p.EstablishedRegions = []Region{{Kind: "MMIO", Domain: p.Domains[0]}} },
		func(p *TargetFacts) {
			p.EstablishedRegions = []Region{{Kind: "Invented", Domain: p.Domains[0], Base: big.NewInt(0), Size: big.NewInt(1)}}
		},
	} {
		target := testTarget()
		change(&target)
		facts, issues := Normalize(target, testDeclarations())
		if len(issues) == 0 || len(facts.Regions)+len(facts.Resources)+len(facts.Endpoints) > 0 {
			t.Fatalf("invalid context published %+v: %+v", facts, issues)
		}
	}
}

func TestReservedHardwareDeclarationNames(t *testing.T) {
	for _, name := range []string{"_", "int", "task", "fn"} {
		declarations := testDeclarations()
		declarations[1].Identity.Name = name
		requireFailure(t, testTarget(), declarations, "reserved")
	}
}
