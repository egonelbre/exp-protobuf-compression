package meshtasticmodel

import (
	"fmt"
	"io"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DecompressV3 decompresses data with hybrid encoding.
func DecompressV3(r io.Reader, msg proto.Message) error {
	mmb := NewModelBuilderV1()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return decompressMessageV3("", msg.ProtoReflect(), dec, mmb)
}

// decompressMessageV3 uses hybrid decoding strategy.
func decompressMessageV3(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *ModelBuilderV1) error {
	// Decode strategy flag
	strategyFlag, err := dec.Decode(mmb.BoolModel())
	if err != nil {
		return fmt.Errorf("strategy flag: %w", err)
	}

	useDeltaEncoding := strategyFlag == 1

	if useDeltaEncoding {
		return decompressMessageDelta(fieldPath, msg, dec, mmb)
	}
	return decompressMessagePresenceBits(fieldPath, msg, dec, mmb)
}

// decompressMessagePresenceBits decodes using presence bits.
func decompressMessagePresenceBits(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *ModelBuilderV1) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Decode presence bits and collect present fields
	var presentFields []protoreflect.FieldDescriptor
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		present, err := dec.Decode(mmb.BoolModel())
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}
		if present == 1 {
			presentFields = append(presentFields, fd)
		}
	}

	// Decode field values
	for _, fd := range presentFields {
		currentPath := pbmodel.BuildFieldPath(fieldPath, string(fd.Name()))

		// Track portnum
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal, err := decodeFieldValueV3(currentPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, enumVal)
			portNum := meshtastic.PortNum(enumVal.Enum())
			mmb.currentPortNum = &portNum
			continue
		}

		if err := decodeFieldV3(currentPath, fd, msg, dec, mmb); err != nil {
			return fmt.Errorf("field %s: %w", fd.Name(), err)
		}

		// Reset portnum
		if md.Name() == "Data" && fd == presentFields[len(presentFields)-1] {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// decompressMessageDelta decodes using delta-encoded field numbers.
func decompressMessageDelta(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *ModelBuilderV1) error {
	md := msg.Descriptor()

	// Decode number of present fields
	numPresent, err := decodeVarintFromDecoderV2(dec, mmb.ByteModel())
	if err != nil {
		return fmt.Errorf("num present: %w", err)
	}

	// Decode fields with deltas
	lastFieldNum := 0
	var presentFields []protoreflect.FieldDescriptor

	for i := 0; i < int(numPresent); i++ {
		// Decode delta
		delta, err := decodeVarintFromDecoderV2(dec, mmb.ByteModel())
		if err != nil {
			return fmt.Errorf("field delta: %w", err)
		}

		currentFieldNum := lastFieldNum + int(delta)
		lastFieldNum = currentFieldNum

		// Find field descriptor
		fd := md.Fields().ByNumber(protoreflect.FieldNumber(currentFieldNum))
		if fd == nil {
			return fmt.Errorf("unknown field number: %d", currentFieldNum)
		}

		presentFields = append(presentFields, fd)
		currentPath := pbmodel.BuildFieldPath(fieldPath, string(fd.Name()))

		// Track portnum
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal, err := decodeFieldValueV3(currentPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, enumVal)
			portNum := meshtastic.PortNum(enumVal.Enum())
			mmb.currentPortNum = &portNum
			continue
		}

		if err := decodeFieldV3(currentPath, fd, msg, dec, mmb); err != nil {
			return fmt.Errorf("field %s: %w", fd.Name(), err)
		}

		// Reset portnum
		if md.Name() == "Data" && i == int(numPresent)-1 {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// decodeFieldV3 decodes a field (shared by both strategies).
func decodeFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, msg protoreflect.Message, dec *arithcode.Decoder, mmb *ModelBuilderV1) error {
	if fd.IsList() {
		list := msg.Mutable(fd).List()
		return decompressRepeatedFieldV3(fieldPath, fd, list, dec, mmb)
	} else if fd.IsMap() {
		m := msg.Mutable(fd).Map()
		return decompressMapFieldV3(fieldPath, fd, m, dec, mmb)
	} else if fd.Kind() == protoreflect.MessageKind {
		nestedMsg := msg.Mutable(fd).Message()
		return decompressMessageV3(fieldPath, nestedMsg, dec, mmb)
	}

	value, err := decodeFieldValueV3(fieldPath, fd, dec, mmb)
	if err != nil {
		return err
	}
	msg.Set(fd, value)
	return nil
}

// decompressRepeatedFieldV3 decompresses repeated fields.
func decompressRepeatedFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mmb *ModelBuilderV1) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.ByteModel()
	}

	length, err := decodeVarintFromDecoderV2(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	elementPath := fieldPath + "[]"
	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := decompressMessageV3(elementPath, elem.Message(), dec, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := decodeFieldValueV3(elementPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// decompressMapFieldV3 decompresses map fields.
func decompressMapFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mmb *ModelBuilderV1) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.ByteModel()
	}

	length, err := decodeVarintFromDecoderV2(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	for i := 0; i < int(length); i++ {
		keyValue, err := decodeFieldValueV3(keyPath, keyFd, dec, mmb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		var mapValue protoreflect.Value
		if valueFd.Kind() == protoreflect.MessageKind {
			elem := m.NewValue()
			if err := decompressMessageV3(valuePath, elem.Message(), dec, mmb); err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			mapValue = elem
		} else {
			val, err := decodeFieldValueV3(valuePath, valueFd, dec, mmb)
			if err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			mapValue = val
		}

		m.Set(keyValue.MapKey(), mapValue)
	}

	return nil
}

// decodeFieldValueV3 decodes a field value (reuses V2 logic).
func decodeFieldValueV3(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mmb *ModelBuilderV1) (protoreflect.Value, error) {
	// Reuse the V2 implementation
	return decodeFieldValueV2(fieldPath, fd, dec, mmb)
}
