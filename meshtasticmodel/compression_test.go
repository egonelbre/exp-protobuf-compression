package meshtasticmodel

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
)

func TestMeshtasticCompressionRatio(t *testing.T) {
	tests := []struct {
		name              string
		msg               proto.Message
		maxCompressionPct float64
	}{
		{
			name: "Position with full data",
			msg: &meshtastic.Position{
				LatitudeI:  proto.Int32(375317890),
				LongitudeI: proto.Int32(-1223898570),
				Altitude:   proto.Int32(100),
				Time:       1703520000,
			},
			maxCompressionPct: 100, // V3 adds 1 byte strategy flag overhead, tiny message with fixed-width fields
		},
		{
			name: "User profile",
			msg: &meshtastic.User{
				Id:        "!12345678",
				LongName:  "Test Meshtastic Node",
				ShortName: "TEST",
				HwModel:   meshtastic.HardwareModel_TBEAM,
			},
			maxCompressionPct: 90,
		},
		{
			name: "Text message packet",
			msg: &meshtastic.MeshPacket{
				From: 123456789,
				To:   987654321,
				PayloadVariant: &meshtastic.MeshPacket_Decoded{
					Decoded: &meshtastic.Data{
						Portnum: meshtastic.PortNum_TEXT_MESSAGE_APP,
						Payload: []byte("This is a test message being sent over the Meshtastic network"),
					},
				},
			},
			maxCompressionPct: 95,
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

			// Compress using V1 (presence bits)
			var bufV1 bytes.Buffer
			err = CompressV1(tt.msg, &bufV1)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}
			compressedSizeV1 := bufV1.Len()

			// Compress using V2 (delta-encoded field numbers)
			var bufV2 bytes.Buffer
			err = CompressV2(tt.msg, &bufV2)
			if err != nil {
				t.Fatalf("CompressV2 failed: %v", err)
			}
			compressedSizeV2 := bufV2.Len()

			// Compress using V3 (hybrid encoding)
			var bufV3 bytes.Buffer
			err = CompressV3(tt.msg, &bufV3)
			if err != nil {
				t.Fatalf("CompressV3 failed: %v", err)
			}
			compressedSizeV3 := bufV3.Len()

			// Compress using V4 (enum prediction)
			var bufV4 bytes.Buffer
			err = CompressV4(tt.msg, &bufV4)
			if err != nil {
				t.Fatalf("CompressV4 failed: %v", err)
			}
			compressedSizeV4 := bufV4.Len()

			// Compress using V5 (context-aware models)
			var bufV5 bytes.Buffer
			err = CompressV5(tt.msg, &bufV5)
			if err != nil {
				t.Fatalf("CompressV5 failed: %v", err)
			}
			compressedSizeV5 := bufV5.Len()

			// Compress using V6 (bit-packed booleans)
			var bufV6 bytes.Buffer
			err = CompressV6(tt.msg, &bufV6)
			if err != nil {
				t.Fatalf("CompressV6 failed: %v", err)
			}
			compressedSizeV6 := bufV6.Len()

			// Compress using V7 (field-specific boolean models)
			var bufV7 bytes.Buffer
			err = CompressV7(tt.msg, &bufV7)
			if err != nil {
				t.Fatalf("CompressV7 failed: %v", err)
			}
			compressedSizeV7 := bufV7.Len()

			// Compress using V8 (varint byte models)
			var bufV8 bytes.Buffer
			err = CompressV8(tt.msg, &bufV8)
			if err != nil {
				t.Fatalf("CompressV8 failed: %v", err)
			}
			compressedSizeV8 := bufV8.Len()

			// Compress using V9 (order-1 string models)
			var bufV9 bytes.Buffer
			err = CompressV9(tt.msg, &bufV9)
			if err != nil {
				t.Fatalf("CompressV9 failed: %v", err)
			}
			compressedSizeV9 := bufV9.Len()

			// Compress using V10 (order-2 string models)
			var bufV10 bytes.Buffer
			err = CompressV10(tt.msg, &bufV10)
			if err != nil {
				t.Fatalf("CompressV10 failed: %v", err)
			}
			compressedSizeV10 := bufV10.Len()

			// Calculate ratios
			ratioV1 := float64(compressedSizeV1) / float64(originalSize) * 100
			ratioV2 := float64(compressedSizeV2) / float64(originalSize) * 100
			ratioV3 := float64(compressedSizeV3) / float64(originalSize) * 100
			ratioV4 := float64(compressedSizeV4) / float64(originalSize) * 100
			ratioV5 := float64(compressedSizeV5) / float64(originalSize) * 100
			ratioV6 := float64(compressedSizeV6) / float64(originalSize) * 100
			ratioV7 := float64(compressedSizeV7) / float64(originalSize) * 100
			ratioV8 := float64(compressedSizeV8) / float64(originalSize) * 100
			ratioV9 := float64(compressedSizeV9) / float64(originalSize) * 100
			ratioV10 := float64(compressedSizeV10) / float64(originalSize) * 100

			t.Logf("Original: %d bytes", originalSize)
			t.Logf("V1 (presence bits): %d bytes, Ratio: %.2f%%", compressedSizeV1, ratioV1)
			t.Logf("V2 (delta fields): %d bytes, Ratio: %.2f%%", compressedSizeV2, ratioV2)
			t.Logf("V3 (hybrid): %d bytes, Ratio: %.2f%%", compressedSizeV3, ratioV3)
			t.Logf("V4 (enum prediction): %d bytes, Ratio: %.2f%%", compressedSizeV4, ratioV4)
			t.Logf("V5 (context-aware): %d bytes, Ratio: %.2f%%", compressedSizeV5, ratioV5)
			t.Logf("V6 (bit-packed bools): %d bytes, Ratio: %.2f%%", compressedSizeV6, ratioV6)
			t.Logf("V7 (boolean models): %d bytes, Ratio: %.2f%%", compressedSizeV7, ratioV7)
			t.Logf("V8 (varint models): %d bytes, Ratio: %.2f%%", compressedSizeV8, ratioV8)
			t.Logf("V9 (order-1 strings): %d bytes, Ratio: %.2f%%", compressedSizeV9, ratioV9)
			t.Logf("V10 (order-2 strings): %d bytes, Ratio: %.2f%%", compressedSizeV10, ratioV10)

			bestSize := compressedSizeV1
			bestVersion := "V1"

			if compressedSizeV2 < bestSize {
				bestSize = compressedSizeV2
				bestVersion = "V2"
			}
			if compressedSizeV3 < bestSize {
				bestSize = compressedSizeV3
				bestVersion = "V3"
			}
			if compressedSizeV4 < bestSize {
				bestSize = compressedSizeV4
				bestVersion = "V4"
			}
			if compressedSizeV5 < bestSize {
				bestSize = compressedSizeV5
				bestVersion = "V5"
			}
			if compressedSizeV6 < bestSize {
				bestSize = compressedSizeV6
				bestVersion = "V6"
			}
			if compressedSizeV7 < bestSize {
				bestSize = compressedSizeV7
				bestVersion = "V7"
			}
			if compressedSizeV8 < bestSize {
				bestSize = compressedSizeV8
				bestVersion = "V8"
			}
			if compressedSizeV9 < bestSize {
				bestSize = compressedSizeV9
				bestVersion = "V9"
			}
			if compressedSizeV10 < bestSize {
				bestSize = compressedSizeV10
				bestVersion = "V10"
			}

			t.Logf("Best: %d bytes (%s saves %d bytes vs V1)", bestSize, bestVersion, compressedSizeV1-bestSize)

			if ratioV10 > tt.maxCompressionPct {
				t.Errorf("Compression ratio %.2f%% exceeds maximum %.2f%%", ratioV10, tt.maxCompressionPct)
			}

			// Verify V1 roundtrip
			resultV1 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV1(&bufV1, resultV1)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV1) {
				t.Error("V1 roundtrip verification failed")
			}

			// Verify V2 roundtrip
			resultV2 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV2(&bufV2, resultV2)
			if err != nil {
				t.Fatalf("DecompressV2 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV2) {
				t.Error("V2 roundtrip verification failed")
			}

			// Verify V3 roundtrip
			resultV3 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV3(&bufV3, resultV3)
			if err != nil {
				t.Fatalf("DecompressV3 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV3) {
				t.Error("V3 roundtrip verification failed")
			}

			// Verify V4 roundtrip
			resultV4 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV4(&bufV4, resultV4)
			if err != nil {
				t.Fatalf("DecompressV4 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV4) {
				t.Error("V4 roundtrip verification failed")
			}

			// Verify V5 roundtrip
			resultV5 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV5(&bufV5, resultV5)
			if err != nil {
				t.Fatalf("DecompressV5 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV5) {
				t.Error("V5 roundtrip verification failed")
			}

			// Verify V6 roundtrip
			resultV6 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV6(&bufV6, resultV6)
			if err != nil {
				t.Fatalf("DecompressV6 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV6) {
				t.Error("V6 roundtrip verification failed")
			}

			// Verify V7 roundtrip
			resultV7 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV7(&bufV7, resultV7)
			if err != nil {
				t.Fatalf("DecompressV7 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV7) {
				t.Error("V7 roundtrip verification failed")
			}

			// Verify V8 roundtrip
			resultV8 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV8(&bufV8, resultV8)
			if err != nil {
				t.Fatalf("DecompressV8 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV8) {
				t.Error("V8 roundtrip verification failed")
			}

			// Verify V9 roundtrip
			resultV9 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV9(&bufV9, resultV9)
			if err != nil {
				t.Fatalf("DecompressV9 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV9) {
				t.Error("V9 roundtrip verification failed")
			}

			// Verify V10 roundtrip
			resultV10 := tt.msg.ProtoReflect().New().Interface()
			err = DecompressV10(&bufV10, resultV10)
			if err != nil {
				t.Fatalf("DecompressV10 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV10) {
				t.Error("V10 roundtrip verification failed")
			}
		})
	}
}
