package pbmodel

import (
	"fmt"
	"io"

	"github.com/egonelbre/protobuf-compression/arithcode"
	"github.com/egonelbre/protobuf-compression/pbmodel/testdata/meshtastic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MeshtasticCompressV3 uses hybrid encoding: automatically chooses between
// presence-bit encoding (for dense messages) and delta-encoded field numbers
// (for sparse messages) based on which is more efficient.
func MeshtasticCompressV3(msg proto.Message, w io.Writer) error {
	mmb := NewMeshtasticModelBuilder()
	enc := arithcode.NewEncoder(w)

	if err := meshtasticCompressMessageV3("", msg.ProtoReflect(), enc, mmb); err != nil {
		return err
	}

	return enc.Close()
}

// meshtasticCompressMessageV3 uses hybrid encoding strategy.
func meshtasticCompressMessageV3(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
	md := msg.Descriptor()
	fields := md.Fields()

	// Collect present fields
	var presentFields []protoreflect.FieldDescriptor
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if msg.Has(fd) {
			presentFields = append(presentFields, fd)
		}
	}

	numPresent := len(presentFields)
	numTotal := fields.Len()

	// Choose encoding strategy based on density
	// Presence bits: ~numTotal/8 bytes overhead
	// Delta encoding: 1 + numPresent bytes overhead (assuming small deltas)
	// Use delta encoding if: 1 + numPresent < numTotal/8
	// Simplified: numPresent < numTotal/8 - 1
	useDeltaEncoding := numPresent > 0 && (8*numPresent+8 < numTotal)

	// Encode strategy flag (0 = presence bits, 1 = delta encoding)
	strategyFlag := 0
	if useDeltaEncoding {
		strategyFlag = 1
	}
	if err := enc.Encode(strategyFlag, mmb.boolModel); err != nil {
		return fmt.Errorf("strategy flag: %w", err)
	}

	if useDeltaEncoding {
		return meshtasticCompressMessageDelta(fieldPath, msg, enc, mmb, presentFields)
	}
	return meshtasticCompressMessagePresenceBits(fieldPath, msg, enc, mmb, fields, presentFields)
}

// meshtasticCompressMessagePresenceBits encodes using presence bits.
func meshtasticCompressMessagePresenceBits(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder, fields protoreflect.FieldDescriptors, presentFields []protoreflect.FieldDescriptor) error {
	md := msg.Descriptor()

	// Create a map for quick lookup
	presentMap := make(map[protoreflect.FieldNumber]bool)
	for _, fd := range presentFields {
		presentMap[fd.Number()] = true
	}

	// Encode presence bits
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		present := 0
		if presentMap[fd.Number()] {
			present = 1
		}
		if err := enc.Encode(present, mmb.boolModel); err != nil {
			return fmt.Errorf("field %s presence: %w", fd.Name(), err)
		}
	}

	// Encode present field values
	for _, fd := range presentFields {
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))
		value := msg.Get(fd)

		// Track portnum
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			enumVal := value.Enum()
			portNum := meshtastic.PortNum(enumVal)
			mmb.currentPortNum = &portNum
		}

		if err := meshtasticEncodeFieldV3(currentPath, fd, value, enc, mmb); err != nil {
			return fmt.Errorf("field %s: %w", fd.Name(), err)
		}

		// Reset portnum
		if md.Name() == "Data" && fd == presentFields[len(presentFields)-1] {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// meshtasticCompressMessageDelta encodes using delta-encoded field numbers.
func meshtasticCompressMessageDelta(fieldPath string, msg protoreflect.Message, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder, presentFields []protoreflect.FieldDescriptor) error {
	md := msg.Descriptor()

	// Encode number of present fields
	numPresentBytes := encodeVarint(uint64(len(presentFields)))
	for _, b := range numPresentBytes {
		if err := enc.Encode(int(b), mmb.byteModel); err != nil {
			return fmt.Errorf("num present: %w", err)
		}
	}

	// Encode fields with deltas
	lastFieldNum := 0
	for _, fd := range presentFields {
		currentPath := buildFieldPath(fieldPath, string(fd.Name()))
		fieldNum := int(fd.Number())

		// Encode delta
		delta := fieldNum - lastFieldNum
		deltaBytes := encodeVarint(uint64(delta))
		for _, b := range deltaBytes {
			if err := enc.Encode(int(b), mmb.byteModel); err != nil {
				return fmt.Errorf("field delta: %w", err)
			}
		}
		lastFieldNum = fieldNum

		// Track portnum
		if fd.Name() == "portnum" && fd.Kind() == protoreflect.EnumKind {
			value := msg.Get(fd)
			enumVal := value.Enum()
			portNum := meshtastic.PortNum(enumVal)
			mmb.currentPortNum = &portNum
		}

		value := msg.Get(fd)
		if err := meshtasticEncodeFieldV3(currentPath, fd, value, enc, mmb); err != nil {
			return fmt.Errorf("field %s: %w", fd.Name(), err)
		}

		// Reset portnum
		if md.Name() == "Data" && fd == presentFields[len(presentFields)-1] {
			mmb.currentPortNum = nil
		}
	}

	return nil
}

// meshtasticEncodeFieldV3 encodes a field value (shared by both strategies).
func meshtasticEncodeFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
	if fd.IsList() {
		return meshtasticCompressRepeatedFieldV3(fieldPath, fd, value.List(), enc, mmb)
	} else if fd.IsMap() {
		return meshtasticCompressMapFieldV3(fieldPath, fd, value.Map(), enc, mmb)
	} else if fd.Kind() == protoreflect.MessageKind {
		return meshtasticCompressMessageV3(fieldPath, value.Message(), enc, mmb)
	}
	return meshtasticCompressFieldValueV3(fieldPath, fd, value, enc, mmb)
}

// meshtasticCompressRepeatedFieldV3 compresses repeated fields.
func meshtasticCompressRepeatedFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, list protoreflect.List, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
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
			if err := meshtasticCompressMessageV3(elementPath, value.Message(), enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		} else {
			if err := meshtasticCompressFieldValueV3(elementPath, fd, value, enc, mmb); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	}

	return nil
}

// meshtasticCompressMapFieldV3 compresses map fields.
func meshtasticCompressMapFieldV3(fieldPath string, fd protoreflect.FieldDescriptor, m protoreflect.Map, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
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
		if err := meshtasticCompressFieldValueV3(keyPath, keyFd, k.Value(), enc, mmb); err != nil {
			encodeErr = fmt.Errorf("map key: %w", err)
			return false
		}

		if valueFd.Kind() == protoreflect.MessageKind {
			if err := meshtasticCompressMessageV3(valuePath, v.Message(), enc, mmb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		} else {
			if err := meshtasticCompressFieldValueV3(valuePath, valueFd, v, enc, mmb); err != nil {
				encodeErr = fmt.Errorf("map value: %w", err)
				return false
			}
		}

		return true
	})

	return encodeErr
}

// meshtasticCompressFieldValueV3 compresses a field value (reuses V1/V2 logic).
func meshtasticCompressFieldValueV3(fieldPath string, fd protoreflect.FieldDescriptor, value protoreflect.Value, enc *arithcode.Encoder, mmb *MeshtasticModelBuilder) error {
	// Reuse the V2 implementation
	return meshtasticCompressFieldValueV2(fieldPath, fd, value, enc, mmb)
}
