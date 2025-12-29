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

// MeshtasticCompressV8 uses varint byte models on top of V7's field-specific boolean models.
func MeshtasticCompressV8(msg proto.Message, w io.Writer) error {
	mcb := NewMeshtasticContextualModelBuilder()
	enc := arithcode.NewEncoder(w)

	// Set initial message type context
	msgType := string(msg.ProtoReflect().Descriptor().Name())
	mcb.SetMessageType(msgType)

	if err := meshtasticCompressMessageV8("", msg.ProtoReflect(), enc, mcb); err != nil {
		return err
	}

	return enc.Close()
}

// encodeVarintWithModels encodes a varint using position-specific byte models.
func encodeVarintWithModels(value uint64, enc *arithcode.Encoder, mcb *MeshtasticContextualModelBuilder) error {
	varintBytes := encodeVarint(value)
	for i, b := range varintBytes {
		model := mcb.GetVarintByteModel(i)
		if err := enc.Encode(int(b), model); err != nil {
			return err
		}
	}
	return nil
}

// meshtasticCompressMessageV8 recursively compresses with field-specific boolean models.
func meshtasticCompressMessageV8(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mcb *MeshtasticContextualModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Update message type context
	prevMsgType := mcb.messageType
	mcb.SetMessageType(string(md.Name()))
	defer func() { mcb.messageType = prevMsgType }()

	// Iterate through all fields in order
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))
		fieldName := string(fd.Name())

		if !msg.Has(fd) {
			// Field not set, encode a "not present" marker
			// Use field-specific boolean model for presence bits
			presenceModel := mcb.GetBooleanModel(fieldName + "_presence")
			if err := enc.Encode(0, presenceModel); err != nil {
				return fmt.Errorf("field %s presence: %w", fd.Name(), err)
			}
			continue
		}

		// Field is present
		presenceModel := mcb.GetBooleanModel(fieldName + "_presence")
		if err := enc.Encode(1, presenceModel); err != nil {
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
			if err := meshtasticCompressRepeatedFieldV8(currentPath, fd, value.List(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := meshtasticCompressMapFieldV8(currentPath, fd, value.Map(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessageV8(currentPath, value.Message(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := meshtasticCompressFieldValueV8(currentPath, fd, value, enc, mcb); err != nil {
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

// meshtasticCompressRepeatedFieldV8 compresses repeated fields.
func meshtasticCompressRepeatedFieldV8(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mcb *MeshtasticContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.byteModel
	}

	length := list.Len()
	if err := encodeVarintWithModels(uint64(length), enc, mcb); err != nil {
		return fmt.Errorf("length: %w", err)
	}

	for i := 0; i < length; i++ {
		elemPath := fmt.Sprintf("%s[%d]", fieldPath, i)
		value := list.Get(i)

		if fd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessageV8(elemPath, value.Message(), enc, mcb); err != nil {
				return fmt.Errorf("element %d: %w", i, err)
			}
		} else {
			if err := meshtasticCompressFieldValueV8(elemPath, fd, value, enc, mcb); err != nil {
				return fmt.Errorf("element %d: %w", i, err)
			}
		}
	}

	return nil
}

// meshtasticCompressMapFieldV8 compresses map fields.
func meshtasticCompressMapFieldV8(fieldPath string, fd protoreflect.FieldDescriptor, mapVal protoreflect.Map, enc *arithcode.Encoder, mcb *MeshtasticContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.byteModel
	}

	length := mapVal.Len()
	if err := encodeVarintWithModels(uint64(length), enc, mcb); err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	var keys []protoreflect.MapKey
	mapVal.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		keys = append(keys, k)
		return true
	})

	for i, k := range keys {
		keyPath := fmt.Sprintf("%s._key[%d]", fieldPath, i)
		valuePath := fmt.Sprintf("%s._value[%d]", fieldPath, i)

		if err := meshtasticCompressFieldValueV8(keyPath, keyFd, k.Value(), enc, mcb); err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		v := mapVal.Get(k)
		if valueFd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessageV8(valuePath, v.Message(), enc, mcb); err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
		} else {
			if err := meshtasticCompressFieldValueV8(valuePath, valueFd, v, enc, mcb); err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
		}
	}

	return nil
}

// meshtasticCompressFieldValueV8 compresses a single field value with field-specific models.
func meshtasticCompressFieldValueV8(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mcb *MeshtasticContextualModelBuilder) error {
	model := mcb.GetContextualFieldModel(fieldPath, fd)
	if model == nil {
		model = mcb.GetFieldModel(fieldPath, fd)
	}

	fieldName := string(fd.Name())

	// Handle special case for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		data := value.Bytes()
		isText := mcb.currentPortNum != nil && *mcb.currentPortNum == meshtastic.PortNum_TEXT_MESSAGE_APP
		textFlag := 0
		if isText {
			textFlag = 1
		}
		textModel := mcb.GetBooleanModel("payload_is_text")
		if err := enc.Encode(textFlag, textModel); err != nil {
			return err
		}

		if isText && utf8.Valid(data) {
			var buf bytes.Buffer
			if err := arithcode.EncodeString(string(data), &buf); err != nil {
				return err
			}
			compressedBytes := buf.Bytes()
			if err := encodeVarintWithModels(uint64(len(compressedBytes)), enc, mcb); err != nil {
				return err
			}
			for _, b := range compressedBytes {
				if err := enc.Encode(int(b), mcb.byteModel); err != nil {
					return err
				}
			}
		} else {
			if err := encodeVarintWithModels(uint64(len(data)), enc, mcb); err != nil {
				return err
			}
			for _, b := range data {
				if err := enc.Encode(int(b), mcb.byteModel); err != nil {
					return err
				}
			}
		}
		return nil
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		// Use field-specific boolean model
		boolModel := mcb.GetBooleanModel(fieldName)
		b := 0
		if value.Bool() {
			b = 1
		}
		return enc.Encode(b, boolModel)

	case protoreflect.EnumKind:
		enumValue := value.Enum()
		// Check if we have a prediction for this enum
		if predictedValue, hasPrediction := mcb.enumPredictions[fieldName]; hasPrediction {
			if enumValue == predictedValue {
				predModel := mcb.GetBooleanModel(fieldName + "_is_predicted")
				return enc.Encode(1, predModel)
			}
			predModel := mcb.GetBooleanModel(fieldName + "_is_predicted")
			if err := enc.Encode(0, predModel); err != nil {
				return err
			}
		}

		enumIndex := 0
		ed := fd.Enum()
		for i := 0; i < ed.Values().Len(); i++ {
			if ed.Values().Get(i).Number() == enumValue {
				enumIndex = i
				break
			}
		}

		enumModel := mcb.GetEnumModel(fieldPath, ed)
		return enc.Encode(enumIndex, enumModel)

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		var uintVal uint64
		switch fd.Kind() {
		case protoreflect.Int32Kind:
			uintVal = uint64(value.Int())
		case protoreflect.Int64Kind:
			uintVal = uint64(value.Int())
		case protoreflect.Uint32Kind:
			uintVal = value.Uint()
		case protoreflect.Uint64Kind:
			uintVal = value.Uint()
		}

		return encodeVarintWithModels(uintVal, enc, mcb)

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		signedVal := value.Int()
		zigzagVal := zigzagEncode(signedVal)
		return encodeVarintWithModels(zigzagVal, enc, mcb)

	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		var val uint32
		if fd.Kind() == protoreflect.Fixed32Kind {
			val = uint32(value.Uint())
		} else {
			val = uint32(value.Int())
		}
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, val)
		if model != nil && model != mcb.boolModel {
			for _, b := range bytes {
				if err := enc.Encode(int(b), model); err != nil {
					return err
				}
			}
		} else {
			for _, b := range bytes {
				if err := enc.Encode(int(b), mcb.byteModel); err != nil {
					return err
				}
			}
		}
		return nil

	case protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		var val uint64
		if fd.Kind() == protoreflect.Fixed64Kind {
			val = value.Uint()
		} else {
			val = uint64(value.Int())
		}
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, val)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mcb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.FloatKind:
		bits := math.Float32bits(float32(value.Float()))
		bytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(bytes, bits)
		if model != nil && model != mcb.boolModel {
			for _, b := range bytes {
				if err := enc.Encode(int(b), model); err != nil {
					return err
				}
			}
		} else {
			for _, b := range bytes {
				if err := enc.Encode(int(b), mcb.byteModel); err != nil {
					return err
				}
			}
		}
		return nil

	case protoreflect.DoubleKind:
		bits := math.Float64bits(value.Float())
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, bits)
		for _, b := range bytes {
			if err := enc.Encode(int(b), mcb.byteModel); err != nil {
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
		if err := encodeVarintWithModels(uint64(len(compressedBytes)), enc, mcb); err != nil {
			return err
		}
		for _, b := range compressedBytes {
			if err := enc.Encode(int(b), mcb.byteModel); err != nil {
				return err
			}
		}
		return nil

	case protoreflect.BytesKind:
		data := value.Bytes()
		if err := encodeVarintWithModels(uint64(len(data)), enc, mcb); err != nil {
			return err
		}
		for _, b := range data {
			if err := enc.Encode(int(b), mcb.byteModel); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported field type: %v", fd.Kind())
	}
}
