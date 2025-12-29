package pbmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// AdaptiveDecompress decompresses data into a protobuf message using field-specific models.
func AdaptiveDecompress(r io.Reader, msg proto.Message) error {
	amb := NewAdaptiveModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return adaptiveDecompressMessage("", msg.ProtoReflect(), dec, amb)
}

// adaptiveDecompressMessage recursively decompresses a protobuf message using adaptive models.
func adaptiveDecompressMessage(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, amb *AdaptiveModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Iterate through all fields in order
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))

		// Decode presence marker
		present, err := dec.Decode(amb.boolModel)
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		if present == 0 {
			// Field not present, skip
			continue
		}

		if fd.IsList() {
			list := msg.Mutable(fd).List()
			if err := adaptiveDecompressRepeatedField(currentPath, fd, list, dec, amb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := adaptiveDecompressMapField(currentPath, fd, m, dec, amb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			// For message fields, decompress directly into the mutable field
			nestedMsg := msg.Mutable(fd).Message()
			if err := adaptiveDecompressMessage(currentPath, nestedMsg, dec, amb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := adaptiveDecompressFieldValue(currentPath, fd, dec, amb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}
	}

	return nil
}

// adaptiveDecompressRepeatedField decompresses a repeated field using field-specific models.
func adaptiveDecompressRepeatedField(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, amb *AdaptiveModelBuilder) error {
	// Decode the length using field-specific model
	lengthPath := fieldPath + "._length"
	lengthModel := amb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = amb.byteModel
	}

	length, err := adaptiveDecodeVarintFromDecoder(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	// Decode each element
	elementPath := fieldPath + "[]"
	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := adaptiveDecompressMessage(elementPath, elem.Message(), dec, amb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := adaptiveDecompressFieldValue(elementPath, fd, dec, amb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// adaptiveDecompressMapField decompresses a map field using field-specific models.
func adaptiveDecompressMapField(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, amb *AdaptiveModelBuilder) error {
	// Decode the length
	lengthPath := fieldPath + "._length"
	lengthModel := amb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = amb.byteModel
	}

	length, err := adaptiveDecodeVarintFromDecoder(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	// Get key and value descriptors
	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	// Decode each key-value pair
	for i := 0; i < int(length); i++ {
		// Decode key
		keyValue, err := adaptiveDecompressFieldValue(keyPath, keyFd, dec, amb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		// Decode value
		var valueValue protoreflect.Value
		if valueFd.Kind() == protoreflect.MessageKind {
			msgDesc := valueFd.Message()
			valueMsg := dynamicpb.NewMessage(msgDesc)
			if err := adaptiveDecompressMessage(valuePath, valueMsg, dec, amb); err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			valueValue = protoreflect.ValueOfMessage(valueMsg)
		} else {
			valueValue, err = adaptiveDecompressFieldValue(valuePath, valueFd, dec, amb)
			if err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
		}

		m.Set(keyValue.MapKey(), valueValue)
	}

	return nil
}

// adaptiveDecompressFieldValue decompresses a single field value using field-specific model.
func adaptiveDecompressFieldValue(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, amb *AdaptiveModelBuilder) (protoreflect.Value, error) {
	model := amb.GetFieldModel(fieldPath, fd)

	switch fd.Kind() {
	case protoreflect.BoolKind:
		b, err := dec.Decode(model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBool(b != 0), nil

	case protoreflect.EnumKind:
		enumDesc := fd.Enum()
		idx, err := dec.Decode(amb.GetEnumModel(fieldPath, enumDesc))
		if err != nil {
			return protoreflect.Value{}, err
		}
		enumValueDesc := enumDesc.Values().Get(idx)
		return protoreflect.ValueOfEnum(enumValueDesc.Number()), nil

	case protoreflect.Int32Kind:
		val, err := adaptiveDecodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Int64Kind:
		val, err := adaptiveDecodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.Uint32Kind:
		val, err := adaptiveDecodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint32(uint32(val)), nil

	case protoreflect.Uint64Kind:
		val, err := adaptiveDecodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sint32Kind:
		zigzag, err := adaptiveDecodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := zigzagDecode(zigzag)
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Sint64Kind:
		zigzag, err := adaptiveDecodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := zigzagDecode(zigzag)
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.Fixed32Kind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := binary.LittleEndian.Uint32(bytes)
		return protoreflect.ValueOfUint32(val), nil

	case protoreflect.Sfixed32Kind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := binary.LittleEndian.Uint32(bytes)
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Fixed64Kind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := binary.LittleEndian.Uint64(bytes)
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sfixed64Kind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := binary.LittleEndian.Uint64(bytes)
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.FloatKind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		bits := binary.LittleEndian.Uint32(bytes)
		val := math.Float32frombits(bits)
		return protoreflect.ValueOfFloat32(val), nil

	case protoreflect.DoubleKind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		bits := binary.LittleEndian.Uint64(bytes)
		val := math.Float64frombits(bits)
		return protoreflect.ValueOfFloat64(val), nil

	case protoreflect.StringKind:
		// Decode the compressed string length
		compressedLen, err := adaptiveDecodeVarintFromDecoder(dec, amb.byteModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode the compressed bytes
		compressedBytes := make([]byte, compressedLen)
		for i := 0; i < int(compressedLen); i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			compressedBytes[i] = byte(b)
		}

		// Decompress the string using the English model
		str, err := arithcode.DecodeString(bytes.NewReader(compressedBytes))
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfString(str), nil

	case protoreflect.BytesKind:
		// Decode length
		length, err := adaptiveDecodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode bytes
		data := make([]byte, length)
		for i := 0; i < int(length); i++ {
			b, err := dec.Decode(amb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			data[i] = byte(b)
		}

		return protoreflect.ValueOfBytes(data), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}

// adaptiveDecodeVarintFromDecoder decodes a varint using the decoder and field-specific model.
func adaptiveDecodeVarintFromDecoder(dec *arithcode.Decoder, model arithcode.Model) (uint64, error) {
	var value uint64
	for i := 0; i < 10; i++ { // Max 10 bytes for uint64
		b, err := dec.Decode(model)
		if err != nil {
			return 0, err
		}
		value |= uint64(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			break
		}
	}
	return value, nil
}
