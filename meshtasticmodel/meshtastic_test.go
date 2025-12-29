package meshtasticmodel

import (
	"bytes"
	"testing"

	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
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
			err = MeshtasticCompress(tt.msg, &bufV1)
			if err != nil {
				t.Fatalf("MeshtasticCompress failed: %v", err)
			}
			compressedSizeV1 := bufV1.Len()

			// Compress using V2 (delta-encoded field numbers)
			var bufV2 bytes.Buffer
			err = MeshtasticCompressV2(tt.msg, &bufV2)
			if err != nil {
				t.Fatalf("MeshtasticCompressV2 failed: %v", err)
			}
			compressedSizeV2 := bufV2.Len()

			// Compress using V3 (hybrid encoding)
			var bufV3 bytes.Buffer
			err = MeshtasticCompressV3(tt.msg, &bufV3)
			if err != nil {
				t.Fatalf("MeshtasticCompressV3 failed: %v", err)
			}
			compressedSizeV3 := bufV3.Len()

			// Compress using V4 (enum prediction)
			var bufV4 bytes.Buffer
			err = MeshtasticCompressV4(tt.msg, &bufV4)
			if err != nil {
				t.Fatalf("MeshtasticCompressV4 failed: %v", err)
			}
			compressedSizeV4 := bufV4.Len()

			// Compress using V5 (context-aware models)
			var bufV5 bytes.Buffer
			err = MeshtasticCompressV5(tt.msg, &bufV5)
			if err != nil {
				t.Fatalf("MeshtasticCompressV5 failed: %v", err)
			}
			compressedSizeV5 := bufV5.Len()

			// Compress using V6 (bit-packed booleans)
			var bufV6 bytes.Buffer
			err = MeshtasticCompressV6(tt.msg, &bufV6)
			if err != nil {
				t.Fatalf("MeshtasticCompressV6 failed: %v", err)
			}
			compressedSizeV6 := bufV6.Len()

			// Compress using V7 (field-specific boolean models)
			var bufV7 bytes.Buffer
			err = MeshtasticCompressV7(tt.msg, &bufV7)
			if err != nil {
				t.Fatalf("MeshtasticCompressV7 failed: %v", err)
			}
			compressedSizeV7 := bufV7.Len()

			// Compress using V8 (varint byte models)
			var bufV8 bytes.Buffer
			err = MeshtasticCompressV8(tt.msg, &bufV8)
			if err != nil {
				t.Fatalf("MeshtasticCompressV8 failed: %v", err)
			}
			compressedSizeV8 := bufV8.Len()

			// Calculate ratios
			ratioV1 := float64(compressedSizeV1) / float64(originalSize) * 100
			ratioV2 := float64(compressedSizeV2) / float64(originalSize) * 100
			ratioV3 := float64(compressedSizeV3) / float64(originalSize) * 100
			ratioV4 := float64(compressedSizeV4) / float64(originalSize) * 100
			ratioV5 := float64(compressedSizeV5) / float64(originalSize) * 100
			ratioV6 := float64(compressedSizeV6) / float64(originalSize) * 100
			ratioV7 := float64(compressedSizeV7) / float64(originalSize) * 100
			ratioV8 := float64(compressedSizeV8) / float64(originalSize) * 100

			t.Logf("Original: %d bytes", originalSize)
			t.Logf("V1 (presence bits): %d bytes, Ratio: %.2f%%", compressedSizeV1, ratioV1)
			t.Logf("V2 (delta fields): %d bytes, Ratio: %.2f%%", compressedSizeV2, ratioV2)
			t.Logf("V3 (hybrid): %d bytes, Ratio: %.2f%%", compressedSizeV3, ratioV3)
			t.Logf("V4 (enum prediction): %d bytes, Ratio: %.2f%%", compressedSizeV4, ratioV4)
			t.Logf("V5 (context-aware): %d bytes, Ratio: %.2f%%", compressedSizeV5, ratioV5)
			t.Logf("V6 (bit-packed bools): %d bytes, Ratio: %.2f%%", compressedSizeV6, ratioV6)
			t.Logf("V7 (boolean models): %d bytes, Ratio: %.2f%%", compressedSizeV7, ratioV7)
			t.Logf("V8 (varint models): %d bytes, Ratio: %.2f%%", compressedSizeV8, ratioV8)

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

			t.Logf("Best: %d bytes (%s saves %d bytes vs V1)", bestSize, bestVersion, compressedSizeV1-bestSize)

			if ratioV8 > tt.maxCompressionPct {
				t.Errorf("Compression ratio %.2f%% exceeds maximum %.2f%%", ratioV8, tt.maxCompressionPct)
			}

			// Verify V1 roundtrip
			resultV1 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompress(&bufV1, resultV1)
			if err != nil {
				t.Fatalf("MeshtasticDecompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV1) {
				t.Error("V1 roundtrip verification failed")
			}

			// Verify V2 roundtrip
			resultV2 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompressV2(&bufV2, resultV2)
			if err != nil {
				t.Fatalf("MeshtasticDecompressV2 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV2) {
				t.Error("V2 roundtrip verification failed")
			}

			// Verify V3 roundtrip
			resultV3 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompressV3(&bufV3, resultV3)
			if err != nil {
				t.Fatalf("MeshtasticDecompressV3 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV3) {
				t.Error("V3 roundtrip verification failed")
			}

			// Verify V4 roundtrip
			resultV4 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompressV4(&bufV4, resultV4)
			if err != nil {
				t.Fatalf("MeshtasticDecompressV4 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV4) {
				t.Error("V4 roundtrip verification failed")
			}

			// Verify V5 roundtrip
			resultV5 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompressV5(&bufV5, resultV5)
			if err != nil {
				t.Fatalf("MeshtasticDecompressV5 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV5) {
				t.Error("V5 roundtrip verification failed")
			}

			// Verify V6 roundtrip
			resultV6 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompressV6(&bufV6, resultV6)
			if err != nil {
				t.Fatalf("MeshtasticDecompressV6 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV6) {
				t.Error("V6 roundtrip verification failed")
			}

			// Verify V7 roundtrip
			resultV7 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompressV7(&bufV7, resultV7)
			if err != nil {
				t.Fatalf("MeshtasticDecompressV7 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV7) {
				t.Error("V7 roundtrip verification failed")
			}

			// Verify V8 roundtrip
			resultV8 := tt.msg.ProtoReflect().New().Interface()
			err = MeshtasticDecompressV8(&bufV8, resultV8)
			if err != nil {
				t.Fatalf("MeshtasticDecompressV8 failed: %v", err)
			}

			if !proto.Equal(tt.msg, resultV8) {
				t.Error("V8 roundtrip verification failed")
			}
		})
	}
}

// TestMeshtasticV6BitPacking tests V6 bit packing for boolean clusters
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
			if err := MeshtasticCompress(tt.msg, &bufV1); err != nil {
				t.Fatalf("V1 compress failed: %v", err)
			}

			// Compress with V6
			var bufV6 bytes.Buffer
			if err := MeshtasticCompressV6(tt.msg, &bufV6); err != nil {
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
			if err := MeshtasticDecompressV6(&bufV6, result); err != nil {
				t.Fatalf("V6 decompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, result) {
				t.Error("V6 roundtrip verification failed")
			}
		})
	}
}

// TestMeshtasticV5ContextAwareness tests V5 context-aware models with varied data
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
			if err := MeshtasticCompress(tt.msg, &bufV1); err != nil {
				t.Fatalf("V1 compress failed: %v", err)
			}

			// Compress with V5
			var bufV5 bytes.Buffer
			if err := MeshtasticCompressV5(tt.msg, &bufV5); err != nil {
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
			if err := MeshtasticDecompressV5(&bufV5, result); err != nil {
				t.Fatalf("V5 decompress failed: %v", err)
			}

			if !proto.Equal(tt.msg, result) {
				t.Error("V5 roundtrip verification failed")
			}
		})
	}
}

// testMeshtasticRoundtrip is a helper function to test compression roundtrip
func testMeshtasticRoundtrip(t *testing.T, name string, msg proto.Message) {
	t.Helper()

	// Compress
	var buf bytes.Buffer
	err := pbmodel.Compress(msg, &buf)
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
	err = pbmodel.Decompress(&buf, result)
	if err != nil {
		t.Fatalf("%s: Decompress failed: %v", name, err)
	}

	// Compare
	if !proto.Equal(msg, result) {
		t.Errorf("%s: Messages differ:\noriginal: %v\nresult:   %v", name, msg, result)
	}
}
