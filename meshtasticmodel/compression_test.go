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

			// Compress with all versions
			type result struct {
				version        Version
				compressedSize int
				buffer         bytes.Buffer
			}
			results := make([]result, 0, len(Versions))

			for _, version := range Versions {
				var buf bytes.Buffer
				err := version.Compress(tt.msg, &buf)
				if err != nil {
					t.Fatalf("%s Compress failed: %v", version.Name, err)
				}
				results = append(results, result{
					version:        version,
					compressedSize: buf.Len(),
					buffer:         buf,
				})
			}

			// Log results
			t.Logf("Original: %d bytes", originalSize)
			for _, r := range results {
				ratio := float64(r.compressedSize) / float64(originalSize) * 100
				t.Logf("%s (%s): %d bytes, Ratio: %.2f%%", r.version.Name, r.version.Short, r.compressedSize, ratio)
			}

			// Find best compression
			bestIdx := 0
			for i, r := range results {
				if r.compressedSize < results[bestIdx].compressedSize {
					bestIdx = i
				}
			}
			bestResult := results[bestIdx]

			// Find V1 for comparison (skip pbmodel which is index 0)
			v1Result := results[1] // V1 is second in the list after pbmodel
			t.Logf("Best: %d bytes (%s saves %d bytes vs %s)",
				bestResult.compressedSize,
				bestResult.version.Name,
				v1Result.compressedSize-bestResult.compressedSize,
				v1Result.version.Name)

			// Check last version (V10) against max compression ratio
			lastResult := results[len(results)-1]
			lastRatio := float64(lastResult.compressedSize) / float64(originalSize) * 100
			if lastRatio > tt.maxCompressionPct {
				t.Errorf("Compression ratio %.2f%% exceeds maximum %.2f%%", lastRatio, tt.maxCompressionPct)
			}

			// Verify all roundtrips
			for _, r := range results {
				resultMsg := tt.msg.ProtoReflect().New().Interface()
				bufCopy := bytes.NewBuffer(r.buffer.Bytes())
				err := r.version.Decompress(bufCopy, resultMsg)
				if err != nil {
					t.Fatalf("%s Decompress failed: %v", r.version.Name, err)
				}

				if !proto.Equal(tt.msg, resultMsg) {
					t.Errorf("%s roundtrip verification failed", r.version.Name)
				}
			}
		})
	}
}
