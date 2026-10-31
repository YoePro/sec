package semantic

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/hardware"
	"sec/internal/sema"
)

// TypeHardwareInstance is HardwareInstance[T]: the software implementation
// instance/authority of one hardware register binding over the nominal
// register type Element (MD-077 §§ 3(4)–(5), 19(2)). It is never a Place and
// never interchangeable with a detached register value.
const TypeHardwareInstance TypeKind = "hardware-instance"

// HardwareBindingID is the dense module-scoped identity of a binding record.
type HardwareBindingID uint32

// HardwareBinding is one `register.hw.bind` root: the association of a
// module-level hardware register binding with its canonical endpoint,
// resource and region and the endpoint's domain extent and transfer facts.
// It establishes authority only; it reads, writes and initializes nothing
// (MD-077 §§ 9(1), 12(2), 19(5)). Identities are the resolver-owned
// `root:path::name` spellings of MD-111 § 2.
type HardwareBinding struct {
	ID           HardwareBindingID
	Symbol       string
	Register     TypeID
	Instance     TypeID
	Mutable      bool
	Alias        bool
	Endpoint     string
	Resource     string
	Region       string
	EndpointKind string
	Domain       string
	Address      *big.Int
	Size         *big.Int
	Alignment    *big.Int
	ReadWidths   []*big.Int
	WriteWidths  []*big.Int
	Location     Location
}

func hardwareIdentity(identity hardware.Identity) string {
	return identity.Module.String() + "::" + identity.Name
}

// buildHardwareBindings records the `register.hw.bind` roots of the module's
// own `@address` register bindings from Sema's resolved binding facts, in
// source order. The builder never reprobes the attribute or derives an
// address from source text (MD-077 § 11(2)).
func (b *builder) buildHardwareBindings(lets []*ast.LetStatement) error {
	resolved := map[string]sema.ResolvedHardwareBinding{}
	for _, binding := range b.analyzer.ResolvedHardwareBindings() {
		resolved[fmt.Sprintf("%s:%d", binding.Token.File, binding.Token.ByteStart)] = binding
	}
	for _, let := range lets {
		if let.Address == nil {
			continue
		}
		binding, ok := resolved[fmt.Sprintf("%s:%d", let.Name.Token.File, let.Name.Token.ByteStart)]
		if !ok {
			return &UnsupportedFeatureError{Feature: "hardware register binding without a resolved endpoint", Package: b.maxPackage, Location: location(let.Name.Token)}
		}
		register, err := b.internRegisterType(binding.Register)
		if err != nil {
			return err
		}
		endpoint := binding.Endpoint
		properties := endpoint.Properties
		record := HardwareBinding{ID: HardwareBindingID(len(b.module.HardwareBindings) + 1), Symbol: b.module.Identity + "::" + binding.Name,
			Register: register, Instance: b.module.Types.Intern(Type{Kind: TypeHardwareInstance, Element: register}),
			Mutable: binding.Mutable, Alias: binding.Alias,
			Endpoint: hardwareIdentity(endpoint.Identity), Resource: hardwareIdentity(hardware.Identity(endpoint.Resource)), Region: hardwareIdentity(hardware.Identity(endpoint.Region)),
			EndpointKind: properties["kind"].Text, Domain: string(endpoint.Domain.Identity),
			Address: new(big.Int).Set(endpoint.Address), Size: new(big.Int).Set(endpoint.Size), Location: location(binding.Token)}
		if alignment := properties["alignment"].Integer; alignment != nil {
			record.Alignment = new(big.Int).Set(alignment)
		}
		for _, width := range properties["read_widths"].Widths {
			record.ReadWidths = append(record.ReadWidths, new(big.Int).Set(width))
		}
		for _, width := range properties["write_widths"].Widths {
			record.WriteWidths = append(record.WriteWidths, new(big.Int).Set(width))
		}
		b.module.HardwareBindings = append(b.module.HardwareBindings, record)
	}
	return nil
}

