package meshtasticmodel

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/egonelbre/exp-protobuf-compression/meshtastic"
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

// TestMeshtasticPowerMetrics tests compression of power telemetry
func TestMeshtasticPowerMetrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_PowerMetrics{
			PowerMetrics: &meshtastic.PowerMetrics{
				Ch1Voltage: proto.Float32(12.0),
				Ch1Current: proto.Float32(2.5),
				Ch2Voltage: proto.Float32(5.0),
				Ch2Current: proto.Float32(1.2),
				Ch3Voltage: proto.Float32(3.3),
				Ch3Current: proto.Float32(0.8),
			},
		},
	}

	testMeshtasticRoundtrip(t, "PowerMetrics", msg)
}

// TestMeshtasticAirQualityMetrics tests compression of air quality telemetry
func TestMeshtasticAirQualityMetrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_AirQualityMetrics{
			AirQualityMetrics: &meshtastic.AirQualityMetrics{
				Pm10Standard:       proto.Uint32(25),
				Pm25Standard:       proto.Uint32(15),
				Pm100Standard:      proto.Uint32(50),
				Pm10Environmental:  proto.Uint32(28),
				Pm25Environmental:  proto.Uint32(18),
				Pm100Environmental: proto.Uint32(55),
				Particles_03Um:     proto.Uint32(1500),
				Particles_05Um:     proto.Uint32(850),
				Particles_10Um:     proto.Uint32(450),
				Particles_25Um:     proto.Uint32(120),
				Particles_50Um:     proto.Uint32(30),
				Particles_100Um:    proto.Uint32(5),
			},
		},
	}

	testMeshtasticRoundtrip(t, "AirQualityMetrics", msg)
}

// TestMeshtasticLocalStats tests compression of mesh statistics
func TestMeshtasticLocalStats(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_LocalStats{
			LocalStats: &meshtastic.LocalStats{
				UptimeSeconds:      259200,
				NumPacketsTx:       1542,
				NumPacketsRx:       3871,
				NumPacketsRxBad:    23,
				NumOnlineNodes:     12,
				NumTotalNodes:      45,
				NumRxDupe:          156,
				NumTxRelay:         234,
				NumTxRelayCanceled: 12,
			},
		},
	}

	testMeshtasticRoundtrip(t, "LocalStats", msg)
}

// TestMeshtasticHealthMetrics tests compression of health telemetry
func TestMeshtasticHealthMetrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_HealthMetrics{
			HealthMetrics: &meshtastic.HealthMetrics{
				HeartBpm:    proto.Uint32(72),
				SpO2:        proto.Uint32(98),
				Temperature: proto.Float32(36.8),
			},
		},
	}

	testMeshtasticRoundtrip(t, "HealthMetrics", msg)
}

// TestMeshtasticRouting tests compression of routing messages
func TestMeshtasticRouting(t *testing.T) {
	msg := &meshtastic.Routing{
		Variant: &meshtastic.Routing_RouteReply{
			RouteReply: &meshtastic.RouteDiscovery{
				Route:       []uint32{123456789, 234567890, 345678901},
				SnrTowards:  []int32{28, 24, 20}, // SNR in dB * 4
				RouteBack:   []uint32{345678901, 234567890, 123456789},
				SnrBack:     []int32{22, 26, 30},
			},
		},
	}

	testMeshtasticRoundtrip(t, "Routing", msg)
}

// TestMeshtasticChannel tests compression of channel configuration
func TestMeshtasticChannel(t *testing.T) {
	msg := &meshtastic.Channel{
		Index: 1,
		Settings: &meshtastic.ChannelSettings{
			Psk:             []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			Name:            "LongFast",
			Id:              12345,
			UplinkEnabled:   false,
			DownlinkEnabled: false,
		},
		Role: meshtastic.Channel_PRIMARY,
	}

	testMeshtasticRoundtrip(t, "Channel", msg)
}

