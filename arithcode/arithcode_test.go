package arithcode

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestUniformModel(t *testing.T) {
	model := NewUniformModel(256)

	if model.SymbolCount() != 256 {
		t.Errorf("Expected 256 symbols, got %d", model.SymbolCount())
	}

	if model.TotalFreq() != 256 {
		t.Errorf("Expected total freq 256, got %d", model.TotalFreq())
	}

	for i := 0; i < 256; i++ {
		low, high := model.Freq(i)
		if low != uint64(i) || high != uint64(i+1) {
			t.Errorf("Symbol %d: expected [%d, %d), got [%d, %d)", i, i, i+1, low, high)
		}

		found := model.Find(uint64(i))
		if found != i {
			t.Errorf("Find(%d) = %d, expected %d", i, found, i)
		}
	}
}

func TestFrequencyTable(t *testing.T) {
	freqs := []uint64{10, 20, 30, 40}
	model := NewFrequencyTable(freqs)

	if model.SymbolCount() != 4 {
		t.Errorf("Expected 4 symbols, got %d", model.SymbolCount())
	}

	if model.TotalFreq() != 100 {
		t.Errorf("Expected total freq 100, got %d", model.TotalFreq())
	}

	expected := [][2]uint64{
		{0, 10},
		{10, 30},
		{30, 60},
		{60, 100},
	}

	for i := 0; i < 4; i++ {
		low, high := model.Freq(i)
		if low != expected[i][0] || high != expected[i][1] {
			t.Errorf("Symbol %d: expected [%d, %d), got [%d, %d)",
				i, expected[i][0], expected[i][1], low, high)
		}
	}

	// Test Find
	tests := []struct {
		cumFreq  uint64
		expected int
	}{
		{0, 0},
		{5, 0},
		{9, 0},
		{10, 1},
		{25, 1},
		{29, 1},
		{30, 2},
		{50, 2},
		{59, 2},
		{60, 3},
		{99, 3},
	}

	for _, tt := range tests {
		symbol := model.Find(tt.cumFreq)
		if symbol != tt.expected {
			t.Errorf("Find(%d) = %d, expected %d", tt.cumFreq, symbol, tt.expected)
		}
	}
}

func TestRoundtripUniform(t *testing.T) {
	model := NewUniformModel(256)
	data := []int{0, 1, 2, 255, 128, 64, 32, 16, 8, 4, 2, 1, 0}

	// Encode
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for _, symbol := range data {
		if err := enc.Encode(symbol, model); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Decode
	dec, err := NewDecoder(&buf)
	if err != nil {
		t.Fatalf("NewDecoder failed: %v", err)
	}

	for i, expected := range data {
		symbol, err := dec.Decode(model)
		if err != nil {
			t.Fatalf("Decode at position %d failed: %v", i, err)
		}
		if symbol != expected {
			t.Errorf("Position %d: expected %d, got %d", i, expected, symbol)
		}
	}
}

func TestRoundtripFrequencyTable(t *testing.T) {
	freqs := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	model := NewFrequencyTable(freqs)
	data := []int{0, 9, 5, 3, 7, 1, 8, 2, 4, 6, 0, 0, 9, 9}

	// Encode
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for _, symbol := range data {
		if err := enc.Encode(symbol, model); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Decode
	dec, err := NewDecoder(&buf)
	if err != nil {
		t.Fatalf("NewDecoder failed: %v", err)
	}

	for i, expected := range data {
		symbol, err := dec.Decode(model)
		if err != nil {
			t.Fatalf("Decode at position %d failed: %v", i, err)
		}
		if symbol != expected {
			t.Errorf("Position %d: expected %d, got %d", i, expected, symbol)
		}
	}
}

func TestRoundtripRandomData(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))

	for trial := 0; trial < 100; trial++ {
		// Generate random model
		numSymbols := 2 + rng.Intn(50)
		freqs := make([]uint64, numSymbols)
		for i := range freqs {
			freqs[i] = 1 + uint64(rng.Intn(100))
		}
		model := NewFrequencyTable(freqs)

		// Generate random data
		dataLen := 10 + rng.Intn(100)
		data := make([]int, dataLen)
		for i := range data {
			data[i] = rng.Intn(numSymbols)
		}

		// Encode
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		for _, symbol := range data {
			if err := enc.Encode(symbol, model); err != nil {
				t.Fatalf("Trial %d: Encode failed: %v", trial, err)
			}
		}
		if err := enc.Close(); err != nil {
			t.Fatalf("Trial %d: Close failed: %v", trial, err)
		}

		// Decode
		dec, err := NewDecoder(&buf)
		if err != nil {
			t.Fatalf("Trial %d: NewDecoder failed: %v", trial, err)
		}

		for i, expected := range data {
			symbol, err := dec.Decode(model)
			if err != nil {
				t.Fatalf("Trial %d, position %d: Decode failed: %v", trial, i, err)
			}
			if symbol != expected {
				t.Errorf("Trial %d, position %d: expected %d, got %d", trial, i, expected, symbol)
			}
		}
	}
}

