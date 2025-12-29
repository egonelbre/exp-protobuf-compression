package meshtasticmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// CompressV2 compresses a protobuf message with optimized field encoding.
// Instead of encoding a presence bit for each field, it encodes only present fields
// using delta-encoded field numbers, significantly reducing overhead for sparse messages.
func CompressV2(msg proto.Message, w io.Writer) error {
	mmb := NewModelBuilder()
	enc := arithcode.NewEncoder(w)

	if err := compressMessageV2("", msg.ProtoReflect(), enc, mmb); err != nil {
		return err
	}

	return enc.Close()
}

// compressMessageV2 recursively compresses with delta-encoded field numbers.
func compressMessageV2(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mmb *ModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Collect present fields
	var presentFields []protoreflect.FieldDescriptor
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if msg.Has(fd) {
			presentFields = append(presentFields, fd)
		}
	}

	// Encode number of present fields
	numPresent := len(presentFields)
	numPresentBytes := pbmodel.EncodeVarint(uint64(numPresent))
	for _, b := range numPresentBytes {
		if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
			return fmt.Errorf("num present fields: %w", err)
		}
	}

	// Encode present fields using delta encoding
	lastFieldNum := 0
	for _, fd := range presentFields {
		currentPath := pbmodel.BuildFieldPath(fieldPath, string(fd.Name()))
		fieldNum := int(fd.Number())

		// Encode delta from last field number
		delta := fieldNum - lastFieldNum
		if delta < 0 {
			return fmt.Errorf("fields not in order: %d after %d", fieldNum, lastFieldNum)
		}
		deltaBytes := pbmodel.EncodeVarint(uint64(delta))
		for _, b := range deltaBytes {
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
				return fmt.Errorf("field delta: %w", err)
			}
		}
		lastFieldNum = fieldNum

		// Track portnum for payload detection
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			value := msg.Get(fd)
			enumVal := value.Enum()
			portNum := meshtastic.PortNum(enumVal)
			mmb.currentPortNum = &portNum
		}

		// Encode field value
		value := msg.Get(fd)
		if fd.IsList() {
			if err := compressRepeatedFieldV2(currentPath, fd, value.List(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldV2(currentPath, fd, value.Map(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			if err := compressMessageV2(currentPath, value.Message(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueV2(currentPath, fd, value, enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}

		// Reset portnum after processing Data message
		if md.Name() == "Data" && fd == presentFields[len(presentFields)-1] {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// compressRepeatedFieldV2 compresses repeated fields.
func compressRepeatedFieldV2(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mmb *ModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.ByteModel()
	}

	length := list.Len()
	lengthBytes := pbmodel.EncodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), lengthModel); err != nil {
			return fmt.Errorf("list length: %w", err)
		}
	}

	elementPath := fieldPath + "[]"
	for i := 0; i < length; i++ {
		value := list.Get(i)
		if fd.Kind() == protoreflect.MessageKind {
			if err := compressMessageV2(elementPath, value.Message(), enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		} else {
			if err := compressFieldValueV2(elementPath, fd, value, enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	}

	return nil
}

// compressMapFieldV2 compresses map fields.
func compressMapFieldV2(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mmb *ModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.ByteModel()
	}

	length := m.Len()
	lengthBytes := pbmodel.EncodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), lengthModel); err != nil {
			return fmt.Errorf("map length: %w", err)
		}
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	var encodeErr error
	m.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		if err := compressFieldValueV2(keyPath, keyFd, k.Value(), enc, mmb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if valueFd.Kind() == protoreflect.MessageKind {
			if err := compressMessageV2(valuePath, v.Message(), enc, mmb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		} else {
			if err := compressFieldValueV2(valuePath, valueFd, v, enc, mmb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		}

		return true
	})

	return encodeErr
}

// compressFieldValueV2 compresses field values with Meshtastic-specific logic.
func compressFieldValueV2(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mmb *ModelBuilder) error {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		data := value.Bytes()

		// Check if this is likely text
		isText := mmb.currentPortNum != nil && *mmb.currentPortNum == meshtastic.PortNum_TEXT_MESSAGE_APP

		if !isText && utf8.Valid(data) {
			printableCount := 0
			for _, b := range data {
				if b >= 32 && b <= 126 || b == '\n' || b == '\r' || b == '\t' {
					printableCount++
				}
			}
			isText = len(data) > 0 && float64(printableCount)/float64(len(data)) > 0.8
		}

		// Encode text flag
		textFlag := 0
		if isText {
			textFlag = 1
		}
		if err := enc.Encode(textFlag, mmb.BoolModel()); err != nil {
			return err
		}

		if isText {
			// Compress as text
			str := string(data)
			var buf bytes.Buffer
			if err := arithcode.EncodeString(str, &buf); err != nil {
				return err
			}
			compressedBytes := buf.Bytes()
			lengthBytes := pbmodel.EncodeVarint(uint64(len(compressedBytes)))
			for _, b := range lengthBytes {
				if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
					return err
				}
			}
			for _, b := range compressedBytes {
				if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
					return err
				}
			}
			return nil
		}
	}

	model := mmb.GetFieldModel(fieldPath, fd)

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
		return enc.Encode(idx, mmb.GetEnumModel(fieldPath, enumDesc))

	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		val := value.Int()
		bytes := pbmodel.EncodeVarint(uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		val := value.Uint()
		bytes := pbmodel.EncodeVarint(val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		val := value.Int()
		zigzag := pbmodel.ZigzagEncode(val)
		bytes := pbmodel.EncodeVarint(zigzag)
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
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed32Kind:
		val := int32(value.Int())
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, uint32(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Fixed64Kind:
		val := value.Uint()
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed64Kind:
		val := int64(value.Int())
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
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
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
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
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
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
		lengthBytes := pbmodel.EncodeVarint(uint64(len(compressedBytes)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
				return err
			}
		}
		for _, b := range compressedBytes {
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.BytesKind:
		data := value.Bytes()
		lengthBytes := pbmodel.EncodeVarint(uint64(len(data)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
		for _, b := range data {
			if err := enc.Encode(int(b), mmb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
