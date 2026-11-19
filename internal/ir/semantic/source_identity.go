package semantic

import (
	"fmt"
	"reflect"
	"sync"
)

// CanonicalizeSourceFiles replaces every source path in a built module with
// its location-independent source identity: Module.SourceFiles and the File
// of every Location reachable from the module. Semantic IR then depends on
// where sources sit only through their root-relative identity, so a Project
// directory move or a different invocation path leaves the module, its
// printed and JSON forms and everything lowered from it unchanged. A path
// the mapping does not know is kept. Two different paths that map to one
// identity are rejected rather than merged. The result maps each identity
// back to the path it replaced, so tools can report the path the user wrote.
// Rules: rules/compiler/semantic_ir.md — § 5(1); rules/projects/projects.md — §§ 34, 50(1).
func CanonicalizeSourceFiles(module *Module, identity func(path string) (string, bool)) (map[string]string, error) {
	if module == nil || identity == nil {
		return map[string]string{}, nil
	}
	mapped := map[string]string{}
	owner := map[string]string{}
	resolve := func(path string) (string, error) {
		if path == "" {
			return path, nil
		}
		if canonical, done := mapped[path]; done {
			return canonical, nil
		}
		canonical, ok := identity(path)
		if !ok || canonical == "" {
			canonical = path
		}
		if previous, taken := owner[canonical]; taken && previous != path {
			return "", fmt.Errorf("source files %q and %q share the source identity %q", previous, path, canonical)
		}
		owner[canonical], mapped[path] = path, canonical
		return canonical, nil
	}
	files := make([]string, 0, len(module.SourceFiles))
	for _, path := range module.SourceFiles {
		canonical, err := resolve(path)
		if err != nil {
			return nil, err
		}
		files = append(files, canonical)
	}
	module.SourceFiles = uniqueSorted(files)
	if err := rewriteLocations(reflect.ValueOf(module).Elem(), resolve, map[uintptr]bool{}); err != nil {
		return nil, err
	}
	return owner, nil
}

var locationType = reflect.TypeOf(Location{})

// rewriteLocations walks the module value and rewrites each Location.File in
// place. Shared pointers are visited once.
func rewriteLocations(value reflect.Value, resolve func(string) (string, error), seen map[uintptr]bool) error {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		if seen[value.Pointer()] {
			return nil
		}
		seen[value.Pointer()] = true
		return rewriteLocations(value.Elem(), resolve, seen)
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		element := value.Elem()
		if element.Kind() == reflect.Pointer {
			return rewriteLocations(element, resolve, seen)
		}
		if containsLocation(element.Type()) {
			copied := reflect.New(element.Type()).Elem()
			copied.Set(element)
			if err := rewriteLocations(copied, resolve, seen); err != nil {
				return err
			}
			if value.CanSet() {
				value.Set(copied)
			}
		}
		return nil
	case reflect.Struct:
		if value.Type() == locationType {
			file := value.FieldByName("File")
			canonical, err := resolve(file.String())
			if err != nil {
				return err
			}
			if file.CanSet() {
				file.SetString(canonical)
			}
			return nil
		}
		for index := 0; index < value.NumField(); index++ {
			if !value.Type().Field(index).IsExported() {
				continue
			}
			if err := rewriteLocations(value.Field(index), resolve, seen); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if !containsLocation(value.Type().Elem()) {
			return nil
		}
		for index := 0; index < value.Len(); index++ {
			if err := rewriteLocations(value.Index(index), resolve, seen); err != nil {
				return err
			}
		}
	case reflect.Map:
		if !containsLocation(value.Type().Elem()) {
			return nil
		}
		for _, key := range value.MapKeys() {
			element := value.MapIndex(key)
			if element.Kind() == reflect.Pointer {
				if err := rewriteLocations(element, resolve, seen); err != nil {
					return err
				}
				continue
			}
			copied := reflect.New(element.Type()).Elem()
			copied.Set(element)
			if err := rewriteLocations(copied, resolve, seen); err != nil {
				return err
			}
			value.SetMapIndex(key, copied)
		}
	}
	return nil
}

// containsLocation reports whether values of typ can hold a Location, so the
// walk skips scalar and string data.
func containsLocation(typ reflect.Type) bool {
	if known, ok := locationTypes.Load(typ); ok {
		return known.(bool)
	}
	result := containsLocationType(typ, map[reflect.Type]bool{})
	locationTypes.Store(typ, result)
	return result
}

var locationTypes sync.Map

func containsLocationType(typ reflect.Type, visiting map[reflect.Type]bool) bool {
	if typ == locationType {
		return true
	}
	if visiting[typ] {
		return false
	}
	visiting[typ] = true
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return containsLocationType(typ.Elem(), visiting)
	case reflect.Interface:
		return true
	case reflect.Struct:
		for index := 0; index < typ.NumField(); index++ {
			if typ.Field(index).IsExported() && containsLocationType(typ.Field(index).Type, visiting) {
				return true
			}
		}
	}
	return false
}
