package app

import (
	"reflect"
	"strings"
)

// Future consent is explicit and persists separately from the saved snapshot.
// Resolve at every authorization/read boundary so newly registered grants follow
// this policy without rewriting existing tokens or pending proposal effects.
func resolveMCPPermissions(p mcpPermissions) mcpPermissions {
	if p.Preset != "yolo" || !p.IncludeFuturePermissions {
		return p
	}
	p.Capabilities = allMCPCapabilities()
	p.Automatic = p.Capabilities
	p.Constraints = mcpConstraints{}
	value, kind := reflect.ValueOf(&p).Elem(), reflect.TypeOf(p)
	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).Kind() == reflect.Bool && strings.HasPrefix(kind.Field(i).Tag.Get("json"), "read_") {
			value.Field(i).SetBool(true)
		}
	}
	return p
}
