package pbmodel

import (
	"bytes"
	"testing"

	"github.com/egonelbre/protobuf-compression/pbmodel/testdata"
)

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

func BenchmarkAdaptiveCompress(b *testing.B) {
	benchmarks := []struct {
		name string
		msg  *testdata.UserProfile
	}{
		{
			name: "small",
			msg: &testdata.UserProfile{
				UserId:   1,
				Username: "test",
				Email:    "test@example.com",
				FullName: "Test",
			},
		},
		{
			name: "medium",
			msg: &testdata.UserProfile{
				UserId:   2,
				Username: "testuser",
				Email:    "test@example.com",
				FullName: "Test User",
				Bio:      repeatText("This is a bio. ", 10),
				Tags:     []string{"tag1", "tag2", "tag3"},
			},
		},
		{
			name: "large",
			msg:  createLargeUserProfile(),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var buf bytes.Buffer
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				if err := AdaptiveCompress(bm.msg, &buf); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkAdaptiveDecompress(b *testing.B) {
	benchmarks := []struct {
		name string
		msg  *testdata.UserProfile
	}{
		{
			name: "small",
			msg: &testdata.UserProfile{
				UserId:   1,
				Username: "test",
				Email:    "test@example.com",
				FullName: "Test",
			},
		},
		{
			name: "medium",
			msg: &testdata.UserProfile{
				UserId:   2,
				Username: "testuser",
				Email:    "test@example.com",
				FullName: "Test User",
				Bio:      repeatText("This is a bio. ", 10),
				Tags:     []string{"tag1", "tag2", "tag3"},
			},
		},
		{
			name: "large",
			msg:  createLargeUserProfile(),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Pre-compress the message
			var compressed bytes.Buffer
			if err := AdaptiveCompress(bm.msg, &compressed); err != nil {
				b.Fatal(err)
			}
			compressedData := compressed.Bytes()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := &testdata.UserProfile{}
				reader := bytes.NewReader(compressedData)
				if err := AdaptiveDecompress(reader, result); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkAdaptiveRoundtrip(b *testing.B) {
	benchmarks := []struct {
		name string
		msg  *testdata.UserProfile
	}{
		{
			name: "small",
			msg: &testdata.UserProfile{
				UserId:   1,
				Username: "test",
				Email:    "test@example.com",
				FullName: "Test",
			},
		},
		{
			name: "medium",
			msg: &testdata.UserProfile{
				UserId:   2,
				Username: "testuser",
				Email:    "test@example.com",
				FullName: "Test User",
				Bio:      repeatText("This is a bio. ", 10),
				Tags:     []string{"tag1", "tag2", "tag3"},
			},
		},
		{
			name: "large",
			msg:  createLargeUserProfile(),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var buf bytes.Buffer
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				if err := AdaptiveCompress(bm.msg, &buf); err != nil {
					b.Fatal(err)
				}
				result := &testdata.UserProfile{}
				if err := AdaptiveDecompress(&buf, result); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkTypeBasedVsAdaptive(b *testing.B) {
	msg := createLargeUserProfile()

	b.Run("type-based-compress", func(b *testing.B) {
		var buf bytes.Buffer
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			if err := Compress(msg, &buf); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("adaptive-compress", func(b *testing.B) {
		var buf bytes.Buffer
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			if err := AdaptiveCompress(msg, &buf); err != nil {
				b.Fatal(err)
			}
		}
	})

	// Pre-compress for decompress benchmarks
	var typeCompressed, adaptiveCompressed bytes.Buffer
	Compress(msg, &typeCompressed)
	AdaptiveCompress(msg, &adaptiveCompressed)
	typeData := typeCompressed.Bytes()
	adaptiveData := adaptiveCompressed.Bytes()

	b.Run("type-based-decompress", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := &testdata.UserProfile{}
			reader := bytes.NewReader(typeData)
			if err := Decompress(reader, result); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("adaptive-decompress", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result := &testdata.UserProfile{}
			reader := bytes.NewReader(adaptiveData)
			if err := AdaptiveDecompress(reader, result); err != nil {
				b.Fatal(err)
			}
		}
	})
}
