// Package validator implements the PhaseValidate pipeline stage. It performs
// structural validation (per-object schema checks) and semantic validation
// (cross-object relationship checks) on a SourceTree. The validator is
// non-mutating: it passes the SourceTree through unchanged and reports all
// violations as diagnostics.
package validator

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

//go:embed schemas/*.json
var schemaFS embed.FS

// kindSchema is a lightweight schema definition for validating RawFields.
type kindSchema struct {
	Kind        string                    `json:"kind"`
	Properties  map[string]propertySchema `json:"properties"`
	Required    []string                  `json:"required"`
	Description string                    `json:"description"`
}

// propertySchema defines validation rules for a single field.
type propertySchema struct {
	Type        string                    `json:"type"`
	Enum        []string                  `json:"enum,omitempty"`
	Required    []string                  `json:"required,omitempty"`
	Properties  map[string]propertySchema `json:"properties,omitempty"`
	Items       *propertySchema           `json:"items,omitempty"`
	Description string                    `json:"description,omitempty"`
}

// validKinds is the set of valid Kind values.
var validKinds = map[model.Kind]bool{
	model.KindInstruction: true,
	model.KindRule:        true,
	model.KindSkill:       true,
	model.KindAgent:       true,
	model.KindHook:        true,
	model.KindCommand:     true,
	model.KindCapability:  true,
	model.KindPlugin:      true,
}

// validPreservations is the set of valid Preservation values.
var validPreservations = map[model.Preservation]bool{
	model.PreservationRequired:  true,
	model.PreservationPreferred: true,
	model.PreservationOptional:  true,
}

// validTargets builds a lookup from build.AllTargets plus wildcard.
var validTargets map[string]bool

func init() {
	validTargets = make(map[string]bool, len(build.AllTargets())+1)
	for _, t := range build.AllTargets() {
		validTargets[string(t)] = true
	}
	validTargets["*"] = true
}

// StructuralValidator validates individual RawObjects against their
// kind-specific schemas and common ObjectMeta rules.
type StructuralValidator struct {
	schemas map[model.Kind]kindSchema
}

// NewStructuralValidator loads embedded schemas and returns a validator.
func NewStructuralValidator() (*StructuralValidator, error) {
	schemas, err := loadSchemas()
	if err != nil {
		return nil, fmt.Errorf("load schemas: %w", err)
	}
	return &StructuralValidator{schemas: schemas}, nil
}

// loadSchemas reads and parses all embedded JSON schemas.
func loadSchemas() (map[model.Kind]kindSchema, error) {
	schemaFiles := map[model.Kind]string{
		model.KindInstruction: "schemas/instruction.schema.json",
		model.KindRule:        "schemas/rule.schema.json",
		model.KindSkill:       "schemas/skill.schema.json",
		model.KindAgent:       "schemas/agent.schema.json",
		model.KindHook:        "schemas/hook.schema.json",
		model.KindCommand:     "schemas/command.schema.json",
		model.KindCapability:  "schemas/capability.schema.json",
		model.KindPlugin:      "schemas/plugin.schema.json",
	}

	result := make(map[model.Kind]kindSchema, len(schemaFiles))
	for kind, path := range schemaFiles {
		data, err := schemaFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read schema %s: %w", path, err)
		}
		var s kindSchema
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parse schema %s: %w", path, err)
		}
		result[kind] = s
	}
	return result, nil
}

