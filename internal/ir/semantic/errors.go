package semantic

import "fmt"

type UnsupportedFeatureError struct {
	Feature  string
	Package  uint8
	Location Location
}

func (e *UnsupportedFeatureError) Error() string {
	pkg := e.Package
	if pkg == 0 {
		pkg = 3
	}
	where := ""
	if e.Location.Line > 0 {
		where = fmt.Sprintf(" at %s:%d:%d", e.Location.File, e.Location.Line, e.Location.Column)
	}
	return fmt.Sprintf("semantic IR feature not implemented in package %d: %s%s", pkg, e.Feature, where)
}
