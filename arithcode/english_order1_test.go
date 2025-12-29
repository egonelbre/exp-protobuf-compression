package arithcode

import (
	"bytes"
	"testing"
)

func TestEnglishOrder1Model(t *testing.T) {
	tests := []string{
		"Hello World",
		"The quick brown fox jumps over the lazy dog",
		"This is a test of order-1 compression",
		"Location: 37.5318,-122.3898",
		"Temperature: 72.5F, Humidity: 45%",
		"Hello! How are you today? I hope everything is going well.",
	}
	
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			// Encode
			var buf bytes.Buffer
			if err := EncodeStringOrder1(test, &buf); err != nil {
				t.Fatalf("EncodeStringOrder1 failed: %v", err)
			}
			
			// Decode
			result, err := DecodeStringOrder1(&buf)
			if err != nil {
				t.Fatalf("DecodeStringOrder1 failed: %v", err)
			}
			
			// Verify
			if result != test {
				t.Errorf("Decoded string doesn't match:\noriginal: %q\nresult:   %q", test, result)
			}
			
			t.Logf("String: %q", test)
			t.Logf("Original: %d bytes, Compressed: %d bytes (%.1f%%)",
				len(test), buf.Len(), float64(buf.Len())/float64(len(test))*100)
		})
	}
}

func TestEnglishOrder1vsOrder0(t *testing.T) {
	testStrings := []string{
		"The quick brown fox jumps over the lazy dog. The dog was very lazy indeed.",
		"Hello! How are you today? I hope you are doing well. Have a great day!",
		"Location: 37.5318,-122.3898. Temperature: 72.5F. Humidity: 45%. Wind: 5mph.",
		"This is a longer text to test the compression. It contains multiple sentences. Each sentence has different words. But there are patterns in English text that should help compression.",
	}
	
	for i, test := range testStrings {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			// Order-0 compression
			var buf0 bytes.Buffer
			if err := EncodeString(test, &buf0); err != nil {
				t.Fatalf("Order-0 encode failed: %v", err)
			}
			
			// Order-1 compression
			var buf1 bytes.Buffer
			if err := EncodeStringOrder1(test, &buf1); err != nil {
				t.Fatalf("Order-1 encode failed: %v", err)
			}
			
			improvement := float64(buf0.Len()-buf1.Len()) / float64(buf0.Len()) * 100
			
			t.Logf("Text: %q", test)
			t.Logf("Original: %d bytes", len(test))
			t.Logf("Order-0:  %d bytes (%.1f%%)", buf0.Len(), float64(buf0.Len())/float64(len(test))*100)
			t.Logf("Order-1:  %d bytes (%.1f%%)", buf1.Len(), float64(buf1.Len())/float64(len(test))*100)
			t.Logf("Improvement: %.1f%%", improvement)
			
			// Order-1 should generally be better or equal
			if buf1.Len() > buf0.Len()*11/10 { // Allow 10% worse in edge cases
				t.Logf("Warning: Order-1 is significantly worse than order-0")
			}
		})
	}
}

func BenchmarkEnglishOrder0(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a test of English text compression using order-0 model."
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := EncodeString(text, &buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEnglishOrder1(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a test of English text compression using order-1 model."
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := EncodeStringOrder1(text, &buf); err != nil {
			b.Fatal(err)
		}
	}
}