func formatHardwareBindings(out *strings.Builder, bindings []HardwareBinding) {
	if len(bindings) == 0 {
		return
	}
	out.WriteString("\n  hardware {\n")
	for _, binding := range bindings {
		fmt.Fprintf(out, "    #%d %q = register.hw.bind !%d : !%d mutable=%t", binding.ID, binding.Symbol, binding.Register, binding.Instance, binding.Mutable)
		if binding.Alias {
			out.WriteString(" alias")
		}
		fmt.Fprintf(out, " endpoint=%q resource=%q region=%q kind=%s domain=%q address=0x%s size=%s", binding.Endpoint, binding.Resource, binding.Region, binding.EndpointKind, binding.Domain, binding.Address.Text(16), binding.Size.String())
		if binding.Alignment != nil {
			fmt.Fprintf(out, " alignment=%s", binding.Alignment.String())
		}
		fmt.Fprintf(out, " reads=%s writes=%s %s\n", formatWidths(binding.ReadWidths), formatWidths(binding.WriteWidths), formatLocation(binding.Location))
	}
	out.WriteString("  }\n")
}

func formatWidths(widths []*big.Int) string {
	parts := make([]string, len(widths))
	for index, width := range widths {
		parts[index] = width.String()
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// verifyHardwareBindings checks each `register.hw.bind` root: dense IDs,
// unique symbols, a register-definition-backed nominal register type, the
// hardware-instance type over exactly that register, resolved identities, a
// MemoryMapped endpoint with a positive extent and transfer widths that fit
// it. It verifies preservation of Sema's facts, not a second hardware planner
// (MD-077 § 11(3)).
func verifyHardwareBindings(module *Module) error {
	symbols := map[string]bool{}
	for index, binding := range module.HardwareBindings {
		if binding.ID != HardwareBindingID(index+1) {
			return fmt.Errorf("hardware binding #%d is out of identity order", binding.ID)
		}
		if binding.Symbol == "" || symbols[binding.Symbol] {
			return fmt.Errorf("hardware binding #%d has an empty or duplicate symbol %q", binding.ID, binding.Symbol)
		}
		symbols[binding.Symbol] = true
		if _, ok := registerDefinition(module, binding.Register); !ok {
			return fmt.Errorf("hardware binding #%d names !%d without a register definition", binding.ID, binding.Register)
		}
		instance, ok := module.Types.Lookup(binding.Instance)
		if !ok || instance.Kind != TypeHardwareInstance || instance.Element != binding.Register {
			return fmt.Errorf("hardware binding #%d instance type is not hardware-instance<!%d>", binding.ID, binding.Register)
		}
		if binding.Endpoint == "" || binding.Resource == "" || binding.Region == "" || binding.Domain == "" {
			return fmt.Errorf("hardware binding #%d lacks a resolved endpoint, resource, region or domain identity", binding.ID)
		}
		if binding.EndpointKind != "MemoryMapped" {
			return fmt.Errorf("hardware binding #%d endpoint kind %q has no admitted binding schema", binding.ID, binding.EndpointKind)
		}
		if binding.Address == nil || binding.Address.Sign() < 0 || binding.Size == nil || binding.Size.Sign() <= 0 {
			return fmt.Errorf("hardware binding #%d has an invalid endpoint extent", binding.ID)
		}
		bits := new(big.Int).Lsh(binding.Size, 3)
		for _, width := range append(append([]*big.Int(nil), binding.ReadWidths...), binding.WriteWidths...) {
			if width == nil || width.Sign() <= 0 || width.Cmp(bits) > 0 {
				return fmt.Errorf("hardware binding #%d transfer width exceeds its endpoint extent", binding.ID)
			}
		}
	}
	return nil
}

// hardwareBindingRoot reports the register.hw.bind root an identifier names.
func (fb *functionBuilder) hardwareBindingRoot(expression ast.Expression) (HardwareBinding, bool) {
	identifier, ok := expression.(*ast.Identifier)
	if !ok {
		return HardwareBinding{}, false
	}
	for _, binding := range fb.owner.module.HardwareBindings {
		if binding.Symbol == fb.owner.module.Identity+"::"+identifier.Value {
			return binding, true
		}
	}
	return HardwareBinding{}, false
}
