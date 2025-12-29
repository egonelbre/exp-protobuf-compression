package pbmodel

import (
	"bytes"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
)

// CompressOrder1 compresses a protobuf message using arithmetic coding with order-1 string compression.
func CompressOrder1(msg proto.Message, w io.Writer) error {
	mb := NewModelBuilder()
	enc := arithcode.NewEncoder(w)

	if err := compressMessageOrder1(msg.ProtoReflect(), enc, mb); err != nil {
		return err
	}

	return enc.Close()
}

// DecompressOrder1 decompresses a protobuf message that was compressed with order-1 string compression.
func DecompressOrder1(r io.Reader, msg proto.Message) error {
	mb := NewModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessageOrder1(msg.ProtoReflect(), dec, mb)
}

// CompressOrder2 compresses a protobuf message using arithmetic coding with order-2 string compression.
func CompressOrder2(msg proto.Message, w io.Writer) error {
	mb := NewModelBuilder()
	enc := arithcode.NewEncoder(w)

	if err := compressMessageOrder2(msg.ProtoReflect(), enc, mb); err != nil {
		return err
	}

	return enc.Close()
}

// DecompressOrder2 decompresses a protobuf message that was compressed with order-2 string compression.
func DecompressOrder2(r io.Reader, msg proto.Message) error {
	mb := NewModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessageOrder2(msg.ProtoReflect(), dec, mb)
}

// compressMessageOrder1 is the same as compressMessage but uses order-1 strings
func compressMessageOrder1(msg protoreflect.Message, enc *arithcode.Encoder, mb *ModelBuilder) error {
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
			if err := compressRepeatedFieldOrder1(fd, value.List(), enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldOrder1(fd, value.Map(), enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueOrder1(fd, value, enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}
	}

	return nil
}

func compressRepeatedFieldOrder1(fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mb *ModelBuilder) error {
	length := list.Len()
	lengthBytes := EncodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), mb.varintModel); err != nil {
			return fmt.Errorf("list length: %w", err)
		}
	}

	for i := 0; i < length; i++ {
		value := list.Get(i)
		if err := compressFieldValueOrder1(fd, value, enc, mb); err != nil {
			return fmt.Errorf("list element %d: %w", i, err)
		}
	}

	return nil
}

func compressMapFieldOrder1(fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mb *ModelBuilder) error {
	length := m.Len()
	lengthBytes := EncodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), mb.varintModel); err != nil {
			return fmt.Errorf("map length: %w", err)
		}
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		if err := compressFieldValueOrder1(keyFd, k.Value(), enc, mb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if err := compressFieldValueOrder1(valueFd, v, enc, mb); err != nil {
			encodeErr = fmt.Errorf("map value: %w", err)
			return false
		}

		return true
	})

	return encodeErr
}

func compressFieldValueOrder1(fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mb *ModelBuilder) error {
	switch fd.Kind() {
	case protoreflect.StringKind:
		// Use order-1 English model for strings
		str := value.String()
		var buf bytes.Buffer
		if err := arithcode.EncodeStringOrder1(str, &buf); err != nil {
			return err
		}
		compressedBytes := buf.Bytes()
		lengthBytes := EncodeVarint(uint64(len(compressedBytes)))
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

	case protoreflect.MessageKind:
		return compressMessageOrder1(value.Message(), enc, mb)

	default:
		// For all other types, use the standard compression
		return compressFieldValue(fd, value, enc, mb)
	}
}

// compressMessageOrder2 is the same as compressMessage but uses order-2 strings
func compressMessageOrder2(msg protoreflect.Message, enc *arithcode.Encoder, mb *ModelBuilder) error {
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
			if err := compressRepeatedFieldOrder2(fd, value.List(), enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldOrder2(fd, value.Map(), enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueOrder2(fd, value, enc, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}
	}

	return nil
}

func compressRepeatedFieldOrder2(fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mb *ModelBuilder) error {
	length := list.Len()
	lengthBytes := EncodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), mb.varintModel); err != nil {
			return fmt.Errorf("list length: %w", err)
		}
	}

	for i := 0; i < length; i++ {
		value := list.Get(i)
		if err := compressFieldValueOrder2(fd, value, enc, mb); err != nil {
			return fmt.Errorf("list element %d: %w", i, err)
		}
	}

	return nil
}

func compressMapFieldOrder2(fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mb *ModelBuilder) error {
	length := m.Len()
	lengthBytes := EncodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), mb.varintModel); err != nil {
			return fmt.Errorf("map length: %w", err)
		}
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		if err := compressFieldValueOrder2(keyFd, k.Value(), enc, mb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if err := compressFieldValueOrder2(valueFd, v, enc, mb); err != nil {
			encodeErr = fmt.Errorf("map value: %w", err)
			return false
		}

		return true
	})

	return encodeErr
}