func TestEnglishStringRoundtrip(t *testing.T) {
	testCases := []string{
		"",
		"a",
		"Hello, World!",
		"The quick brown fox jumps over the lazy dog.",
		"This is a longer test string with multiple sentences. It includes punctuation, numbers like 123, and symbols like @#$%.",
		"UPPERCASE and lowercase and MiXeD cAsE",
		"Tabs\tand\nnewlines\r\nare supported",
		"Unicode: café, naïve, 日本語", // Test non-English characters
	}

	for i, original := range testCases {
		var buf bytes.Buffer
		if err := EncodeString(original, &buf); err != nil {
			t.Fatalf("Test %d: EncodeString failed: %v", i, err)
		}

		decoded, err := DecodeString(&buf)
		if err != nil {
			t.Fatalf("Test %d: DecodeString failed: %v", i, err)
		}

		if decoded != original {
			t.Errorf("Test %d: strings don't match.\nOriginal: %q\nDecoded:  %q", i, original, decoded)
		}
	}
}

func TestEnglishStringCompression(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog. " +
		"This is a test of the English text model compression. " +
		"Common English words should compress well."

	var buf bytes.Buffer
	if err := EncodeString(text, &buf); err != nil {
		t.Fatalf("EncodeString failed: %v", err)
	}

	compressedSize := buf.Len()
	originalSize := len(text)

	t.Logf("Original size: %d bytes, Compressed size: %d bytes, Ratio: %.2f%%",
		originalSize, compressedSize, 100.0*float64(compressedSize)/float64(originalSize))

	// We expect some compression for English text
	// This is a weak test since the string is short, but it should still compress somewhat
	if compressedSize >= originalSize {
		t.Logf("Warning: Compressed size (%d) >= original size (%d)", compressedSize, originalSize)
	}
}

func TestEmptyData(t *testing.T) {
	model := NewUniformModel(256)
	data := []int{}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for _, symbol := range data {
		if err := enc.Encode(symbol, model); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Should produce some output even for empty data (the final state)
	if buf.Len() == 0 {
		t.Error("Expected non-zero output for empty data")
	}
}

func BenchmarkEncode(b *testing.B) {
	model := NewUniformModel(256)
	data := make([]int, 1000)
	rng := rand.New(rand.NewSource(42))
	for i := range data {
		data[i] = rng.Intn(256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		for _, symbol := range data {
			enc.Encode(symbol, model)
		}
		enc.Close()
	}
}

func BenchmarkDecode(b *testing.B) {
	model := NewUniformModel(256)
	data := make([]int, 1000)
	rng := rand.New(rand.NewSource(42))
	for i := range data {
		data[i] = rng.Intn(256)
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for _, symbol := range data {
		enc.Encode(symbol, model)
	}
	enc.Close()

	compressed := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec, _ := NewDecoder(bytes.NewReader(compressed))
		for j := 0; j < len(data); j++ {
			dec.Decode(model)
		}
	}
}

// TestCompressionRatioEnglishText tests compression effectiveness on English text of various sizes.
