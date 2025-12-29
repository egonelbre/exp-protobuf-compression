package pbmodel

import (
	"bytes"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
)

// CompressVarintModelsOrder1 combines varint byte models with order-1 string compression.
func CompressVarintModelsOrder1(msg proto.Message, w io.Writer) error {
	mb := NewModelBuilder()
	vm := newVarintByteModels()
	enc := arithcode.NewEncoder(w)

	if err := compressMessageVarintModelsOrder1(msg.ProtoReflect(), enc, mb, vm); err != nil {
		return err
	}

	return enc.Close()
}

// DecompressVarintModelsOrder1 decompresses messages compressed with varint byte models and order-1 strings.
func DecompressVarintModelsOrder1(r io.Reader, msg proto.Message) error {
	mb := NewModelBuilder()
	vm := newVarintByteModels()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessageVarintModelsOrder1(msg.ProtoReflect(), dec, mb, vm)
}

// CompressVarintModelsOrder2 combines varint byte models with order-2 string compression.
func CompressVarintModelsOrder2(msg proto.Message, w io.Writer) error {
	mb := NewModelBuilder()
	vm := newVarintByteModels()
	enc := arithcode.NewEncoder(w)

	if err := compressMessageVarintModelsOrder2(msg.ProtoReflect(), enc, mb, vm); err != nil {
		return err
	}

	return enc.Close()
}

// DecompressVarintModelsOrder2 decompresses messages compressed with varint byte models and order-2 strings.
func DecompressVarintModelsOrder2(r io.Reader, msg proto.Message) error {
	mb := NewModelBuilder()
	vm := newVarintByteModels()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessageVarintModelsOrder2(msg.ProtoReflect(), dec, mb, vm)
}

// Order-1 implementation

func compressMessageVarintModelsOrder1(msg protoreflect.Message, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
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
			if err := compressRepeatedFieldVarintModelsOrder1(fd, value.List(), enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldVarintModelsOrder1(fd, value.Map(), enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueVarintModelsOrder1(fd, value, enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}
	}

	return nil
}

func compressRepeatedFieldVarintModelsOrder1(fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	length := list.Len()
	if err := encodeVarintWithModels(uint64(length), enc, vm); err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < length; i++ {
		value := list.Get(i)
		if err := compressFieldValueVarintModelsOrder1(fd, value, enc, mb, vm); err != nil {
			return fmt.Errorf("list element %d: %w", i, err)
		}
	}

	return nil
}

func compressMapFieldVarintModelsOrder1(fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	length := m.Len()
	if err := encodeVarintWithModels(uint64(length), enc, vm); err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		if err := compressFieldValueVarintModelsOrder1(keyFd, k.Value(), enc, mb, vm); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if err := compressFieldValueVarintModelsOrder1(valueFd, v, enc, mb, vm); err != nil {
			encodeErr = fmt.Errorf("map value: %w", err)
			return false
		}

		return true
	})

	return encodeErr
}

