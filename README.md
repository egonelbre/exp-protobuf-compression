# Arithmetic Coding for Protocol Buffers

This project implements arithmetic coding compression for Protocol Buffer messages in Go. It provides intelligent compression by building models based on protobuf message structure, with specialized handling for English text in string fields.

## Features

- **Arithmetic Coding Engine**: High-precision arithmetic coding implementation with proper handling of numerical stability
- **Protobuf-Aware Compression**: Automatically builds compression models based on protobuf message structure and field types
- **English Text Optimization**: Specialized character-level model for compressing English text in string fields
- **Nested Message Support**: Handles deeply nested protobuf message structures recursively
- **Comprehensive Type Support**: Supports all protobuf field types including:
  - Numeric types (int32, int64, uint32, uint64, sint32, sint64, fixed32, fixed64, sfixed32, sfixed64)
  - Floating point (float, double)
  - Boolean values
  - Strings (with English text model)
  - Bytes
  - Enums
  - Repeated fields
  - Maps
  - Nested messages
  - Oneofs

## Project Structure

```
.
├── arithcode/          # Core arithmetic coding package
│   ├── encoder.go      # Compression implementation
│   ├── decoder.go      # Decompression implementation
│   ├── model.go        # Model interface and basic implementations
│   ├── english.go      # English text model
│   └── arithcode_test.go # Unit tests
│
└── pbmodel/            # Protobuf-specific compression
    ├── builder.go      # Model building from protobuf descriptors
    ├── compress.go     # Compress protobuf messages
    ├── decompress.go   # Decompress to protobuf messages
    ├── pbmodel_test.go # Unit and roundtrip tests
    ├── fuzz_test.go    # Fuzzing tests
    └── testdata/
        └── test.proto  # Test protobuf definitions
```

## Installation

```bash
go get github.com/egonelbre/protobuf-compression
```

## Usage

### Basic Example

```go
package main

import (
    "bytes"
    "fmt"
    "log"

    "github.com/egonelbre/protobuf-compression/pbmodel"
    "github.com/egonelbre/protobuf-compression/pbmodel/testdata"
)

func main() {
    // Create a protobuf message
    original := &testdata.SimpleMessage{
        Id:     12345,
        Name:   "Alice",
        Active: true,
    }

    // Compress the message
    var compressed bytes.Buffer
    if err := pbmodel.Compress(original, &compressed); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Original size: %d bytes\n", proto.Size(original))
    fmt.Printf("Compressed size: %d bytes\n", compressed.Len())

    // Decompress the message
    decoded := &testdata.SimpleMessage{}
    if err := pbmodel.Decompress(&compressed, decoded); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Decoded: %v\n", decoded)
}
```

### Compressing Complex Messages

```go
// Create a complex message with nested structures
profile := &testdata.UserProfile{
    UserId:   123456,
    Username: "johndoe",
    Email:    "john.doe@example.com",
    FullName: "John Doe",
    Bio:      "Software engineer passionate about compression algorithms and distributed systems.",
    Tags:     []string{"golang", "protobuf", "compression", "algorithms"},
    AccountStatus: testdata.Status_ACTIVE,
    Address: &testdata.UserProfile_Address{
        Street:  "123 Main St",
        City:    "San Francisco",
        State:   "CA",
        Zip:     "94105",
        Country: "USA",
    },
    CreatedAt: 1609459200,
    UpdatedAt: 1640995200,
    Metadata: map[string]string{
        "theme":    "dark",
        "language": "en",
        "timezone": "PST",
    },
}

// Compress
var buf bytes.Buffer
if err := pbmodel.Compress(profile, &buf); err != nil {
    log.Fatal(err)
}

// Decompress
decodedProfile := &testdata.UserProfile{}
if err := pbmodel.Decompress(&buf, decodedProfile); err != nil {
    log.Fatal(err)
}
```

### Using the Arithmetic Coder Directly

The arithmetic coding engine can be used independently of protobuf:

