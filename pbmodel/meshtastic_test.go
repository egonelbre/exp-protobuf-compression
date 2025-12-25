package pbmodel

import (
	"bytes"
	"testing"

	"github.com/egonelbre/protobuf-compression/pbmodel/testdata/meshtastic"
	"google.golang.org/protobuf/proto"
)

// TestMeshtasticPosition tests compression of Meshtastic Position messages
func TestMeshtasticPosition(t *testing.T) {
	msg := &meshtastic.Position{
		LatitudeI:                 proto.Int32(375317890),   // 37.5317890 degrees
		LongitudeI:                proto.Int32(-1223898570), // -122.3898570 degrees
		Altitude:                  proto.Int32(100),
		Time:                      1703520000,
		LocationSource:            meshtastic.Position_LOC_MANUAL,
		AltitudeSource:            meshtastic.Position_ALT_MANUAL,
		Timestamp:                 1703520000,
		TimestampMillisAdjust:     500,
		AltitudeHae:               proto.Int32(120),
		AltitudeGeoidalSeparation: proto.Int32(20),
		PDOP:                      150,
		HDOP:                      100,
		VDOP:                      200,
		GpsAccuracy:               5,
		GroundSpeed:               proto.Uint32(0),
		GroundTrack:               proto.Uint32(0),
		SatsInView:                8,
		PrecisionBits:             8,
	}

	testMeshtasticRoundtrip(t, "Position", msg)
}

// TestMeshtasticUser tests compression of Meshtastic User messages
func TestMeshtasticUser(t *testing.T) {
	msg := &meshtastic.User{
		Id:         "!12345678",
		LongName:   "Test User Node",
		ShortName:  "TEST",
		Macaddr:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		HwModel:    meshtastic.HardwareModel_TBEAM,
		IsLicensed: false,
		Role:       meshtastic.Config_DeviceConfig_Role(meshtastic.Config_DeviceConfig_CLIENT),
		PublicKey:  []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
	}

	testMeshtasticRoundtrip(t, "User", msg)
}

// TestMeshtasticWaypoint tests compression of Meshtastic Waypoint messages
func TestMeshtasticWaypoint(t *testing.T) {
	msg := &meshtastic.Waypoint{
		Id:          12345,
		LatitudeI:   proto.Int32(375317890),
		LongitudeI:  proto.Int32(-1223898570),
		Expire:      1703520000,
		LockedTo:    0,
		Name:        "Base Camp",
		Description: "Main camping area near the trail head",
		Icon:        0,
	}

	testMeshtasticRoundtrip(t, "Waypoint", msg)
}

// TestMeshtasticNodeInfo tests compression of Meshtastic NodeInfo messages
func TestMeshtasticNodeInfo(t *testing.T) {
	msg := &meshtastic.NodeInfo{
		Num: 123456789,
		User: &meshtastic.User{
			Id:        "!87654321",
			LongName:  "Remote Node",
			ShortName: "RMT",
			HwModel:   meshtastic.HardwareModel_TBEAM,
		},
		Position: &meshtastic.Position{
			LatitudeI:  proto.Int32(375317890),
			LongitudeI: proto.Int32(-1223898570),
			Altitude:   proto.Int32(50),
			Time:       1703520000,
		},
		Snr:       8.5,
		LastHeard: 1703520100,
		DeviceMetrics: &meshtastic.DeviceMetrics{
			BatteryLevel:       proto.Uint32(75),
			Voltage:            proto.Float32(4.1),
			ChannelUtilization: proto.Float32(15.5),
			AirUtilTx:          proto.Float32(5.2),
			UptimeSeconds:      proto.Uint32(86400),
		},
		Channel:  0,
		ViaMqtt:  false,
		HopsAway: proto.Uint32(2),
	}

	testMeshtasticRoundtrip(t, "NodeInfo", msg)
}

