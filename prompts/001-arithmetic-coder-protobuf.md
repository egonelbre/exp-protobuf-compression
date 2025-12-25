<objective>
Implement an arithmetic coding compression system for Protocol Buffer messages in Go. The system should intelligently build compression models based on protobuf message structure, with specialized handling for English text in string fields. This will enable efficient compression of structured data while maintaining the ability to perfectly reconstruct the original messages.
</objective>

<context>
Arithmetic coding is a form of entropy encoding that can achieve compression rates close to the theoretical Shannon limit. Unlike Huffman coding which encodes symbols into whole bits, arithmetic coding encodes entire messages into a single fractional number between 0 and 1.

For protobuf messages, we can leverage the known structure (field types, nesting, etc.) to build specialized models that improve compression efficiency. String fields containing English text benefit from character-level models trained on English language statistics.

The implementation should be production-ready with thorough testing including fuzzing to ensure compression/decompression is lossless across all possible inputs.
</context>

<requirements>
1. Create two separate Go packages:
   - **Arithmetic coder package**: Core compression/decompression algorithms, model interfaces, and English text model
   - **Protobuf model package**: Protobuf-specific model building and message handling

2. **Arithmetic coder package** must include:
   - `Compress(data, model) -> compressed bytes` function
   - `Decompress(compressed bytes, model) -> original data` function
   - Model interface that supports probability distributions
   - English text model optimized for character-level compression of English strings
   - Proper handling of precision and numerical stability

3. **Protobuf model package** must:
   - Analyze protobuf message descriptors to build appropriate models
   - Handle nested message structures recursively
   - Support all protobuf field types (int32, int64, string, bytes, bool, enums, repeated fields, etc.)
   - Route string fields through the English text model
   - Build context-aware models based on field types

4. **Testing requirements**:
   - Randomized tests that generate random protobuf messages and verify compress/decompress roundtrips
   - Fuzzing tests using Go's fuzzing framework to test edge cases
   - Tests should cover: empty messages, deeply nested structures, large strings, repeated fields, all field types
   - Verify that decompressed messages exactly match original messages (bit-for-bit equality)

5. Handle edge cases:
   - Empty messages
   - Messages with only default values
   - Very long strings
   - Deeply nested structures
   - Messages with unknown fields
</requirements>

<implementation>
**Package structure:**
```
./arithcode/          # Arithmetic coding package
  encoder.go          # Core compression algorithm
  decoder.go          # Core decompression algorithm
  model.go            # Model interface and implementations
  english.go          # English text model

./pbmodel/            # Protobuf model package
  builder.go          # Build models from protobuf descriptors
  compress.go         # Compress protobuf messages
  decompress.go       # Decompress to protobuf messages
```

**Key considerations:**

1. **Arithmetic coding precision**: Use fixed-point or arbitrary precision arithmetic to avoid floating-point errors. Consider using uint64 with careful overflow handling, or Go's big.Int for arbitrary precision.

2. **English text model**: Build a character-level model with:
   - Letter frequency distributions (common in English: e, t, a, o, i, n, s, h, r...)
   - Contextual modeling (previous character influences probability of next)
   - Handle uppercase, lowercase, numbers, punctuation, and whitespace
   - Fall back gracefully for non-English characters

3. **Protobuf model building**: Use protobuf reflection (google.golang.org/protobuf/reflect/protoreflect) to:
   - Inspect message descriptors at runtime
   - Determine field types and build appropriate models
   - Handle repeated fields with length encoding
   - Handle oneofs and optional fields
   - Preserve field ordering

4. **Why separate packages**: The arithmetic coder is a general-purpose compression algorithm that could work with any structured data. The protobuf model builder is a specific application. This separation allows reuse of the arithmetic coder for other data formats and keeps concerns cleanly separated.

5. **Testing strategy**: 
   - Create several test .proto files with various field types
   - Generate random messages using protobuf's reflection API
   - Use Go 1.18+ fuzzing with `func FuzzCompress(f *testing.F)` pattern
   - Measure compression ratios to ensure the system is actually compressing

**What to avoid and WHY:**
- Don't use floating-point arithmetic for the coder core—floating-point errors will cause decompression failures because we need exact bit-level precision
- Don't hard-code protobuf field handling—use reflection so the system works with any protobuf message
- Don't skip testing with large messages (>1MB)—this is where precision issues and performance problems appear
- Don't assume string fields are always English—provide fallback handling for arbitrary UTF-8
</implementation>

<output>
Create the following files:

**Arithmetic coding package:**
- `./arithcode/encoder.go` - Compression implementation
- `./arithcode/decoder.go` - Decompression implementation  
- `./arithcode/model.go` - Model interface and basic implementations
- `./arithcode/english.go` - English text model
- `./arithcode/arithcode_test.go` - Unit tests for the arithmetic coder

**Protobuf model package:**
- `./pbmodel/builder.go` - Model building from protobuf descriptors
- `./pbmodel/compress.go` - Compress protobuf messages
- `./pbmodel/decompress.go` - Decompress to protobuf messages
- `./pbmodel/pbmodel_test.go` - Unit and integration tests

**Test support:**
- `./pbmodel/testdata/test.proto` - Test protobuf definitions with various field types
- `./pbmodel/fuzz_test.go` - Fuzzing tests

**Documentation:**
- `./README.md` - Overview of the implementation, usage examples, compression ratios achieved

Include Go module setup if not already present (go.mod).
</output>

<verification>
Before declaring complete, verify your work by:

1. **Compilation**: Run `go build ./...` to ensure all packages compile without errors

2. **Unit tests**: Run `go test ./...` to verify all tests pass

3. **Fuzzing**: Run `go test -fuzz=FuzzCompress -fuzztime=30s ./pbmodel` to verify fuzzing works and finds no issues

4. **Roundtrip test**: Create a test that:
   - Generates a complex nested protobuf message
   - Compresses it
   - Decompresses it
   - Verifies the result matches the original using proto.Equal()

5. **Compression effectiveness**: Verify that the system actually compresses data by testing with realistic messages and confirming compressed size < original size

6. **Edge cases**: Verify handling of:
   - Empty messages (should compress to minimal bytes)
   - Messages with all default values
   - Very long strings (10KB+ English text)
   - Deeply nested messages (5+ levels)
</verification>

<success_criteria>
- Both packages compile without errors or warnings
- All unit tests pass
- Fuzzing tests run without finding crashes or panics
- Roundtrip property holds: decompress(compress(msg)) == msg for all valid messages
- English text compresses better than arbitrary binary data
- Code is well-documented with package comments and key function documentation
- README provides clear usage examples and demonstrates the compression working
</success_criteria>