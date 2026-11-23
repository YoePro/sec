package sema

// substituteGenericMemberSurface applies the same substitution to associated
// properties/events and interface requirements, detaching slices so concrete
// instances never overwrite each other or their template.
// Rules: rules/declarations/generics.md — §§8,14,22,25;
// rules/declarations/interfaces.md — §§4–6,9.
func substituteGenericMemberSurface(out *Type, template Type, substitution map[string]Type) {
	out.Properties = append([]Property(nil), template.Properties...)
	for i := range out.Properties {
		property := &out.Properties[i]
		property.Type = substituteGenericType(property.Type, substitution)
		if property.Error != nil {
			errorType := substituteGenericType(*property.Error, substitution)
			property.Error = &errorType
		}
	}
	out.Events = append([]Event(nil), template.Events...)
	for i := range out.Events {
		out.Events[i].Type = substituteGenericType(out.Events[i].Type, substitution)
		out.Events[i].Payload = substituteGenericType(out.Events[i].Payload, substitution)
	}
	out.InterfaceProperties = append([]InterfaceProperty(nil), template.InterfaceProperties...)
	for i := range out.InterfaceProperties {
		out.InterfaceProperties[i].Type = substituteGenericType(out.InterfaceProperties[i].Type, substitution)
	}
	out.InterfaceEvents = append([]InterfaceEvent(nil), template.InterfaceEvents...)
	for i := range out.InterfaceEvents {
		out.InterfaceEvents[i].Payload = substituteGenericType(out.InterfaceEvents[i].Payload, substitution)
	}
}
