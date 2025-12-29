package meshtasticmodel

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/egonelbre/exp-protobuf-compression/arithcode"
)

// ContextualModelBuilder creates highly specialized models based on
// message type and field context within Meshtastic protocol messages.
type ContextualModelBuilder struct {
	*ModelBuilderV1

	// Context tracking
	messageType     string // Current message type (Position, User, etc.)
	contextModels   map[string]arithcode.Model
	enumPredictions map[string]protoreflect.EnumNumber
	booleanModels   map[string]arithcode.Model // Field-specific boolean models

	// Varint byte models
	varintFirstByteModel arithcode.Model // Model for first byte of varint
	varintContByteModel  arithcode.Model // Model for continuation bytes
}

// NewContextualModelBuilder creates a context-aware model builder.
func NewContextualModelBuilder() *ContextualModelBuilder {
	return &ContextualModelBuilder{
		ModelBuilderV1:       NewModelBuilderV1(),
		contextModels:        make(map[string]arithcode.Model),
		enumPredictions:      getCommonEnumValues(),
		booleanModels:        make(map[string]arithcode.Model),
		varintFirstByteModel: createVarintFirstByteModel(),
		varintContByteModel:  createVarintContinuationByteModel(),
	}
}

// GetContextualFieldModel returns a model optimized for the specific field context.
func (mcb *ContextualModelBuilder) GetContextualFieldModel(fieldPath string, fd protoreflect.FieldDescriptor) arithcode.Model {
	// Build context key
	contextKey := mcb.messageType + ":" + fieldPath

	// Check cache
	if model, ok := mcb.contextModels[contextKey]; ok {
		return model
	}

	// Create context-specific model
	model := mcb.createContextSpecificModel(fieldPath, fd)
	if model != nil {
		mcb.contextModels[contextKey] = model
		return model
	}

	// Fall back to standard field model
	return mcb.GetFieldModel(fieldPath, fd)
}

// createContextSpecificModel creates specialized models for known Meshtastic field patterns.
func (mcb *ContextualModelBuilder) createContextSpecificModel(fieldPath string, fd protoreflect.FieldDescriptor) arithcode.Model {
	fieldName := string(fd.Name())

	// Coordinate models (latitude_i, longitude_i)
	if fieldName == "latitude_i" || fieldName == "longitude_i" {
		return createCoordinateModel()
	}

	// Altitude models (typically -500 to 9000 meters)
	if fieldName == "altitude" || fieldName == "altitude_hae" {
		return createAltitudeModel()
	}

	// Node ID models (large 32-bit integers)
	if fieldName == "from" || fieldName == "to" || fieldName == "num" || fieldName == "dest" || fieldName == "source" {
		return createNodeIDModel()
	}

	// Battery level (0-100%)
	if fieldName == "battery_level" {
		return createBatteryLevelModel()
	}

	// Signal quality (RSSI: -120 to -30 dBm)
	if fieldName == "rx_rssi" {
		return createRSSIModel()
	}

	// Signal quality (SNR: -20 to +20 dB)
	if fieldName == "rx_snr" || fieldName == "snr" {
		return createSNRModel()
	}

	// Voltage (2.0 to 5.0V)
	if fieldName == "voltage" {
		return createVoltageModel()
	}

	// Channel utilization (0-100%)
	if fieldName == "channel_utilization" || fieldName == "air_util_tx" {
		return createUtilizationModel()
	}

	// Hop limit (typically 0-7)
	if fieldName == "hop_limit" || fieldName == "hops_away" {
		return createHopCountModel()
	}

	// Channel number (typically 0-7)
	if fieldName == "channel" {
		return createChannelNumberModel()
	}

	// Satellite count (0-20 typically)
	if fieldName == "sats_in_view" {
		return createSatelliteCountModel()
	}

	// Precision/accuracy values (typically small positive integers)
	if fieldName == "precision_bits" || fieldName == "gps_accuracy" {
		return createPrecisionModel()
	}

	// DOP values (10-1000, representing 1.0-100.0)
	if fieldName == "pdop" || fieldName == "hdop" || fieldName == "vdop" {
		return createDOPModel()
	}

	// Speed values (0-200 km/h typically)
	if fieldName == "ground_speed" {
		return createSpeedModel()
	}

	// Request ID (small sequential numbers)
	if fieldName == "request_id" {
		return createRequestIDModel()
	}

	// Packet ID (larger numbers but sequential)
	if fieldName == "id" && mcb.messageType == "MeshPacket" {
		return createPacketIDModel()
	}

	// Temperature (-40 to 85°C typical sensor range)
	if fieldName == "temperature" {
		return createTemperatureModel()
	}

	// Humidity (0-100%)
	if fieldName == "relative_humidity" {
		return createHumidityModel()
	}

	// Barometric pressure (300-1100 hPa)
	if fieldName == "barometric_pressure" {
		return createPressureModel()
	}

	// IAQ (Indoor Air Quality, 0-500)
	if fieldName == "iaq" {
		return createIAQModel()
	}

	return nil
}