```go
package main

import (
    "bytes"
    "fmt"

    "github.com/egonelbre/protobuf-compression/arithcode"
)

func main() {
    // Create a model (uniform distribution over 256 symbols)
    model := arithcode.NewUniformModel(256)

    // Data to compress
    data := []int{72, 101, 108, 108, 111} // "Hello" in ASCII

    // Compress
    var compressed bytes.Buffer
    enc := arithcode.NewEncoder(&compressed)
    for _, symbol := range data {
        if err := enc.Encode(symbol, model); err != nil {
            panic(err)
        }
    }
    enc.Close()

    // Decompress
    dec, _ := arithcode.NewDecoder(&compressed)
    decoded := make([]int, len(data))
    for i := range decoded {
        symbol, _ := dec.Decode(model)
        decoded[i] = symbol
    }

    fmt.Printf("Original: %v\n", data)
    fmt.Printf("Decoded:  %v\n", decoded)
}
```

### Compressing English Text

```go
package main

import (
    "bytes"
    "fmt"

    "github.com/egonelbre/protobuf-compression/arithcode"
)

func main() {
    text := "The quick brown fox jumps over the lazy dog."

    // Compress using English model
    var compressed bytes.Buffer
    if err := arithcode.EncodeString(text, &compressed); err != nil {
        panic(err)
    }

    // Decompress
    decoded, err := arithcode.DecodeString(&compressed)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Original:   %q (%d bytes)\n", text, len(text))
    fmt.Printf("Compressed: %d bytes (%.1f%%)\n", 
        compressed.Len(), 100.0*float64(compressed.Len())/float64(len(text)))
    fmt.Printf("Decoded:    %q\n", decoded)
}
```

## How It Works

### Arithmetic Coding

Arithmetic coding represents a message as a fractional value between 0 and 1, achieving compression rates close to the theoretical Shannon limit. Unlike Huffman coding which encodes symbols into whole bits, arithmetic coding can represent symbols with fractional bits, making it more efficient for data with non-uniform probability distributions.

The implementation uses 32-bit fixed-point arithmetic to maintain precision while avoiding floating-point errors that could cause decompression failures.

### Protobuf Model Building

The protobuf compression builds specialized models for each field type:

- **Boolean fields**: 2-symbol uniform model (true/false)
- **Numeric fields**: Variable-length encoding with models favoring smaller values
- **String fields**: English text model with character frequency statistics
- **Bytes fields**: Uniform byte model
- **Enum fields**: Uniform model over enum values
- **Nested messages**: Recursive compression using field-specific models

### English Text Model

The English text model uses character-level compression with:
- Frequency-based probability distributions for common English characters
- Support for uppercase, lowercase, digits, punctuation, and whitespace
- Fallback handling for non-English Unicode characters
- Higher probabilities for common letters (e, t, a, o, i, n, s, h, r)

## Performance

The compression ratio depends on the message content and structure. Here are some typical results:

- **Simple messages**: Varies based on field values and types
- **English text**: 40-60% compression for typical English sentences
- **Repeated structures**: Better compression due to predictable patterns
- **Binary data**: Limited compression (uniform distribution)

## Testing

The project includes comprehensive testing:

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run fuzzing tests (requires Go 1.18+)
go test -fuzz=FuzzSimpleMessage -fuzztime=30s ./pbmodel
go test -fuzz=FuzzNumericMessage -fuzztime=30s ./pbmodel
go test -fuzz=FuzzMessageWithBytes -fuzztime=30s ./pbmodel

# Run benchmarks
go test -bench=. ./...
```

## Limitations

- **Precision**: Uses 32-bit arithmetic which provides good balance between precision and performance
- **Model Adaptation**: Models are static; adaptive models could improve compression for specific data patterns
- **String Language**: English text model is optimized for English; other languages may not compress as well
- **Error Handling**: Any bit corruption in compressed data will cause decompression failure (as expected with arithmetic coding)

## Future Improvements

Potential enhancements:

- Adaptive models that learn from the data being compressed
- Context-based modeling for improved string compression
- Parallel compression of independent message fields
- Support for streaming compression/decompression
- Additional language models beyond English

## License

This is a demonstration project for educational purposes.

## References

- Arithmetic Coding: https://en.wikipedia.org/wiki/Arithmetic_coding
- Protocol Buffers: https://protobuf.dev/
- Shannon Entropy: https://en.wikipedia.org/wiki/Entropy_(information_theory)
