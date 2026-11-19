package main

import (
	"os"
	"sort"

	"sec/internal/ir/semantic"
	mlirtoolchain "sec/internal/mlir"
)

// importedModuleObjects compiles each module that the entry module's Semantic
// IR declares imported functions from, transitively, into its own object:
// its own Semantic IR, Sec MLIR with module-independent symbols, LLVM IR and
// object code. Modules are visited in canonical identity order, so the
// object list does not depend on import or file order. The returned cleanup
// removes the temporary files.
// Rules: rules/compiler/semantic_ir.md — § 5(1); rules/compiler/linking.md — §§ 9, 15, 18–19.
func importedModuleObjects(analyzed analyzedProgram, inputFile string, target CompilerTarget, entry *semantic.Module, mlirBin string, triple string) ([]string, func()) {
	temporary := []string{}
	cleanup := func() {
		for _, path := range temporary {
			_ = os.Remove(path)
		}
	}
	built := map[string]bool{entry.Identity: true}
	pending := importedModulesOf(entry, built)
	objects := []string{}
	for len(pending) > 0 {
		sort.Strings(pending)
		name := pending[0]
		pending = pending[1:]
		if built[name] {
			continue
		}
		built[name] = true
		secMLIR, module := emitSecDialectUnit(analyzed, inputFile, target, nativeTrackMaxPackage, name, true)
		pending = append(pending, importedModulesOf(module, built)...)
		llvmDialectPath, removeDialect := lowerToLLVMDialect(secMLIR, "", mlirBin, "")
		llvmPath, removeLLVM, err := createTempOutputPath(".ll")
		if err != nil {
			removeDialect()
			cleanup()
			reportToolError("temp file", "%v", err)
			exitCLI(1)
		}
		if removeLLVM {
			temporary = append(temporary, llvmPath)
		}
		translateToLLVMIR(llvmDialectPath, llvmPath, mlirBin)
		removeDialect()
		objectPath, removeObject, err := createTempOutputPath(".o")
		if err != nil {
			cleanup()
			reportToolError("temp file", "%v", err)
			exitCLI(1)
		}
		if removeObject {
			temporary = append(temporary, objectPath)
		}
		if err := mlirtoolchain.NewToolchain(mlirBin).CompileObject(llvmPath, objectPath, triple); err != nil {
			cleanup()
			reportPipelineError("object code generation", err)
			exitCLI(4)
		}
		objects = append(objects, objectPath)
	}
	return objects, cleanup
}

// importedModulesOf lists the defining modules of a module's imported
// declarations that are not built yet.
func importedModulesOf(module *semantic.Module, built map[string]bool) []string {
	seen := map[string]bool{}
	modules := []string{}
	for _, function := range module.Functions {
		if function.Imported && !built[function.DefiningModule] && !seen[function.DefiningModule] {
			seen[function.DefiningModule] = true
			modules = append(modules, function.DefiningModule)
		}
	}
	sort.Strings(modules)
	return modules
}