func compressFieldValueOrder2(fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mb *ModelBuilder) error {
	switch fd.Kind() {
	case protoreflect.StringKind:
		// Use order-2 English model for strings
		str := value.String()
		var buf bytes.Buffer
		if err := arithcode.EncodeStringOrder2(str, &buf); err != nil {
			return err
		}
		compressedBytes := buf.Bytes()
		lengthBytes := EncodeVarint(uint64(len(compressedBytes)))
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

	case protoreflect.MessageKind:
		return compressMessageOrder2(value.Message(), enc, mb)

	default:
		// For all other types, use the standard compression
		return compressFieldValue(fd, value, enc, mb)
	}
}

// Decompression functions

func decompressMessageOrder1(msg protoreflect.Message, dec *arithcode.Decoder, mb *ModelBuilder) error {
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
			if err := decompressRepeatedFieldOrder1(fd, list, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapFieldOrder1(fd, m, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			// For message fields, decompress directly into the mutable field
			// to preserve the concrete type
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessageOrder1(nestedMsg, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decompressFieldValueOrder1(fd, dec, mb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}
	}

	return nil
}

func decompressRepeatedFieldOrder1(fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mb *ModelBuilder) error {
	length, err := decodeVarintFromDecoder(dec, mb.varintModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessageOrder1(elem.Message(), dec, mb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decompressFieldValueOrder1(fd, dec, mb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

func decompressMapFieldOrder1(fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mb *ModelBuilder) error {
	length, err := decodeVarintFromDecoder(dec, mb.varintModel)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	for i := 0; i < int(length); i++ {
		key, err := decompressFieldValueOrder1(keyFd, dec, mb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		value, err := decompressFieldValueOrder1(valueFd, dec, mb)
		if err != nil {
			return fmt.Errorf("map value %d: %w", i, err)
		}

		m.Set(key.MapKey(), value)
	}

	return nil
}

func decompressFieldValueOrder1(fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mb *ModelBuilder) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		// Decode length
		length, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode compressed bytes
		compressedBytes := make([]byte, length)
		for i := range compressedBytes {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			compressedBytes[i] = byte(b)
		}

		// Decompress string using order-1
		buf := bytes.NewBuffer(compressedBytes)
		str, err := arithcode.DecodeStringOrder1(buf)
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfString(str), nil

	case protoreflect.MessageKind:
		msgDesc := fd.Message()
		msg := dynamicpb.NewMessage(msgDesc)
		if err := decompressMessageOrder1(msg, dec, mb); err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(msg), nil

	default:
		// For all other types, use standard decompression
		return decompressFieldValue(fd, dec, mb)
	}
}

func decompressMessageOrder2(msg protoreflect.Message, dec *arithcode.Decoder, mb *ModelBuilder) error {
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
			if err := decompressRepeatedFieldOrder2(fd, list, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapFieldOrder2(fd, m, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			// For message fields, decompress directly into the mutable field
			// to preserve the concrete type
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessageOrder2(nestedMsg, dec, mb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decompressFieldValueOrder2(fd, dec, mb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}
	}

	return nil
}

func decompressRepeatedFieldOrder2(fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mb *ModelBuilder) error {
	length, err := decodeVarintFromDecoder(dec, mb.varintModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessageOrder2(elem.Message(), dec, mb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decompressFieldValueOrder2(fd, dec, mb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

func decompressMapFieldOrder2(fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mb *ModelBuilder) error {
	length, err := decodeVarintFromDecoder(dec, mb.varintModel)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	for i := 0; i < int(length); i++ {
		key, err := decompressFieldValueOrder2(keyFd, dec, mb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		value, err := decompressFieldValueOrder2(valueFd, dec, mb)
		if err != nil {
			return fmt.Errorf("map value %d: %w", i, err)
		}

		m.Set(key.MapKey(), value)
	}

	return nil
}

func decompressFieldValueOrder2(fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mb *ModelBuilder) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		// Decode length
		length, err := decodeVarintFromDecoder(dec, mb.varintModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		// Decode compressed bytes
		compressedBytes := make([]byte, length)
		for i := range compressedBytes {
			b, err := dec.Decode(mb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			compressedBytes[i] = byte(b)
		}

		// Decompress string using order-2
		buf := bytes.NewBuffer(compressedBytes)
		str, err := arithcode.DecodeStringOrder2(buf)
		if err != nil {
			return protoreflect.Value{}, err
		}

		return protoreflect.ValueOfString(str), nil

	case protoreflect.MessageKind:
		msgDesc := fd.Message()
		msg := dynamicpb.NewMessage(msgDesc)
		if err := decompressMessageOrder2(msg, dec, mb); err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(msg), nil

	default:
		// For all other types, use standard decompression
		return decompressFieldValue(fd, dec, mb)
	}
}
