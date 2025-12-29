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
)

// Compress compresses a protobuf message using arithmetic coding.
func Compress(msg proto.Message, w io.Writer) error {
	mb := NewModelBuilder()
	enc := arithcode.NewEncoder(w)

	if err := compressMessage(msg.ProtoReflect(), enc, mb); err != nil {
		return err
	}

	return enc.Close()
}

// compressMessage recursively compresses a protobuf message.
func compressMessage(msg protoreflect.Message, enc *arithcode.Encoder, mb *ModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Iterate through all fields in order
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)

		if !msg.Has(fd) {
			// Field not set, encode a "not present" marker
			if err := enc.Encode(0, mb.boolModel); err != nil {
				return fmt.Errorf("field %s presence: %w", fd.Name(), err)
			}
			continue
		}

		// Field is present
		if err := enc.Encode(1, mb.boolModel); err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		value := msg.Get(fd)

		if fd.IsList() {
			if err := compressRepeatedField(fd, value.List(), enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapField(fd, value.Map(), enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValue(fd, value, enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}
	}

	return nil
}

// compressRepeatedField compresses a repeated field.
func compressRepeatedField(fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mb *ModelBuilder) error {
	// Encode the length
	length := list.Len()
	lengthBytes := encodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), mb.varintModel); err != nil {
			return fmt.Errorf("list length: %w", err)
		}
	}

	// Encode each element
	for i := 0; i < length; i++ {
		value := list.Get(i)
		if err := compressFieldValue(fd, value, enc, mb); err != nil {
			return fmt.Errorf("list element %d: %w", i, err)
		}
	}

	return nil
}

// compressMapField compresses a map field.
func compressMapField(fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mb *ModelBuilder) error {
	// Encode the length
	length := m.Len()
	lengthBytes := encodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), mb.varintModel); err != nil {
			return fmt.Errorf("map length: %w", err)
		}
	}

	// Get key and value descriptors
	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	// Encode each key-value pair
	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		// Encode key
		if err := compressFieldValue(keyFd, k.Value(), enc, mb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		// Encode value
		if err := compressFieldValue(valueFd, v, enc, mb); err != nil {
			encodeErr = fmt.Errorf("map value: %w", err)
			return false
		}

		return true
	})

	return encodeErr
}

// compressFieldValue compresses a single field value.
func compressFieldValue(fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mb *ModelBuilder) error {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		b := 0
		if value.Bool() {
			b = 1
		}
		return enc.Encode(b, mb.boolModel)

	case protoreflect.EnumKind:
		// Encode enum as its index in the enum descriptor
		enumValue := value.Enum()
		enumDesc := fd.Enum()
		enumValueDesc := enumDesc.Values().ByNumber(enumValue)
		if enumValueDesc == nil {
			// Unknown enum value, encode as -1 (will need special handling)
			return fmt.Errorf("unknown enum value: %d", enumValue)
		}
		idx := enumValueDesc.Index()
		return enc.Encode(idx, mb.GetEnumModel(enumDesc))

	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		val := value.Int()
		bytes := encodeVarint(uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mb.varintModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		val := value.Uint()
		bytes := encodeVarint(val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mb.varintModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		val := value.Int()
		zigzag := zigzagEncode(val)
		bytes := encodeVarint(zigzag)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mb.varintModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Fixed32Kind:
		val := uint32(value.Uint())
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed32Kind:
		val := int32(value.Int())
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, uint32(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Fixed64Kind:
		val := value.Uint()
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed64Kind:
		val := int64(value.Int())
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
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
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
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
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.StringKind:
		// Use the English model for strings
		str := value.String()
		var buf bytes.Buffer
		if err := arithcode.EncodeString(str, &buf); err != nil {
			return err
		}
		// Encode the compressed string bytes
		compressedBytes := buf.Bytes()
		lengthBytes := encodeVarint(uint64(len(compressedBytes)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), mb.varintModel); err != nil {
				return err
			}
		}
		for _, b := range compressedBytes {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.BytesKind:
		data := value.Bytes()
		// Encode length
		lengthBytes := encodeVarint(uint64(len(data)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), mb.varintModel); err != nil {
				return err
			}
		}
		// Encode bytes
		for _, b := range data {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.MessageKind:
		// Recursively compress the nested message
		return compressMessage(value.Message(), enc, mb)

	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
