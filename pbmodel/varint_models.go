package pbmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
)

// varintByteModels holds position-specific byte models for varint encoding.
type varintByteModels struct {
	firstByteModel arithcode.Model // Model for first byte of varint
	contByteModel  arithcode.Model // Model for continuation bytes
}

func newVarintByteModels() *varintByteModels {
	return &varintByteModels{
		firstByteModel: createVarintFirstByteModel(),
		contByteModel:  createVarintContinuationByteModel(),
	}
}

func createVarintFirstByteModel() arithcode.Model {
	freqs := make([]uint64, 256)

	// Bytes 0-127: Terminal bytes (no continuation)
	// Favor smaller values - they're more common in practice
	for i := 0; i < 128; i++ {
		freqs[i] = 200 - uint64(i)
		if freqs[i] < 20 {
			freqs[i] = 20
		}
	}

	// Bytes 128-255: Continuation bytes (more bytes follow)
	for i := 128; i < 256; i++ {
		freqs[i] = 15
	}

	return arithcode.NewFrequencyTable(freqs)
}

func createVarintContinuationByteModel() arithcode.Model {
	freqs := make([]uint64, 256)

	// Continuation bytes have different distribution
	// Bytes 0-127: Terminal bytes (no more bytes follow)
	for i := 0; i < 128; i++ {
		freqs[i] = 100
	}

	// Bytes 128-255: More bytes follow
	for i := 128; i < 256; i++ {
		freqs[i] = 50
	}

	return arithcode.NewFrequencyTable(freqs)
}

func (vm *varintByteModels) getByteModel(byteIndex int) arithcode.Model {
	if byteIndex == 0 {
		return vm.firstByteModel
	}
	return vm.contByteModel
}

func encodeVarintWithModels(value uint64, enc *arithcode.Encoder, vm *varintByteModels) error {
	varintBytes := EncodeVarint(value)
	for i, b := range varintBytes {
		model := vm.getByteModel(i)
		if err := enc.Encode(int(b), model); err != nil {
			return err
		}
	}
	return nil
}

func decodeVarintWithModels(dec *arithcode.Decoder, vm *varintByteModels) (uint64, error) {
	var result uint64
	var shift uint
	byteIndex := 0

	for {
		model := vm.getByteModel(byteIndex)
		b, err := dec.Decode(model)
		if err != nil {
			return 0, err
		}

		result |= uint64(b&0x7F) << shift
		if b < 128 {
			return result, nil
		}
		shift += 7
		byteIndex++
		if shift >= 64 {
			return 0, fmt.Errorf("varint too long")
		}
	}
}

// CompressVarintModels compresses a protobuf message using arithmetic coding with varint byte models.
func CompressVarintModels(msg proto.Message, w io.Writer) error {
	mb := NewModelBuilder()
	vm := newVarintByteModels()
	enc := arithcode.NewEncoder(w)

	if err := compressMessageVarintModels(msg.ProtoReflect(), enc, mb, vm); err != nil {
		return err
	}

	return enc.Close()
}

// DecompressVarintModels decompresses a protobuf message that was compressed with varint byte models.
func DecompressVarintModels(r io.Reader, msg proto.Message) error {
	mb := NewModelBuilder()
	vm := newVarintByteModels()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessageVarintModels(msg.ProtoReflect(), dec, mb, vm)
}

func compressMessageVarintModels(msg protoreflect.Message, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	md := msg.Descriptor()
	fields := md.Fields()

	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)

		if !msg.Has(fd) {
			if err := enc.Encode(0, mb.boolModel); err != nil {
				return fmt.Errorf("field %s presence: %w", fd.Name(), err)
			}
			continue
		}

		if err := enc.Encode(1, mb.boolModel); err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		value := msg.Get(fd)

		if fd.IsList() {
			if err := compressRepeatedFieldVarintModels(fd, value.List(), enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldVarintModels(fd, value.Map(), enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueVarintModels(fd, value, enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}
	}

	return nil
}

func compressRepeatedFieldVarintModels(fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	length := list.Len()
	if err := encodeVarintWithModels(uint64(length), enc, vm); err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < length; i++ {
		value := list.Get(i)
		if err := compressFieldValueVarintModels(fd, value, enc, mb, vm); err != nil {
			return fmt.Errorf("list element %d: %w", i, err)
		}
	}

	return nil
}

func compressMapFieldVarintModels(fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	length := m.Len()
	if err := encodeVarintWithModels(uint64(length), enc, vm); err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		if err := compressFieldValueVarintModels(keyFd, k.Value(), enc, mb, vm); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if err := compressFieldValueVarintModels(valueFd, v, enc, mb, vm); err != nil {
			encodeErr = fmt.Errorf("map value: %w", err)
			return false
		}

		return true
	})

	return encodeErr
}

