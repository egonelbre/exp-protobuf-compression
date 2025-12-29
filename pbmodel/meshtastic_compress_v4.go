package pbmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MeshtasticCompressV4 adds enum value prediction on top of V1.
// Common enum values are encoded with just 1 bit instead of full enum encoding.
func MeshtasticCompressV4(msg proto.Message, w io.Writer) error {
	mmb := NewMeshtasticModelBuilderV4()
	enc := arithcode.NewEncoder(w)

	if err := meshtasticCompressMessageV4("", msg.ProtoReflect(), enc, mmb); err != nil {
		return err
	}

	return enc.Close()
}

// MeshtasticModelBuilderV4 extends the model builder with enum prediction.
type MeshtasticModelBuilderV4 struct {
	*MeshtasticModelBuilder
	enumPredictions map[string]protoreflect.EnumNumber
}

// NewMeshtasticModelBuilderV4 creates a builder with enum predictions.
func NewMeshtasticModelBuilderV4() *MeshtasticModelBuilderV4 {
	return &MeshtasticModelBuilderV4{
		MeshtasticModelBuilder: NewMeshtasticModelBuilder(),
		enumPredictions:        getCommonEnumValues(),
	}
}

// getCommonEnumValues returns the most common enum values in Meshtastic messages.
func getCommonEnumValues() map[string]protoreflect.EnumNumber {
	return map[string]protoreflect.EnumNumber{
		// Position.location_source - most positions from internal GPS
		"location_source": protoreflect.EnumNumber(meshtastic.Position_LOC_INTERNAL),

		// Position.altitude_source - most altitudes from internal GPS
		"altitude_source": protoreflect.EnumNumber(meshtastic.Position_ALT_INTERNAL),

		// User.hw_model - TBEAM is very common
		"hw_model": protoreflect.EnumNumber(meshtastic.HardwareModel_TBEAM),

		// Config.DeviceConfig.role - most nodes are CLIENT
		"role": protoreflect.EnumNumber(meshtastic.Config_DeviceConfig_CLIENT),

		// MeshPacket.priority - DEFAULT is most common
		"priority": protoreflect.EnumNumber(meshtastic.MeshPacket_DEFAULT),

		// MeshPacket.delayed - NO_DELAY is most common
		"delayed": protoreflect.EnumNumber(meshtastic.MeshPacket_NO_DELAY),
	}
}

// meshtasticCompressMessageV4 recursively compresses with enum prediction.
func meshtasticCompressMessageV4(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mmb *MeshtasticModelBuilderV4) error {
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
			if err := meshtasticCompressRepeatedFieldV4(currentPath, fd, value.List(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := meshtasticCompressMapFieldV4(currentPath, fd, value.Map(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessageV4(currentPath, value.Message(), enc, mmb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := meshtasticCompressFieldValueV4(currentPath, fd, value, enc, mmb); err != nil {
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

// meshtasticCompressRepeatedFieldV4 compresses repeated fields.
func meshtasticCompressRepeatedFieldV4(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mmb *MeshtasticModelBuilderV4) error {
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
			if err := meshtasticCompressMessageV4(elementPath, value.Message(), enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		} else {
			if err := meshtasticCompressFieldValueV4(elementPath, fd, value, enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	}

	return nil
}

// meshtasticCompressMapFieldV4 compresses map fields.
func meshtasticCompressMapFieldV4(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mmb *MeshtasticModelBuilderV4) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.byteModel
	}

	length := m.Len()
	lengthBytes := encodeVarint(uint64(length))
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
		if err := meshtasticCompressFieldValueV4(keyPath, keyFd, k.Value(), enc, mmb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if valueFd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessageV4(valuePath, v.Message(), enc, mmb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		} else {
			if err := meshtasticCompressFieldValueV4(valuePath, valueFd, v, enc, mmb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		}

		return true
	})

	return encodeErr
}

// meshtasticCompressFieldValueV4 compresses field values with enum prediction.
func meshtasticCompressFieldValueV4(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mmb *MeshtasticModelBuilderV4) error {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		data := value.Bytes()

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

		textFlag := 0
		if isText {
			textFlag = 1
		}
		if err := enc.Encode(textFlag, mmb.boolModel); err != nil {
			return err
		}

		if isText {
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

		// Check if this enum has a predicted common value
		fieldName := string(fd.Name())
		if predictedValue, hasPrediction := mmb.enumPredictions[fieldName]; hasPrediction {
			if enumValue == predictedValue {
				// Common value - encode just 1 bit
				return enc.Encode(1, mmb.boolModel)
			}
			// Not the common value - encode 0 bit + full value
			if err := enc.Encode(0, mmb.boolModel); err != nil {
				return err
			}
		}

		// Encode the actual enum value
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
		lengthBytes := encodeVarint(uint64(len(data)))
		for _, b := range lengthBytes {
			if err := enc.Encode(int(b), model); err != nil {
				return err
			}
		}
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
