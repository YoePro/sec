package hardware

import (
	"fmt"
	"math/big"
	"regexp"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/modules"
	"sort"
	"strings"
)

var integerLiteral = regexp.MustCompile(`^(?:0[xX][0-9a-fA-F](?:_?[0-9a-fA-F])*|[0-9](?:_?[0-9])*)$`)

// ParseInteger decodes only exact unsigned decimal/hex metadata literals. It
// never interprets a leading zero as octal or narrows via host int (MD-111 §3(3)).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func ParseInteger(text string) (*big.Int, bool) {
	if !integerLiteral.MatchString(text) {
		return nil, false
	}
	text = strings.ReplaceAll(text, "_", "")
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base = 16
		text = text[2:]
	}
	v, ok := new(big.Int).SetString(text, base)
	return v, ok
}

type normalizer struct {
	target    TargetFacts
	decls     []Declaration
	symbols   map[modules.ModuleIdentity]map[string][]int
	records   map[int]Record
	regions   map[Identity]Region
	resources map[Identity]Resource
	issues    []Issue
}

// Normalize validates source metadata in dependency order, without evaluating
// Sec expressions or performing hardware I/O. Any invalid declaration prevents
// publication of a usable fact set. Warnings retain valid facts.
// Rules: MD-111 §§2–8,10(6),12(2)3–4,13(2),(4)–(5).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func Normalize(target TargetFacts, declarations []Declaration) (Facts, []Issue) {
	n := normalizer{target: target, decls: declarations, symbols: map[modules.ModuleIdentity]map[string][]int{}, records: map[int]Record{}, regions: map[Identity]Region{}, resources: map[Identity]Resource{}}
	// Validate trusted input too: never accept unresolved or duplicate domains as
	// positive target evidence. No domain is synthesized from pointer width.
	seenDomains := map[string]bool{}
	for _, d := range target.Domains {
		if err := d.Validate(); err != nil || seenDomains[d.Identity] {
			n.issue("context", Origin{}, Origin{}, "invalid or duplicate selected address domain %q", d.Identity)
		}
		seenDomains[d.Identity] = true
	}
	for _, known := range target.EstablishedRegions {
		matched := false
		for _, domain := range target.Domains {
			if domain == known.Domain {
				matched = true
				break
			}
		}
		kindKnown := false
		for _, kind := range Enum("HardwareRegionKind") {
			if known.Kind == kind.Name {
				kindKnown = true
				break
			}
		}
		if err := known.Domain.Validate(); err != nil || !matched || !kindKnown {
			n.issue("context", known.Origin, Origin{}, "invalid established target/Device region facts")
			continue
		}
		n.rangeInDomain(known.Base, known.Size, known.Domain.Limit(), known.Origin)
	}
	if len(declarations) > 0 && target.UintBits != 32 && target.UintBits != 64 {
		n.issue("context", declarations[0].Origin, Origin{}, "hardware metadata requires a resolved target uint width")
	}
	for i, d := range declarations {
		if _, err := modules.NewCrossModuleDeclarationIdentity(d.Identity.Module, d.Identity.Name); err != nil || d.Scope.Module != d.Identity.Module || d.Scope.File == "" {
			n.issue("context", d.Origin, Origin{}, "hardware declaration %q requires canonical resolver-owned module and source context", d.Identity.Name)
			continue
		}
		if lexer.IsReservedDeclarationName(d.Identity.Name) {
			n.issue("reserved", d.Origin, Origin{}, "hardware declaration name %q is reserved by the language", d.Identity.Name)
			continue
		}
		if n.symbols[d.Identity.Module] == nil {
			n.symbols[d.Identity.Module] = map[string][]int{}
		}
		same := n.symbols[d.Identity.Module][d.Identity.Name]
		if len(same) > 0 {
			n.issue("member", d.Origin, declarations[same[0]].Origin, "duplicate hardware declaration %q", d.Identity.Name)
		}
		n.symbols[d.Identity.Module][d.Identity.Name] = append(same, i)
		if record, ok := n.checkSchema(d); ok {
			n.records[i] = record
		}
	}
	for i, d := range declarations {
		if r, ok := n.records[i]; ok {
			switch d.Identity.Category {
			case RegionCategory:
				n.region(r)
			case ResourceCategory:
				n.resource(r)
			}
		}
	}
	n.overlaps()
	endpoints := []Endpoint{}
	for i, d := range declarations {
		if r, ok := n.records[i]; ok && d.Identity.Category == EndpointCategory {
			if e, ok := n.endpoint(d, r); ok {
				endpoints = append(endpoints, e)
			}
		}
	}
	for _, issue := range n.issues {
		if !issue.Warning {
			return Facts{}, n.issues
		}
	}
	facts := Facts{UintBits: target.UintBits, Domains: append([]layout.AddressDomain(nil), target.Domains...), Endpoints: endpoints}
	for _, r := range n.regions {
		facts.Regions = append(facts.Regions, r)
	}
	for _, r := range n.resources {
		facts.Resources = append(facts.Resources, r)
	}
	sort.Slice(facts.Regions, func(i, j int) bool { return identityLess(facts.Regions[i].Identity, facts.Regions[j].Identity) })
	sort.Slice(facts.Resources, func(i, j int) bool { return identityLess(facts.Resources[i].Identity, facts.Resources[j].Identity) })
	sort.Slice(facts.Endpoints, func(i, j int) bool { return identityLess(facts.Endpoints[i].Identity, facts.Endpoints[j].Identity) })
	return facts.Clone(), n.issues
}

