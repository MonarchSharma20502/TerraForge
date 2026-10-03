package spec

// Schema is the JSON Schema for the Auto-nation v1 DSL.
//
// It is generated from the Go types in this package so the schema and the parser
// cannot drift. The UI inspector form is generated from this schema, which keeps
// the catalog as data rather than hand-built per resource kind.

// SchemaJSON returns the JSON Schema for the DSL as a Go map.
//
// The schema describes globals plus one nested object per resource family. It is
// deliberately permissive on the family objects: the catalog is the authority on
// per-kind properties, and the schema only constrains the document shape.
func SchemaJSON() map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://autonation.dev/schemas/v1/spec.json",
		"title":                "Auto-nation architecture spec",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"apiVersion", "metadata"},
		"properties": map[string]any{
			"apiVersion": map[string]any{
				"type":  "string",
				"const": "autonation/v1",
			},
			"metadata": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"businessUnit", "platform", "environment", "location"},
				"properties": map[string]any{
					"businessUnit":   map[string]any{"type": "string"},
					"platform":       map[string]any{"type": "string"},
					"environment":    map[string]any{"type": "string"},
					"location":       map[string]any{"type": "string"},
					"subscriptionId": map[string]any{"type": "string"},
					"owner":          map[string]any{"type": "string"},
					"tags": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{"type": "string"},
					},
				},
			},
			"resources": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
	}
}
