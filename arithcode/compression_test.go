package arithcode

import (
	"bytes"
	"math/rand"
	"testing"
)

// TestCompressionRatioEnglishText tests compression effectiveness on English text of various sizes.
func TestCompressionRatioEnglishText(t *testing.T) {
	// Sample English text for testing
	sampleText := "The quick brown fox jumps over the lazy dog. " +
		"This is a comprehensive test of the English text model compression algorithm. " +
		"Common English words and phrases should compress very well due to the character " +
		"frequency model that favors common letters like 'e', 't', 'a', 'o', 'i', 'n', and 's'. " +
		"The arithmetic coding engine provides near-optimal compression rates by encoding " +
		"symbols based on their probability distributions. "

	testCases := []struct {
		name     string
		text     string
		maxRatio float64 // Maximum acceptable ratio (lower is better)
	}{
		{
			name:     "Small (100 bytes)",
			text:     sampleText[:100],
			maxRatio: 80.0, // Overhead is higher for small texts
		},
		{
			name:     "Medium (1KB)",
			text:     repeatText(sampleText, 1024),
			maxRatio: 65.0, // Better compression with more data
		},
		{
			name:     "Large (10KB)",
			text:     repeatText(sampleText, 10*1024),
			maxRatio: 60.0, // Even better compression
		},
		{
			name:     "Very Large (100KB)",
			text:     repeatText(sampleText, 100*1024),
			maxRatio: 60.0, // Best compression ratio
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := EncodeString(tc.text, &buf); err != nil {
				t.Fatalf("EncodeString failed: %v", err)
			}

			originalSize := len(tc.text)
			compressedSize := buf.Len()
			ratio := 100.0 * float64(compressedSize) / float64(originalSize)

			t.Logf("Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
				originalSize, compressedSize, ratio)

			if ratio > tc.maxRatio {
				t.Errorf("Compression ratio %.2f%% exceeds maximum %.2f%%", ratio, tc.maxRatio)
			}

			// Verify decompression works
			decoded, err := DecodeString(&buf)
			if err != nil {
				t.Fatalf("DecodeString failed: %v", err)
			}
			if decoded != tc.text {
				t.Error("Decoded text doesn't match original")
			}
		})
	}
}

// TestCompressionRatioBinaryData tests compression on random binary data (worst case).
func TestCompressionRatioBinaryData(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))

	testCases := []struct {
		name string
		size int
	}{
		{"Small (100 bytes)", 100},
		{"Medium (1KB)", 1024},
		{"Large (10KB)", 10 * 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate random binary data (should not compress well)
			data := make([]byte, tc.size)
			rng.Read(data)

			// Convert to string for encoding
			text := string(data)

			var buf bytes.Buffer
			if err := EncodeString(text, &buf); err != nil {
				t.Fatalf("EncodeString failed: %v", err)
			}

			originalSize := len(text)
			compressedSize := buf.Len()
			ratio := 100.0 * float64(compressedSize) / float64(originalSize)

			t.Logf("Random data - Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
				originalSize, compressedSize, ratio)

			// Binary data should not compress much (expect close to 100% or slightly more due to overhead)
			if ratio < 95.0 {
				t.Logf("Note: Random data compressed to %.2f%% (unexpected)", ratio)
			}
		})
	}
}

// TestCompressionComparison compares English text vs binary data compression.
func TestCompressionComparison(t *testing.T) {
	size := 10 * 1024 // 10KB

	// English text
	englishText := repeatText("The quick brown fox jumps over the lazy dog. ", size)
	var englishBuf bytes.Buffer
	if err := EncodeString(englishText, &englishBuf); err != nil {
		t.Fatalf("EncodeString failed: %v", err)
	}
	englishRatio := 100.0 * float64(englishBuf.Len()) / float64(len(englishText))

	// Random binary data
	rng := rand.New(rand.NewSource(42))
	binaryData := make([]byte, size)
	rng.Read(binaryData)
	var binaryBuf bytes.Buffer
	if err := EncodeString(string(binaryData), &binaryBuf); err != nil {
		t.Fatalf("EncodeString failed: %v", err)
	}
	binaryRatio := 100.0 * float64(binaryBuf.Len()) / float64(len(binaryData))

	t.Logf("English text compression: %.2f%%", englishRatio)
	t.Logf("Binary data compression: %.2f%%", binaryRatio)
	t.Logf("Improvement: %.2f percentage points", binaryRatio-englishRatio)

	// English should compress significantly better
	if englishRatio >= binaryRatio {
		t.Errorf("English text (%.2f%%) should compress better than binary data (%.2f%%)",
			englishRatio, binaryRatio)
	}
}

// repeatText repeats text to reach approximately the target size.
func repeatText(text string, targetSize int) string {
	if len(text) >= targetSize {
		return text[:targetSize]
	}

	repeats := (targetSize / len(text)) + 1
	var result string
	for i := 0; i < repeats; i++ {
		result += text
	}
	return result[:targetSize]
}
