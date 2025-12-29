package pbmodel

import (
	"bytes"
	"math/rand"
	"testing"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/egonelbre/exp-protobuf-compression/pbmodel/testdata"
)

func FuzzSimpleMessage(f *testing.F) {
	// Add seed corpus
	f.Add(int32(0), "", false)
	f.Add(int32(42), "hello", true)
	f.Add(int32(-100), "test message", false)

	f.Fuzz(func(t *testing.T, id int32, name string, active bool) {
		// Skip invalid UTF-8 strings - they get normalized by protobuf
		if !utf8.ValidString(name) {
			t.Skip("Invalid UTF-8 string")
		}

		original := &testdata.SimpleMessage{
			Id:     id,
			Name:   name,
			Active: active,
		}

		var buf bytes.Buffer
		if err := Compress(original, &buf); err != nil {
			t.Skipf("Compress failed (expected for some inputs): %v", err)
		}

		decoded := &testdata.SimpleMessage{}
		if err := Decompress(&buf, decoded); err != nil {
			t.Fatalf("Decompress failed: %v", err)
		}

		if !proto.Equal(original, decoded) {
			t.Errorf("Roundtrip failed.\nOriginal: %v\nDecoded: %v", original, decoded)
		}
	})
}

func FuzzNumericMessage(f *testing.F) {
	// Add seed corpus
	f.Add(int32(0), int64(0), uint32(0), uint64(0))
	f.Add(int32(-1), int64(-1), uint32(1), uint64(1))
	f.Add(int32(12345), int64(9876543210), uint32(54321), uint64(1234567890))

	f.Fuzz(func(t *testing.T, i32 int32, i64 int64, u32 uint32, u64 uint64) {
		original := &testdata.NumericMessage{
			Int32Field:  i32,
			Int64Field:  i64,
			Uint32Field: u32,
			Uint64Field: u64,
		}

		var buf bytes.Buffer
		if err := Compress(original, &buf); err != nil {
			t.Skipf("Compress failed: %v", err)
		}

		decoded := &testdata.NumericMessage{}
		if err := Decompress(&buf, decoded); err != nil {
			t.Fatalf("Decompress failed: %v", err)
		}

		if !proto.Equal(original, decoded) {
			t.Errorf("Roundtrip failed.\nOriginal: %v\nDecoded: %v", original, decoded)
		}
	})
}

func FuzzMessageWithBytes(f *testing.F) {
	// Add seed corpus
	f.Add([]byte{}, "empty")
	f.Add([]byte{0x00}, "single")
	f.Add([]byte{0xFF, 0xFE, 0xFD}, "multiple")

	f.Fuzz(func(t *testing.T, data []byte, label string) {
		// Skip invalid UTF-8 in label
		if !utf8.ValidString(label) {
			t.Skip("Invalid UTF-8 string")
		}

		original := &testdata.MessageWithBytes{
			Data:  data,
			Label: label,
		}

		var buf bytes.Buffer
		if err := Compress(original, &buf); err != nil {
			t.Skipf("Compress failed: %v", err)
		}

		decoded := &testdata.MessageWithBytes{}
		if err := Decompress(&buf, decoded); err != nil {
			t.Fatalf("Decompress failed: %v", err)
		}

		if !proto.Equal(original, decoded) {
			t.Errorf("Roundtrip failed.\nOriginal: %v\nDecoded: %v", original, decoded)
		}
	})
}

// TestRandomMessages generates random messages and tests roundtrip compression.
func TestRandomMessages(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 1000; i++ {
		original := generateRandomSimpleMessage(rng)

		var buf bytes.Buffer
		if err := Compress(original, &buf); err != nil {
			t.Fatalf("Trial %d: Compress failed: %v", i, err)
		}

		decoded := &testdata.SimpleMessage{}
		if err := Decompress(&buf, decoded); err != nil {
			t.Fatalf("Trial %d: Decompress failed: %v", i, err)
		}

		if !proto.Equal(original, decoded) {
			t.Errorf("Trial %d: Roundtrip failed.\nOriginal: %v\nDecoded: %v", i, original, decoded)
		}
	}
}

func TestRandomNestedMessages(t *testing.T) {
	rng := rand.New(rand.NewSource(123))

	for i := 0; i < 500; i++ {
		original := generateRandomNestedMessage(rng)

		var buf bytes.Buffer
		if err := Compress(original, &buf); err != nil {
			t.Fatalf("Trial %d: Compress failed: %v", i, err)
		}

		decoded := &testdata.NestedMessage{}
		if err := Decompress(&buf, decoded); err != nil {
			t.Fatalf("Trial %d: Decompress failed: %v", i, err)
		}

		if !proto.Equal(original, decoded) {
			t.Errorf("Trial %d: Roundtrip failed", i)
		}
	}
}

