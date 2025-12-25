package pbmodel

import (
	"bytes"
	"testing"

	"github.com/egonelbre/protobuf-compression/pbmodel/testdata"
	"google.golang.org/protobuf/proto"
)

func TestSimpleMessageRoundtrip(t *testing.T) {
	original := &testdata.SimpleMessage{
		Id:     12345,
		Name:   "Alice",
		Active: true,
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.SimpleMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestNumericMessageRoundtrip(t *testing.T) {
	original := &testdata.NumericMessage{
		Int32Field:    -12345,
		Int64Field:    -9876543210,
		Uint32Field:   12345,
		Uint64Field:   9876543210,
		Sint32Field:   -100,
		Sint64Field:   -200,
		Fixed32Field:  42,
		Fixed64Field:  84,
		Sfixed32Field: -42,
		Sfixed64Field: -84,
		FloatField:    3.14159,
		DoubleField:   2.71828182845,
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.NumericMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestRepeatedMessageRoundtrip(t *testing.T) {
	original := &testdata.RepeatedMessage{
		Numbers: []int32{1, 2, 3, 4, 5, -1, -2, -3},
		Words:   []string{"hello", "world", "test", "compression"},
		Flags:   []bool{true, false, true, true, false},
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.RepeatedMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestNestedMessageRoundtrip(t *testing.T) {
	original := &testdata.NestedMessage{
		Inner: &testdata.NestedMessage_Inner{
			Value: "inner value",
			Count: 42,
		},
		InnerList: []*testdata.NestedMessage_Inner{
			{Value: "first", Count: 1},
			{Value: "second", Count: 2},
			{Value: "third", Count: 3},
		},
		OuterField: "outer value",
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.NestedMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestDeepNestingRoundtrip(t *testing.T) {
	original := &testdata.DeepNesting{
		Level1: &testdata.DeepNesting_Level1{
			Level2: &testdata.DeepNesting_Level1_Level2{
				Level3: &testdata.DeepNesting_Level1_Level2_Level3{
					Level4: &testdata.DeepNesting_Level1_Level2_Level3_Level4{
						Level5: &testdata.DeepNesting_Level1_Level2_Level3_Level4_Level5{
							DeepValue:  "deeply nested value",
							DeepNumber: 99999,
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.DeepNesting{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestMessageWithEnumRoundtrip(t *testing.T) {
	original := &testdata.MessageWithEnum{
		Status:      testdata.Status_ACTIVE,
		Description: "This is an active status",
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.MessageWithEnum{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestMessageWithOneofRoundtrip(t *testing.T) {
	testCases := []struct {
		name string
		msg  *testdata.MessageWithOneof
	}{
		{
			name: "string value",
			msg: &testdata.MessageWithOneof{
				Value:       &testdata.MessageWithOneof_StringValue{StringValue: "test string"},
				CommonField: "common",
			},
		},
		{
			name: "int value",
			msg: &testdata.MessageWithOneof{
				Value:       &testdata.MessageWithOneof_IntValue{IntValue: 42},
				CommonField: "common",
			},
		},
		{
			name: "bool value",
			msg: &testdata.MessageWithOneof{
				Value:       &testdata.MessageWithOneof_BoolValue{BoolValue: true},
				CommonField: "common",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Compress(tc.msg, &buf); err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			decoded := &testdata.MessageWithOneof{}
			if err := Decompress(&buf, decoded); err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}

			if !proto.Equal(tc.msg, decoded) {
				t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", tc.msg, decoded)
			}
		})
	}
}

func TestMessageWithBytesRoundtrip(t *testing.T) {
	original := &testdata.MessageWithBytes{
		Data:  []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		Label: "binary data",
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.MessageWithBytes{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestMessageWithMapRoundtrip(t *testing.T) {
	original := &testdata.MessageWithMap{
		Counts: map[string]int32{
			"apple":  5,
			"banana": 3,
			"cherry": 8,
		},
		Lookup: map[int32]string{
			1: "one",
			2: "two",
			3: "three",
		},
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.MessageWithMap{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestUserProfileRoundtrip(t *testing.T) {
	original := &testdata.UserProfile{
		UserId:        123456,
		Username:      "johndoe",
		Email:         "john.doe@example.com",
		FullName:      "John Doe",
		Bio:           "Software engineer passionate about compression algorithms and distributed systems.",
		Tags:          []string{"golang", "protobuf", "compression", "algorithms"},
		AccountStatus: testdata.Status_ACTIVE,
		Address: &testdata.UserProfile_Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			Zip:     "94105",
			Country: "USA",
		},
		CreatedAt: 1609459200,
		UpdatedAt: 1640995200,
		Metadata: map[string]string{
			"theme":    "dark",
			"language": "en",
			"timezone": "PST",
		},
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	t.Logf("Original size: %d bytes (proto), Compressed size: %d bytes, Ratio: %.2f%%",
		proto.Size(original), buf.Len(), 100.0*float64(buf.Len())/float64(proto.Size(original)))

	decoded := &testdata.UserProfile{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestEmptyMessageRoundtrip(t *testing.T) {
	original := &testdata.EmptyMessage{}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.EmptyMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestMessageWithDefaultValues(t *testing.T) {
	original := &testdata.SimpleMessage{
		Id:     0,
		Name:   "",
		Active: false,
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.SimpleMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestLargeStringCompression(t *testing.T) {
	// Create a large English text
	largeText := ""
	for i := 0; i < 100; i++ {
		largeText += "The quick brown fox jumps over the lazy dog. "
	}

	original := &testdata.SimpleMessage{
		Id:     1,
		Name:   largeText,
		Active: true,
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	protoSize := proto.Size(original)
	compressedSize := buf.Len()

	t.Logf("Large string test - Proto size: %d bytes, Compressed size: %d bytes, Ratio: %.2f%%",
		protoSize, compressedSize, 100.0*float64(compressedSize)/float64(protoSize))

	decoded := &testdata.SimpleMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match")
	}
}

func TestEmptyRepeatedFields(t *testing.T) {
	original := &testdata.RepeatedMessage{
		Numbers: []int32{},
		Words:   []string{},
		Flags:   []bool{},
	}

	var buf bytes.Buffer
	if err := Compress(original, &buf); err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decoded := &testdata.RepeatedMessage{}
	if err := Decompress(&buf, decoded); err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !proto.Equal(original, decoded) {
		t.Errorf("Messages don't match.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}
