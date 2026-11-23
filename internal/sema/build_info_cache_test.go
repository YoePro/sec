package sema

import (
	"reflect"
	"testing"
)

// Every observable field, optional presence and exact integer value is part of
// reuse validity; pointer identity and field-separator spelling are not.
// Rules: rules/compiler/compiler_known_members.md — §113B(4);
// rules/projects/projects.md — §§33–34.
func TestBuildInfoCacheIdentity(t *testing.T) {
	base := testBuildInfo
	baseKey := base.CacheIdentity()
	fields := reflect.TypeOf(base)
	for i := 0; i < fields.NumField(); i++ {
		t.Run(fields.Field(i).Name, func(t *testing.T) {
			changed := base
			field := reflect.ValueOf(&changed).Elem().Field(i)
			switch field.Kind() {
			case reflect.String:
				field.SetString(field.String() + "changed")
			case reflect.Uint64:
				field.SetUint(^uint64(0))
			case reflect.Pointer:
				field.Set(reflect.New(field.Type().Elem()))
			default:
				t.Fatalf("new snapshot field needs an identity test: %s", field.Type())
			}
			if changed.CacheIdentity() == baseKey {
				t.Fatal("observable change reused identity")
			}
			if cloneBuildInfoSnapshot(changed).CacheIdentity() != changed.CacheIdentity() {
				t.Fatal("identity depends on pointer addresses")
			}
		})
	}
	empty, flag := "", false
	present := base
	present.SourceRevision, present.DirtySourceStatus = &empty, &flag
	key := present.CacheIdentity()
	flag = true
	if present.CacheIdentity() == key {
		t.Fatal("dirty state missing from identity")
	}
	left, right := base, base
	left.ProjectName, left.TargetName = "a:b", "c"
	right.ProjectName, right.TargetName = "a", "b:c"
	if left.CacheIdentity() == right.CacheIdentity() {
		t.Fatal("field boundaries collide")
	}
}
