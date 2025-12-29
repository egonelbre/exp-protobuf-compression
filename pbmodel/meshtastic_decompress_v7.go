package pbmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MeshtasticDecompressV7 decompresses a message using field-specific boolean models.
func MeshtasticDecompressV7(r io.Reader, msg proto.Message) error {
	mcb := NewMeshtasticContextualModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	msgType := string(msg.ProtoReflect().Descriptor().Name())
	mcb.SetMessageType(msgType)

	return meshtasticDecompressMessageV7("", msg.ProtoReflect(), dec, mcb)
}

// meshtasticDecompressMessageV7 recursively decompresses a message.
func meshtasticDecompressMessageV7(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	prevMsgType := mcb.messageType
	mcb.SetMessageType(string(md.Name()))
	defer func() { mcb.messageType = prevMsgType }()

	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))
		fieldName := string(fd.Name())

		// Check if field is present
		presenceModel := mcb.GetBooleanModel(fieldName + "_presence")
		present, err := dec.Decode(presenceModel)
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		if present == 0 {
			continue
		}

		// Track portnum for payload detection
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			// We'll decode the value and then set currentPortNum
			defer func() {
				if msg.Has(fd) {
					enumVal := msg.Get(fd).Enum()
					portNum := meshtastic.PortNum(enumVal)
					mcb.currentPortNum = &portNum
				}
			}()
		}

		if fd.IsList() {
			list := msg.Mutable(fd).List()
			if err := meshtasticDecompressRepeatedFieldV7(currentPath, fd, list, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			mapVal := msg.Mutable(fd).Map()
			if err := meshtasticDecompressMapFieldV7(currentPath, fd, mapVal, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			subMsg := msg.Mutable(fd).Message()
			if err := meshtasticDecompressMessageV7(currentPath, subMsg, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := meshtasticDecompressFieldValueV7(currentPath, fd, dec, mcb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)
		}

		if md.Name() == "Data" && i == fields.Len()-1 {
			mcb.currentPortNum = nil
		}
	}

	return nil
}

// meshtasticDecompressRepeatedFieldV7 decompresses a repeated field.
func meshtasticDecompressRepeatedFieldV7(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.byteModel
	}

	var lengthBytes []byte
	for {
		symbol, err := dec.Decode(lengthModel)
		if err != nil {
			return fmt.Errorf("length: %w", err)
		}
		lengthBytes = append(lengthBytes, byte(symbol))
		if symbol < 128 {
			break
		}
	}
	length := int(decodeVarint(lengthBytes))

	for i := 0; i < length; i++ {
		elemPath := fmt.Sprintf("%s[%d]", fieldPath, i)

		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := meshtasticDecompressMessageV7(elemPath, elem.Message(), dec, mcb); err != nil {
				return fmt.Errorf("element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := meshtasticDecompressFieldValueV7(elemPath, fd, dec, mcb)
			if err != nil {
				return fmt.Errorf("element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// meshtasticDecompressMapFieldV7 decompresses a map field.
func meshtasticDecompressMapFieldV7(fieldPath string, fd protoreflect.FieldDescriptor, mapVal protoreflect.Map, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.byteModel
	}

	var lengthBytes []byte
	for {
		symbol, err := dec.Decode(lengthModel)
		if err != nil {
			return fmt.Errorf("map length: %w", err)
		}
		lengthBytes = append(lengthBytes, byte(symbol))
		if symbol < 128 {
			break
		}
	}
	length := int(decodeVarint(lengthBytes))

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()

	for i := 0; i < length; i++ {
		keyPath := fmt.Sprintf("%s._key[%d]", fieldPath, i)
		valuePath := fmt.Sprintf("%s._value[%d]", fieldPath, i)

		keyValue, err := meshtasticDecompressFieldValueV7(keyPath, keyFd, dec, mcb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		var value protoreflect.Value
		if valueFd.Kind() == protoreflect.MessageKind {
			valueMsg := mapVal.NewValue()
			if err := meshtasticDecompressMessageV7(valuePath, valueMsg.Message(), dec, mcb); err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			value = valueMsg
		} else {
			var err error
			value, err = meshtasticDecompressFieldValueV7(valuePath, valueFd, dec, mcb)
			if err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
		}

		mapVal.Set(keyValue.MapKey(), value)
	}

	return nil
}

// meshtasticDecompressFieldValueV7 decompresses a single field value.
func meshtasticDecompressFieldValueV7(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) (protoreflect.Value, error) {
	model := mcb.GetContextualFieldModel(fieldPath, fd)
	if model == nil {
		model = mcb.GetFieldModel(fieldPath, fd)
	}

	fieldName := string(fd.Name())

	// Handle special case for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		textModel := mcb.GetBooleanModel("payload_is_text")
		textFlag, err := dec.Decode(textModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		var lengthBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			lengthBytes = append(lengthBytes, byte(symbol))
			if symbol < 128 {
				break
			}
		}
		length := int(decodeVarint(lengthBytes))

		if textFlag == 1 {
			compressedBytes := make([]byte, length)
			for i := 0; i < length; i++ {
				symbol, err := dec.Decode(mcb.byteModel)
				if err != nil {
					return protoreflect.Value{}, err
				}
				compressedBytes[i] = byte(symbol)
			}

			str, err := arithcode.DecodeString(bytes.NewReader(compressedBytes))
			if err != nil {
				return protoreflect.Value{}, err
			}
			return protoreflect.ValueOfBytes([]byte(str)), nil
		}

		data := make([]byte, length)
		for i := 0; i < length; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			data[i] = byte(symbol)
		}
		return protoreflect.ValueOfBytes(data), nil
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		boolModel := mcb.GetBooleanModel(fieldName)
		symbol, err := dec.Decode(boolModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBool(symbol != 0), nil

	case protoreflect.EnumKind:
		// Check if we have a prediction for this enum
		if predictedValue, hasPrediction := mcb.enumPredictions[fieldName]; hasPrediction {
			predModel := mcb.GetBooleanModel(fieldName + "_is_predicted")
			flag, err := dec.Decode(predModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			if flag == 1 {
				return protoreflect.ValueOfEnum(predictedValue), nil
			}
		}

		ed := fd.Enum()
		enumModel := mcb.GetEnumModel(fieldPath, ed)
		enumIndex, err := dec.Decode(enumModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		if enumIndex >= ed.Values().Len() {
			return protoreflect.Value{}, fmt.Errorf("invalid enum index %d", enumIndex)
		}
		enumValue := ed.Values().Get(enumIndex).Number()
		return protoreflect.ValueOfEnum(enumValue), nil

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		var varintBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			varintBytes = append(varintBytes, byte(symbol))
			if symbol < 128 {
				break
			}
		}
		uintVal := decodeVarint(varintBytes)

		switch fd.Kind() {
		case protoreflect.Int32Kind:
			return protoreflect.ValueOfInt32(int32(uintVal)), nil
		case protoreflect.Int64Kind:
			return protoreflect.ValueOfInt64(int64(uintVal)), nil
		case protoreflect.Uint32Kind:
			return protoreflect.ValueOfUint32(uint32(uintVal)), nil
		case protoreflect.Uint64Kind:
			return protoreflect.ValueOfUint64(uintVal), nil
		}

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		var varintBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			varintBytes = append(varintBytes, byte(symbol))
			if symbol < 128 {
				break
			}
		}
		zigzagVal := decodeVarint(varintBytes)
		signedVal := zigzagDecode(zigzagVal)

		if fd.Kind() == protoreflect.Sint32Kind {
			return protoreflect.ValueOfInt32(int32(signedVal)), nil
		}
		return protoreflect.ValueOfInt64(signedVal), nil

	case protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		bytes := make([]byte, 4)
		if model != nil && model != mcb.boolModel {
			for i := 0; i < 4; i++ {
				symbol, err := dec.Decode(model)
				if err != nil {
					return protoreflect.Value{}, err
				}
				bytes[i] = byte(symbol)
			}
		} else {
			for i := 0; i < 4; i++ {
				symbol, err := dec.Decode(mcb.byteModel)
				if err != nil {
					return protoreflect.Value{}, err
				}
				bytes[i] = byte(symbol)
			}
		}
		val := binary.LittleEndian.Uint32(bytes)

		if fd.Kind() == protoreflect.Fixed32Kind {
			return protoreflect.ValueOfUint32(val), nil
		}
		return protoreflect.ValueOfInt32(int32(val)), nil

	case protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(symbol)
		}
		val := binary.LittleEndian.Uint64(bytes)

		if fd.Kind() == protoreflect.Fixed64Kind {
			return protoreflect.ValueOfUint64(val), nil
		}
		return protoreflect.ValueOfInt64(int64(val)), nil

	case protoreflect.FloatKind:
		bytes := make([]byte, 4)
		if model != nil && model != mcb.boolModel {
			for i := 0; i < 4; i++ {
				symbol, err := dec.Decode(model)
				if err != nil {
					return protoreflect.Value{}, err
				}
				bytes[i] = byte(symbol)
			}
		} else {
			for i := 0; i < 4; i++ {
				symbol, err := dec.Decode(mcb.byteModel)
				if err != nil {
					return protoreflect.Value{}, err
				}
				bytes[i] = byte(symbol)
			}
		}
		bits := binary.LittleEndian.Uint32(bytes)
		floatVal := math.Float32frombits(bits)
		return protoreflect.ValueOfFloat32(floatVal), nil

	case protoreflect.DoubleKind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(symbol)
		}
		bits := binary.LittleEndian.Uint64(bytes)
		doubleVal := math.Float64frombits(bits)
		return protoreflect.ValueOfFloat64(doubleVal), nil

	case protoreflect.StringKind:
		var lengthBytes []byte
		for {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			lengthBytes = append(lengthBytes, byte(symbol))
			if symbol < 128 {
				break
			}
		}
		compressedLength := int(decodeVarint(lengthBytes))

		compressedBytes := make([]byte, compressedLength)
		for i := 0; i < compressedLength; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			compressedBytes[i] = byte(symbol)
		}

		str, err := arithcode.DecodeString(bytes.NewReader(compressedBytes))
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfString(str), nil

	case protoreflect.BytesKind:
		var lengthBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			lengthBytes = append(lengthBytes, byte(symbol))
			if symbol < 128 {
				break
			}
		}
		length := int(decodeVarint(lengthBytes))

		data := make([]byte, length)
		for i := 0; i < length; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			data[i] = byte(symbol)
		}
		return protoreflect.ValueOfBytes(data), nil

	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field type: %v", fd.Kind())
	}

	return protoreflect.Value{}, fmt.Errorf("unreachable")
}
