package meshtasticmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
)

// CompressV5 uses context-aware models that are optimized for specific
// field types and value ranges commonly found in Meshtastic messages.
func CompressV5(msg proto.Message, w io.Writer) error {
	mcb := NewContextualModelBuilder()
	enc := arithcode.NewEncoder(w)

	// Set initial message type context
	msgType := string(msg.ProtoReflect().Descriptor().Name())
	mcb.SetMessageType(msgType)

	if err := compressMessageV5("", msg.ProtoReflect(), enc, mcb); err != nil {
		return err
	}

	return enc.Close()
}

// compressMessageV5 recursively compresses with context-aware models.
func compressMessageV5(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Update message type context
	prevMsgType := mcb.messageType
	mcb.SetMessageType(string(md.Name()))
	defer func() { mcb.messageType = prevMsgType }()

	// Iterate through all fields in order
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		currentPath := pbmodel.BuildFieldPath(fieldPath, string(fd.Name()))

		if !msg.Has(fd) {
			// Field not set, encode a "not present" marker
			if err := enc.Encode(0, mcb.BoolModel()); err != nil {
				return fmt.Errorf("field %s presence: %w", fd.Name(), err)
			}
			continue
		}

		// Field is present
		if err := enc.Encode(1, mcb.BoolModel()); err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		value := msg.Get(fd)

		// Track portnum for payload detection
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal := value.Enum()
			portNum := meshtastic.PortNum(enumVal)
			mcb.currentPortNum = &portNum
		}

		if fd.IsList() {
			if err := compressRepeatedFieldV5(currentPath, fd, value.List(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldV5(currentPath, fd, value.Map(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			if err := compressMessageV5(currentPath, value.Message(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueV5(currentPath, fd, value, enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		}

		// Reset portnum after processing Data message
		if md.Name() == "Data" && i == fields.Len()-1 {
			mcb.currentPortNum = nil
		}
	}

	return nil
}

// compressRepeatedFieldV5 compresses repeated fields with context-aware models.
func compressRepeatedFieldV5(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.ByteModel()
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
			if err := compressMessageV5(elementPath, value.Message(), enc, mcb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		} else {
			if err := compressFieldValueV5(elementPath, fd, value, enc, mcb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	}

	return nil
}

// compressMapFieldV5 compresses map fields with context-aware models.
func compressMapFieldV5(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.ByteModel()
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
		if err := compressFieldValueV5(keyPath, keyFd, k.Value(), enc, mcb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if valueFd.Kind() == protoreflect.MessageKind {
			if err := compressMessageV5(valuePath, v.Message(), enc, mcb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		} else {
			if err := compressFieldValueV5(valuePath, valueFd, v, enc, mcb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		}

		return true
	})

	return encodeErr
}

// compressFieldValueV5 compresses field values with context-aware models.
func compressFieldValueV5(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		data := value.Bytes()

		isText := mcb.currentPortNum != nil && *mcb.currentPortNum == meshtastic.PortNum_TEXT_MESSAGE_APP

		if !isText && utf8.Valid(data) {
			printableCount := 0
			for _, b := range data {
				if b >= 32 && b <= 126 || b == '\n' || b == '\r' || b == '\t' {
					printableCount++
				}
			}
			isText = len(data) > 0 && float64(printableCount)/float64(len(data)) > 0.8
		}

		textFlag := 0
		if isText {
			textFlag = 1
		}
		if err := enc.Encode(textFlag, mcb.BoolModel()); err != nil {
			return err
		}

		if isText {
			str := string(data)
			var buf bytes.Buffer
			if err := arithcode.EncodeString(str, &buf); err != nil {
				return err
			}
			compressedBytes := buf.Bytes()
			lengthBytes := pbmodel.EncodeVarint(uint64(len(compressedBytes)))
			for _, b := range lengthBytes {
				if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
					return err
				}
			}
			for _, b := range compressedBytes {
				if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
					return err
				}
			}
			return nil
		}
	}

	// Get context-aware model for this specific field
	model := mcb.GetContextualFieldModel(fieldPath, fd)

	switch fd.Kind() {
	case protoreflect.BoolKind:
		b := 0
		if value.Bool() {
			b = 1
		}
		return enc.Encode(b, model)

	case protoreflect.EnumKind:
		enumValue := value.Enum()

		// Check if this enum has a predicted common value (from V4)
		fieldName := string(fd.Name())
		if predictedValue, hasPrediction := mcb.enumPredictions[fieldName]; hasPrediction {
			if enumValue == predictedValue {
				return enc.Encode(1, mcb.BoolModel())
			}
			if err := enc.Encode(0, mcb.BoolModel()); err != nil {
				return err
			}
		}

		enumDesc := fd.Enum()
		enumValueDesc := enumDesc.Values().ByNumber(enumValue)
		if enumValueDesc == nil {
			return fmt.Errorf("unknown enum value: %d", enumValue)
		}
		idx := enumValueDesc.Index()
		return enc.Encode(idx, mcb.GetEnumModel(fieldPath, enumDesc))

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
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed32Kind:
		val := int32(value.Int())
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, uint32(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Fixed64Kind:
		val := value.Uint()
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.Sfixed64Kind:
		val := int64(value.Int())
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, uint64(val))
		for _, b := range bytes {
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.FloatKind:
		val := float32(value.Float())
		bits := math.Float32bits(val)
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		// Use context-aware model if available
		if model != nil && model != mcb.BoolModel() {
			for _, b := range bytes {
				if err := enc.Encode(int(b), model); err != nil {
					return err
				}
			}
		} else {
			for _, b := range bytes {
				if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
					return err
				}
			}
		}
		return nil

	case protoreflect.DoubleKind:
		val := value.Float()
		bits := math.Float64bits(val)
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, bits)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
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
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
				return err
			}
		}
		for _, b := range compressedBytes {
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
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
			if err := enc.Encode(int(b), mcb.ByteModel()); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
