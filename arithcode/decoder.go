package arithcode

import (
	"io"
)

// Decoder decompresses data using arithmetic coding.
type Decoder struct {
	input *bitReader
	low   uint64 // Lower bound of the current interval
	high  uint64 // Upper bound of the current interval
	value uint64 // Current value being decoded
}

// NewDecoder creates a new arithmetic decoder that reads from r.
func NewDecoder(r io.Reader) (*Decoder, error) {
	br := newBitReader(r)

	// Read initial value (stateBits bits)
	var value uint64
	for i := 0; i < stateBits; i++ {
		bit, err := br.ReadBit()
		if err != nil {
			if err == io.EOF && i > 0 {
				// Partial read is acceptable for short messages
				value <<= (stateBits - i)
				break
			}
			return nil, err
		}
		value = (value << 1) | uint64(bit)
	}

	return &Decoder{
		input: br,
		low:   0,
		high:  stateMax,
		value: value,
	}, nil
}

// Decode reads and returns the next symbol using the given model.
func (d *Decoder) Decode(model Model) (int, error) {
	// Calculate the position within the current interval
	total := model.TotalFreq()
	rangeSize := d.high - d.low + 1
	cumFreq := ((d.value-d.low+1)*total - 1) / rangeSize

	// Find the symbol corresponding to this cumulative frequency
	symbol := model.Find(cumFreq)
	symLow, symHigh := model.Freq(symbol)

	// Update the interval
	d.high = d.low + (rangeSize*symHigh)/total - 1
	d.low = d.low + (rangeSize*symLow)/total

	// Normalize the interval
	for {
		if d.high < half {
			// Do nothing
		} else if d.low >= half {
			d.low -= half
			d.high -= half
			d.value -= half
		} else if d.low >= quarter && d.high < 3*quarter {
			d.low -= quarter
			d.high -= quarter
			d.value -= quarter
		} else {
			break
		}

		// Scale up the interval
		d.low = (d.low << 1) & stateMax
		d.high = ((d.high << 1) & stateMax) | 1

		// Read next bit into value
		bit, err := d.input.ReadBit()
		if err != nil {
			if err == io.EOF {
				bit = 0 // Treat EOF as 0 bits
			} else {
				return 0, err
			}
		}
		d.value = ((d.value << 1) & stateMax) | uint64(bit)
	}

	return symbol, nil
}

// bitReader reads individual bits from an io.Reader.
type bitReader struct {
	input       io.Reader
	accumulator byte
	numBits     int
}

func newBitReader(r io.Reader) *bitReader {
	return &bitReader{input: r}
}

func (br *bitReader) ReadBit() (byte, error) {
	if br.numBits == 0 {
		buf := make([]byte, 1)
		n, err := br.input.Read(buf)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		br.accumulator = buf[0]
		br.numBits = 8
	}

	br.numBits--
	bit := (br.accumulator >> br.numBits) & 1
	return bit, nil
}
