package pbmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
)

// Decompress decompresses data into a protobuf message using arithmetic coding.
func Decompress(r io.Reader, msg proto.Message) error {
	mb := NewModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessage(msg.ProtoReflect(), dec, mb)
}

// decompressMessage recursively decompresses a protobuf message.
func decompressMessage(msg protoreflect.Message, dec *arithcode.Decoder, mb *ModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Iterate through all fields in order
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)

		// Decode presence marker
		present, err := dec.Decode(mb.boolModel)
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		if present == 0 {
			// Field not present, skip
			continue
		}

		if fd.IsList() {
			list := msg.Mutable(fd).List()
			if err := decompressRepeatedField(fd, list, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapField(fd, m, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			// For message fields, decompress directly into the mutable field
			// to preserve the concrete type
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessage(nestedMsg, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decompressFieldValue(fd, dec, mb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}
	}

	return nil
}

// decompressRepeatedField decompresses a repeated field.
func decompressRepeatedField(fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mb *ModelBuilder) error {
	// Decode the length
	length, err := decodeVarintFromDecoder(dec, mb.varintModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	// Decode each element
	for i := 0; i < int(length); i++ {
		// For message fields, we need to create the element through the list
		// to get the proper concrete type, not a dynamic message
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessage(elem.Message(), dec, mb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decompressFieldValue(fd, dec, mb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// decompressMapField decompresses a map field.
func decompressMapField(fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mb *ModelBuilder) error {
	// Decode the length
	length, err := decodeVarintFromDecoder(dec, mb.varintModel)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	// Get key and value descriptors
	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	// Decode each key-value pair
	for i := 0; i < int(length); i++ {
		// Decode key
		keyValue, err := decompressFieldValue(keyFd, dec, mb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		// Decode value
		valueValue, err := decompressFieldValue(valueFd, dec, mb)
		if err != nil {
			return fmt.Errorf("map value %d: %w", i, err)
		}

		m.Set(keyValue.MapKey(), valueValue)
	}

	return nil
}

// decompressFieldValue decompresses a single field value.
func decompressFieldValue(fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mb *ModelBuilder) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		b, err := dec.Decode(mb.boolModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBool(b != 0), nil

	case protoreflect.EnumKind:
		enumDesc := fd.Enum()
		idx, err := dec.Decode(mb.GetEnumModel(enumDesc))
		if err != nil {
			return protoreflect.Value{}, err
		}
		enumValueDesc := enumDesc.Values().Get(idx)
		return protoreflect.ValueOfEnum(enumValueDesc.Number()), nil

	case protoreflect.Int32Kind:
		val, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Int64Kind:
		val, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.Uint32Kind:
		val, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint32(uint32(val)), nil

	case protoreflect.Uint64Kind:
		val, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sint32Kind:
		zigzag, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := ZigzagDecode(zigzag)
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Sint64Kind:
		zigzag, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := ZigzagDecode(zigzag)
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.Fixed32Kind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := dec.Decode(mb.byteModel)
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
			b, err := dec.Decode(mb.byteModel)
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
			b, err := dec.Decode(mb.byteModel)
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
			b, err := dec.Decode(mb.byteModel)
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
			b, err := dec.Decode(mb.byteModel)
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
			b, err := dec.Decode(mb.byteModel)
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
		compressedLen, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode the compressed bytes
		compressedBytes := make([]byte, compressedLen)
		for i := 0; i < int(compressedLen); i++ {
			b, err := dec.Decode(mb.byteModel)
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
		length, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode bytes
		data := make([]byte, length)
		for i := 0; i < int(length); i++ {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			data[i] = byte(b)
		}

		return protoreflect.ValueOfBytes(data), nil

	case protoreflect.MessageKind:
		// Create a new message and recursively decompress it
		msgDesc := fd.Message()
		msg := dynamicpb.NewMessage(msgDesc)
		if err := decompressMessage(msg, dec, mb); err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(msg), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}

// decodeVarintFromDecoder decodes a varint using the decoder and model.
func decodeVarintFromDecoder(dec *arithcode.Decoder, model arithcode.Model) (uint64, error) {
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