// TestMeshtasticEnvironmentMetricsExtended tests compression with extended environmental data
func TestMeshtasticEnvironmentMetricsExtended(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_EnvironmentMetrics{
			EnvironmentMetrics: &meshtastic.EnvironmentMetrics{
				Temperature:        proto.Float32(18.5),
				RelativeHumidity:   proto.Float32(82.0),
				BarometricPressure: proto.Float32(1008.5),
				GasResistance:      proto.Float32(125000.0),
				Iaq:                proto.Uint32(125),
				Distance:           proto.Float32(2500.0),
				Lux:                proto.Float32(450.0),
				WhiteLux:           proto.Float32(380.0),
				IrLux:              proto.Float32(70.0),
				UvLux:              proto.Float32(15.0),
				WindDirection:      proto.Uint32(270), // West
				WindSpeed:          proto.Float32(8.5),
				WindGust:           proto.Float32(12.3),
				WindLull:           proto.Float32(5.2),
			},
		},
	}

	testMeshtasticRoundtrip(t, "EnvironmentMetricsExtended", msg)
}

// TestMeshtasticWeatherMetrics tests compression with weather-specific data
func TestMeshtasticWeatherMetrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_EnvironmentMetrics{
			EnvironmentMetrics: &meshtastic.EnvironmentMetrics{
				Temperature:        proto.Float32(12.8),
				RelativeHumidity:   proto.Float32(91.0),
				BarometricPressure: proto.Float32(995.2),
				WindDirection:      proto.Uint32(135), // SE
				WindSpeed:          proto.Float32(15.2),
				WindGust:           proto.Float32(22.8),
				Rainfall_1H:        proto.Float32(5.5),
				Rainfall_24H:       proto.Float32(18.3),
			},
		},
	}

	testMeshtasticRoundtrip(t, "WeatherMetrics", msg)
}

// TestMeshtasticSoilMetrics tests compression with soil sensor data
func TestMeshtasticSoilMetrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_EnvironmentMetrics{
			EnvironmentMetrics: &meshtastic.EnvironmentMetrics{
				SoilMoisture:    proto.Uint32(65),
				SoilTemperature: proto.Float32(18.5),
			},
		},
	}

	testMeshtasticRoundtrip(t, "SoilMetrics", msg)
}

// TestMeshtasticCO2Metrics tests compression with CO2 sensor data
func TestMeshtasticCO2Metrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_AirQualityMetrics{
			AirQualityMetrics: &meshtastic.AirQualityMetrics{
				Co2:              proto.Uint32(850),
				Co2Temperature:   proto.Float32(23.5),
				Co2Humidity:      proto.Float32(55.0),
				FormFormaldehyde: proto.Float32(0.08),
				FormTemperature:  proto.Float32(23.2),
				FormHumidity:     proto.Float32(54.5),
			},
		},
	}

	testMeshtasticRoundtrip(t, "CO2Metrics", msg)
}

// TestMeshtasticHostMetrics tests compression with host system data
func TestMeshtasticHostMetrics(t *testing.T) {
	msg := &meshtastic.Telemetry{
		Time: 1703520000,
		Variant: &meshtastic.Telemetry_HostMetrics{
			HostMetrics: &meshtastic.HostMetrics{
				UptimeSeconds:  5184000,      // 60 days
				FreememBytes:   2147483648,   // 2GB
				Diskfree1Bytes: 53687091200,  // 50GB
				Load1:          125,          // 1.25
				Load5:          98,           // 0.98
				Load15:         75,           // 0.75
			},
		},
	}

	testMeshtasticRoundtrip(t, "HostMetrics", msg)
}

