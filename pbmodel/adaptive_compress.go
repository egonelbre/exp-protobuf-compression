package pbmodel

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/egonelbre/protobuf-compression/arithcode"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// AdaptiveCompress compresses a protobuf message using field-specific models.
// This variant creates a separate compression model for each field, allowing
// better compression by learning field-specific patterns.
func AdaptiveCompress(msg proto.Message, w io.Writer) error {
	amb := NewAdaptiveModelBuilder()
	enc := arithcode.NewEncoder(w)

	if err := adaptiveCompressMessage("", msg.ProtoReflect(), enc, amb); err != nil {
		return err
	}

	return enc.Close()
}

// adaptiveCompressMessage recursively compresses a protobuf message using adaptive models.
func adaptiveCompressMessage(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, amb *AdaptiveModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Iterate through all fields in order
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))

		if !msg.Has(fd) {
			// Field not set, encode a "not present" marker
			if err := enc.Encode(0, amb.boolModel); err != nil {
				return fmt.Errorf("field %s presence: %w", fd.Name(), err)
			}
			continue
		}

		// Field is present
		if err := enc.Encode(1, amb.boolModel); err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		value := msg.Get(fd)

		if fd.IsList() {
			if err := adaptiveCompressRepeatedField(currentPath, fd, value.List(), enc, amb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := adaptiveCompressMapField(currentPath, fd, value.Map(), enc, amb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			// For message fields, recurse with updated path
			nestedMsg := value.Message()
			if err := adaptiveCompressMessage(currentPath, nestedMsg, enc, amb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := adaptiveCompressFieldValue(currentPath, fd, value, enc, amb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}
	}

	return nil
}

// adaptiveCompressRepeatedField compresses a repeated field with field-specific model.
func adaptiveCompressRepeatedField(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, amb *AdaptiveModelBuilder) error {
	// Encode the length using a field-specific model for list lengths
	lengthPath := fieldPath + "._length"
	lengthModel := amb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = amb.byteModel
	}

	length := list.Len()
	lengthBytes := encodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), lengthModel); err != nil {
			return fmt.Errorf("list length: %w", err)
		}
	}

	// Encode each element with element-specific path
	elementPath := fieldPath + "[]"
	for i := 0; i < length; i++ {
		value := list.Get(i)
		if fd.Kind() == protoreflect.MessageKind {
			if err := adaptiveCompressMessage(elementPath, value.Message(), enc, amb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		} else {
			if err := adaptiveCompressFieldValue(elementPath, fd, value, enc, amb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	}

	return nil
}

// adaptiveCompressMapField compresses a map field with field-specific models.
func adaptiveCompressMapField(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, amb *AdaptiveModelBuilder) error {
	// Encode the length
	lengthPath := fieldPath + "._length"
	lengthModel := amb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = amb.byteModel
	}

	length := m.Len()
	lengthBytes := encodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), lengthModel); err != nil {
			return fmt.Errorf("map length: %w", err)
		}
	}

	// Get key and value descriptors
	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	// Encode each key-value pair
	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		// Encode key
		if err := adaptiveCompressFieldValue(keyPath, keyFd, k.Value(), enc, amb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		// Encode value
		if valueFd.Kind() == protoreflect.MessageKind {
			if err := adaptiveCompressMessage(valuePath, v.Message(), enc, amb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		} else {
			if err := adaptiveCompressFieldValue(valuePath, valueFd, v, enc, amb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		}

		return true
	})

	return encodeErr
}

// adaptiveCompressFieldValue compresses a single field value using field-specific model.
func adaptiveCompressFieldValue(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, amb *AdaptiveModelBuilder) error {
	model := amb.GetFieldModel(fieldPath, fd)

	switch fd.Kind() {
	case protoreflect.BoolKind:
		b := 0
		if value.Bool() {
			b = 1
		}
		return enc.Encode(b, model)

	case protoreflect.EnumKind:
		enumValue := value.Enum()
		enumDesc := fd.Enum()
		enumValueDesc := enumDesc.Values().ByNumber(enumValue)
		if enumValueDesc == nil {
			return fmt.Errorf("unknown enum value: %d", enumValue)
		}
		idx := enumValueDesc.Index()
		return enc.Encode(idx, amb.GetEnumModel(fieldPath, enumDesc))

	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		val := value.Int()
		bytes := encodeVarint(uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		val := value.Uint()
		bytes := encodeVarint(val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		val := value.Int()
		zigzag := zigzagEncode(val)
		bytes := encodeVarint(zigzag)
		for _, b := range bytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Fixed32Kind:
		val := uint32(value.Uint())
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed32Kind:
		val := int32(value.Int())
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, uint32(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Fixed64Kind:
		val := value.Uint()
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed64Kind:
		val := int64(value.Int())
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.FloatKind:
		val := float32(value.Float())
		bits := math.Float32bits(val)
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		for _, b := range bytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.DoubleKind:
		val := value.Float()
		bits := math.Float64bits(val)
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, bits)
		for _, b := range bytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.StringKind:
		// Encode string length then bytes
		str := value.String()
		strBytes := []byte(str)

		// Encode length
		lengthBytes := encodeVarint(uint64(len(strBytes)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}

		// Encode string bytes
		for _, b := range strBytes {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.BytesKind:
		data := value.Bytes()
		// Encode length
		lengthBytes := encodeVarint(uint64(len(data)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
		// Encode bytes
		for _, b := range data {
			if err := enc.Encode(int(b), amb.byteModel); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
