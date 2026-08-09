package layout

import "testing"

func TestResolvedScalarPlanValidation(t *testing.T) {
	valid := ResolvedScalarPlan{TargetOS: "linux", TargetArch: "amd64", PointerWidthBits: 64, Endianness: LittleEndian}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []ResolvedScalarPlan{
		{},
		{TargetOS: "linux", TargetArch: "any", PointerWidthBits: 0, Endianness: LittleEndian},
		{TargetOS: "linux", TargetArch: "amd64", PointerWidthBits: 64, Endianness: "middle"},
	} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("accepted invalid scalar plan %#v", invalid)
		}
	}
}