// createCoordinateModel creates a model for latitude/longitude values.
// Coordinates are stored as int32 with 1e-7 degree precision.
// Typical range: -1800000000 to 1800000000 (±180°)
func createCoordinateModel() arithcode.Model {
	// Favor mid-range bytes for coordinate values
	freqs := make([]uint64, 256)
	for i := 0; i < 256; i++ {
		// Coordinates have fairly uniform byte distribution
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createAltitudeModel creates a model for altitude values (-500 to 9000m typical).
func createAltitudeModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Favor smaller varint bytes (most altitudes fit in 2 bytes when varint encoded)
	for i := 0; i < 128; i++ {
		freqs[i] = 80 - uint64(i/4)
		if freqs[i] < 10 {
			freqs[i] = 10
		}
	}
	for i := 128; i < 256; i++ {
		freqs[i] = 15
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createNodeIDModel creates a model for node IDs (typically large 32-bit values).
func createNodeIDModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Node IDs use full 32-bit range, relatively uniform
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createBatteryLevelModel creates a model for battery percentage (0-100).
func createBatteryLevelModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Battery levels 0-100, favor higher values (most devices well charged)
	// First varint byte will be 0-100
	for i := 0; i <= 100; i++ {
		// Favor 50-100% range
		if i >= 50 {
			freqs[i] = 100
		} else if i >= 20 {
			freqs[i] = 50
		} else {
			freqs[i] = 20
		}
	}
	// Continuation bytes unlikely
	for i := 101; i < 128; i++ {
		freqs[i] = 1
	}
	for i := 128; i < 256; i++ {
		freqs[i] = 1
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createRSSIModel creates a model for RSSI values (-120 to -30 dBm).
// Stored as sint32 using zigzag encoding.
func createRSSIModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// RSSI typically -120 to -30, zigzag encoded
	// Zigzag: -1→1, -2→3, -30→59, -95→189
	// After varint, first byte has continuation bit
	for i := 0; i < 128; i++ {
		// Small negative values are common (-30 to -50)
		freqs[i] = 50
	}
	for i := 128; i < 240; i++ {
		// Larger negative values (-51 to -95)
		freqs[i] = 80
	}
	for i := 240; i < 256; i++ {
		// Very weak signals
		freqs[i] = 30
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createSNRModel creates a model for SNR values (-20 to +20 dB).
// Stored as float32, but we model the byte distribution.
func createSNRModel() arithcode.Model {
	// SNR as float uses 4 bytes, uniform distribution
	freqs := make([]uint64, 256)
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createVoltageModel creates a model for voltage values (2.0-5.0V).
func createVoltageModel() arithcode.Model {
	// Voltage as float32, 4 bytes
	freqs := make([]uint64, 256)
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createUtilizationModel creates a model for channel utilization (0-100%).
func createUtilizationModel() arithcode.Model {
	// Utilization as float32, but typically low values (0-30%)
	freqs := make([]uint64, 256)
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createHopCountModel creates a model for hop counts (0-7 typically).
func createHopCountModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Hops are small (0-7), heavily favor small values
	for i := 0; i <= 7; i++ {
		freqs[i] = 100 - uint64(i*10)
	}
	for i := 8; i < 128; i++ {
		freqs[i] = 5
	}
	for i := 128; i < 256; i++ {
		freqs[i] = 1
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createChannelNumberModel creates a model for channel numbers (0-7).
func createChannelNumberModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Channel 0 is most common (default)
	freqs[0] = 200
	for i := 1; i <= 7; i++ {
		freqs[i] = 50
	}
	for i := 8; i < 256; i++ {
		freqs[i] = 1
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createSatelliteCountModel creates a model for satellite counts (0-20).
func createSatelliteCountModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Typical GPS sees 4-12 satellites
	for i := 0; i <= 20; i++ {
		if i >= 4 && i <= 12 {
			freqs[i] = 100
		} else {
			freqs[i] = 30
		}
	}
	for i := 21; i < 256; i++ {
		freqs[i] = 1
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createPrecisionModel creates a model for precision/accuracy values.
func createPrecisionModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Small positive integers, typically 1-20
	for i := 0; i <= 20; i++ {
		freqs[i] = 80 - uint64(i*2)
		if freqs[i] < 10 {
			freqs[i] = 10
		}
	}
	for i := 21; i < 256; i++ {
		freqs[i] = 5
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createDOPModel creates a model for DOP values (10-1000 representing 1.0-100.0).
func createDOPModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// DOP values typically 10-300 (1.0-30.0)
	// Favor smaller values (good signal)
	for i := 0; i < 128; i++ {
		freqs[i] = 70 - uint64(i/3)
		if freqs[i] < 10 {
			freqs[i] = 10
		}
	}
	for i := 128; i < 256; i++ {
		freqs[i] = 20
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createSpeedModel creates a model for ground speed (0-200 km/h).
func createSpeedModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Most nodes are stationary or slow moving
	for i := 0; i <= 50; i++ {
		freqs[i] = 100 - uint64(i)
		if freqs[i] < 10 {
			freqs[i] = 10
		}
	}
	for i := 51; i < 256; i++ {
		freqs[i] = 5
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createRequestIDModel creates a model for request IDs (small sequential).
func createRequestIDModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Small sequential values
	for i := 0; i < 128; i++ {
		freqs[i] = 80 - uint64(i/2)
		if freqs[i] < 10 {
			freqs[i] = 10
		}
	}
	for i := 128; i < 256; i++ {
		freqs[i] = 15
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createPacketIDModel creates a model for packet IDs.
func createPacketIDModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Packet IDs are larger but somewhat sequential
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createTemperatureModel creates a model for temperature (-40 to 85°C).
// Stored as float32.
func createTemperatureModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Temperature range, float32 representation
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createHumidityModel creates a model for relative humidity (0-100%).
func createHumidityModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Humidity as float32
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createPressureModel creates a model for barometric pressure (300-1100 hPa).
func createPressureModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// Pressure as float32
	for i := 0; i < 256; i++ {
		freqs[i] = 40
	}
	return arithcode.NewFrequencyTable(freqs)
}

// createIAQModel creates a model for Indoor Air Quality index (0-500).
func createIAQModel() arithcode.Model {
	freqs := make([]uint64, 256)
	// IAQ typically 0-300, favor good air quality (0-150)
	for i := 0; i <= 150; i++ {
		freqs[i] = 80 - uint64(i/3)
		if freqs[i] < 20 {
			freqs[i] = 20
		}
	}
	for i := 151; i < 256; i++ {
		freqs[i] = 15
	}
	return arithcode.NewFrequencyTable(freqs)
}

// SetMessageType sets the current message type context for better model selection.
func (mcb *ContextualModelBuilder) SetMessageType(msgType string) {
	mcb.messageType = msgType
}

// GetBooleanModel returns a field-specific boolean model optimized for the given field.
// Different boolean fields have different probability distributions - some are almost
// always false, some are often true, etc.
func (mcb *ContextualModelBuilder) GetBooleanModel(fieldName string) arithcode.Model {
	// Check cache
	if model, ok := mcb.booleanModels[fieldName]; ok {
		return model
	}

	// Create field-specific boolean model
	model := createBooleanModel(fieldName)
	mcb.booleanModels[fieldName] = model
	return model
}

// createBooleanModel creates a probability model for a specific boolean field.
// The frequencies are [false, true] where higher values mean higher probability.
func createBooleanModel(fieldName string) arithcode.Model {
	switch fieldName {
	// Fields that are almost always false (95% false, 5% true)
	case "want_ack", "via_mqtt", "pki_encrypted", "want_response":
		return arithcode.NewFrequencyTable([]uint64{950, 50})

	// Fields that are very rarely true (90% false, 10% true)
	case "is_licensed", "is_unmessagable", "is_favorite", "is_ignored",
		"is_key_manually_verified", "retained", "rebooted", "disconnect":
		return arithcode.NewFrequencyTable([]uint64{900, 100})

	// Configuration flags - usually disabled (80% false, 20% true)
	case "enabled", "encryption_enabled", "json_enabled", "tls_enabled",
		"proxy_to_client_enabled", "map_reporting_enabled", "should_report_location",
		"allow_undefined_pin_access", "transmit_over_lora", "send_bell",
		"use_pullup", "codec2_enabled", "echo", "override_console_serial_port",
		"active", "alert_message", "alert_message_vibra", "alert_message_buzzer",
		"alert_bell", "alert_bell_vibra", "alert_bell_buzzer", "use_pwm",
		"use_i2s_as_buzzer", "heartbeat", "is_server", "save", "clear_on_reboot",
		"environment_measurement_enabled", "environment_screen_enabled",
		"environment_display_fahrenheit", "air_quality_enabled",
		"power_measurement_enabled", "power_screen_enabled",
		"health_measurement_enabled", "health_screen_enabled",
		"device_telemetry_enabled", "rotary1_enabled", "updown1_enabled",
		"led_state", "screen_lock", "settings_lock", "alert_enabled",
		"banner_enabled", "is_clockface_analog", "follow_gps":
		return arithcode.NewFrequencyTable([]uint64{800, 200})

	// Display/UI settings - often true (40% false, 60% true)
	case "gps_enabled", "wifi_enabled", "eth_enabled", "ipv6_enabled",
		"flip_screen", "heading_bold", "wake_on_tap_or_motion",
		"use_12h_clock", "use_long_node_name", "position_broadcast_smart_enabled",
		"fixed_position", "is_power_saving":
		return arithcode.NewFrequencyTable([]uint64{400, 600})

	// Fields with specific biases based on Meshtastic usage patterns
	case "use_preset":
		// Most users use presets (30% false, 70% true)
		return arithcode.NewFrequencyTable([]uint64{300, 700})

	case "tx_enabled":
		// Transmit usually enabled (20% false, 80% true)
		return arithcode.NewFrequencyTable([]uint64{200, 800})

	case "override_duty_cycle", "sx126x_rx_boosted_gain", "pa_fan_disabled":
		// Advanced settings rarely changed (85% false, 15% true)
		return arithcode.NewFrequencyTable([]uint64{850, 150})

	case "ignore_mqtt", "config_ok_to_mqtt":
		// MQTT settings rarely used (90% false, 10% true)
		return arithcode.NewFrequencyTable([]uint64{900, 100})

	case "is_managed", "serial_enabled", "debug_log_api_enabled", "admin_channel_enabled":
		// Admin/debug features rarely enabled (85% false, 15% true)
		return arithcode.NewFrequencyTable([]uint64{850, 150})

	case "uplink_enabled", "downlink_enabled":
		// Channel settings (70% false, 30% true)
		return arithcode.NewFrequencyTable([]uint64{700, 300})

	case "is_muted":
		// Channels rarely muted (90% false, 10% true)
		return arithcode.NewFrequencyTable([]uint64{900, 100})

	case "canShutdown", "hasWifi", "hasBluetooth", "hasEthernet",
		"hasRemoteHardware", "hasPKC":
		// Device capabilities vary widely (50% false, 50% true - conservative)
		return arithcode.NewFrequencyTable([]uint64{500, 500})

	case "request_transfer", "accept_transfer":
		// File transfers rare (95% false, 5% true)
		return arithcode.NewFrequencyTable([]uint64{950, 50})

	case "double_tap_as_button_press", "disable_triple_click", "led_heartbeat_disabled":
		// Button/LED settings (70% false, 30% true)
		return arithcode.NewFrequencyTable([]uint64{700, 300})

	case "compass_north_top":
		// Display orientation (60% false, 40% true)
		return arithcode.NewFrequencyTable([]uint64{600, 400})

	case "unknown_switch", "offline_switch", "public_key_switch",
		"position_switch", "chat_switch", "telemetry_switch", "iaq_switch":
		// UI switches - context dependent (65% false, 35% true)
		return arithcode.NewFrequencyTable([]uint64{650, 350})

	default:
		// Conservative default: slightly biased toward false (60% false, 40% true)
		// This is better than uniform 50/50 for most boolean fields in protocols
		return arithcode.NewFrequencyTable([]uint64{600, 400})
	}
}

// createVarintFirstByteModel creates a probability model for the first byte of a varint.
// The first byte is special because:
// - Bit 7 (0x80) is the continuation bit: 0 = last byte, 1 = more bytes follow
// - Bits 0-6 contain the value
// - Small values (0-127) fit in one byte with bit 7 = 0
// - Larger values need multiple bytes with bit 7 = 1
func createVarintFirstByteModel() arithcode.Model {
	freqs := make([]uint64, 256)

	// Bytes 0-127: Terminal bytes (no continuation)
	// These represent small values that fit in a single byte
	// Favor smaller values - they're more common in practice
	for i := 0; i < 128; i++ {
		// Exponential decay: smaller values are much more common
		// 0 gets highest frequency, decreasing as value increases
		freqs[i] = 200 - uint64(i)
		if freqs[i] < 20 {
			freqs[i] = 20
		}
	}

	// Bytes 128-255: Continuation bytes (more bytes follow)
	// These represent larger values needing multiple bytes
	// Less common than single-byte values
	for i := 128; i < 256; i++ {
		// Lower bits of first byte when value is large
		// More uniform distribution within this range
		freqs[i] = 15
	}

	return arithcode.NewFrequencyTable(freqs)
}

// createVarintContinuationByteModel creates a probability model for continuation bytes.
// Continuation bytes appear after the first byte in multi-byte varints.
// They follow a different pattern than the first byte.
func createVarintContinuationByteModel() arithcode.Model {
	freqs := make([]uint64, 256)

	// Bytes 0-127: Last continuation byte (bit 7 = 0)
	// The continuation ends, these are more common
	for i := 0; i < 128; i++ {
		// Relatively uniform for termination bytes
		// Slightly favor smaller values
		freqs[i] = 100 - uint64(i/4)
		if freqs[i] < 30 {
			freqs[i] = 30
		}
	}

	// Bytes 128-255: More continuation bytes follow (bit 7 = 1)
	// Less common - values rarely need 3+ bytes
	for i := 128; i < 256; i++ {
		freqs[i] = 20
	}

	return arithcode.NewFrequencyTable(freqs)
}

// GetVarintByteModel returns the appropriate model for a varint byte.
// byteIndex: 0 for first byte, 1+ for continuation bytes
func (mcb *ContextualModelBuilder) GetVarintByteModel(byteIndex int) arithcode.Model {
	if byteIndex == 0 {
		return mcb.varintFirstByteModel
	}
	return mcb.varintContByteModel
}
