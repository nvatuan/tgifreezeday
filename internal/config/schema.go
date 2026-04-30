package config

import _ "embed"

//go:embed schema-v1.yaml
var schemaV1YAML []byte

const CurrentSchemaVersion = "v1"

// SchemaYAML returns the embedded schema definition for the given version.
func SchemaYAML(version string) ([]byte, bool) {
	switch version {
	case "v1":
		return schemaV1YAML, true
	default:
		return nil, false
	}
}
