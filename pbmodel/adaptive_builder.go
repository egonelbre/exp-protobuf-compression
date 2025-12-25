package pbmodel

import (
	"github.com/egonelbre/vibing/arithcode"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// AdaptiveModelBuilder creates field-specific compression models.
// Unlike ModelBuilder which uses type-based models, this builder creates
// a separate model for each field, allowing it to learn field-specific patterns.
type AdaptiveModelBuilder struct {
	// Field-specific models indexed by full field path
	fieldModels map[string]arithcode.Model

	// Shared models for common types
	boolModel    arithcode.Model
	byteModel    arithcode.Model
	englishModel *arithcode.EnglishModel
}

// NewAdaptiveModelBuilder creates a new adaptive model builder.
func NewAdaptiveModelBuilder() *AdaptiveModelBuilder {
	return &AdaptiveModelBuilder{
		fieldModels:  make(map[string]arithcode.Model),
		boolModel:    arithcode.NewUniformModel(2),
		byteModel:    arithcode.NewUniformModel(256),
		englishModel: arithcode.NewEnglishModel(),
	}
}

// GetFieldModel returns a field-specific model for the given field descriptor.
// The model is created based on the field's path in the message hierarchy,
// allowing different fields of the same type to have different compression characteristics.
func (amb *AdaptiveModelBuilder) GetFieldModel(fieldPath string, fd protoreflect.FieldDescriptor) arithcode.Model {
	// Check if we already have a model for this specific field
	if model, ok := amb.fieldModels[fieldPath]; ok {
		return model
	}

	// Create a new model based on field type
	var model arithcode.Model

	switch fd.Kind() {
	case protoreflect.BoolKind:
		return amb.boolModel

	case protoreflect.EnumKind:
		// Create enum-specific model
		numValues := fd.Enum().Values().Len()
		model = arithcode.NewUniformModel(numValues)

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		// For integers, create a field-specific varint model
		// This could be optimized based on observed value ranges
		model = createAdaptiveVarintModel(fieldPath)

	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		// Fixed-width fields use byte model
		return amb.byteModel

	case protoreflect.StringKind:
		// Strings use the English model (could be made adaptive per field)
		return nil // Handled specially in compress/decompress

	case protoreflect.BytesKind:
		// Bytes use uniform byte model
		return amb.byteModel

	case protoreflect.MessageKind:
		// Messages are handled recursively
		return nil

	default:
		return amb.byteModel
	}

	// Cache the model for this field
	amb.fieldModels[fieldPath] = model
	return model
}

// GetEnumModel returns the enum model for a specific field path.
func (amb *AdaptiveModelBuilder) GetEnumModel(fieldPath string, ed protoreflect.EnumDescriptor) arithcode.Model {
	if model, ok := amb.fieldModels[fieldPath]; ok {
		return model
	}

	numValues := ed.Values().Len()
	model := arithcode.NewUniformModel(numValues)
	amb.fieldModels[fieldPath] = model
	return model
}

// createAdaptiveVarintModel creates a varint model that could be adapted
// based on the field's typical value distribution.
func createAdaptiveVarintModel(fieldPath string) arithcode.Model {
	// For now, use a simple model that favors smaller values
	// In a production implementation, this could analyze the field name
	// or be trained on sample data to determine optimal frequency distribution
	
	freqs := make([]uint64, 256)
	
	// Default distribution favoring small values
	for i := 0; i < 128; i++ {
		freqs[i] = 100 - uint64(i/2)
		if freqs[i] < 1 {
			freqs[i] = 1
		}
	}
	for i := 128; i < 256; i++ {
		freqs[i] = 10
	}

	// Field-specific optimizations based on common field names
	// This is a simple heuristic; a real implementation would learn from data
	switch {
	case containsPattern(fieldPath, "id", "user_id", "account_id"):
		// IDs tend to be larger numbers, adjust distribution
		for i := 0; i < 256; i++ {
			freqs[i] = 50 // More uniform for IDs
		}
	case containsPattern(fieldPath, "count", "size", "length"):
		// Counts tend to be small, keep default
	case containsPattern(fieldPath, "timestamp", "created_at", "updated_at"):
		// Timestamps are large, uniform distribution
		for i := 0; i < 256; i++ {
			freqs[i] = 50
		}
	}

	return arithcode.NewFrequencyTable(freqs)
}

// containsPattern checks if the field path contains any of the given patterns.
func containsPattern(fieldPath string, patterns ...string) bool {
	for _, pattern := range patterns {
		if len(fieldPath) >= len(pattern) {
			for i := 0; i <= len(fieldPath)-len(pattern); i++ {
				if fieldPath[i:i+len(pattern)] == pattern {
					return true
				}
			}
		}
	}
	return false
}

// buildFieldPath constructs a unique path for a field in the message hierarchy.
func buildFieldPath(parentPath string, fieldName string) string {
	if parentPath == "" {
		return fieldName
	}
	return parentPath + "." + fieldName
}
