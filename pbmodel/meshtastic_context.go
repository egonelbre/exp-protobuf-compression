package pbmodel

import (
	"github.com/egonelbre/protobuf-compression/arithcode"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MeshtasticContextualModelBuilder creates highly specialized models based on
// message type and field context within Meshtastic protocol messages.
type MeshtasticContextualModelBuilder struct {
	*MeshtasticModelBuilder
	
	// Context tracking
	messageType     string // Current message type (Position, User, etc.)
	contextModels   map[string]arithcode.Model
	enumPredictions map[string]protoreflect.EnumNumber
}

// NewMeshtasticContextualModelBuilder creates a context-aware model builder.
func NewMeshtasticContextualModelBuilder() *MeshtasticContextualModelBuilder {
	return &MeshtasticContextualModelBuilder{
		MeshtasticModelBuilder: NewMeshtasticModelBuilder(),
		contextModels:          make(map[string]arithcode.Model),
		enumPredictions:        getCommonEnumValues(),
	}
}

// GetContextualFieldModel returns a model optimized for the specific field context.
func (mcb *MeshtasticContextualModelBuilder) GetContextualFieldModel(fieldPath string, fd protoreflect.FieldDescriptor) arithcode.Model {
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
func (mcb *MeshtasticContextualModelBuilder) createContextSpecificModel(fieldPath string, fd protoreflect.FieldDescriptor) arithcode.Model {
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
func (mcb *MeshtasticContextualModelBuilder) SetMessageType(msgType string) {
	mcb.messageType = msgType
}