// identityLess gives deterministic canonical identity order, independent of
// hash iteration, addresses or display names (MD-111 §§2,12).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func identityLess(a, b Identity) bool {
	if a.Module.ImportRootIdentity != b.Module.ImportRootIdentity {
		return a.Module.ImportRootIdentity < b.Module.ImportRootIdentity
	}
	if a.Module.CanonicalImportPath != b.Module.CanonicalImportPath {
		return a.Module.CanonicalImportPath < b.Module.CanonicalImportPath
	}
	if a.Category != b.Category {
		return a.Category < b.Category
	}
	return a.Name < b.Name
}

// issue preserves the failed property's location and related declaration (§13(2)).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) issue(code string, at, related Origin, format string, args ...any) {
	help := map[string]string{
		"reserved":    "Choose a non-reserved identifier for this hardware declaration.",
		"type":        "Use a literal of the member's exact metadata type; runtime expressions and implicit coercions are not accepted.",
		"member":      "Use only the category's declared member names, each once, and a distinct declaration name in the canonical module.",
		"enum":        "Write EnumName.Case using a case from that member's closed metadata enum.",
		"domain":      "Name an address domain supplied by the selected target; declarations cannot invent domains.",
		"range":       "Use a positive extent wholly contained in the selected region and domain, a valid aligned address, and widths in bits; uint members must fit the target uint.",
		"symbol":      "Use a visible hardware declaration of the expected category and an actual unambiguous imported-module qualifier.",
		"capability":  "Supply the required capabilities for each enabled direction, remove disabled-direction members, and respect region permissions.",
		"conflict":    "Correct the region contract to agree with established target/Device facts and overlapping region kinds; alias acknowledgement cannot waive this error.",
		"unsupported": "This recognized hardware kind has no supported declaration schema; do not invent a scalar location or physical operation for it.",
		"context":     "Supply the selected target and resolver-owned canonical module context before collecting hardware metadata; do not infer identity from a short name or source path.",
	}[code]
	n.issues = append(n.issues, Issue{Code: code, Message: fmt.Sprintf(format, args...), Origin: at, Related: related, Help: help})
}

