// Package pbmodel provides protobuf-specific compression using arithmetic coding.
// It builds compression models based on protobuf message structure and field types.
package pbmodel

import (
	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ModelBuilder creates compression models for protobuf messages.
type ModelBuilder struct {
	// Models for different data types
	boolModel    arithcode.Model
	byteModel    arithcode.Model
	varintModel  arithcode.Model
	enumModels   map[string]arithcode.Model
	englishModel *arithcode.EnglishModel
}

// NewModelBuilder creates a new protobuf model builder.
func NewModelBuilder() *ModelBuilder {
	return &ModelBuilder{
		boolModel:    arithcode.NewUniformModel(2), // true/false
		byteModel:    arithcode.NewUniformModel(256),
		varintModel:  createVarintModel(),
		enumModels:   make(map[string]arithcode.Model),
		englishModel: arithcode.NewEnglishModel(),
	}
}

// createVarintModel creates a model optimized for variable-length integers.
// Small integers are more common in practice, so we give them higher probability.
func createVarintModel() arithcode.Model {
	// Create a model that favors smaller values
	// This is a simplified model; a more sophisticated one could be adaptive
	freqs := make([]uint64, 256)
	for i := 0; i < 128; i++ {
		freqs[i] = 100 - uint64(i/2) // Smaller values more likely
		if freqs[i] < 1 {
			freqs[i] = 1
		}
	}
	for i := 128; i < 256; i++ {
		freqs[i] = 10 // Larger values less likely
	}
	return arithcode.NewFrequencyTable(freqs)
}

// GetEnumModel returns a model for the given enum type.
// Enum models assume uniform distribution across enum values.
func (mb *ModelBuilder) GetEnumModel(ed protoreflect.EnumDescriptor) arithcode.Model {
	fullName := string(ed.FullName())
	if model, ok := mb.enumModels[fullName]; ok {
		return model
	}

	// Create uniform model for enum values
	numValues := ed.Values().Len()
	model := arithcode.NewUniformModel(numValues)
	mb.enumModels[fullName] = model
	return model
}

// GetFieldModel returns the appropriate model for a protobuf field.
func (mb *ModelBuilder) GetFieldModel(fd protoreflect.FieldDescriptor) arithcode.Model {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return mb.boolModel

	case protoreflect.EnumKind:
		return mb.GetEnumModel(fd.Enum())

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		// For integers, we use varint encoding with the varint model
		return mb.varintModel

	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		// Fixed32 is always 4 bytes
		return mb.byteModel

	case protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		// Fixed64 is always 8 bytes
		return mb.byteModel

	case protoreflect.FloatKind:
		// Float is 4 bytes
		return mb.byteModel

	case protoreflect.DoubleKind:
		// Double is 8 bytes
		return mb.byteModel

	case protoreflect.StringKind:
		// Strings use the English model (handled separately)
		return nil

	case protoreflect.BytesKind:
		// Bytes use uniform byte model
		return mb.byteModel

	case protoreflect.MessageKind:
		// Messages are handled recursively
		return nil

	default:
		// Default to byte model
		return mb.byteModel
	}
}

// encodeVarint encodes an integer as a variable-length quantity.
// Returns the bytes to encode with the varint model.
func encodeVarint(value uint64) []byte {
	var buf []byte
	for {
		b := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if value == 0 {
			break
		}
	}
	return buf
}

// decodeVarint decodes a variable-length quantity.
func decodeVarint(bytes []byte) uint64 {
	var value uint64
	for i, b := range bytes {
		value |= uint64(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			break
		}
	}
	return value
}

// zigzagEncode encodes a signed integer using zigzag encoding.
// This maps negative values to positive values: 0, -1, 1, -2, 2, ...
func zigzagEncode(n int64) uint64 {
	return uint64((n << 1) ^ (n >> 63))
}

// zigzagDecode decodes a zigzag-encoded integer.
func zigzagDecode(n uint64) int64 {
	return int64((n >> 1) ^ -(n & 1))
}
