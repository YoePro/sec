package modules

import (
	"reflect"
	"testing"
)

func TestModuleInstanceGraphsSeparateVariantSelections(t *testing.T) {
	identity := testIdentity("platform")
	linux, err := NewModuleInstanceGraph("app/linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	firmware, err := NewModuleInstanceGraph("app/firmware-arm64")
	if err != nil {
		t.Fatal(err)
	}
	if err := linux.AddModule(identity, []string{"platform/common.sec", "platform/linux.sec"}); err != nil {
		t.Fatal(err)
	}
	if err := firmware.AddModule(identity, []string{"platform/arm64.sec", "platform/common.sec"}); err != nil {
		t.Fatal(err)
	}

	linuxInstance, ok := linux.Instance(identity)
	if !ok {
		t.Fatal("linux module instance is missing")
	}
	firmwareInstance, ok := firmware.Instance(identity)
	if !ok {
		t.Fatal("firmware module instance is missing")
	}
	if linuxInstance.Identity != firmwareInstance.Identity {
		t.Fatalf("logical identity changed across variants: %v and %v", linuxInstance, firmwareInstance)
	}
	if linuxInstance == firmwareInstance || linuxInstance.CompilationPlanID == firmwareInstance.CompilationPlanID {
		t.Fatalf("plan-scoped instances were conflated: %v and %v", linuxInstance, firmwareInstance)
	}
	if got, want := linux.DependencyFacts().Modules[0].SelectedSources, []string{"platform/common.sec", "platform/linux.sec"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("linux sources = %v, want %v", got, want)
	}
	if got, want := firmware.DependencyFacts().Modules[0].SelectedSources, []string{"platform/arm64.sec", "platform/common.sec"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("firmware sources = %v, want %v", got, want)
	}
}

func TestModuleInstanceGraphRejectsUnselectedImportDestination(t *testing.T) {
	graph, err := NewModuleInstanceGraph("app/test")
	if err != nil {
		t.Fatal(err)
	}
	main := testIdentity("main")
	dependency := testIdentity("dependency")
	if err := graph.AddModule(main, []string{"main.sec"}); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddImport(ImportEdge{From: main, To: dependency, SourceFile: "main.sec", Line: 3, Column: 1}); err == nil {
		t.Fatal("import to an unselected plan destination unexpectedly succeeded")
	}
}

func TestModuleDependencyFactsRetainImportsWithoutClaimingInitializationOrder(t *testing.T) {
	graph, err := NewModuleInstanceGraph("app/test")
	if err != nil {
		t.Fatal(err)
	}
	importer := testIdentity("a-importer")
	dependency := testIdentity("z-dependency")
	if err := graph.AddModule(importer, []string{"a/main.sec", "a/helpers.sec", "a/main.sec"}); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddModule(dependency, []string{"z/dependency.sec"}); err != nil {
		t.Fatal(err)
	}
	edge := ImportEdge{From: importer, To: dependency, SourceFile: "a/main.sec", Line: 4, Column: 2}
	if err := graph.AddImport(edge); err != nil {
		t.Fatal(err)
	}

	facts := graph.DependencyFacts()
	if got, want := facts.CompilationPlanID, "app/test"; got != want {
		t.Fatalf("plan = %q, want %q", got, want)
	}
	if got, want := []ModuleIdentity{facts.Modules[0].Instance.Identity, facts.Modules[1].Instance.Identity}, []ModuleIdentity{importer, dependency}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fact modules = %v, want canonical identity order %v", got, want)
	}
	order, cycles := graph.TopologicalOrder()
	if len(cycles) != 0 {
		t.Fatalf("unexpected cycles: %v", cycles)
	}
	if got, want := []ModuleIdentity{order[0].Identity, order[1].Identity}, []ModuleIdentity{dependency, importer}; !reflect.DeepEqual(got, want) {
		t.Fatalf("semantic dependency order = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(facts.Imports, []ImportEdge{edge}) {
		t.Fatalf("import facts = %#v, want %#v", facts.Imports, []ImportEdge{edge})
	}
	if got, want := facts.Modules[0].SelectedSources, []string{"a/helpers.sec", "a/main.sec"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected sources = %v, want %v", got, want)
	}

	facts.Modules[0].SelectedSources[0] = "changed.sec"
	facts.Imports[0].SourceFile = "changed.sec"
	again := graph.DependencyFacts()
	if again.Modules[0].SelectedSources[0] != "a/helpers.sec" || again.Imports[0].SourceFile != "a/main.sec" {
		t.Fatal("dependency facts alias mutable graph storage")
	}
}

func TestModuleInstanceGraphRejectsPlanOrSelectionMutation(t *testing.T) {
	if _, err := NewModuleInstanceGraph(""); err == nil {
		t.Fatal("empty compilation-plan identity unexpectedly succeeded")
	}
	graph, err := NewModuleInstanceGraph("app/test")
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity("main")
	if err := graph.AddModule(identity, []string{"b.sec", "a.sec"}); err != nil {
		t.Fatal(err)
	}
	if err := graph.AddModule(identity, []string{"a.sec", "b.sec"}); err != nil {
		t.Fatalf("equivalent source selection was rejected: %v", err)
	}
	if err := graph.AddModule(identity, []string{"different.sec"}); err == nil {
		t.Fatal("in-place source selection change unexpectedly succeeded")
	}
}
