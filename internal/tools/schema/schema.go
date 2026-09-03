// Package schema builds the JSON Schema documents that describe a tool's
// arguments.
//
// Written as literal maps, a schema buries its descriptions -- the only
// instructions the model ever reads -- under punctuation. These helpers exist
// so the description sits next to the field it describes and stays legible
// enough to be edited when the model gets a call wrong.
//
// JSON Schema is what every provider we target accepts, so the output needs no
// translation for a local OpenAI-compatible endpoint or for Bedrock.
package schema

// Fields maps a property name to its schema.
type Fields map[string]any

// Object builds an object schema. Names listed in required are mandatory;
// pass none and the object is entirely optional.
func Object(description string, properties Fields, required ...string) map[string]any {
	node := map[string]any{
		"type":       "object",
		"properties": map[string]any(properties),
	}
	if description != "" {
		node["description"] = description
	}
	if len(required) > 0 {
		node["required"] = required
	}
	return node
}

// String builds a free-text field.
func String(description string) map[string]any {
	return scalar("string", description)
}

// Enum builds a string field closed over a fixed set of values.
//
// Prefer it to String wherever the vocabulary is closed. A value the model
// invents matches nothing in the database, and the schema is the cheapest
// place to prevent that -- cheaper than a round trip spent rejecting the call.
func Enum(description string, values ...string) map[string]any {
	node := scalar("string", description)
	node["enum"] = values
	return node
}

// Integer builds a whole-number field.
func Integer(description string) map[string]any {
	return scalar("integer", description)
}

// Number builds a field for a possibly fractional number.
func Number(description string) map[string]any {
	return scalar("number", description)
}

// Boolean builds a true/false field.
func Boolean(description string) map[string]any {
	return scalar("boolean", description)
}

// Array builds a list field over the given item schema.
func Array(description string, items map[string]any) map[string]any {
	node := map[string]any{"type": "array", "items": items}
	if description != "" {
		node["description"] = description
	}
	return node
}

func scalar(kind, description string) map[string]any {
	node := map[string]any{"type": kind}
	if description != "" {
		node["description"] = description
	}
	return node
}
