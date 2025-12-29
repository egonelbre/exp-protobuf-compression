package meshtasticmodel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DecompressV6 decompresses a message using bit-packed booleans.
func DecompressV6(r io.Reader, msg proto.Message) error {
	mcb := NewContextualModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	// Set initial message type context
	msgType := string(msg.ProtoReflect().Descriptor().Name())
	mcb.SetMessageType(msgType)

	return decompressMessageV6("", msg.ProtoReflect(), dec, mcb)
}

// decompressMessageV6 recursively decompresses with bit-packed booleans.
func decompressMessageV6(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mcb *ContextualModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Update message type context
	prevMsgType := mcb.messageType
	mcb.SetMessageType(string(md.Name()))
	defer func() { mcb.messageType = prevMsgType }()

	// Identify boolean clusters (same logic as compress)
	boolClusters := identifyBooleanClusters(fields, msg)

	// Track which fields are in boolean clusters
	inCluster := make(map[int]bool)
	for _, cluster := range boolClusters {
		for _, idx := range cluster.fieldIndices {
			inCluster[idx] = true
		}
	}

	// Decode boolean clusters first
	for _, cluster := range boolClusters {
		if err := decodeBooleanCluster(cluster, msg, dec, mcb); err != nil {
			return fmt.Errorf("boolean cluster: %w", err)
		}
	}

	// Iterate through all non-boolean-cluster fields
	for i := 0; i < fields.Len(); i++ {
		if inCluster[i] {
			continue // Already handled in cluster
		}

		fd := fields.Get(i)
		currentPath := pbmodel.BuildFieldPath(fieldPath, string(fd.Name()))

		// Check if field is present
		present, err := dec.Decode(mcb.BoolModel())
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}

		if present == 0 {
			// Field not present, skip
			continue
		}

		if fd.IsList() {
			list := msg.Mutable(fd).List()
			if err := decompressRepeatedFieldV6(currentPath, fd, list, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			m := msg.Mutable(fd).Map()
			if err := decompressMapFieldV6(currentPath, fd, m, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := msg.Mutable(fd).Message()
			if err := decompressMessageV6(currentPath, nestedMsg, dec, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			value, err := decompressFieldValueV6(currentPath, fd, dec, mcb)
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

// decodeBooleanCluster unpacks multiple boolean fields from compact representation.
func decodeBooleanCluster(cluster BooleanCluster, msg protoreflect.Message, dec *arithcode.Decoder, mcb *ContextualModelBuilder) error {
	// Decode presence bits
	presenceBitsSymbol, err := dec.Decode(mcb.ByteModel())
	if err != nil {
		return fmt.Errorf("cluster presence bits: %w", err)
	}
	presenceBits := uint8(presenceBitsSymbol)

	// Decode value bits
	valueBitsSymbol, err := dec.Decode(mcb.ByteModel())
	if err != nil {
		return fmt.Errorf("cluster value bits: %w", err)
	}
	valueBits := uint8(valueBitsSymbol)

	// Set fields based on bits
	for i, fd := range cluster.fieldDescs {
		isPresent := (presenceBits & (1 << i)) != 0
		if isPresent {
			value := (valueBits & (1 << i)) != 0
			msg.Set(fd, protoreflect.ValueOfBool(value))
		}
	}

	return nil
}

// decompressRepeatedFieldV6 decompresses repeated fields.
func decompressRepeatedFieldV6(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mcb *ContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.ByteModel()
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
	length := int(pbmodel.DecodeVarint(lengthBytes))

	elementPath := fieldPath + "[]"
	for i := 0; i < length; i++ {
		if fd.Kind() == protoreflect.MessageKind {
			nestedMsg := list.NewElement().Message()
			if err := decompressMessageV6(elementPath, nestedMsg, dec, mcb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(protoreflect.ValueOfMessage(nestedMsg))
		} else {
			value, err := decompressFieldValueV6(elementPath, fd, dec, mcb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// decompressMapFieldV6 decompresses map fields.
func decompressMapFieldV6(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mcb *ContextualModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mcb.GetContextualFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mcb.ByteModel()
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
	length := int(pbmodel.DecodeVarint(lengthBytes))

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	for i := 0; i < length; i++ {
		// Decode key
		keyValue, err := decompressFieldValueV6(keyPath, keyFd, dec, mcb)
		if err != nil {
			return fmt.Errorf("map key: %w", err)
		}

		// Decode value
		var mapValue protoreflect.Value
		if valueFd.Kind() == protoreflect.MessageKind {
			nestedMsg := m.NewValue().Message()
			if err := decompressMessageV6(valuePath, nestedMsg, dec, mcb); err != nil {
				return fmt.Errorf("map value: %w", err)
			}
			mapValue = protoreflect.ValueOfMessage(nestedMsg)
		} else {
			var err error
			mapValue, err = decompressFieldValueV6(valuePath, valueFd, dec, mcb)
			if err != nil {
				return fmt.Errorf("map value: %w", err)
			}
		}

		m.Set(keyValue.MapKey(), mapValue)
	}

	return nil
}

// decompressFieldValueV6 decompresses a field value (reuses V5 logic).
func decompressFieldValueV6(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mcb *ContextualModelBuilder) (protoreflect.Value, error) {
	// Special handling for Data.payload field
	if fd.Name() == "payload" && fd.Kind() == protoreflect.BytesKind {
		// Decode text flag
		textFlag, err := dec.Decode(mcb.BoolModel())
		if err != nil {
			return protoreflect.Value{}, err
		}

		if textFlag == 1 {
			// Decode as compressed text
			var lengthBytes []byte
			for {
				symbol, err := dec.Decode(mcb.ByteModel())
				if err != nil {
					return protoreflect.Value{}, err
				}
				lengthBytes = append(lengthBytes, byte(symbol))
				if symbol&0x80 == 0 {
					break
				}
			}
			compressedLen := int(pbmodel.DecodeVarint(lengthBytes))

			compressedBytes := make([]byte, compressedLen)
			for i := 0; i < compressedLen; i++ {
				symbol, err := dec.Decode(mcb.ByteModel())
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
			flag, err := dec.Decode(mcb.BoolModel())
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
		val := int64(pbmodel.DecodeVarint(valueBytes))
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
		val := pbmodel.DecodeVarint(valueBytes)
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
		zigzag := pbmodel.DecodeVarint(valueBytes)
		val := pbmodel.ZigzagDecode(zigzag)
		if fd.Kind() == protoreflect.Sint32Kind {
			return protoreflect.ValueOfInt32(int32(val)), nil
		}
		return protoreflect.ValueOfInt64(val), nil

	case protoreflect.Fixed32Kind:
		bytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			symbol, err := dec.Decode(mcb.ByteModel())
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
			symbol, err := dec.Decode(mcb.ByteModel())
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
			symbol, err := dec.Decode(mcb.ByteModel())
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
			symbol, err := dec.Decode(mcb.ByteModel())
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
		if model != nil && model != mcb.BoolModel() {
			for i := 0; i < 4; i++ {
				symbol, err := dec.Decode(model)
				if err != nil {
					return protoreflect.Value{}, err
				}
				bytes[i] = byte(symbol)
			}
		} else {
			for i := 0; i < 4; i++ {
				symbol, err := dec.Decode(mcb.ByteModel())
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
			symbol, err := dec.Decode(mcb.ByteModel())
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
			symbol, err := dec.Decode(mcb.ByteModel())
			if err != nil {
				return protoreflect.Value{}, err
			}
			lengthBytes = append(lengthBytes, byte(symbol))
			if symbol&0x80 == 0 {
				break
			}
		}
		compressedLen := int(pbmodel.DecodeVarint(lengthBytes))

		// Decode compressed bytes
		compressedBytes := make([]byte, compressedLen)
		for i := 0; i < compressedLen; i++ {
			symbol, err := dec.Decode(mcb.ByteModel())
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
		length := int(pbmodel.DecodeVarint(lengthBytes))

		// Decode bytes
		data := make([]byte, length)
		for i := 0; i < length; i++ {
			symbol, err := dec.Decode(mcb.ByteModel())
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
