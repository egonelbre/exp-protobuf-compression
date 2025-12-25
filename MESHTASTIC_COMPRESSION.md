# Meshtastic Protobuf Compression Improvements

## Summary

This document describes the compression improvements implemented for Meshtastic protobuf messages.

## Encoding Strategy Comparison

Two field encoding strategies were implemented and compared:

### V1: Presence Bits (Recommended)
- Encodes a presence bit for each field in order
- **Best for dense messages** where most fields are present
- Fixed overhead per message based on total fields

### V2: Delta-Encoded Field Numbers
- Encodes only present fields using delta-encoded field numbers  
- **Best for sparse messages** where few fields are present
- Variable overhead based on number of present fields

### V3: Hybrid Encoding (Automatic Selection)
- Automatically chooses between V1 and V2 based on message density
- Adds 1 byte overhead for strategy flag
- **Best for mixed workloads** with both sparse and dense messages

### V4: Enum Value Prediction (Recommended) ⭐
- Extends V1 with predictive encoding for common enum values
- Common values: 1 bit, uncommon values: 1 bit + full encoding
- **Predicts**: LOC_INTERNAL, ALT_INTERNAL, TBEAM, CLIENT, DEFAULT, NO_DELAY
- **No overhead** for messages without predicted enums
- **Best overall** - combines V1 efficiency with enum optimization

### Performance Comparison

| Message Type | Fields Present | V1 | V2 | V3 | V4 | Best |
|--------------|----------------|----|----|----|----|------|
| Position | 4/70 fields | **16** | 19 | 17 | **16** | **V1/V4** |
| User profile | 4/8 fields | 36 | 39 | 36 | **35** | **V4** ✅ |
| Text message | 3/15 fields | **49** | 52 | 49 | **49** | **V1/V4** |

**V4 Improvements**:
- ✅ **User profile**: 36→35 bytes (saves 1 byte via hw_model prediction)
- ✅ **No overhead** when predictions don't match
- ✅ **Better or equal** to V1 in all cases

**Key Findings**:
- **Meshtastic messages are generally dense**, making presence-bit encoding (V1) optimal
- **V2 delta encoding is worse** for dense messages due to field number overhead
- **V3 hybrid encoding** correctly chooses V1 strategy but adds 1 byte flag overhead
- **V4 enum prediction** adds intelligence with zero overhead on miss
- **For tiny messages** (<20 bytes), even 1 byte overhead is significant

**Recommendation**: Use **V4** for Meshtastic - it's V1 plus enum optimization with no downside.

## Results

### Before (Basic Compressor)
- **Position with full data**: 17→16 bytes (94.12% - only 1 byte saved)
- **User profile**: 41→35 bytes (85.37% - 6 bytes saved)
- **Text message packet**: 77→75 bytes (97.40% - only 2 bytes saved) ❌

### After (Meshtastic-Specific Compressor)
- **Position with full data**: 17→16 bytes (94.12% - only 1 byte saved)
- **User profile**: 41→36 bytes (87.80% - 5 bytes saved)
- **Text message packet**: 77→49 bytes (63.64% - **28 bytes saved**) ✅

## Key Improvement: Smart Byte Field Compression

### Problem
The Meshtastic `Data.payload` field is defined as `bytes`, not `string`, but often contains text messages. The basic and adaptive compressors treated all byte fields as binary data, missing the opportunity to compress text.

### Solution
Created a Meshtastic-specific compressor (`MeshtasticCompress`) that:

1. **Detects text in byte fields** using multiple heuristics:
   - Checks if `portnum` is `TEXT_MESSAGE_APP`
   - Validates UTF-8 encoding
   - Analyzes printable character ratio (>80% printable = likely text)

2. **Encodes a text flag** (1 bit) to indicate compression method

3. **Applies English language model** when text is detected, achieving much better compression for natural language

### Code Location
- Compressor: `pbmodel/meshtastic_compress.go`
- Decompressor: `pbmodel/meshtastic_decompress.go`

## Why Position Messages Don't Compress Well

Position messages use `sfixed32` for coordinates:
- `latitude_i`: sfixed32 (always 4 bytes)
- `longitude_i`: sfixed32 (always 4 bytes)  
- `time`: fixed32 (always 4 bytes)

These fixed-width fields contain large, varied values that appear random to arithmetic coding. A 17-byte message with 12 bytes of fixed-width data leaves little room for compression.

## Future Improvement Opportunities

### 1. **Delta Encoding for Coordinates** (Not Implemented)
GPS coordinates change slowly between readings. Storing only the delta from a reference point would compress much better:
- First position: store absolute coordinates
- Subsequent positions: store small deltas
- Typical delta: <100 meters = small varint

### 2. **Quantization/Precision Reduction** (Not Implemented)
Meshtastic doesn't need millimeter precision:
- Current: 1e-7 degrees ≈ 1cm precision
- Reduced: 1e-5 degrees ≈ 1m precision
- Savings: Could reduce coordinates from 4 bytes to 2-3 bytes

⚠️ Note: This changes decoded values, may not be acceptable

### 3. **Timestamp Delta Encoding** (Not Implemented)
Timestamps are large but sequential:
- Store base timestamp (4 bytes)
- Subsequent messages: store delta in seconds (1-2 bytes)

### 4. **Batch Compression** (Not Implemented)
For multiple messages in a session:
- Build shared dictionary from message patterns
- Leverage inter-message redundancy
- Could achieve 50%+ compression on batches

### 5. **Field-Specific Integer Models** (Partially Implemented)
The adaptive model builder includes field-specific optimizations but could be enhanced:
- Detect ID fields (larger, more uniform distribution)
- Detect counter fields (small values, 0-10 range)
- Detect coordinate deltas (small when present)

## Recommendations

1. **Use `MeshtasticCompress` for text-heavy messages** - 36% compression ratio vs 97% with basic compressor

2. **For position-only messages**, consider:
   - Batch compression when sending multiple positions
   - Delta encoding if implementing a custom protocol
   - Quantization if 1m accuracy is acceptable

3. **For production deployment**:
   - Implement delta encoding for coordinates
   - Add timestamp delta encoding
   - Consider quantization based on use case requirements

## Testing

Run compression tests:
```bash
cd pbmodel
go test -v -run TestMeshtasticCompressionRatio
```

## Usage

```go
import "github.com/egonelbre/protobuf-compression/pbmodel"

// Compress
var buf bytes.Buffer
err := pbmodel.MeshtasticCompress(msg, &buf)

// Decompress
result := &meshtastic.MeshPacket{}
err = pbmodel.MeshtasticDecompress(&buf, result)
```
