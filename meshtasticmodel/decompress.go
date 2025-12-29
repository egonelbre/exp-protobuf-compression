package meshtasticmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Decompress decompresses data into a protobuf message using Meshtastic-specific optimizations.
func Decompress(r io.Reader, msg proto.Message) error {
	mmb := NewModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessage("", msg.ProtoReflect(), dec, mmb)
}

// decompressMessage recursively decompresses with Meshtastic-specific optimizations.
func decompressMessage(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *ModelBuilder) error {
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
			enumVal, err := decodeFieldValue(currentPath, fd, dec, mmb)
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
			if err := decompressRepeatedField(currentPath, fd, list, dec, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := pbmodel.AdaptiveDecompressMapField(currentPath, fd, m, dec, mmb.AdaptiveModelBuilder); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessage(currentPath, nestedMsg, dec, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decodeFieldValue(currentPath, fd, dec, mmb)
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

// decompressRepeatedField decompresses repeated fields.
func decompressRepeatedField(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mmb *ModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.ByteModel()
	}

	length, err := decodeVarintFromDecoder(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	elementPath := fieldPath + "[]"
	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessage(elementPath, elem.Message(), dec, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decodeFieldValue(elementPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// decodeFieldValue decodes field values with Meshtastic-specific logic.
func decodeFieldValue(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mmb *ModelBuilder) (protoreflect.Value, error) {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		// Decode the text flag
		textFlag, err := dec.Decode(mmb.BoolModel())
		if err != nil {
			return protoreflect.Value{}, err
		}

		if textFlag == 1 {
			// Decompress as text
			compressedLen, err := decodeVarintFromDecoder(dec, mmb.ByteModel())
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

		// Fall through to normal bytes decoding
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
		idx, err := dec.Decode(mmb.GetEnumModel(fieldPath, fd.Enum()))
		if err != nil {
			return protoreflect.Value{}, err
		}
		enumDesc := fd.Enum()
		if idx >= enumDesc.Values().Len() {
			return protoreflect.Value{}, fmt.Errorf("invalid enum index: %d", idx)
		}
		enumValue := enumDesc.Values().Get(idx).Number()
		return protoreflect.ValueOfEnum(enumValue), nil

	case protoreflect.Int32Kind:
		val, err := decodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Int64Kind:
		val, err := decodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.Uint32Kind:
		val, err := decodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint32(uint32(val)), nil

	case protoreflect.Uint64Kind:
		val, err := decodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sint32Kind:
		encoded, err := decodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := pbmodel.ZigzagDecode(encoded)
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Sint64Kind:
		encoded, err := decodeVarintFromDecoder(dec, model)
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
		// Decode the compressed string length
		compressedLen, err := decodeVarintFromDecoder(dec, mmb.ByteModel())
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode the compressed bytes
		compressedBytes := make([]byte, compressedLen)
		for i := 0; i < int(compressedLen); i++ {
			b, err := dec.Decode(mmb.ByteModel())
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
		length, err := decodeVarintFromDecoder(dec, model)
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode bytes
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

// decodeVarintFromDecoder decodes a varint from the decoder.
func decodeVarintFromDecoder(dec *arithcode.Decoder, model arithcode.Model) (uint64, error) {
	var value uint64
	var shift uint
	for {
		b, err := dec.Decode(model)
		if err != nil {
			return 0, err
		}
		value |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("varint overflow")
		}
	}
	return value, nil
}
