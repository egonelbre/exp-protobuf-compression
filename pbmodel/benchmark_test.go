package pbmodel

import (
	"bytes"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/egonelbre/exp-protobuf-compression/pbmodel/testdata"
)

// BenchmarkCompressMessages benchmarks compression of various message types and sizes.
func BenchmarkCompressMessages(b *testing.B) {
	benchmarks := []struct {
		name string
		msg  proto.Message
	}{
		{
			name: "SimpleMessage",
			msg: &testdata.SimpleMessage{
				Id:     12345,
				Name:   "Alice Johnson",
				Active: true,
			},
		},
		{
			name: "UserProfile_Small",
			msg: &testdata.UserProfile{
				UserId:   123456,
				Username: "johndoe",
				Email:    "john.doe@example.com",
				FullName: "John Doe",
				Bio:      "Software engineer",
			},
		},
		{
			name: "UserProfile_Large",
			msg:  createLargeUserProfile(),
		},
		{
			name: "RepeatedMessage_1000",
			msg: &testdata.RepeatedMessage{
				Numbers: func() []int32 {
					nums := make([]int32, 1000)
					for i := range nums {
						nums[i] = int32(i)
					}
					return nums
				}(),
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			size := proto.Size(bm.msg)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				if err := Compress(bm.msg, &buf); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDecompressMessages benchmarks decompression of various message types.
func BenchmarkDecompressMessages(b *testing.B) {
	benchmarks := []struct {
		name string
		msg  proto.Message
	}{
		{
			name: "SimpleMessage",
			msg: &testdata.SimpleMessage{
				Id:     12345,
				Name:   "Alice Johnson",
				Active: true,
			},
		},
		{
			name: "UserProfile_Large",
			msg:  createLargeUserProfile(),
		},
		{
			name: "RepeatedMessage_1000",
			msg: &testdata.RepeatedMessage{
				Numbers: func() []int32 {
					nums := make([]int32, 1000)
					for i := range nums {
						nums[i] = int32(i)
					}
					return nums
				}(),
			},
		},
	}

	for _, bm := range benchmarks {
		// Pre-compress the message
		var compressed bytes.Buffer
		if err := Compress(bm.msg, &compressed); err != nil {
			b.Fatal(err)
		}
		compressedData := compressed.Bytes()
		size := proto.Size(bm.msg)

		b.Run(bm.name, func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				decoded := reflect.New(reflect.TypeOf(bm.msg).Elem()).Interface().(proto.Message)
				if err := Decompress(bytes.NewReader(compressedData), decoded); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRoundtripMessages benchmarks full compress+decompress cycle.
func BenchmarkRoundtripMessages(b *testing.B) {
	msg := createLargeUserProfile()
	size := proto.Size(msg)

	b.SetBytes(int64(size))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := Compress(msg, &buf); err != nil {
			b.Fatal(err)
		}

		decoded := &testdata.UserProfile{}
		if err := Decompress(&buf, decoded); err != nil {
			b.Fatal(err)
		}
	}
}