func compressFieldValueVarintModels(fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		b := 0
		if value.Bool() {
			b = 1
		}
		return enc.Encode(b, mb.boolModel)

	case protoreflect.EnumKind:
		enumValue := value.Enum()
		enumDesc := fd.Enum()
		enumValueDesc := enumDesc.Values().ByNumber(enumValue)
		if enumValueDesc == nil {
			return fmt.Errorf("unknown enum value: %d", enumValue)
		}
		idx := enumValueDesc.Index()
		return enc.Encode(idx, mb.GetEnumModel(enumDesc))

	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		val := value.Int()
		return encodeVarintWithModels(uint64(val), enc, vm)

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		val := value.Uint()
		return encodeVarintWithModels(val, enc, vm)

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		val := value.Int()
		zigzag := ZigzagEncode(val)
		return encodeVarintWithModels(zigzag, enc, vm)

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
		str := value.String()
		var buf bytes.Buffer
		if err := arithcode.EncodeString(str, &buf); err != nil {
			return err
		}
		compressedBytes := buf.Bytes()
		if err := encodeVarintWithModels(uint64(len(compressedBytes)), enc, vm); err != nil {
			return err
		}
		for _, b := range compressedBytes {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.BytesKind:
		data := value.Bytes()
		if err := encodeVarintWithModels(uint64(len(data)), enc, vm); err != nil {
			return err
		}
		for _, b := range data {
			if err := enc.Encode(int(b), mb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.MessageKind:
		return compressMessageVarintModels(value.Message(), enc, mb, vm)

	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}

// Decompression functions

func decompressMessageVarintModels(msg protoreflect.Message, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
	md := msg.Descriptor()
	fields := md.Fields()

	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)

		present, err := dec.Decode(mb.boolModel)
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		if present == 0 {
			continue
		}

		if fd.IsList() {
			list := msg.Mutable(fd).List()
			if err := decompressRepeatedFieldVarintModels(fd, list, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapFieldVarintModels(fd, m, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessageVarintModels(nestedMsg, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decompressFieldValueVarintModels(fd, dec, mb, vm)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}
	}

	return nil
}

func decompressRepeatedFieldVarintModels(fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
	length, err := decodeVarintWithModels(dec, vm)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessageVarintModels(elem.Message(), dec, mb, vm); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decompressFieldValueVarintModels(fd, dec, mb, vm)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

func decompressMapFieldVarintModels(fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
	length, err := decodeVarintWithModels(dec, vm)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	for i := 0; i < int(length); i++ {
		key, err := decompressFieldValueVarintModels(keyFd, dec, mb, vm)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		value, err := decompressFieldValueVarintModels(valueFd, dec, mb, vm)
		if err != nil {
			return fmt.Errorf("map value %d: %w", i, err)
		}

		m.Set(key.MapKey(), value)
	}

	return nil
}

func decompressFieldValueVarintModels(fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) (protoreflect.Value, error) {
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
		if idx >= enumDesc.Values().Len() {
			return protoreflect.Value{}, fmt.Errorf("enum index %d out of range", idx)
		}
		enumValue := enumDesc.Values().Get(idx).Number()
		return protoreflect.ValueOfEnum(enumValue), nil

	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		val, err := decodeVarintWithModels(dec, vm)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		val, err := decodeVarintWithModels(dec, vm)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		zigzag, err := decodeVarintWithModels(dec, vm)
		if err != nil {
			return protoreflect.Value{}, err
		}
		val := ZigzagDecode(zigzag)
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.Fixed32Kind:
		bytes := make([]byte, 4)
		for i := range bytes {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := binary.LittleEndian.Uint32(bytes)
		return protoreflect.ValueOfUint64(uint64(val)), nil

	case protoreflect.Sfixed32Kind:
		bytes := make([]byte, 4)
		for i := range bytes {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := int32(binary.LittleEndian.Uint32(bytes))
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.Fixed64Kind:
		bytes := make([]byte, 8)
		for i := range bytes {
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
		for i := range bytes {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		val := int64(binary.LittleEndian.Uint64(bytes))
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.FloatKind:
		bytes := make([]byte, 4)
		for i := range bytes {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(b)
		}
		bits := binary.LittleEndian.Uint32(bytes)
		val := math.Float32frombits(bits)
		return protoreflect.ValueOfFloat64(float64(val)), nil

	case protoreflect.DoubleKind:
		bytes := make([]byte, 8)
		for i := range bytes {
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
		length, err := decodeVarintWithModels(dec, vm)
		if err != nil {
			return protoreflect.Value{}, err
		}

		compressedBytes := make([]byte, length)
		for i := range compressedBytes {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			compressedBytes[i] = byte(b)
		}

		buf := bytes.NewBuffer(compressedBytes)
		str, err := arithcode.DecodeString(buf)
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfString(str), nil

	case protoreflect.BytesKind:
		length, err := decodeVarintWithModels(dec, vm)
		if err != nil {
			return protoreflect.Value{}, err
		}

		data := make([]byte, length)
		for i := range data {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			data[i] = byte(b)
		}

		return protoreflect.ValueOfBytes(data), nil

	case protoreflect.MessageKind:
		// This case shouldn't be reached due to the special handling in decompressMessageVarintModels
		return protoreflect.Value{}, fmt.Errorf("message kind should be handled separately")

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