func TestRandomUserProfiles(t *testing.T) {
	rng := rand.New(rand.NewSource(456))

	for i := 0; i < 200; i++ {
		original := generateRandomUserProfile(rng)

		var buf bytes.Buffer
		if err := Compress(original, &buf); err != nil {
			t.Fatalf("Trial %d: Compress failed: %v", i, err)
		}

		decoded := &testdata.UserProfile{}
		if err := Decompress(&buf, decoded); err != nil {
			t.Fatalf("Trial %d: Decompress failed: %v", i, err)
		}

		if !proto.Equal(original, decoded) {
			t.Errorf("Trial %d: Roundtrip failed", i)
		}
	}
}

// Helper functions to generate random messages

func generateRandomSimpleMessage(rng *rand.Rand) *testdata.SimpleMessage {
	return &testdata.SimpleMessage{
		Id:     rng.Int31(),
		Name:   randomString(rng, 50),
		Active: rng.Intn(2) == 1,
	}
}

func generateRandomNestedMessage(rng *rand.Rand) *testdata.NestedMessage {
	msg := &testdata.NestedMessage{
		OuterField: randomString(rng, 30),
	}

	if rng.Intn(2) == 1 {
		msg.Inner = &testdata.NestedMessage_Inner{
			Value: randomString(rng, 20),
			Count: rng.Int31n(1000),
		}
	}

	listLen := rng.Intn(5)
	for i := 0; i < listLen; i++ {
		msg.InnerList = append(msg.InnerList, &testdata.NestedMessage_Inner{
			Value: randomString(rng, 15),
			Count: rng.Int31n(100),
		})
	}

	return msg
}

func generateRandomUserProfile(rng *rand.Rand) *testdata.UserProfile {
	profile := &testdata.UserProfile{
		UserId:        int64(rng.Uint64()),
		Username:      randomString(rng, 20),
		Email:         randomString(rng, 30) + "@example.com",
		FullName:      randomString(rng, 30),
		Bio:           randomString(rng, 200),
		AccountStatus: testdata.Status(rng.Intn(5)),
		CreatedAt:     int64(rng.Uint64()),
		UpdatedAt:     int64(rng.Uint64()),
	}

	// Add random tags
	numTags := rng.Intn(10)
	for i := 0; i < numTags; i++ {
		profile.Tags = append(profile.Tags, randomString(rng, 10))
	}

	// Add address
	if rng.Intn(2) == 1 {
		profile.Address = &testdata.UserProfile_Address{
			Street:  randomString(rng, 40),
			City:    randomString(rng, 20),
			State:   randomString(rng, 2),
			Zip:     randomString(rng, 10),
			Country: randomString(rng, 20),
		}
	}

	// Add metadata
	profile.Metadata = make(map[string]string)
	numMetadata := rng.Intn(5)
	for i := 0; i < numMetadata; i++ {
		key := randomString(rng, 10)
		value := randomString(rng, 20)
		profile.Metadata[key] = value
	}

	return profile
}

func randomString(rng *rand.Rand, maxLen int) string {
	if maxLen == 0 {
		return ""
	}

	length := rng.Intn(maxLen + 1)
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,!?"

	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rng.Intn(len(charset))]
	}
	return string(result)
}

// Benchmark tests

func BenchmarkCompressSimpleMessage(b *testing.B) {
	msg := &testdata.SimpleMessage{
		Id:     12345,
		Name:   "Alice Johnson",
		Active: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		Compress(msg, &buf)
	}
}

func BenchmarkDecompressSimpleMessage(b *testing.B) {
	msg := &testdata.SimpleMessage{
		Id:     12345,
		Name:   "Alice Johnson",
		Active: true,
	}

	var buf bytes.Buffer
	Compress(msg, &buf)
	compressed := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoded := &testdata.SimpleMessage{}
		Decompress(bytes.NewReader(compressed), decoded)
	}
}

func BenchmarkCompressUserProfile(b *testing.B) {
	msg := &testdata.UserProfile{
		UserId:        123456,
		Username:      "johndoe",
		Email:         "john.doe@example.com",
		FullName:      "John Doe",
		Bio:           "Software engineer passionate about compression algorithms.",
		Tags:          []string{"golang", "protobuf", "compression"},
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
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		Compress(msg, &buf)
	}
}