// checkSchema enforces closed member names/types and directional requiredness.
// Rules: MD-111 §§3–7,13(2).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) checkSchema(d Declaration) (Record, bool) {
	start := len(n.issues)
	table := Schema(d.Identity.Category)
	r := Record{Identity: d.Identity, Origin: d.Origin, Properties: map[string]ResolvedValue{}}
	if len(table) == 0 {
		n.issue("member", d.Origin, Origin{}, "unknown hardware category %q", d.Identity.Category)
		return r, false
	}
	seen := map[string]Origin{}
	for _, p := range d.Properties {
		if prev, exists := seen[p.Name]; exists {
			n.issue("member", p.Origin, prev, "duplicate hardware member %q", p.Name)
			continue
		}
		seen[p.Name] = p.Origin
		var member *Member
		for i := range table {
			if table[i].Name == p.Name {
				member = &table[i]
				break
			}
		}
		if member == nil {
			n.issue("member", p.Origin, Origin{}, "unknown hardware %s member %q", d.Identity.Category, p.Name)
			continue
		}
		beforeValue := len(n.issues)
		if v, ok := n.value(member.Type, p.Value); ok {
			v.Origin = p.Origin
			r.Properties[p.Name] = v
		}
		for i := beforeValue; i < len(n.issues); i++ {
			n.issues[i].Message = fmt.Sprintf("hardware %s member %q: %s", d.Identity.Category, p.Name, n.issues[i].Message)
		}
	}
	reads := len(r.Properties["read_widths"].Widths) > 0
	writes := len(r.Properties["write_widths"].Widths) > 0
	unsupported := d.Identity.Category == EndpointCategory && r.Properties["kind"].Text != "" && r.Properties["kind"].Text != "MemoryMapped"
	if unsupported {
		n.issue("unsupported", r.Properties["kind"].Origin, Origin{}, "unsupported endpoint-kind schema %s", r.Properties["kind"].Text)
	}
	for _, m := range table {
		required := m.Required == "always" || m.Required == "linear" || m.Required == "memory-mapped" && !unsupported || m.Required == "read" && reads || m.Required == "write" && writes || m.Required == "enabled" && (reads || writes)
		_, present := seen[m.Name]
		if required && !present {
			n.issue("capability", d.Origin, Origin{}, "missing required hardware %s member %q (%s)", d.Identity.Category, m.Name, m.Type)
		}
		if present && (m.Required == "read" && !reads || m.Required == "write" && !writes) {
			n.issue("capability", seen[m.Name], Origin{}, "hardware member %q must be absent when its direction is disabled", m.Name)
		}
	}
	if d.Identity.Category == EndpointCategory && !reads && !writes {
		n.issue("capability", d.Origin, Origin{}, "hardware endpoint must enable at least one read or write width")
	}
	return r, len(n.issues) == start
}

// value resolves only the member's metadata type; ordinary runtime declarations
// cannot shadow schema enums or coerce literal kinds. Rules: MD-111 §§3,7–8.
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) value(typ string, v Value) (ResolvedValue, bool) {
	result := ResolvedValue{Type: typ, Origin: v.Origin, Source: cloneValue(v)}
	wrong := func() (ResolvedValue, bool) {
		n.issue("type", v.Origin, Origin{}, "hardware member requires %s, got %s", typ, v.Kind)
		return result, false
	}
	switch typ {
	case "string":
		if v.Kind != "string" {
			return wrong()
		}
		result.Text = v.Text
	case "bool":
		if v.Kind != "bool" {
			return wrong()
		}
		result.Bool = v.Bool
	case "uint", "HardwareDomainAddress", "HardwareDomainExtent":
		if v.Kind != "integer" {
			return wrong()
		}
		x, ok := ParseInteger(v.Text)
		if !ok {
			return wrong()
		}
		if typ == "uint" && (n.target.UintBits == 0 || x.BitLen() > int(n.target.UintBits)) {
			n.issue("range", v.Origin, Origin{}, "hardware uint %s exceeds selected target uint%d", x, n.target.UintBits)
			return result, false
		}
		result.Integer = x
	case "list<uint>":
		if v.Kind != "list" {
			return wrong()
		}
		seen := map[string]Origin{}
		for _, element := range v.Elements {
			item, ok := n.value("uint", element)
			if !ok {
				return result, false
			}
			if item.Integer.Sign() == 0 {
				n.issue("capability", element.Origin, Origin{}, "hardware transfer width must be positive bits")
				return result, false
			}
			key := item.Integer.String()
			if prev, found := seen[key]; found {
				n.issue("capability", element.Origin, prev, "duplicate hardware transfer width %s", key)
				return result, false
			}
			seen[key] = element.Origin
			result.Widths = append(result.Widths, item.Integer)
		}
		sort.Slice(result.Widths, func(i, j int) bool { return result.Widths[i].Cmp(result.Widths[j]) < 0 })
	case "HardwareRegionRef", "HardwareResourceRef":
		if v.Kind != "path" || len(v.Path) < 1 || len(v.Path) > 2 {
			return wrong()
		}
		result.Text = strings.Join(v.Path, ".")
	default:
		if v.Kind != "path" {
			return wrong()
		}
		if len(v.Path) != 2 || v.Path[0] != typ {
			n.issue("enum", v.Origin, Origin{}, "hardware enum requires qualified %s.Case, got %s", typ, strings.Join(v.Path, "."))
			return result, false
		}
		for _, c := range Enum(typ) {
			if c.Name == v.Path[1] {
				result.Text = c.Name
				return result, true
			}
		}
		n.issue("enum", v.Origin, Origin{}, "unknown %s case %q", typ, v.Path[1])
		return result, false
	}
	return result, true
}

