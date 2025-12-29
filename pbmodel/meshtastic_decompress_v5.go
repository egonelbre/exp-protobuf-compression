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

// MeshtasticDecompressV5 decompresses a message using context-aware models.
func MeshtasticDecompressV5(r io.Reader, msg proto.Message) error {
	mcb := NewMeshtasticContextualModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	// Set initial message type context
	msgType := string(msg.ProtoReflect().Descriptor().Name())
	mcb.SetMessageType(msgType)

	return meshtasticDecompressMessageV5("", msg.ProtoReflect(), dec, mcb)
}

// meshtasticDecompressMessageV5 recursively decompresses with context-aware models.
func meshtasticDecompressMessageV5(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) error {
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

		// Check if field is present
		present, err := dec.Decode(mcb.boolModel)
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		if present == 0 {
			// Field not present, skip
			continue
		}

		if fd.IsList() {
			list := msg.Mutable(fd).List()
			if err := meshtasticDecompressRepeatedFieldV5(currentPath, fd, list, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := meshtasticDecompressMapFieldV5(currentPath, fd, m, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := msg.Mutable(fd).Message()
			if err := meshtasticDecompressMessageV5(currentPath, nestedMsg, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := meshtasticDecompressFieldValueV5(currentPath, fd, dec, mcb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, value)

			// Track portnum for payload detection
			if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
				enumVal := value.Enum()
				portNum := meshtastic.PortNum(enumVal)
				mcb.currentPortNum = &portNum
			}
		}

		// Reset portnum after processing Data message
		if md.Name() == "Data" && i == fields.Len()-1 {
			mcb.currentPortNum = nil
		}
	}

	return nil
}

// meshtasticDecompressRepeatedFieldV5 decompresses repeated fields.
func meshtasticDecompressRepeatedFieldV5(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.byteModel
	}

	// Decode length
	var lengthBytes []byte
	for {
		symbol, err := dec.Decode(lengthModel)
		if err != nil {
			return fmt.Errorf("list length: %w", err)
		}
		lengthBytes = append(lengthBytes, byte(symbol))
		if symbol&0x80 == 0 {
			break
		}
	}
	length := int(decodeVarint(lengthBytes))

	elementPath := fieldPath + "[]"
	for i := 0; i < length; i++ {
		if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := list.NewElement().Message()
			if err := meshtasticDecompressMessageV5(elementPath, nestedMsg, dec, mcb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(protoreflect.ValueOfMessage(nestedMsg))
		} else {
			value, err := meshtasticDecompressFieldValueV5(elementPath, fd, dec, mcb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// meshtasticDecompressMapFieldV5 decompresses map fields.
func meshtasticDecompressMapFieldV5(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.byteModel
	}

	// Decode length
	var lengthBytes []byte
	for {
		symbol, err := dec.Decode(lengthModel)
		if err != nil {
			return fmt.Errorf("map length: %w", err)
		}
		lengthBytes = append(lengthBytes, byte(symbol))
		if symbol&0x80 == 0 {
			break
		}
	}
	length := int(decodeVarint(lengthBytes))

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	for i := 0; i < length; i++ {
		// Decode key
		keyValue, err := meshtasticDecompressFieldValueV5(keyPath, keyFd, dec, mcb)
		if err != nil {
			return fmt.Errorf("map key: %w", err)
		}

		// Decode value
		var mapValue protoreflect.Value
		if valueFd.Kind() == protoreflect.MessageKind {
			nestedMsg := m.NewValue().Message()
			if err := meshtasticDecompressMessageV5(valuePath, nestedMsg, dec, mcb); err != nil {
				return fmt.Errorf("map value: %w", err)
			}
			mapValue = protoreflect.ValueOfMessage(nestedMsg)
		} else {
			var err error
			mapValue, err = meshtasticDecompressFieldValueV5(valuePath, valueFd, dec, mcb)
			if err != nil {
				return fmt.Errorf("map value: %w", err)
			}
		}

		m.Set(keyValue.MapKey(), mapValue)
	}

	return nil
}

// meshtasticDecompressFieldValueV5 decompresses a field value.
func meshtasticDecompressFieldValueV5(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mcb *MeshtasticContextualModelBuilder) (protoreflect.Value, error) {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		// Decode text flag
		textFlag, err := dec.Decode(mcb.boolModel)
		if err != nil {
			return protoreflect.Value{}, err
		}

		if textFlag == 1 {
			// Decode as compressed text
			var lengthBytes []byte
			for {
				symbol, err := dec.Decode(mcb.byteModel)
				if err != nil {
					return protoreflect.Value{}, err
				}
				lengthBytes = append(lengthBytes, byte(symbol))
				if symbol&0x80 == 0 {
					break
				}
			}
			compressedLen := int(decodeVarint(lengthBytes))

			compressedBytes := make([]byte, compressedLen)
			for i := 0; i < compressedLen; i++ {
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
	}

	model := mcb.GetContextualFieldModel(fieldPath, fd)

	switch fd.Kind() {
	case protoreflect.BoolKind:
		symbol, err := dec.Decode(model)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfBool(symbol == 1), nil

	case protoreflect.EnumKind:
		// Check if this enum has a predicted common value
		fieldName := string(fd.Name())
		if predictedValue, hasPrediction := mcb.enumPredictions[fieldName]; hasPrediction {
			flag, err := dec.Decode(mcb.boolModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			if flag == 1 {
				// Common value
				return protoreflect.ValueOfEnum(predictedValue), nil
			}
			// Not common value, decode actual value below
		}

		enumDesc := fd.Enum()
		enumModel := mcb.GetEnumModel(fieldPath, enumDesc)
		idx, err := dec.Decode(enumModel)
		if err != nil {
			return protoreflect.Value{}, err
		}
		enumValue := enumDesc.Values().Get(idx).Number()
		return protoreflect.ValueOfEnum(enumValue), nil

	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		var valueBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			valueBytes = append(valueBytes, byte(symbol))
			if symbol&0x80 == 0 {
				break
			}
		}
		val := int64(decodeVarint(valueBytes))
		if fd.Kind() == protoreflect.Int32Kind {
			return protoreflect.ValueOfInt32(int32(val)), nil
		}
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		var valueBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			valueBytes = append(valueBytes, byte(symbol))
			if symbol&0x80 == 0 {
				break
			}
		}
		val := decodeVarint(valueBytes)
		if fd.Kind() == protoreflect.Uint32Kind {
			return protoreflect.ValueOfUint32(uint32(val)), nil
		}
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		var valueBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			valueBytes = append(valueBytes, byte(symbol))
			if symbol&0x80 == 0 {
				break
			}
		}
		zigzag := decodeVarint(valueBytes)
		val := zigzagDecode(zigzag)
		if fd.Kind() == protoreflect.Sint32Kind {
			return protoreflect.ValueOfInt32(int32(val)), nil
		}
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.Fixed32Kind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(symbol)
		}
		val := binary.LittleEndian.Uint32(bytes)
		return protoreflect.ValueOfUint32(val), nil

	case protoreflect.Sfixed32Kind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(symbol)
		}
		val := int32(binary.LittleEndian.Uint32(bytes))
		return protoreflect.ValueOfInt32(val), nil

	case protoreflect.Fixed64Kind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(symbol)
		}
		val := binary.LittleEndian.Uint64(bytes)
		return protoreflect.ValueOfUint64(val), nil

	case protoreflect.Sfixed64Kind:
		bytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			bytes[i] = byte(symbol)
		}
		val := int64(binary.LittleEndian.Uint64(bytes))
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.FloatKind:
		bytes := make([]byte, 4)
		// Use context-aware model if available
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
		val := math.Float32frombits(bits)
		return protoreflect.ValueOfFloat32(val), nil

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
		val := math.Float64frombits(bits)
		return protoreflect.ValueOfFloat64(val), nil

	case protoreflect.StringKind:
		// Decode compressed length
		var lengthBytes []byte
		for {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			lengthBytes = append(lengthBytes, byte(symbol))
			if symbol&0x80 == 0 {
				break
			}
		}
		compressedLen := int(decodeVarint(lengthBytes))

		// Decode compressed bytes
		compressedBytes := make([]byte, compressedLen)
		for i := 0; i < compressedLen; i++ {
			symbol, err := dec.Decode(mcb.byteModel)
			if err != nil {
				return protoreflect.Value{}, err
			}
			compressedBytes[i] = byte(symbol)
		}

		// Decompress string
		str, err := arithcode.DecodeString(bytes.NewReader(compressedBytes))
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfString(str), nil

	case protoreflect.BytesKind:
		// Decode length
		var lengthBytes []byte
		for {
			symbol, err := dec.Decode(model)
			if err != nil {
				return protoreflect.Value{}, err
			}
			lengthBytes = append(lengthBytes, byte(symbol))
			if symbol&0x80 == 0 {
				break
			}
		}
		length := int(decodeVarint(lengthBytes))

		// Decode bytes
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
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind: %v", fd.Kind())
	}
}
