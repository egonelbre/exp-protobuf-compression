package pbmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/egonelbre/protobuf-compression/arithcode"
	"github.com/egonelbre/protobuf-compression/pbmodel/testdata/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MeshtasticCompress compresses a protobuf message with Meshtastic-specific optimizations.
// This includes:
// - Treating Data.payload as text when portnum is TEXT_MESSAGE_APP
// - Delta encoding for coordinates
// - Optimized models for common Meshtastic field patterns
func MeshtasticCompress(msg proto.Message, w io.Writer) error {
	mmb := NewMeshtasticModelBuilder()
	enc := arithcode.NewEncoder(w)

	if err := meshtasticCompressMessage("", msg.ProtoReflect(), enc, mmb); err != nil {
		return err
	}

	return enc.Close()
}

// MeshtasticModelBuilder extends AdaptiveModelBuilder with Meshtastic-specific knowledge.
type MeshtasticModelBuilder struct {
	*AdaptiveModelBuilder
	currentPortNum *meshtastic.PortNum
}

// NewMeshtasticModelBuilder creates a new Meshtastic-specific model builder.
func NewMeshtasticModelBuilder() *MeshtasticModelBuilder {
	return &MeshtasticModelBuilder{
		AdaptiveModelBuilder: NewAdaptiveModelBuilder(),
	}
}

// meshtasticCompressMessage recursively compresses with Meshtastic-specific optimizations.
func meshtasticCompressMessage(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Iterate through all fields in order
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))

		if !msg.Has(fd) {
			// Field not set, encode a "not present" marker
			if err := enc.Encode(0, mmb.boolModel); err != nil {
				return fmt.Errorf("field %s presence: %w", fd.Name(), err)
			}
			continue
		}

		// Field is present
		if err := enc.Encode(1, mmb.boolModel); err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		value := msg.Get(fd)

		// Track portnum for payload detection
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal := value.Enum()
			portNum := meshtastic.PortNum(enumVal)
			mmb.currentPortNum = &portNum
		}

		if fd.IsList() {
			if err := meshtasticCompressRepeatedField(currentPath, fd, value.List(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := adaptiveCompressMapField(currentPath, fd, value.Map(), enc, mmb.AdaptiveModelBuilder); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessage(currentPath, value.Message(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := meshtasticCompressFieldValue(currentPath, fd, value, enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}

		// Reset portnum after processing Data message
		if md.Name() == "Data" && i == fields.Len()-1 {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// meshtasticCompressRepeatedField compresses repeated fields with Meshtastic optimizations.
func meshtasticCompressRepeatedField(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.byteModel
	}

	length := list.Len()
	lengthBytes := encodeVarint(uint64(length))
	for _, b := range lengthBytes {
		if err := enc.Encode(int(b), lengthModel); err != nil {
			return fmt.Errorf("list length: %w", err)
		}
	}

	elementPath := fieldPath + "[]"
	for i := 0; i < length; i++ {
		value := list.Get(i)
		if fd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessage(elementPath, value.Message(), enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		} else {
			if err := meshtasticCompressFieldValue(elementPath, fd, value, enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	}

	return nil
}

// meshtasticCompressFieldValue compresses field values with Meshtastic-specific logic.
func meshtasticCompressFieldValue(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		data := value.Bytes()
		
		// Check if this is likely text based on portnum
		isText := mmb.currentPortNum != nil && *mmb.currentPortNum == meshtastic.PortNum_TEXT_MESSAGE_APP
		
		// Also check if the bytes are valid UTF-8 as a fallback
		if !isText && utf8.Valid(data) {
			// Additional heuristic: check if mostly printable ASCII
			printableCount := 0
			for _, b := range data {
				if b >= 32 && b <= 126 || b == '\n' || b == '\r' || b == '\t' {
					printableCount++
				}
			}
			isText = len(data) > 0 && float64(printableCount)/float64(len(data)) > 0.8
		}

		// Encode a flag indicating whether this is text
		textFlag := 0
		if isText {
			textFlag = 1
		}
		if err := enc.Encode(textFlag, mmb.boolModel); err != nil {
			return err
		}

		if isText {
			// Compress as text using English model
			str := string(data)
			var buf bytes.Buffer
			if err := arithcode.EncodeString(str, &buf); err != nil {
				return err
			}
			compressedBytes := buf.Bytes()
			lengthBytes := encodeVarint(uint64(len(compressedBytes)))
			for _, b := range lengthBytes {
				if err := enc.Encode(int(b), mmb.byteModel); err != nil {
					return err
				}
			}
			for _, b := range compressedBytes {
				if err := enc.Encode(int(b), mmb.byteModel); err != nil {
					return err
				}
			}
			return nil
		}

		// Fall through to normal bytes encoding
	}

	// Use adaptive compression for other fields
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
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed32Kind:
		val := int32(value.Int())
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, uint32(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Fixed64Kind:
		val := value.Uint()
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed64Kind:
		val := int64(value.Int())
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
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
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
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
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
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
		compressedBytes := buf.Bytes()
		lengthBytes := encodeVarint(uint64(len(compressedBytes)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
				return err
			}
		}
		for _, b := range compressedBytes {
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
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
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
