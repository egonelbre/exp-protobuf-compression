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

// CompressV6 uses bit packing for boolean clusters on top of V5 context-aware models.
func CompressV6(msg proto.Message, w io.Writer) error {
	mcb := NewContextualModelBuilder()
	enc := arithcode.NewEncoder(w)

	// Set initial message type context
	msgType := string(msg.ProtoReflect().Descriptor().Name())
	mcb.SetMessageType(msgType)

	if err := compressMessageV6("", msg.ProtoReflect(), enc, mcb); err != nil {
		return err
	}

	return enc.Close()
}

// compressMessageV6 recursively compresses with bit-packed booleans.
func compressMessageV6(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Update message type context
	prevMsgType := mcb.messageType
	mcb.SetMessageType(string(md.Name()))
	defer func() { mcb.messageType = prevMsgType }()

	// Identify boolean clusters in this message
	boolClusters := identifyBooleanClusters(fields, msg)

	// Track which fields are in boolean clusters
	inCluster := make(map[int]bool)
	for _, cluster := range boolClusters {
		for _, idx := range cluster.fieldIndices {
			inCluster[idx] = true
		}
	}

	// Encode boolean clusters first
	for _, cluster := range boolClusters {
		if err := encodeBooleanCluster(cluster, msg, enc, mcb); err != nil {
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
			if err := compressRepeatedFieldV6(currentPath, fd, value.List(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.IsMap() {
			if err := compressMapFieldV6(currentPath, fd, value.Map(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else if fd.Kind() == protoreflect.MessageKind {
			if err := compressMessageV6(currentPath, value.Message(), enc, mcb); err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
		} else {
			if err := compressFieldValueV6(currentPath, fd, value, enc, mcb); err != nil {
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

// BooleanCluster represents a group of related boolean fields.
type BooleanCluster struct {
	fieldIndices []int
	fieldDescs   []protoreflect.FieldDescriptor
}

// identifyBooleanClusters finds clusters of boolean fields to pack together.
func identifyBooleanClusters(fields protoreflect.FieldDescriptors, msg protoreflect.Message) []BooleanCluster {
	var clusters []BooleanCluster
	var currentCluster BooleanCluster

	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)

		// Only cluster boolean fields
		if fd.Kind() == protoreflect.BoolKind {
			currentCluster.fieldIndices = append(currentCluster.fieldIndices, i)
			currentCluster.fieldDescs = append(currentCluster.fieldDescs, fd)

			// Limit cluster size to 8 booleans (1 byte)
			if len(currentCluster.fieldIndices) >= 8 {
				clusters = append(clusters, currentCluster)
				currentCluster = BooleanCluster{}
			}
		} else {
			// Non-boolean field breaks the cluster
			if len(currentCluster.fieldIndices) >= 2 {
				clusters = append(clusters, currentCluster)
			}
			currentCluster = BooleanCluster{}
		}
	}

	// Add remaining cluster if it has at least 2 booleans
	if len(currentCluster.fieldIndices) >= 2 {
		clusters = append(clusters, currentCluster)
	}

	return clusters
}

// encodeBooleanCluster packs multiple boolean fields into a compact representation.
func encodeBooleanCluster(cluster BooleanCluster, msg protoreflect.Message, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
	// First, encode presence bits for all booleans in cluster
	var presenceBits uint8
	for i, fd := range cluster.fieldDescs {
		if msg.Has(fd) {
			presenceBits |= (1 << i)
		}
	}

	// Encode presence bits
	if err := enc.Encode(int(presenceBits), mcb.ByteModel()); err != nil {
		return fmt.Errorf("cluster presence bits: %w", err)
	}

	// Then, encode values for present booleans
	var valueBits uint8
	for i, fd := range cluster.fieldDescs {
		if msg.Has(fd) {
			value := msg.Get(fd)
			if value.Bool() {
				valueBits |= (1 << i)
			}
		}
	}

	// Encode value bits
	if err := enc.Encode(int(valueBits), mcb.ByteModel()); err != nil {
		return fmt.Errorf("cluster value bits: %w", err)
	}

	return nil
}

// compressRepeatedFieldV6 compresses repeated fields.
func compressRepeatedFieldV6(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
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
			if err := compressMessageV6(elementPath, value.Message(), enc, mcb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		} else {
			if err := compressFieldValueV6(elementPath, fd, value, enc, mcb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	}

	return nil
}

// compressMapFieldV6 compresses map fields.
func compressMapFieldV6(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
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
		if err := compressFieldValueV6(keyPath, keyFd, k.Value(), enc, mcb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if valueFd.Kind() == protoreflect.MessageKind {
			if err := compressMessageV6(valuePath, v.Message(), enc, mcb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		} else {
			if err := compressFieldValueV6(valuePath, valueFd, v, enc, mcb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		}

		return true
	})

	return encodeErr
}

// compressFieldValueV6 compresses field values (reuses V5 logic).
func compressFieldValueV6(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mcb *ContextualModelBuilder) error {
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
		// Individual booleans (not in cluster) still encoded normally
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
