package meshtasticmodel

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
)

func TestMeshtasticV5ContextAwareness(t *testing.T) {
	tests := []struct {
		name string
		msg  proto.Message
	}{
		{
			name: "Position with typical GPS data",
			msg: &meshtastic.Position{
				LatitudeI:     proto.Int32(375317890),
				LongitudeI:    proto.Int32(-1223898570),
				Altitude:      proto.Int32(150), // Small altitude value
				Time:          1703520000,
				SatsInView:    8,   // Typical satellite count
				GpsAccuracy:   5,   // Good accuracy
				PDOP:          120, // Good DOP
				HDOP:          90,
				VDOP:          100,
				GroundSpeed:   proto.Uint32(25), // Moderate speed
				PrecisionBits: 8,
			},
		},
		{
			name: "Telemetry with typical battery and signal",
			msg: &meshtastic.Telemetry{
				Time: 1703520000,
				Variant: &meshtastic.Telemetry_DeviceMetrics{
					DeviceMetrics: &meshtastic.DeviceMetrics{
						BatteryLevel:       proto.Uint32(75), // Mid-high battery
						Voltage:            proto.Float32(4.1),
						ChannelUtilization: proto.Float32(15.5), // Low utilization
						AirUtilTx:          proto.Float32(5.2),
						UptimeSeconds:      proto.Uint32(86400),
					},
				},
			},
		},
		{
			name: "MeshPacket with typical hop count and signal",
			msg: &meshtastic.MeshPacket{
				From:     123456789,
				To:       987654321,
				Channel:  0,   // Default channel
				HopLimit: 3,   // Typical hop limit
				RxRssi:   -85, // Typical RSSI
				RxSnr:    7.5,
				Priority: meshtastic.MeshPacket_DEFAULT,
				Delayed:  meshtastic.MeshPacket_NO_DELAY,
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

			// Compress with V5
			var bufV5 bytes.Buffer
			if err := CompressV5(tt.msg, &bufV5); err != nil {
				t.Fatalf("V5 compress failed: %v", err)
			}

			ratioV1 := float64(bufV1.Len()) / float64(originalSize) * 100
			ratioV5 := float64(bufV5.Len()) / float64(originalSize) * 100
			improvement := bufV1.Len() - bufV5.Len()

			t.Logf("Original: %d bytes", originalSize)
			t.Logf("V1: %d bytes (%.2f%%)", bufV1.Len(), ratioV1)
			t.Logf("V5: %d bytes (%.2f%%)", bufV5.Len(), ratioV5)
			if improvement > 0 {
				t.Logf("V5 improvement: %d bytes (%.2f%% reduction)", improvement, float64(improvement)/float64(bufV1.Len())*100)
			}

			// Verify roundtrip
			result := tt.msg.ProtoReflect().New().Interface()
			if err := DecompressV5(&bufV5, result); err != nil {
				t.Fatalf("V5 decompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, result) {
				t.Error("V5 roundtrip verification failed")
			}
		})
	}
}
