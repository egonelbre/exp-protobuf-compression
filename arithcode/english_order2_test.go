package arithcode

import (
	"bytes"
	"testing"
)

func TestEnglishOrder2Model(t *testing.T) {
	tests := []string{
		"Hello World",
		"The quick brown fox jumps over the lazy dog",
		"This is a test of order-2 compression",
		"Location: 37.5318,-122.3898",
		"Temperature: 72.5F, Humidity: 45%",
		"Hello! How are you today? I hope everything is going well.",
	}
	
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			// Encode
			var buf bytes.Buffer
			if err := EncodeStringOrder2(test, &buf); err != nil {
				t.Fatalf("EncodeStringOrder2 failed: %v", err)
			}
			
			// Decode
			result, err := DecodeStringOrder2(&buf)
			if err != nil {
				t.Fatalf("DecodeStringOrder2 failed: %v", err)
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

func TestEnglishOrderComparison(t *testing.T) {
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
			
			// Order-2 compression
			var buf2 bytes.Buffer
			if err := EncodeStringOrder2(test, &buf2); err != nil {
				t.Fatalf("Order-2 encode failed: %v", err)
			}
			
			improvement01 := float64(buf0.Len()-buf1.Len()) / float64(buf0.Len()) * 100
			improvement12 := float64(buf1.Len()-buf2.Len()) / float64(buf1.Len()) * 100
			improvement02 := float64(buf0.Len()-buf2.Len()) / float64(buf0.Len()) * 100
			
			t.Logf("Text: %q", test)
			t.Logf("Original: %d bytes", len(test))
			t.Logf("Order-0:  %d bytes (%.1f%%)", buf0.Len(), float64(buf0.Len())/float64(len(test))*100)
			t.Logf("Order-1:  %d bytes (%.1f%%) - %.1f%% better than order-0", buf1.Len(), float64(buf1.Len())/float64(len(test))*100, improvement01)
			t.Logf("Order-2:  %d bytes (%.1f%%) - %.1f%% better than order-1, %.1f%% better than order-0", 
				buf2.Len(), float64(buf2.Len())/float64(len(test))*100, improvement12, improvement02)
		})
	}
}

func BenchmarkEnglishOrder2(b *testing.B) {
	text := "The quick brown fox jumps over the lazy dog. This is a test of English text compression using order-2 model."
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := EncodeStringOrder2(text, &buf); err != nil {
			b.Fatal(err)
		}
	}
}
