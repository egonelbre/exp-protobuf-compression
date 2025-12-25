<objective>
Add comprehensive compression ratio and performance benchmark tests to the arithmetic coder implementation. These tests will measure and validate compression effectiveness across different data types and sizes, and benchmark encoding/decoding performance to ensure the system meets performance expectations.
</objective>

<context>
The arithmetic coding system for protobuf messages has been implemented with:
- Core arithmetic coder in `./arithcode/`
- Protobuf-specific compression in `./pbmodel/`
- English text model for string optimization
- Support for all protobuf field types

We need benchmarks to:
1. Demonstrate compression effectiveness (especially for English text)
2. Measure performance characteristics (throughput, memory usage)
3. Compare compression ratios across different message types
4. Validate that the system provides meaningful compression benefits

@arithcode/arithcode_test.go
@pbmodel/pbmodel_test.go
@pbmodel/testdata/test.proto
</context>

<requirements>
1. **Compression Ratio Tests** - Add tests that measure and report compression ratios:
   - Test compression of realistic English text (various lengths: 100 bytes, 1KB, 10KB, 100KB)
   - Test compression of structured data (nested protobuf messages)
   - Test compression of numeric data (repeated integers, mixed types)
   - Test compression of binary data (to show baseline/worst case)
   - Report metrics: original size, compressed size, compression ratio percentage

2. **Performance Benchmarks** - Add Go benchmarks for:
   - Encoding throughput (messages/sec or MB/sec)
   - Decoding throughput (messages/sec or MB/sec)
   - Memory allocations per operation
   - Performance across message sizes (small: <1KB, medium: 1-10KB, large: >10KB)

3. **Comparative Analysis** - Add tests that compare:
   - Arithmetic coder compression vs raw protobuf size
   - English text model vs uniform byte model
   - Different message structures (flat vs nested)

4. **Test Organization**:
   - Add compression ratio tests to existing test files
   - Use Go's `testing.B` for benchmarks
   - Include table-driven tests for multiple scenarios
   - Add helper functions to generate realistic test data
   - Log results clearly for easy interpretation
</requirements>

<implementation>
**File locations:**
- Add compression ratio tests to `./arithcode/arithcode_test.go` (for string compression)
- Add compression ratio tests to `./pbmodel/pbmodel_test.go` (for protobuf messages)
- Add benchmark tests to both files using Go's benchmark framework

**Key considerations:**

1. **Realistic test data**: Use actual English text samples, realistic protobuf messages (like UserProfile), and varied data patterns

2. **Compression ratio calculation**: 
   ```go
   ratio := 100.0 * float64(compressedSize) / float64(originalSize)
   ```
   Report as percentage (e.g., "62.4%" means compressed is 62.4% of original)

3. **Benchmark naming**: Follow Go conventions:
   ```go
   func BenchmarkCompressSmallMessage(b *testing.B) { ... }
   func BenchmarkCompressLargeMessage(b *testing.B) { ... }
   ```

4. **Memory benchmarks**: Use `b.ReportAllocs()` to track allocations

5. **Throughput reporting**: Use `b.SetBytes()` to report MB/sec automatically

6. **Test output**: Use `t.Logf()` for compression ratio reports so they appear with `-v` flag

**What to avoid and WHY:**
- Don't use tiny test data (<10 bytes) - compression overhead makes ratios meaningless
- Don't benchmark with empty messages - focus on realistic workloads
- Don't compare different protobuf message types directly - they have different characteristics
- Don't forget to reset the timer in benchmarks after setup: `b.ResetTimer()`
</implementation>

<output>
Modify these existing files:
- `./arithcode/arithcode_test.go` - Add compression ratio tests and benchmarks for string/text compression
- `./pbmodel/pbmodel_test.go` - Add compression ratio tests and benchmarks for protobuf message compression

Add test functions like:
- `TestCompressionRatioEnglishText` - Tests various sizes of English text
- `TestCompressionRatioBinaryData` - Baseline test with random binary data
- `TestCompressionRatioStructuredData` - Tests realistic protobuf messages
- `BenchmarkCompressSmallText` - Benchmark small text compression
- `BenchmarkCompressLargeMessage` - Benchmark large message compression
- `BenchmarkDecompressLargeMessage` - Benchmark decompression performance
</output>

<verification>
Before declaring complete, verify your work:

1. **Run tests**: Execute `go test -v ./...` and confirm:
   - All new tests pass
   - Compression ratio logs appear in output
   - Ratios are reasonable (English text < 70%, binary data ~100%)

2. **Run benchmarks**: Execute `go test -bench=. -benchmem ./...` and confirm:
   - Benchmarks run without errors
   - Performance metrics are reported (ns/op, MB/s, allocs/op)
   - Memory allocations are reasonable

3. **Validate results**: Check that:
   - English text shows better compression than binary data
   - Larger messages show better compression ratios (overhead amortization)
   - Performance is reasonable (aim for >1MB/sec throughput)
   - Results are clearly logged and easy to interpret

4. **Documentation**: Add comments explaining what each test validates and what results indicate
</verification>

<success_criteria>
- Compression ratio tests added for multiple data types and sizes
- Performance benchmarks added for encoding and decoding operations
- All tests pass when run with `go test -v ./...`
- Benchmarks run successfully with `go test -bench=. ./...`
- Test output clearly shows compression ratios and performance metrics
- English text demonstrates better compression than binary data
- Results are well-documented and interpretable
</success_criteria>
