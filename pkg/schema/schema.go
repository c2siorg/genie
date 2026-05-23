// Package schema is a minimal in-process JSON Schema validator + the
// SchemaPolicy that uses it.
//
// We support the small subset Genie needs: type, required, properties,
// enum, items, minLength, maxLength, minimum, maximum, additionalProperties.
// This is enough to validate every agent's structured output without pulling
// a multi-MB validation library.
package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Schema is the JSON-Schema subset Genie understands.
type Schema struct {
	Type                 string             `json:"type,omitempty"`     // object|array|string|number|integer|boolean
	Required             []string           `json:"required,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`
}

// Parse decodes a JSON-Schema document.
func Parse(body []byte) (*Schema, error) {
	var s Schema
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("schema parse: %w", err)
	}
	return &s, nil
}

// ValidateJSON parses payload as JSON and validates it.
func (s *Schema) ValidateJSON(payload []byte) error {
	var v any
	if err := json.Unmarshal(payload, &v); err != nil {
		return fmt.Errorf("payload is not valid json: %w", err)
	}
	return s.Validate(v)
}

// Validate runs the schema against a decoded value.
func (s *Schema) Validate(v any) error {
	return validate(s, v, "")
}

func validate(s *Schema, v any, path string) error {
	switch s.Type {
	case "", "any":
		// no-op
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return mkErr(path, "expected object")
		}
		for _, r := range s.Required {
			if _, has := obj[r]; !has {
				return mkErr(path+"/"+r, "missing required property")
			}
		}
		for k, val := range obj {
			sub, ok := s.Properties[k]
			if !ok {
				if s.AdditionalProperties != nil && !*s.AdditionalProperties {
					return mkErr(path+"/"+k, "unknown property")
				}
				continue
			}
			if err := validate(sub, val, path+"/"+k); err != nil {
				return err
			}
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return mkErr(path, "expected array")
		}
		if s.Items != nil {
			for i, it := range arr {
				if err := validate(s.Items, it, fmt.Sprintf("%s/%d", path, i)); err != nil {
					return err
				}
			}
		}
	case "string":
		str, ok := v.(string)
		if !ok {
			return mkErr(path, "expected string")
		}
		if s.MinLength != nil && len(str) < *s.MinLength {
			return mkErr(path, fmt.Sprintf("min length %d", *s.MinLength))
		}
		if s.MaxLength != nil && len(str) > *s.MaxLength {
			return mkErr(path, fmt.Sprintf("max length %d", *s.MaxLength))
		}
		if len(s.Enum) > 0 && !enumContains(s.Enum, str) {
			return mkErr(path, "not in enum")
		}
	case "number", "integer":
		f, ok := toFloat(v)
		if !ok {
			return mkErr(path, "expected number")
		}
		if s.Type == "integer" && f != float64(int64(f)) {
			return mkErr(path, "expected integer")
		}
		if s.Minimum != nil && f < *s.Minimum {
			return mkErr(path, "below minimum")
		}
		if s.Maximum != nil && f > *s.Maximum {
			return mkErr(path, "above maximum")
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return mkErr(path, "expected boolean")
		}
	default:
		return mkErr(path, "unknown type "+s.Type)
	}
	return nil
}

func mkErr(path, msg string) error {
	if path == "" {
		return errors.New(msg)
	}
	return fmt.Errorf("%s: %s", strings.TrimPrefix(path, "/"), msg)
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

func enumContains(list []any, want any) bool {
	for _, x := range list {
		if x == want {
			return true
		}
	}
	return false
}