// TestMeshtasticPositionExtended tests compression with all position fields
func TestMeshtasticPositionExtended(t *testing.T) {
	msg := &meshtastic.Position{
		LatitudeI:                 proto.Int32(402345678),  // 40.2345678 degrees
		LongitudeI:                proto.Int32(-739876543), // -73.9876543 degrees
		Altitude:                  proto.Int32(325),
		Time:                      1703520000,
		LocationSource:            meshtastic.Position_LOC_INTERNAL,
		AltitudeSource:            meshtastic.Position_ALT_INTERNAL,
		Timestamp:                 1703520000,
		TimestampMillisAdjust:     -150,
		AltitudeHae:               proto.Int32(348),
		AltitudeGeoidalSeparation: proto.Int32(-23),
		PDOP:                      185,
		HDOP:                      95,
		VDOP:                      145,
		GpsAccuracy:               8,
		GroundSpeed:               proto.Uint32(1250), // 12.5 m/s
		GroundTrack:               proto.Uint32(12500), // 125.00 degrees
		FixQuality:                3,
		FixType:                   3,
		SatsInView:                12,
		SensorId:                  1,
		NextUpdate:                30,
		SeqNumber:                 42,
		PrecisionBits:             16,
	}

	testMeshtasticRoundtrip(t, "PositionExtended", msg)
}

// TestMeshtasticDataPayloads tests different payload types
func TestMeshtasticDataPayloads(t *testing.T) {
	tests := []struct {
		name    string
		portnum meshtastic.PortNum
		payload []byte
	}{
		{
			name:    "TextMessage",
			portnum: meshtastic.PortNum_TEXT_MESSAGE_APP,
			payload: []byte("The quick brown fox jumps over the lazy dog"),
		},
		{
			name:    "BinaryPayload",
			portnum: meshtastic.PortNum_PRIVATE_APP,
			payload: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC},
		},
		{
			name:    "EmptyPayload",
			portnum: meshtastic.PortNum_NODEINFO_APP,
			payload: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &meshtastic.Data{
				Portnum:      tt.portnum,
				Payload:      tt.payload,
				WantResponse: false,
				Dest:         987654321,
				Source:       123456789,
			}
			testMeshtasticRoundtrip(t, "Data_"+tt.name, msg)
		})
	}
}

// TestMeshtasticMeshPacketPriorities tests different packet priorities
func TestMeshtasticMeshPacketPriorities(t *testing.T) {
	priorities := []meshtastic.MeshPacket_Priority{
		meshtastic.MeshPacket_MIN,
		meshtastic.MeshPacket_BACKGROUND,
		meshtastic.MeshPacket_DEFAULT,
		meshtastic.MeshPacket_RELIABLE,
		meshtastic.MeshPacket_ACK,
		meshtastic.MeshPacket_MAX,
	}

	for _, priority := range priorities {
		t.Run(priority.String(), func(t *testing.T) {
			msg := &meshtastic.MeshPacket{
				From:    123456789,
				To:      987654321,
				Channel: 0,
				PayloadVariant: &meshtastic.MeshPacket_Decoded{
					Decoded: &meshtastic.Data{
						Portnum: meshtastic.PortNum_TEXT_MESSAGE_APP,
						Payload: []byte("Priority test"),
					},
				},
				Id:       12345,
				Priority: priority,
				HopLimit: 3,
				HopStart: 3,
			}
			testMeshtasticRoundtrip(t, "MeshPacket_Priority", msg)
		})
	}
}

// testMeshtasticRoundtrip is a helper function to test compression roundtrip
func testMeshtasticRoundtrip(t *testing.T, name string, msg proto.Message) {
	t.Helper()

	originalData, _ := proto.Marshal(msg)

	// Test all versions
	for _, version := range Versions {
		t.Run(version.Name, func(t *testing.T) {
			// Compress
			var buf bytes.Buffer
			err := version.Compress(msg, &buf)
			if err != nil {
				t.Fatalf("%s/%s: Compress failed: %v", name, version.Name, err)
			}

			// Get compression stats
			ratio := float64(buf.Len()) / float64(len(originalData)) * 100
			t.Logf("%s/%s: Original %d bytes -> Compressed %d bytes (%.1f%%)",
				name, version.Name, len(originalData), buf.Len(), ratio)

			// Decompress
			result := msg.ProtoReflect().New().Interface()
			err = version.Decompress(&buf, result)
			if err != nil {
				t.Fatalf("%s/%s: Decompress failed: %v", name, version.Name, err)
			}

			// Compare
			if !proto.Equal(msg, result) {
				t.Errorf("%s/%s: Messages differ:\noriginal: %v\nresult:   %v", name, version.Name, msg, result)
			}
		})
	}
}
