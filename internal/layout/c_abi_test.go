package layout

import "testing"

// Every registered C ABI model is coherent and resolves every C::
// fundamental; data model, char signedness, and long double format come from
// the model rather than from the Sec spelling.
//
// Rules:
//   - rules/platform/abi.md — § 19 "C scalar representation"
//   - rules/platform/ffi.md — §5 "Fundamental C ABI scalar family"
func TestCABIModelsResolveEveryFundamental(t *testing.T) {
	models := []CABIModel{CModelSysVX8664, CModelAAPCS64, CModelDarwinX8664, CModelDarwinARM64, CModelWindowsX64, CModelWindowsARM64, CModelAAPCSLinuxHF, CModelAAPCSBareMetal, CModelRISCVILP32, CModelRISCVLP64D, CModelAAPCSFreeBSDHF}
	for _, model := range models {
		if err := model.Validate(); err != nil {
			t.Fatalf("%s: %v", model.Name, err)
		}
		for _, name := range CFundamentalTypeNames() {
			scalar, ok := model.Fundamental(name)
			if !ok || scalar.Name != "C::"+name || scalar.StorageBits < scalar.ValueBits && scalar.Kind != CScalarBool {
				t.Fatalf("%s %s = %+v, %v", model.Name, name, scalar, ok)
			}
		}
		if _, ok := model.Fundamental("size_t"); ok {
			t.Fatalf("%s resolved size_t as a fundamental C:: type", model.Name)
		}
	}
	checks := []struct {
		model CABIModel
		name  string
		kind  CScalarKind
		bits  uint16
	}{
		{CModelSysVX8664, "long", CScalarSigned, 64},
		{CModelWindowsX64, "long", CScalarSigned, 32},
		{CModelAAPCSLinuxHF, "ulong", CScalarUnsigned, 32},
		{CModelSysVX8664, "char", CScalarSigned, 8},
		{CModelAAPCS64, "char", CScalarUnsigned, 8},
		{CModelDarwinARM64, "char", CScalarSigned, 8},
		{CModelSysVX8664, "long_double", CScalarFloat, 80},
		{CModelAAPCS64, "long_double", CScalarFloat, 128},
		{CModelWindowsX64, "long_double", CScalarFloat, 64},
		{CModelRISCVILP32, "long_long", CScalarSigned, 64},
		{CModelRISCVLP64D, "long", CScalarSigned, 64},
		{CModelRISCVLP64D, "char", CScalarUnsigned, 8},
		{CModelRISCVLP64D, "long_double", CScalarFloat, 128},
		{CModelAAPCSFreeBSDHF, "long", CScalarSigned, 32},
		{CModelAAPCSFreeBSDHF, "char", CScalarUnsigned, 8},
	}
	for _, check := range checks {
		scalar, _ := check.model.Fundamental(check.name)
		if scalar.Kind != check.kind || scalar.ValueBits != check.bits {
			t.Fatalf("%s C::%s = %+v, want %s %d bits", check.model.Name, check.name, scalar, check.kind, check.bits)
		}
	}
	if x87, _ := CModelSysVX8664.Fundamental("long_double"); x87.StorageBits != 128 {
		t.Fatalf("x87 long double storage = %d, want 128", x87.StorageBits)
	}
}

func TestScalarPlanRejectsIncoherentCABIModel(t *testing.T) {
	plan := ResolvedScalarPlan{TargetOS: "linux", TargetArch: "amd64", PointerWidthBits: 32, Endianness: LittleEndian, CABI: CModelSysVX8664}
	if err := plan.Validate(); err == nil {
		t.Fatal("plan accepted a 64-bit C ABI model on a 32-bit target")
	}
	broken := CModelSysVX8664
	broken.IntBits = 8
	plan = ResolvedScalarPlan{TargetOS: "linux", TargetArch: "amd64", PointerWidthBits: 64, Endianness: LittleEndian, CABI: broken}
	if err := plan.Validate(); err == nil {
		t.Fatal("plan accepted int narrower than short")
	}
	plan.CABI = CABIModel{}
	if err := plan.Validate(); err != nil {
		t.Fatalf("a target without a C ABI model must stay valid: %v", err)
	}
}
