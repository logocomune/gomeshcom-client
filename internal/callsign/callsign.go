// Package callsign provides callsign normalization and validation shared across
// the config loader and the runtime station identity component.
package callsign

import (
	"regexp"
	"strings"
)

// Pattern matches a valid MeshCom callsign: 3-10 alphanumeric characters with
// an optional numeric SSID suffix (e.g. IU5PMP, IU5PMP-1, IU5PMP-10).
var Pattern = regexp.MustCompile(`^[A-Z0-9]{3,10}(?:-[0-9]{1,2})?$`)

// Normalize returns value trimmed of whitespace and uppercased.
func Normalize(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// IsValid reports whether value is a valid callsign after normalization.
func IsValid(value string) bool {
	return Pattern.MatchString(Normalize(value))
}
