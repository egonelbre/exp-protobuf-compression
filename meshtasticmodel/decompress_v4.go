package meshtasticmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
)

// DecompressV4 decompresses data with enum value prediction.
func DecompressV4(r io.Reader, msg proto.Message) error {
	mmb := NewModelBuilderV4()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessageV4("", msg.ProtoReflect(), dec, mmb)
}

// decompressMessageV4 recursively decompresses with enum prediction.
func decompressMessageV4(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *ModelBuilderV4) error {
	md := msg.Descriptor()
	fields := md.Fields()

	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		currentPath := pbmodel.BuildFieldPath(fieldPath, string(fd.Name()))

		// Decode presence marker
		present, err := dec.Decode(mmb.BoolModel())
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		if present == 0 {
			continue
		}

		// Track portnum for payload detection
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal, err := decodeFieldValueV4(currentPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, enumVal)
			portNum := meshtastic.PortNum(enumVal.Enum())
			mmb.currentPortNum = &portNum
			continue
		}

		if fd.IsList() {
			list := msg.Mutable(fd).List()
			if err := decompressRepeatedFieldV4(currentPath, fd, list, dec, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapFieldV4(currentPath, fd, m, dec, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessageV4(currentPath, nestedMsg, dec, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decodeFieldValueV4(currentPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}

		// Reset portnum after processing Data message
		if md.Name() == "Data" && i == fields.Len()-1 {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// decompressRepeatedFieldV4 decompresses repeated fields.
func decompressRepeatedFieldV4(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mmb *ModelBuilderV4) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.ByteModel()
	}

	length, err := decodeVarintFromDecoderV1(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	elementPath := fieldPath + "[]"
	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessageV4(elementPath, elem.Message(), dec, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decodeFieldValueV4(elementPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// decompressMapFieldV4 decompresses map fields.
func decompressMapFieldV4(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mmb *ModelBuilderV4) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.ByteModel()
	}

	length, err := decodeVarintFromDecoderV1(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	for i := 0; i < int(length); i++ {
		keyValue, err := decodeFieldValueV4(keyPath, keyFd, dec, mmb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		var mapValue protoreflect.Value
		if valueFd.Kind() == protoreflect.MessageKind {
			elem := m.NewValue()
			if err := decompressMessageV4(valuePath, elem.Message(), dec, mmb); err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			mapValue = elem
		} else {
			val, err := decodeFieldValueV4(valuePath, valueFd, dec, mmb)
			if err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			mapValue = val
		}

		m.Set(keyValue.MapKey(), mapValue)
	}

	return nil
}

// decodeFieldValueV4 decodes field values with enum prediction.
func decodeFieldValueV4(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mmb *ModelBuilderV4) (protoreflect.Value, error) {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		textFlag, err := dec.Decode(mmb.BoolModel())
		if err != nil {
			return protoreflect.Value{}, err
		}

		if textFlag == 1 {
			compressedLen, err := decodeVarintFromDecoderV1(dec, mmb.ByteModel())
			if err != nil {
				return protoreflect.Value{}, err
			}

			compressedBytes := make([]byte, compressedLen)
			for i := 0; i < int(compressedLen); i++ {
				b, err := dec.Decode(mmb.ByteModel())
				if err != nil {
					return protoreflect.Value{}, err
				}
				compressedBytes[i] = byte(b)
			}

			str, err := arithcode.DecodeString(bytes.NewReader(compressedBytes))
			if err != nil {
				return protoreflect.Value{}, err
			}

			return protoreflect.ValueOfBytes([]byte(str)), nil
		}
	}

	model := mmb.GetFieldModel(fieldPath, fd)

	switch fd.Kind() {
	case protoreflect.BoolKind:
		b, err := dec.Decode(model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBool(b != 0), nil

	case protoreflect.EnumKind:
		enumDesc := fd.Enum()

		// Check if this enum has a predicted common value
		fieldName := string(fd.Name())
		if predictedValue, hasPrediction := mmb.enumPredictions[fieldName]; hasPrediction {
			// Decode prediction bit
			isPredicted, err := dec.Decode(mmb.BoolModel())
			if err != nil {
				return protoreflect.Value{}, err
			}

			if isPredicted == 1 {
				// Common value
				return protoreflect.ValueOfEnum(predictedValue), nil
			}
			// Not common value, continue to decode full value
		}

		// Decode full enum value
		idx, err := dec.Decode(mmb.GetEnumModel(fieldPath, enumDesc))
		if err != nil {
			return protoreflect.Value{}, err
		}
		if idx >= enumDesc.Values().Len() {
			return protoreflect.Value{}, fmt.Errorf("invalid enum index: %d", idx)
		}
		enumValue := enumDesc.Values().Get(idx).Number()
		return protoreflect.ValueOfEnum(enumValue), nil

	case protoreflect.Int32Kind:
		val, err := decodeVarintFromDecoderV1(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Int64Kind:
		val, err := decodeVarintFromDecoderV1(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.Uint32Kind:
		val, err := decodeVarintFromDecoderV1(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint32(uint32(val)), nil

	case protoreflect.Uint64Kind:
		val, err := decodeVarintFromDecoderV1(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sint32Kind:
		encoded, err := decodeVarintFromDecoderV1(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := pbmodel.ZigzagDecode(encoded)
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Sint64Kind:
		encoded, err := decodeVarintFromDecoderV1(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := pbmodel.ZigzagDecode(encoded)
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.Fixed32Kind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := dec.Decode(mmb.ByteModel())
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
			b, err := dec.Decode(mmb.ByteModel())
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := int32(binary.LittleEndian.Uint32(bytes))
		return protoreflect.ValueOfInt32(val), nil

	case protoreflect.Fixed64Kind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			b, err := dec.Decode(mmb.ByteModel())
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
			b, err := dec.Decode(mmb.ByteModel())
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := int64(binary.LittleEndian.Uint64(bytes))
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.FloatKind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := dec.Decode(mmb.ByteModel())
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
			b, err := dec.Decode(mmb.ByteModel())
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		bits := binary.LittleEndian.Uint64(bytes)
		val := math.Float64frombits(bits)
		return protoreflect.ValueOfFloat64(val), nil

	case protoreflect.StringKind:
		compressedLen, err := decodeVarintFromDecoderV1(dec, mmb.ByteModel())
		if err != nil {
			return protoreflect.Value{}, err
		}

		compressedBytes := make([]byte, compressedLen)
		for i := 0; i < int(compressedLen); i++ {
			b, err := dec.Decode(mmb.ByteModel())
			if err != nil {
				return protoreflect.Value{}, err
			}
			compressedBytes[i] = byte(b)
		}

		str, err := arithcode.DecodeString(bytes.NewReader(compressedBytes))
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfString(str), nil

	case protoreflect.BytesKind:
		length, err := decodeVarintFromDecoderV1(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}

		data := make([]byte, length)
		for i := 0; i < int(length); i++ {
			b, err := dec.Decode(mmb.ByteModel())
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
