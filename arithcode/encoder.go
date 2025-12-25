package arithcode

import (
	"io"
)

const (
	// stateBits defines the precision of the arithmetic coding state.
	// We use 32 bits to balance precision and performance.
	stateBits = 32
	// stateMax is the maximum value of the state (2^32 - 1).
	stateMax uint64 = (1 << stateBits) - 1
	// half is the midpoint of the state range.
	half uint64 = 1 << (stateBits - 1)
	// quarter is one quarter of the state range.
	quarter uint64 = 1 << (stateBits - 2)
)

// Encoder compresses data using arithmetic coding.
type Encoder struct {
	output      *bitWriter
	low         uint64 // Lower bound of the current interval
	high        uint64 // Upper bound of the current interval
	pendingBits int    // Number of pending underflow bits
}

// NewEncoder creates a new arithmetic encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		output: newBitWriter(w),
		low:    0,
		high:   stateMax,
	}
}

// Encode writes a symbol using the given model.
func (e *Encoder) Encode(symbol int, model Model) error {
	// Get the symbol's frequency range
	symLow, symHigh := model.Freq(symbol)
	total := model.TotalFreq()

	// Calculate the new interval
	rangeSize := e.high - e.low + 1
	e.high = e.low + (rangeSize*symHigh)/total - 1
	e.low = e.low + (rangeSize*symLow)/total

	// Normalize the interval
	for {
		if e.high < half {
			// High is in lower half, output 0
			if err := e.output.WriteBit(0); err != nil {
				return err
			}
			// Output pending 1s
			for e.pendingBits > 0 {
				if err := e.output.WriteBit(1); err != nil {
					return err
				}
				e.pendingBits--
			}
		} else if e.low >= half {
			// Low is in upper half, output 1
			if err := e.output.WriteBit(1); err != nil {
				return err
			}
			// Output pending 0s
			for e.pendingBits > 0 {
				if err := e.output.WriteBit(0); err != nil {
					return err
				}
				e.pendingBits--
			}
			e.low -= half
			e.high -= half
		} else if e.low >= quarter && e.high < 3*quarter {
			// Underflow: interval straddles the middle
			e.pendingBits++
			e.low -= quarter
			e.high -= quarter
		} else {
			break
		}

		// Scale up the interval
		e.low = (e.low << 1) & stateMax
		e.high = ((e.high << 1) & stateMax) | 1
	}

	return nil
}

// Close finalizes the encoding and flushes any remaining bits.
func (e *Encoder) Close() error {
	// Output enough bits to disambiguate the final interval
	e.pendingBits++

	if e.low < quarter {
		if err := e.output.WriteBit(0); err != nil {
			return err
		}
		for e.pendingBits > 0 {
			if err := e.output.WriteBit(1); err != nil {
				return err
			}
			e.pendingBits--
		}
	} else {
		if err := e.output.WriteBit(1); err != nil {
			return err
		}
		for e.pendingBits > 0 {
			if err := e.output.WriteBit(0); err != nil {
				return err
			}
			e.pendingBits--
		}
	}

	return e.output.Flush()
}

// bitWriter writes individual bits to an io.Writer.
type bitWriter struct {
	output      io.Writer
	accumulator byte
	numBits     int
}

func newBitWriter(w io.Writer) *bitWriter {
	return &bitWriter{output: w}
}

func (bw *bitWriter) WriteBit(bit byte) error {
	bw.accumulator = (bw.accumulator << 1) | (bit & 1)
	bw.numBits++

	if bw.numBits == 8 {
		if _, err := bw.output.Write([]byte{bw.accumulator}); err != nil {
			return err
		}
		bw.accumulator = 0
		bw.numBits = 0
	}

	return nil
}

func (bw *bitWriter) Flush() error {
	if bw.numBits > 0 {
		// Pad with zeros to complete the byte
		bw.accumulator <<= (8 - bw.numBits)
		if _, err := bw.output.Write([]byte{bw.accumulator}); err != nil {
			return err
		}
		bw.accumulator = 0
		bw.numBits = 0
	}
	return nil
}