// TestMeshtasticTelemetry tests compression of Meshtastic telemetry messages
func TestMeshtasticTelemetry(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_DeviceMetrics{
			DeviceMetrics: &meshtastic.DeviceMetrics{
				BatteryLevel:       proto.Uint32(85),
				Voltage:            proto.Float32(4.15),
				ChannelUtilization: proto.Float32(12.3),
				AirUtilTx:          proto.Float32(4.5),
				UptimeSeconds:      proto.Uint32(172800),
			},
		},
	}

	testMeshtasticRoundtrip(t, "Telemetry", msg)
}

// TestMeshtasticEnvironmentMetrics tests compression of environment telemetry
func TestMeshtasticEnvironmentMetrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_EnvironmentMetrics{
			EnvironmentMetrics: &meshtastic.EnvironmentMetrics{
				Temperature:        proto.Float32(22.5),
				RelativeHumidity:   proto.Float32(65.0),
				BarometricPressure: proto.Float32(1013.25),
				GasResistance:      proto.Float32(50000.0),
				Voltage:            proto.Float32(3.3),
				Current:            proto.Float32(0.15),
				Iaq:                proto.Uint32(50),
			},
		},
	}

	testMeshtasticRoundtrip(t, "EnvironmentMetrics", msg)
}

// TestMeshtasticMeshPacket tests compression of full mesh packets
func TestMeshtasticMeshPacket(t *testing.T) {
	msg := &meshtastic.MeshPacket{
		From:    123456789,
		To:      987654321,
		Channel: 0,
		PayloadVariant: &meshtastic.MeshPacket_Decoded{
			Decoded: &meshtastic.Data{
				Portnum:      meshtastic.PortNum_TEXT_MESSAGE_APP,
				Payload:      []byte("Hello from the mesh network!"),
				WantResponse: false,
				Dest:         987654321,
				Source:       123456789,
				RequestId:    12345,
			},
		},
		Id:       987654,
		RxTime:   1703520000,
		RxSnr:    7.5,
		HopLimit: 3,
		WantAck:  false,
		Priority: meshtastic.MeshPacket_DEFAULT,
		RxRssi:   -95,
		Delayed:  meshtastic.MeshPacket_NO_DELAY,
	}

	testMeshtasticRoundtrip(t, "MeshPacket", msg)
}

// TestMeshtasticCompressionRatio tests compression effectiveness on Meshtastic messages
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
			maxCompressionPct: 95, // Small message with mostly fixed-width fields, limited compression possible
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

			// Compress using Meshtastic-specific compressor
			var buf bytes.Buffer
			err = MeshtasticCompress(tt.msg, &buf)
			if err != nil {
				t.Fatalf("MeshtasticCompress failed: %v", err)
			}
			compressedSize := buf.Len()

			// Calculate ratio
			ratio := float64(compressedSize) / float64(originalSize) * 100

			t.Logf("Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%",
				originalSize, compressedSize, ratio)

			if ratio > tt.maxCompressionPct {
				t.Errorf("Compression ratio %.2f%% exceeds maximum %.2f%%", ratio, tt.maxCompressionPct)
			}

			// Verify roundtrip
			result := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompress(&buf, result)
			if err != nil {
				t.Fatalf("MeshtasticDecompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, result) {
				t.Error("Roundtrip verification failed")
			}
		})
	}
}

// testMeshtasticRoundtrip is a helper function to test compression roundtrip
func testMeshtasticRoundtrip(t *testing.T, name string, msg proto.Message) {
	t.Helper()

	// Compress
	var buf bytes.Buffer
	err := Compress(msg, &buf)
	if err != nil {
		t.Fatalf("%s: Compress failed: %v", name, err)
	}

	// Get compression stats
	originalData, _ := proto.Marshal(msg)
	ratio := float64(buf.Len()) / float64(len(originalData)) * 100
	t.Logf("%s: Original %d bytes -> Compressed %d bytes (%.1f%%)",
		name, len(originalData), buf.Len(), ratio)

	// Decompress
	result := msg.ProtoReflect().New().Interface()
	err = Decompress(&buf, result)
	if err != nil {
		t.Fatalf("%s: Decompress failed: %v", name, err)
	}

	// Compare
	if !proto.Equal(msg, result) {
		t.Errorf("%s: Messages differ:\noriginal: %v\nresult:   %v", name, msg, result)
	}
}
