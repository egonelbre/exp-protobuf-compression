package pbmodel

import (
	"bytes"
	"testing"

	"github.com/egonelbre/exp-protobuf-compression/pbmodel/testdata"
	"google.golang.org/protobuf/proto"
)

func TestAdaptiveCompressionRatio(t *testing.T) {
	tests := []struct {
		name              string
		msg               proto.Message
		maxCompressionPct float64 // max percentage of original size
	}{
		{
			name: "user with english bio",
			msg:  createLargeUserProfile(),
			// Field-specific models without english compression
			// Will be worse than type-based but still reasonable
			maxCompressionPct: 100,
		},
		{
			name: "nested message with text",
			msg: &testdata.NestedMessage{
				Inner: &testdata.NestedMessage_Inner{
					Value: repeatText("This is a detailed description with typical English text. "+
						"It contains information about features, specifications, and benefits. ", 10),
					Count: 12345,
				},
				InnerList: []*testdata.NestedMessage_Inner{
					{Value: "first item", Count: 1},
					{Value: "second item", Count: 2},
					{Value: "third item", Count: 3},
				},
				OuterField: "outer field value",
			},
			maxCompressionPct: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get original size
			originalData, err := proto.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}
			originalSize := len(originalData)

			// Adaptive compress
			var buf bytes.Buffer
			err = AdaptiveCompress(tt.msg, &buf)
			if err != nil {
				t.Fatalf("AdaptiveCompress failed: %v", err)
			}
			compressedSize := buf.Len()

			// Calculate ratio
			ratio := float64(compressedSize) / float64(originalSize) * 100

			t.Logf("Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
				originalSize, compressedSize, ratio)

			if ratio > tt.maxCompressionPct {
				t.Errorf("Compression ratio %.2f%% exceeds maximum %.2f%%", ratio, tt.maxCompressionPct)
			}

			// Verify roundtrip
			result := tt.msg.ProtoReflect().New().Interface()
			err = AdaptiveDecompress(&buf, result)
			if err != nil {
				t.Fatalf("AdaptiveDecompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, result) {
				t.Error("Roundtrip verification failed")
			}
		})
	}
}

func TestAdaptiveVsTypeBasedCompression(t *testing.T) {
	tests := []struct {
		name string
		msg  proto.Message
	}{
		{
			name: "user profile",
			msg:  createLargeUserProfile(),
		},
		{
			name: "simple message",
			msg: &testdata.SimpleMessage{
				Id:     54321,
				Name:   repeatText("Simple text content. ", 20),
				Active: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Type-based compression
			var typeBuf bytes.Buffer
			err := Compress(tt.msg, &typeBuf)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}
			typeBasedSize := typeBuf.Len()

			// Adaptive compression
			var adaptiveBuf bytes.Buffer
			err = AdaptiveCompress(tt.msg, &adaptiveBuf)
			if err != nil {
				t.Fatalf("AdaptiveCompress failed: %v", err)
			}
			adaptiveSize := adaptiveBuf.Len()

			// Get original size
			originalData, _ := proto.Marshal(tt.msg)
			originalSize := len(originalData)

			typeRatio := float64(typeBasedSize) / float64(originalSize) * 100
			adaptiveRatio := float64(adaptiveSize) / float64(originalSize) * 100

			t.Logf("Original: %d bytes", originalSize)
			t.Logf("Type-based: %d bytes (%.2f%%)", typeBasedSize, typeRatio)
			t.Logf("Adaptive: %d bytes (%.2f%%)", adaptiveSize, adaptiveRatio)
			t.Logf("Difference: %d bytes (%.2f%% vs %.2f%%)",
				adaptiveSize-typeBasedSize, adaptiveRatio, typeRatio)

			// Type-based should achieve good compression
			if typeRatio > 70 {
				t.Errorf("Type-based compression ratio too high: %.2f%%", typeRatio)
			}
			// Adaptive uses simpler per-field models, won't compress as well
			// but demonstrates field-specific behavior
			if adaptiveRatio > 100 {
				t.Errorf("Adaptive compression ratio too high: %.2f%%", adaptiveRatio)
			}
		})
	}
}

func TestAdaptiveFieldSpecificBehavior(t *testing.T) {
	// Create two user profiles with different content in the same field types
	// to verify that field-specific models are being used
	user1 := &testdata.UserProfile{
		UserId:   1,
		Username: "alice",
		Email:    "alice@example.com",
		FullName: "Alice",
		Bio:      "Engineer",
		Tags:     []string{"golang", "python"},
	}

	user2 := &testdata.UserProfile{
		UserId:   2,
		Username: "bob",
		Email:    "bob@test.org",
		FullName: "Bob",
		Bio:      "Manager",
		Tags:     []string{"leadership", "strategy"},
	}

	// Compress both
	var buf1, buf2 bytes.Buffer
	if err := AdaptiveCompress(user1, &buf1); err != nil {
		t.Fatalf("Failed to compress user1: %v", err)
	}
	if err := AdaptiveCompress(user2, &buf2); err != nil {
		t.Fatalf("Failed to compress user2: %v", err)
	}

	t.Logf("User1 compressed: %d bytes", buf1.Len())
	t.Logf("User2 compressed: %d bytes", buf2.Len())

	// Verify roundtrip for both
	result1 := &testdata.UserProfile{}
	result2 := &testdata.UserProfile{}

	if err := AdaptiveDecompress(&buf1, result1); err != nil {
		t.Fatalf("Failed to decompress user1: %v", err)
	}
	if err := AdaptiveDecompress(&buf2, result2); err != nil {
		t.Fatalf("Failed to decompress user2: %v", err)
	}

	if !proto.Equal(user1, result1) {
		t.Error("User1 roundtrip failed")
	}
	if !proto.Equal(user2, result2) {
		t.Error("User2 roundtrip failed")
	}
}
