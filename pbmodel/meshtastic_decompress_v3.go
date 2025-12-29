package pbmodel

import (
	"fmt"
	"io"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MeshtasticDecompressV3 decompresses data with hybrid encoding.
func MeshtasticDecompressV3(r io.Reader, msg proto.Message) error {
	mmb := NewMeshtasticModelBuilder()
	dec, err := arithcode.NewDecoder(r)
	if err != nil {
		return err
	}

	return meshtasticDecompressMessageV3("", msg.ProtoReflect(), dec, mmb)
}

// meshtasticDecompressMessageV3 uses hybrid decoding strategy.
func meshtasticDecompressMessageV3(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *MeshtasticModelBuilder) error {
	// Decode strategy flag
	strategyFlag, err := dec.Decode(mmb.boolModel)
	if err != nil {
		return fmt.Errorf("strategy flag: %w", err)
	}

	useDeltaEncoding := strategyFlag == 1

	if useDeltaEncoding {
		return meshtasticDecompressMessageDelta(fieldPath, msg, dec, mmb)
	}
	return meshtasticDecompressMessagePresenceBits(fieldPath, msg, dec, mmb)
}

// meshtasticDecompressMessagePresenceBits decodes using presence bits.
func meshtasticDecompressMessagePresenceBits(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *MeshtasticModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Decode presence bits and collect present fields
	var presentFields []protoreflect.FieldDescriptor
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		present, err := dec.Decode(mmb.boolModel)
		if err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}
		if present == 1 {
			presentFields = append(presentFields, fd)
		}
	}

	// Decode field values
	for _, fd := range presentFields {
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))

		// Track portnum
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal, err := meshtasticDecodeFieldValueV3(currentPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, enumVal)
			portNum := meshtastic.PortNum(enumVal.Enum())
			mmb.currentPortNum = &portNum
			continue
		}

		if err := meshtasticDecodeFieldV3(currentPath, fd, msg, dec, mmb); err != nil {
			return fmt.Errorf("field %s: %w", fd.Name(), err)
		}

		// Reset portnum
		if md.Name() == "Data" && fd == presentFields[len(presentFields)-1] {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// meshtasticDecompressMessageDelta decodes using delta-encoded field numbers.
func meshtasticDecompressMessageDelta(fieldPath string, msg protoreflect.Message, dec *arithcode.Decoder, mmb *MeshtasticModelBuilder) error {
	md := msg.Descriptor()

	// Decode number of present fields
	numPresent, err := meshtasticDecodeVarintFromDecoderV2(dec, mmb.byteModel)
	if err != nil {
		return fmt.Errorf("num present: %w", err)
	}

	// Decode fields with deltas
	lastFieldNum := 0
	var presentFields []protoreflect.FieldDescriptor

	for i := 0; i < int(numPresent); i++ {
		// Decode delta
		delta, err := meshtasticDecodeVarintFromDecoderV2(dec, mmb.byteModel)
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
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))

		// Track portnum
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal, err := meshtasticDecodeFieldValueV3(currentPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("field %s: %w", fd.Name(), err)
			}
			msg.Set(fd, enumVal)
			portNum := meshtastic.PortNum(enumVal.Enum())
			mmb.currentPortNum = &portNum
			continue
		}

		if err := meshtasticDecodeFieldV3(currentPath, fd, msg, dec, mmb); err != nil {
			return fmt.Errorf("field %s: %w", fd.Name(), err)
		}

		// Reset portnum
		if md.Name() == "Data" && i == int(numPresent)-1 {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// meshtasticDecodeFieldV3 decodes a field (shared by both strategies).
func meshtasticDecodeFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, msg protoreflect.Message, dec *arithcode.Decoder, mmb *MeshtasticModelBuilder) error {
	if fd.IsList() {
		list := msg.Mutable(fd).List()
		return meshtasticDecompressRepeatedFieldV3(fieldPath, fd, list, dec, mmb)
	} else if fd.IsMap() {
		m := msg.Mutable(fd).Map()
		return meshtasticDecompressMapFieldV3(fieldPath, fd, m, dec, mmb)
	} else if fd.Kind() == protoreflect.MessageKind {
		nestedMsg := msg.Mutable(fd).Message()
		return meshtasticDecompressMessageV3(fieldPath, nestedMsg, dec, mmb)
	}

	value, err := meshtasticDecodeFieldValueV3(fieldPath, fd, dec, mmb)
	if err != nil {
		return err
	}
	msg.Set(fd, value)
	return nil
}

// meshtasticDecompressRepeatedFieldV3 decompresses repeated fields.
func meshtasticDecompressRepeatedFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, dec *arithcode.Decoder, mmb *MeshtasticModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.byteModel
	}

	length, err := meshtasticDecodeVarintFromDecoderV2(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("list length: %w", err)
	}

	elementPath := fieldPath + "[]"
	for i := 0; i < int(length); i++ {
		if fd.Kind() == protoreflect.MessageKind {
			elem := list.NewElement()
			if err := meshtasticDecompressMessageV3(elementPath, elem.Message(), dec, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(elem)
		} else {
			value, err := meshtasticDecodeFieldValueV3(elementPath, fd, dec, mmb)
			if err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
			list.Append(value)
		}
	}

	return nil
}

// meshtasticDecompressMapFieldV3 decompresses map fields.
func meshtasticDecompressMapFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, dec *arithcode.Decoder, mmb *MeshtasticModelBuilder) error {
	lengthPath := fieldPath + "._length"
	lengthModel := mmb.GetFieldModel(lengthPath, fd)
	if lengthModel == nil {
		lengthModel = mmb.byteModel
	}

	length, err := meshtasticDecodeVarintFromDecoderV2(dec, lengthModel)
	if err != nil {
		return fmt.Errorf("map length: %w", err)
	}

	keyFd := fd.MapKey()
	valueFd := fd.MapValue()
	keyPath := fieldPath + "._key"
	valuePath := fieldPath + "._value"

	for i := 0; i < int(length); i++ {
		keyValue, err := meshtasticDecodeFieldValueV3(keyPath, keyFd, dec, mmb)
		if err != nil {
			return fmt.Errorf("map key %d: %w", i, err)
		}

		var mapValue protoreflect.Value
		if valueFd.Kind() == protoreflect.MessageKind {
			elem := m.NewValue()
			if err := meshtasticDecompressMessageV3(valuePath, elem.Message(), dec, mmb); err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			mapValue = elem
		} else {
			val, err := meshtasticDecodeFieldValueV3(valuePath, valueFd, dec, mmb)
			if err != nil {
				return fmt.Errorf("map value %d: %w", i, err)
			}
			mapValue = val
		}

		m.Set(keyValue.MapKey(), mapValue)
	}

	return nil
}

// meshtasticDecodeFieldValueV3 decodes a field value (reuses V2 logic).
func meshtasticDecodeFieldValueV3(fieldPath string, fd protoreflect.FieldDescriptor, dec *arithcode.Decoder, mmb *MeshtasticModelBuilder) (protoreflect.Value, error) {
	// Reuse the V2 implementation
	return meshtasticDecodeFieldValueV2(fieldPath, fd, dec, mmb)
}
