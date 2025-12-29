# Arithmetic Coder Performance Analysis

This document summarizes the compression ratios and performance characteristics of the arithmetic coding implementation.

## Compression Ratios

### English Text Compression

| Data Size | Original (bytes) | Compressed (bytes) | Ratio |
|-----------|------------------|-------------------|-------|
| Small (100B) | 100 | 62 | 62.00% |
| Medium (1KB) | 1,024 | 611 | 59.67% |
| Large (10KB) | 10,240 | 6,091 | 59.48% |
| Very Large (100KB) | 102,400 | 60,878 | 59.45% |

**Key Findings:**
- English text compresses to ~60% of original size
- Compression ratio improves with larger text (overhead amortization)
- Consistent performance across different sizes

### Binary Data (Baseline)

| Data Size | Original (bytes) | Compressed (bytes) | Ratio |
|-----------|------------------|-------------------|-------|
| Small (100B) | 100 | 264 | 264.00% |
| Medium (1KB) | 1,024 | 2,580 | 251.95% |
| Large (10KB) | 10,240 | 26,420 | 258.01% |

**Note:** Random binary data expands due to encoding overhead, which is expected behavior.

### Protobuf Message Compression

| Message Type | Proto Size (bytes) | Compressed (bytes) | Ratio |
|--------------|-------------------|-------------------|-------|
| SimpleMessage | 20 | 14 | 70.00% |
| UserProfile (Small) | 138 | 95 | 68.84% |
| UserProfile (Large) | 954 | 634 | 66.46% |
| NestedMessage | 179 | 102 | 56.98% |
| RepeatedMessage (1000 items) | 2,097 | 1,967 | 93.80% |

**Key Findings:**
- Structured data with English text compresses well (56-70%)
- Nested messages achieve best compression (56.98%)
- Numeric data compresses less effectively (93.80%)
- Larger messages generally compress better

### Compression Comparison

- **English text:** 65.12% compression ratio
- **Binary data:** 254.57% expansion
- **Improvement:** 189.45 percentage points better for English text

## Performance Benchmarks

### String Encoding/Decoding Performance

#### Encoding Throughput

| Data Size | Throughput | ns/op | Allocations |
|-----------|------------|-------|-------------|
| 100B | 25.53 MB/s | 3,917 | 77 allocs |
| 1KB | 34.88 MB/s | 29,359 | 642 allocs |
| 10KB | 19.97 MB/s | 512,685 | 6,222 allocs |
| 100KB | 17.72 MB/s | 5,780,019 | 61,993 allocs |

#### Decoding Throughput

| Data Size | Throughput | ns/op | Allocations |
|-----------|------------|-------|-------------|
| 100B | 23.17 MB/s | 4,316 | 107 allocs |
| 1KB | 32.93 MB/s | 31,096 | 663 allocs |
| 10KB | 20.42 MB/s | 501,485 | 6,241 allocs |
| 100KB | 17.77 MB/s | 5,762,855 | 62,014 allocs |

**Performance Characteristics:**
- Peak throughput: ~35 MB/s for 1KB messages
- Encode and decode speeds are comparable
- Performance is consistent for medium to large sizes
- Overhead is higher for very small messages

### Protobuf Message Performance

#### Compression Throughput

| Message Type | Throughput | ns/op | Allocations |
|--------------|------------|-------|-------------|
| SimpleMessage | 4.86 MB/s | 4,119 | 58 allocs |
| UserProfile (Small) | 6.04 MB/s | 10,602 | 165 allocs |
| UserProfile (Large) | 9.02 MB/s | 105,730 | 1,818 allocs |
| RepeatedMessage (1000) | 23.25 MB/s | 80,641 | 1,960 allocs |

#### Decompression Throughput

| Message Type | Throughput | ns/op | Allocations |
|--------------|------------|-------|-------------|
| SimpleMessage | 4.34 MB/s | 4,609 | 118 allocs |
| UserProfile (Large) | 7.28 MB/s | 131,020 | 3,125 allocs |
| RepeatedMessage (1000) | 9.51 MB/s | 197,175 | 4,734 allocs |

#### Roundtrip Performance

- **UserProfile (Large):** 3.82 MB/s, 249,482 ns/op, 4,942 allocations

**Performance Characteristics:**
- Throughput ranges from 4-23 MB/s depending on message complexity
- Repeated fields achieve highest throughput (23.25 MB/s)
- Larger messages benefit from better amortization
- Memory allocations scale linearly with message size

## Conclusions

1. **Compression Effectiveness:**
   - English text achieves ~40% size reduction (60% ratio)
   - Structured messages with text compress to 56-70% of original size
   - Binary/numeric data does not compress well

2. **Performance:**
   - Consistent throughput of 17-35 MB/s for most workloads
   - Suitable for real-time applications with moderate data volumes
   - Memory allocations are reasonable but could be optimized further

3. **Use Cases:**
   - Best for: Text-heavy protobuf messages, logging, serialization
   - Not ideal for: Pure binary data, already-compressed data, tiny messages

4. **Trade-offs:**
   - CPU overhead for compression/decompression
   - Memory allocations during encoding/decoding
   - Significant space savings for text data (40% reduction)
   - Better compression with larger messages
