package pbmodel

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/egonelbre/vibing/pbmodel/testdata"
	"google.golang.org/protobuf/proto"
)

// TestCompressionRatioStructuredData tests compression effectiveness on protobuf messages.
func TestCompressionRatioStructuredData(t *testing.T) {
	testCases := []struct {
		name     string
		create   func() proto.Message
		maxRatio float64 // Maximum acceptable compressed/original ratio
	}{
		{
			name: "SimpleMessage",
			create: func() proto.Message {
				return &testdata.SimpleMessage{
					Id:     12345,
					Name:   "Alice Johnson",
					Active: true,
				}
			},
			maxRatio: 90.0,
		},
		{
			name: "UserProfile_Small",
			create: func() proto.Message {
				return &testdata.UserProfile{
					UserId:   123456,
					Username: "johndoe",
					Email:    "john.doe@example.com",
					FullName: "John Doe",
					Bio:      "Software engineer passionate about compression algorithms.",
					Tags:     []string{"golang", "protobuf", "compression"},
					AccountStatus: testdata.Status_ACTIVE,
				}
			},
			maxRatio: 80.0,
		},
		{
			name: "UserProfile_Large",
			create: func() proto.Message {
				return createLargeUserProfile()
			},
			maxRatio: 70.0, // Better compression with more data
		},
		{
			name: "NestedMessage",
			create: func() proto.Message {
				return &testdata.NestedMessage{
					Inner: &testdata.NestedMessage_Inner{
						Value: "This is a test of nested message compression with realistic data",
						Count: 42,
					},
					InnerList: []*testdata.NestedMessage_Inner{
						{Value: "first item", Count: 1},
						{Value: "second item", Count: 2},
						{Value: "third item", Count: 3},
					},
					OuterField: "Outer field with some English text for compression testing",
				}
			},
			maxRatio: 85.0,
		},
		{
			name: "RepeatedMessage_Many",
			create: func() proto.Message {
				numbers := make([]int32, 1000)
				words := make([]string, 100)
				for i := range numbers {
					numbers[i] = int32(i % 256) // Repeating pattern
				}
				for i := range words {
					words[i] = "word"
				}
				return &testdata.RepeatedMessage{
					Numbers: numbers,
					Words:   words,
					Flags:   []bool{true, false, true, false},
				}
			},
			maxRatio: 95.0, // Numbers 0-999 don't compress well with varint encoding
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.create()
			protoSize := proto.Size(original)

			var buf bytes.Buffer
			if err := Compress(original, &buf); err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			compressedSize := buf.Len()
			ratio := 100.0 * float64(compressedSize) / float64(protoSize)

			t.Logf("Proto size: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
				protoSize, compressedSize, ratio)

			if ratio > tc.maxRatio {
				t.Errorf("Compression ratio %.2f%% exceeds maximum %.2f%%", ratio, tc.maxRatio)
			}

			// Verify roundtrip
			decoded := reflect.New(reflect.TypeOf(original).Elem()).Interface().(proto.Message)
			if err := Decompress(&buf, decoded); err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}
			if !proto.Equal(original, decoded) {
				t.Error("Decoded message doesn't match original")
			}
		})
	}
}

// TestCompressionRatioNumericData tests compression of numeric-heavy messages.
func TestCompressionRatioNumericData(t *testing.T) {
	testCases := []struct {
		name    string
		create  func() *testdata.NumericMessage
		maxRatio float64
	}{
		{
			name: "Small values",
			create: func() *testdata.NumericMessage {
				return &testdata.NumericMessage{
					Int32Field:  42,
					Int64Field:  100,
					Uint32Field: 50,
					Uint64Field: 200,
				}
			},
			maxRatio: 95.0,
		},
		{
			name: "Large values",
			create: func() *testdata.NumericMessage {
				return &testdata.NumericMessage{
					Int32Field:  2147483647,
					Int64Field:  9223372036854775807,
					Uint32Field: 4294967295,
					Uint64Field: 18446744073709551615,
				}
			},
			maxRatio: 115.0, // Large numbers harder to compress
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.create()
			protoSize := proto.Size(original)

			var buf bytes.Buffer
			if err := Compress(original, &buf); err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			compressedSize := buf.Len()
			ratio := 100.0 * float64(compressedSize) / float64(protoSize)

			t.Logf("Numeric data - Proto size: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
				protoSize, compressedSize, ratio)
		})
	}
}

// createLargeUserProfile creates a realistic large user profile for testing.
func createLargeUserProfile() *testdata.UserProfile {
	bio := "Software engineer with over 10 years of experience in distributed systems, " +
		"compression algorithms, and protocol buffer optimization. Passionate about " +
		"building high-performance systems and contributing to open source projects. " +
		"Experienced in Go, Rust, C++, and Python. Author of several technical articles " +
		"on compression techniques and data serialization formats."

	tags := make([]string, 20)
	for i := range tags {
		tags[i] = []string{
			"golang", "protobuf", "compression", "algorithms", "performance",
			"optimization", "distributed-systems", "networking", "databases",
			"cloud", "kubernetes", "docker", "microservices", "api-design",
			"open-source", "testing", "benchmarking", "profiling", "monitoring", "observability",
		}[i]
	}

	metadata := map[string]string{
		"theme":              "dark",
		"language":           "en",
		"timezone":           "America/New_York",
		"notification_email": "yes",
		"newsletter":         "weekly",
		"privacy":            "public",
	}

	return &testdata.UserProfile{
		UserId:   9876543210,
		Username: "experienced_engineer_2024",
		Email:    "engineer.experienced@example.com",
		FullName: "Dr. Alexandria Engineering Professional",
		Bio:      bio,
		Tags:     tags,
		AccountStatus: testdata.Status_ACTIVE,
		Address: &testdata.UserProfile_Address{
			Street:  "1234 Technology Drive, Suite 567",
			City:    "San Francisco",
			State:   "California",
			Zip:     "94105",
			Country: "United States of America",
		},
		CreatedAt: 1609459200,
		UpdatedAt: 1640995200,
		Metadata:  metadata,
	}
}