// region validates exact half-open domain ranges and known target/Device
// restrictions. RAM/ROM facts do not create Places (MD-111 §§3(4),4).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) region(r Record) {
	p := r.Properties
	domainName := p["domain"].Text
	var region Region
	found := false
	for _, d := range n.target.Domains {
		if d.Identity == domainName {
			region.Domain = d
			found = true
			break
		}
	}
	if !found {
		n.issue("domain", p["domain"].Origin, Origin{}, "unknown hardware address domain %q", domainName)
		return
	}
	if !region.Domain.Linear {
		n.issue("unsupported", p["domain"].Origin, Origin{}, "unsupported non-linear hardware region schema for domain %q", domainName)
		return
	}
	region.Record = r
	region.Kind = p["kind"].Text
	region.Base = p["base"].Integer
	region.Size = p["size"].Integer
	region.Readable = p["readable"].Bool
	region.Writable = p["writable"].Bool
	if !n.rangeInDomain(region.Base, region.Size, region.Domain.Limit(), p["size"].Origin) {
		return
	}
	if region.Kind == "ROM" && region.Writable {
		n.issue("conflict", p["writable"].Origin, Origin{}, "ROM hardware region cannot grant write permission")
		return
	}
	for _, known := range n.target.EstablishedRegions {
		if region.Domain.Identity == known.Domain.Identity && rangesOverlap(region.Base, region.Size, known.Base, known.Size) && (region.Kind != known.Kind || region.Readable && !known.Readable || region.Writable && !known.Writable) {
			n.issue("conflict", r.Origin, known.Origin, "hardware region conflicts with established target/Device region")
			return
		}
	}
	n.regions[r.Identity] = region
}

// rangeInDomain never wraps or narrows a domain address/extent (MD-111 §3(4)).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) rangeInDomain(base, size, limit *big.Int, at Origin) bool {
	if base == nil || size == nil || base.Sign() < 0 || base.Cmp(limit) >= 0 || size.Sign() <= 0 || new(big.Int).Add(base, size).Cmp(limit) > 0 {
		n.issue("range", at, Origin{}, "invalid hardware address extent: nonempty [base, base + size) must fit the selected domain")
		return false
	}
	return true
}

// resource represents logical bits independently of location (MD-111 §5).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) resource(r Record) {
	width := r.Properties["width"].Integer
	if width.Sign() <= 0 {
		n.issue("capability", r.Properties["width"].Origin, Origin{}, "hardware resource width must be positive bits")
		return
	}
	n.resources[r.Identity] = Resource{Record: r, Width: width}
}

// resolve selects exactly one local/imported canonical module and checks the
// hardware category and ordinary underscore visibility, never runtime values.
// Rules: MD-111 §§2,8(3); names_scopes_visibility.md §12.
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) resolve(d Declaration, v ResolvedValue, category Category) (Identity, bool) {
	parts := strings.Split(v.Text, ".")
	module := d.Scope.Module
	name := parts[len(parts)-1]
	if len(parts) == 2 {
		candidates := d.Scope.Imports[parts[0]]
		if len(candidates) != 1 {
			n.issue("symbol", v.Origin, Origin{}, "unknown or ambiguous imported hardware qualifier %q", parts[0])
			return Identity{}, false
		}
		module = candidates[0]
	}
	indices := n.symbols[module][name]
	if len(indices) != 1 {
		n.issue("symbol", v.Origin, Origin{}, "unknown or ambiguous hardware %s %q", category, v.Text)
		return Identity{}, false
	}
	target := n.decls[indices[0]]
	if target.Identity.Category != category {
		n.issue("symbol", v.Origin, target.Origin, "hardware symbol %q is %s, expected %s", v.Text, target.Identity.Category, category)
		return Identity{}, false
	}
	if strings.HasPrefix(name, "__") && target.Scope.File != d.Scope.File || strings.HasPrefix(name, "_") && target.Identity.Module != d.Identity.Module {
		n.issue("symbol", v.Origin, target.Origin, "inaccessible hardware symbol %q", v.Text)
		return Identity{}, false
	}
	return target.Identity, true
}

