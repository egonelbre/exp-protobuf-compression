package meshtasticmodel

import (
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/egonelbre/exp-protobuf-compression/pbmodel"
)

// Version represents a compression/decompression implementation version
type Version struct {
	Name        string
	Short       string // Short description for compact display
	Description string // Full description
	Compress    func(proto.Message, io.Writer) error
	Decompress  func(io.Reader, proto.Message) error
}

// Versions is a table of all compression implementations
var Versions = []Version{
	{
		Name:        "pbmodel",
		Short:       "baseline",
		Description: "Generic protobuf compression baseline (order-0 strings)",
		Compress:    pbmodel.Compress,
		Decompress:  pbmodel.Decompress,
	},
	{
		Name:        "pbmodel-o1",
		Short:       "baseline+order-1",
		Description: "Generic protobuf compression with order-1 string compression",
		Compress:    pbmodel.CompressOrder1,
		Decompress:  pbmodel.DecompressOrder1,
	},
	{
		Name:        "pbmodel-o2",
		Short:       "baseline+order-2",
		Description: "Generic protobuf compression with order-2 string compression",
		Compress:    pbmodel.CompressOrder2,
		Decompress:  pbmodel.DecompressOrder2,
	},
	{
		Name:        "V1",
		Short:       "presence bits",
		Description: "Meshtastic-specific optimizations: text payload detection, coordinate delta encoding, optimized field models",
		Compress:    CompressV1,
		Decompress:  DecompressV1,
	},
	{
		Name:        "V2",
		Short:       "delta fields",
		Description: "Delta-encoded field numbers for sparse messages (no presence bits)",
		Compress:    CompressV2,
		Decompress:  DecompressV2,
	},
	{
		Name:        "V3",
		Short:       "hybrid",
		Description: "Hybrid encoding: auto-selects between presence-bit and delta-encoded field numbers",
		Compress:    CompressV3,
		Decompress:  DecompressV3,
	},
	{
		Name:        "V4",
		Short:       "enum prediction",
		Description: "V1 + enum value prediction (common enums encoded with 1 bit)",
		Compress:    CompressV4,
		Decompress:  DecompressV4,
	},
	{
		Name:        "V5",
		Short:       "context-aware",
		Description: "Context-aware models optimized for specific field types and value ranges",
		Compress:    CompressV5,
		Decompress:  DecompressV5,
	},
	{
		Name:        "V6",
		Short:       "bit-packed bools",
		Description: "V5 + bit packing for boolean clusters",
		Compress:    CompressV6,
		Decompress:  DecompressV6,
	},
	{
		Name:        "V7",
		Short:       "boolean models",
		Description: "V6 + field-specific boolean models",
		Compress:    CompressV7,
		Decompress:  DecompressV7,
	},
	{
		Name:        "V8",
		Short:       "varint models",
		Description: "V7 + varint byte models",
		Compress:    CompressV8,
		Decompress:  DecompressV8,
	},
	{
		Name:        "V9",
		Short:       "order-1 strings",
		Description: "V8 + order-1 English string compression",
		Compress:    CompressV9,
		Decompress:  DecompressV9,
	},
	{
		Name:        "V10",
		Short:       "order-2 strings",
		Description: "V8 + order-2 English string compression",
		Compress:    CompressV10,
		Decompress:  DecompressV10,
	},
}
