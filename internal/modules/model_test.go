package modules

import "testing"

func TestValidateImportPath(t *testing.T) {
	valid := []string{"orders", "domain/orders", "std/io", "platform/linux/amd64", "Mixed/Case"}
	for _, path := range valid {
		if err := ValidateImportPath(path); err != nil {
			t.Errorf("ValidateImportPath(%q): %v", path, err)
		}
	}
	invalid := []string{"", "/orders", "orders/", "domain//orders", ".", "./orders", "../orders", "domain/../orders", `domain\\orders`, "C:/orders"}
	for _, path := range invalid {
		if err := ValidateImportPath(path); err == nil {
			t.Errorf("ValidateImportPath(%q) unexpectedly succeeded", path)
		}
	}
}

func TestModuleIdentityIncludesImportRoot(t *testing.T) {
	first, err := NewModuleIdentity("project:first", "order")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewModuleIdentity("project:second", "order")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first.String() == second.String() {
		t.Fatalf("different roots produced equal identities: %v and %v", first, second)
	}
}