func compressFieldValueVarintModelsOrder1(fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	switch fd.Kind() {
	case protoreflect.StringKind:
		str := value.String()
		var buf bytes.Buffer
		if err := arithcode.EncodeStringOrder1(str, &buf); err != nil {
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

	case protoreflect.MessageKind:
		return compressMessageVarintModelsOrder1(value.Message(), enc, mb, vm)

	default:
		return compressFieldValueVarintModels(fd, value, enc, mb, vm)
	}
}

func decompressMessageVarintModelsOrder1(msg protoreflect.Message, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
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
			if err := decompressRepeatedFieldVarintModelsOrder1(fd, list, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapFieldVarintModelsOrder1(fd, m, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessageVarintModelsOrder1(nestedMsg, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decompressFieldValueVarintModelsOrder1(fd, dec, mb, vm)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}
	}

	return nil
}

func decompressRepeatedFieldVarintModelsOrder1(fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
	length, err := decodeVarintWithModels(dec, vm)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessageVarintModelsOrder1(elem.Message(), dec, mb, vm); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decompressFieldValueVarintModelsOrder1(fd, dec, mb, vm)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

func decompressMapFieldVarintModelsOrder1(fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
	length, err := decodeVarintWithModels(dec, vm)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	for i := 0; i < int(length); i++ {
		key, err := decompressFieldValueVarintModelsOrder1(keyFd, dec, mb, vm)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		value, err := decompressFieldValueVarintModelsOrder1(valueFd, dec, mb, vm)
		if err != nil {
			return fmt.Errorf("map value %d: %w", i, err)
		}

		m.Set(key.MapKey(), value)
	}

	return nil
}

func decompressFieldValueVarintModelsOrder1(fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) (protoreflect.Value, error) {
	switch fd.Kind() {
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
		str, err := arithcode.DecodeStringOrder1(buf)
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfString(str), nil

	case protoreflect.MessageKind:
		return protoreflect.Value{}, fmt.Errorf("message kind should be handled separately")

	default:
		return decompressFieldValueVarintModels(fd, dec, mb, vm)
	}
}

// Order-2 implementation

func compressMessageVarintModelsOrder2(msg protoreflect.Message, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
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
			if err := compressRepeatedFieldVarintModelsOrder2(fd, value.List(), enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldVarintModelsOrder2(fd, value.Map(), enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueVarintModelsOrder2(fd, value, enc, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}
	}

	return nil
}

func compressRepeatedFieldVarintModelsOrder2(fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	length := list.Len()
	if err := encodeVarintWithModels(uint64(length), enc, vm); err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < length; i++ {
		value := list.Get(i)
		if err := compressFieldValueVarintModelsOrder2(fd, value, enc, mb, vm); err != nil {
			return fmt.Errorf("list element %d: %w", i, err)
		}
	}

	return nil
}

func compressMapFieldVarintModelsOrder2(fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	length := m.Len()
	if err := encodeVarintWithModels(uint64(length), enc, vm); err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		if err := compressFieldValueVarintModelsOrder2(keyFd, k.Value(), enc, mb, vm); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if err := compressFieldValueVarintModelsOrder2(valueFd, v, enc, mb, vm); err != nil {
			encodeErr = fmt.Errorf("map value: %w", err)
			return false
		}

		return true
	})

	return encodeErr
}

func compressFieldValueVarintModelsOrder2(fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mb *ModelBuilder, vm *varintByteModels) error {
	switch fd.Kind() {
	case protoreflect.StringKind:
		str := value.String()
		var buf bytes.Buffer
		if err := arithcode.EncodeStringOrder2(str, &buf); err != nil {
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

	case protoreflect.MessageKind:
		return compressMessageVarintModelsOrder2(value.Message(), enc, mb, vm)

	default:
		return compressFieldValueVarintModels(fd, value, enc, mb, vm)
	}
}

func decompressMessageVarintModelsOrder2(msg protoreflect.Message, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
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
			if err := decompressRepeatedFieldVarintModelsOrder2(fd, list, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapFieldVarintModelsOrder2(fd, m, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessageVarintModelsOrder2(nestedMsg, dec, mb, vm); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decompressFieldValueVarintModelsOrder2(fd, dec, mb, vm)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}
	}

	return nil
}

func decompressRepeatedFieldVarintModelsOrder2(fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
	length, err := decodeVarintWithModels(dec, vm)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessageVarintModelsOrder2(elem.Message(), dec, mb, vm); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decompressFieldValueVarintModelsOrder2(fd, dec, mb, vm)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

func decompressMapFieldVarintModelsOrder2(fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) error {
	length, err := decodeVarintWithModels(dec, vm)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	for i := 0; i < int(length); i++ {
		key, err := decompressFieldValueVarintModelsOrder2(keyFd, dec, mb, vm)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		value, err := decompressFieldValueVarintModelsOrder2(valueFd, dec, mb, vm)
		if err != nil {
			return fmt.Errorf("map value %d: %w", i, err)
		}

		m.Set(key.MapKey(), value)
	}

	return nil
}

func decompressFieldValueVarintModelsOrder2(fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mb *ModelBuilder, vm *varintByteModels) (protoreflect.Value, error) {
	switch fd.Kind() {
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
		str, err := arithcode.DecodeStringOrder2(buf)
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfString(str), nil

	case protoreflect.MessageKind:
		return protoreflect.Value{}, fmt.Errorf("message kind should be handled separately")

	default:
		return decompressFieldValueVarintModels(fd, dec, mb, vm)
	}
}