// endpoint validates MMIO declarations, exact region/domain containment and
// declared direction permissions without inventing a physical plan (§6).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) endpoint(d Declaration, r Record) (Endpoint, bool) {
	start := len(n.issues)
	p := r.Properties
	regionID, regionOK := n.resolve(d, p["region"], RegionCategory)
	resourceID, resourceOK := n.resolve(d, p["resource"], ResourceCategory)
	region, regionValid := n.regions[regionID]
	_, resourceValid := n.resources[resourceID]
	if !regionOK || !resourceOK {
		return Endpoint{}, false
	}
	if !regionValid || !resourceValid {
		n.issue("symbol", r.Origin, Origin{}, "hardware endpoint references an invalid region or resource")
		return Endpoint{}, false
	}
	rv := p["region"]
	rv.Reference = regionID
	p["region"] = rv
	sv := p["resource"]
	sv.Reference = resourceID
	p["resource"] = sv
	offset, size, alignment := p["offset"].Integer, p["size"].Integer, p["alignment"].Integer
	if !n.rangeInDomain(offset, size, region.Domain.Limit(), p["size"].Origin) {
		return Endpoint{}, false
	}
	if new(big.Int).Add(offset, size).Cmp(region.Size) > 0 {
		n.issue("range", p["size"].Origin, region.Origin, "hardware endpoint extent is not contained in its region")
	}
	address := new(big.Int).Add(region.Base, offset)
	if !n.rangeInDomain(address, size, region.Domain.Limit(), p["offset"].Origin) {
		return Endpoint{}, false
	}
	if alignment.Sign() <= 0 || new(big.Int).Mod(address, alignment).Sign() != 0 {
		n.issue("range", p["alignment"].Origin, region.Origin, "hardware endpoint alignment must be positive and divide its domain address")
	}
	if region.Kind == "Reserved" {
		n.issue("conflict", p["region"].Origin, region.Origin, "reserved region cannot supply hardware endpoint access")
	}
	if len(p["read_widths"].Widths) > 0 && !region.Readable || len(p["write_widths"].Widths) > 0 && !region.Writable {
		n.issue("capability", r.Origin, region.Origin, "hardware endpoint direction is forbidden by its region")
	}
	bits := new(big.Int).Mul(size, new(big.Int).SetUint64(uint64(region.Domain.UnitBits)))
	for _, name := range []string{"read_widths", "write_widths"} {
		for _, width := range p[name].Widths {
			if width.Cmp(bits) > 0 {
				n.issue("range", p[name].Origin, region.Origin, "hardware transfer width %s bits exceeds mapped extent %s bits", width, bits)
			}
		}
	}
	return Endpoint{Record: r, Region: MemoryRegionIdentity(regionID), Resource: HardwareResourceIdentity(resourceID), Domain: region.Domain, Address: address, Size: size}, len(n.issues) == start
}

// rangesOverlap uses mathematical half-open extents, not host addresses (§4).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func rangesOverlap(a, as, b, bs *big.Int) bool {
	return a != nil && as != nil && b != nil && bs != nil && a.Cmp(new(big.Int).Add(b, bs)) < 0 && b.Cmp(new(big.Int).Add(a, as)) < 0
}

// overlaps distinguishes incompatible region contracts from compatible views;
// it never claims common resource identity or acknowledges binding aliases.
// Rules: MD-111 §§4(2),10(6),13(2).
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2–8,13(2).
func (n *normalizer) overlaps() {
	regions := make([]Region, 0, len(n.regions))
	for _, r := range n.regions {
		regions = append(regions, r)
	}
	sort.Slice(regions, func(i, j int) bool { return identityLess(regions[i].Identity, regions[j].Identity) })
	for i, a := range regions {
		for _, b := range regions[:i] {
			if a.Domain.Identity != b.Domain.Identity || !rangesOverlap(a.Base, a.Size, b.Base, b.Size) {
				continue
			}
			if a.Kind != b.Kind {
				n.issue("conflict", a.Origin, b.Origin, "incompatible overlapping hardware region kinds %s and %s", a.Kind, b.Kind)
			} else {
				n.issues = append(n.issues, Issue{Code: "overlap", Origin: a.Origin, Related: b.Origin, Warning: true, Message: "hardware regions overlap without establishing shared resource identity", Help: "Check the region extents; binding alias acknowledgements do not suppress region overlap warnings."})
			}
		}
	}
}
