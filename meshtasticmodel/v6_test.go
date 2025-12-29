package meshtasticmodel

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
)

func TestMeshtasticV6BitPacking(t *testing.T) {
	tests := []struct {
		name string
		msg  proto.Message
	}{
		{
			name: "MeshPacket with multiple booleans",
			msg: &meshtastic.MeshPacket{
				From:         123456789,
				To:           987654321,
				Channel:      0,
				WantAck:      true,
				ViaMqtt:      false,
				PkiEncrypted: false,
				HopLimit:     3,
				Priority:     meshtastic.MeshPacket_DEFAULT,
			},
		},
		{
			name: "User with boolean flags",
			msg: &meshtastic.User{
				Id:         "!12345678",
				LongName:   "Test Node",
				ShortName:  "TEST",
				IsLicensed: false,
				HwModel:    meshtastic.HardwareModel_TBEAM,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalData, err := proto.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}
			originalSize := len(originalData)

			// Compress with V1
			var bufV1 bytes.Buffer
			if err := CompressV1(tt.msg, &bufV1); err != nil {
				t.Fatalf("V1 compress failed: %v", err)
			}

			// Compress with V6
			var bufV6 bytes.Buffer
			if err := CompressV6(tt.msg, &bufV6); err != nil {
				t.Fatalf("V6 compress failed: %v", err)
			}

			ratioV1 := float64(bufV1.Len()) / float64(originalSize) * 100
			ratioV6 := float64(bufV6.Len()) / float64(originalSize) * 100
			improvement := bufV1.Len() - bufV6.Len()

			t.Logf("Original: %d bytes", originalSize)
			t.Logf("V1: %d bytes (%.2f%%)", bufV1.Len(), ratioV1)
			t.Logf("V6: %d bytes (%.2f%%)", bufV6.Len(), ratioV6)
			if improvement > 0 {
				t.Logf("V6 improvement: %d bytes (%.2f%% reduction)", improvement, float64(improvement)/float64(bufV1.Len())*100)
			} else if improvement < 0 {
				t.Logf("V6 overhead: %d bytes", -improvement)
			}

			// Verify roundtrip
			result := tt.msg.ProtoReflect().New().Interface()
			if err := DecompressV6(&bufV6, result); err != nil {
				t.Fatalf("V6 decompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, result) {
				t.Error("V6 roundtrip verification failed")
			}
		})
	}
}