// Validate checks a single RawObject for structural correctness.
// Returns diagnostics for any violations found.
func (v *StructuralValidator) Validate(obj pipeline.RawObject) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic

	// Meta.ID is required.
	if obj.Meta.ID == "" {
		diags = append(diags, pipeline.Diagnostic{
			Severity:   "error",
			Code:       string(pipeline.ErrValidation),
			Message:    "object id is required",
			SourcePath: obj.SourcePath,
			Phase:      pipeline.PhaseValidate,
		})
	}

	// Meta.Kind must be a known kind.
	if !validKinds[obj.Meta.Kind] {
		diags = append(diags, pipeline.Diagnostic{
			Severity:   "error",
			Code:       string(pipeline.ErrValidation),
			Message:    fmt.Sprintf("unknown kind %q", obj.Meta.Kind),
			SourcePath: obj.SourcePath,
			ObjectID:   obj.Meta.ID,
			Phase:      pipeline.PhaseValidate,
		})
		// Cannot validate RawFields without a known kind.
		return diags
	}

	// Preservation must be valid if set.
	if obj.Meta.Preservation != "" && !validPreservations[obj.Meta.Preservation] {
		diags = append(diags, pipeline.Diagnostic{
			Severity:   "error",
			Code:       string(pipeline.ErrValidation),
			Message:    fmt.Sprintf("invalid preservation value %q; expected one of: required, preferred, optional", obj.Meta.Preservation),
			SourcePath: obj.SourcePath,
			ObjectID:   obj.Meta.ID,
			Phase:      pipeline.PhaseValidate,
		})
	}

	// AppliesTo.Targets must be valid targets.
	for _, t := range obj.Meta.AppliesTo.Targets {
		if !validTargets[t] {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("invalid target %q in appliesTo.targets; valid targets: %s", t, validTargetList()),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}

	// TargetOverride keys must be valid targets.
	for key := range obj.Meta.TargetOverrides {
		if !validTargets[key] {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("invalid target %q in targetOverrides; valid targets: %s", key, validTargetList()),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}

	// Validate RawFields against kind-specific schema.
	schema, ok := v.schemas[obj.Meta.Kind]
	if ok {
		diags = append(diags, v.validateFields(obj, schema)...)
	}

	return diags
}

// validateFields checks RawFields against the kind-specific schema.
func (v *StructuralValidator) validateFields(obj pipeline.RawObject, schema kindSchema) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic

	// Check required fields are present in RawFields.
	for _, req := range schema.Required {
		if _, exists := obj.RawFields[req]; !exists {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("required field %q is missing for kind %q", req, obj.Meta.Kind),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}
	}

	// Type-check fields that are present.
	for fieldName, fieldVal := range obj.RawFields {
		propSchema, known := schema.Properties[fieldName]
		if !known {
			// Unknown fields are allowed (forward compatibility).
			continue
		}
		diags = append(diags, v.validateFieldType(obj, fieldName, fieldVal, propSchema)...)
	}

	return diags
}

// validateFieldType checks that a field value matches the expected type.
func (v *StructuralValidator) validateFieldType(
	obj pipeline.RawObject,
	fieldName string,
	value any,
	schema propertySchema,
) []pipeline.Diagnostic {
	var diags []pipeline.Diagnostic

	if value == nil {
		return diags
	}

	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("field %q must be a string, got %T", fieldName, value),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		} else if len(schema.Enum) > 0 {
			diags = append(diags, v.validateEnum(obj, fieldName, value.(string), schema.Enum)...)
		}

	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32:
			// Valid numeric types.
		default:
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("field %q must be a number, got %T", fieldName, value),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("field %q must be a boolean, got %T", fieldName, value),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}

	case "array":
		if _, ok := value.([]any); !ok {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("field %q must be an array, got %T", fieldName, value),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		}

	case "object":
		m, ok := value.(map[string]any)
		if !ok {
			diags = append(diags, pipeline.Diagnostic{
				Severity:   "error",
				Code:       string(pipeline.ErrValidation),
				Message:    fmt.Sprintf("field %q must be an object, got %T", fieldName, value),
				SourcePath: obj.SourcePath,
				ObjectID:   obj.Meta.ID,
				Phase:      pipeline.PhaseValidate,
			})
		} else {
			// Validate required sub-fields.
			for _, req := range schema.Required {
				if _, exists := m[req]; !exists {
					diags = append(diags, pipeline.Diagnostic{
						Severity:   "error",
						Code:       string(pipeline.ErrValidation),
						Message:    fmt.Sprintf("required sub-field %q.%q is missing for kind %q", fieldName, req, obj.Meta.Kind),
						SourcePath: obj.SourcePath,
						ObjectID:   obj.Meta.ID,
						Phase:      pipeline.PhaseValidate,
					})
				}
			}
			// Validate sub-field types.
			for subName, subVal := range m {
				if subSchema, found := schema.Properties[subName]; found {
					diags = append(diags, v.validateFieldType(obj, fieldName+"."+subName, subVal, subSchema)...)
				}
			}
		}
	}

	return diags
}

// validateEnum checks that a string value is one of the allowed enum values.
func (v *StructuralValidator) validateEnum(
	obj pipeline.RawObject,
	fieldName string,
	value string,
	allowed []string,
) []pipeline.Diagnostic {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return []pipeline.Diagnostic{{
		Severity:   "error",
		Code:       string(pipeline.ErrValidation),
		Message:    fmt.Sprintf("field %q value %q is not one of: %s", fieldName, value, strings.Join(allowed, ", ")),
		SourcePath: obj.SourcePath,
		ObjectID:   obj.Meta.ID,
		Phase:      pipeline.PhaseValidate,
	}}
}

// validTargetList returns a comma-separated list of valid targets for error messages.
func validTargetList() string {
	targets := build.AllTargets()
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}
