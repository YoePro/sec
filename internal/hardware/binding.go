package hardware

import "strings"

// LookupEndpoint resolves the endpoint path of an `@address` binding in the
// binding's resolver-owned scope, with the same module selection and
// underscore visibility as metadata references: `Name` in the scope's module,
// `qualifier.Name` through exactly one actual import candidate. A failure is
// returned as a "symbol" issue; it never falls back to a numeric address or
// a display-name search.
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§2,8(3),10(1).
func (f Facts) LookupEndpoint(scope Scope, path []string, origin Origin) (Endpoint, *Issue) {
	failure := func(related Origin, message string) (Endpoint, *Issue) {
		return Endpoint{}, &Issue{Code: "symbol", Origin: origin, Related: related, Message: message, Help: "Bind to a declared `hardware endpoint` of this module or an actual import qualifier."}
	}
	text := strings.Join(path, ".")
	module := scope.Module
	switch len(path) {
	case 1:
	case 2:
		candidates := scope.Imports[path[0]]
		if len(candidates) != 1 {
			return failure(Origin{}, "unknown or ambiguous imported hardware qualifier "+quote(path[0]))
		}
		module = candidates[0]
	default:
		return failure(Origin{}, "hardware endpoint path "+quote(text)+" must be Name or qualifier.Name")
	}
	name := path[len(path)-1]
	for _, endpoint := range f.Endpoints {
		if endpoint.Identity.Module != module || endpoint.Identity.Name != name {
			continue
		}
		if strings.HasPrefix(name, "__") && endpoint.Origin.Token.File != scope.File || strings.HasPrefix(name, "_") && module != scope.Module {
			return failure(endpoint.Origin, "inaccessible hardware endpoint "+quote(text))
		}
		return endpoint, nil
	}
	for _, region := range f.Regions {
		if region.Identity.Module == module && region.Identity.Name == name {
			return failure(region.Origin, "hardware symbol "+quote(text)+" is region, expected endpoint")
		}
	}
	for _, resource := range f.Resources {
		if resource.Identity.Module == module && resource.Identity.Name == name {
			return failure(resource.Origin, "hardware symbol "+quote(text)+" is resource, expected endpoint")
		}
	}
	return failure(Origin{}, "unknown hardware endpoint "+quote(text))
}

func quote(text string) string { return `"` + text + `"` }
