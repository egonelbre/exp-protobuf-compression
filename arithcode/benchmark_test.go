package arithcode

import (
	"bytes"
	"testing"
)

// BenchmarkEncodeString benchmarks string encoding with various sizes.
func BenchmarkEncodeString(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a test of compression performance. "

	benchmarks := []struct {
		name string
		text string
	}{
		{"Small_100B", repeatText(text, 100)},
		{"Medium_1KB", repeatText(text, 1024)},
		{"Large_10KB", repeatText(text, 10*1024)},
		{"VeryLarge_100KB", repeatText(text, 100*1024)},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.SetBytes(int64(len(bm.text)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				if err := EncodeString(bm.text, &buf); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDecodeString benchmarks string decoding with various sizes.
func BenchmarkDecodeString(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a test of compression performance. "

	benchmarks := []struct {
		name string
		text string
	}{
		{"Small_100B", repeatText(text, 100)},
		{"Medium_1KB", repeatText(text, 1024)},
		{"Large_10KB", repeatText(text, 10*1024)},
		{"VeryLarge_100KB", repeatText(text, 100*1024)},
	}

	for _, bm := range benchmarks {
		// Pre-compress the data
		var compressed bytes.Buffer
		if err := EncodeString(bm.text, &compressed); err != nil {
			b.Fatal(err)
		}
		compressedData := compressed.Bytes()

		b.Run(bm.name, func(b *testing.B) {
			b.SetBytes(int64(len(bm.text)))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := DecodeString(bytes.NewReader(compressedData))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
