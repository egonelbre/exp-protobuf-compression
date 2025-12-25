// Package arithcode implements arithmetic coding for data compression.
// Arithmetic coding is an entropy encoding technique that represents
// messages as fractional values, achieving compression rates close to
// the theoretical Shannon limit.
package arithcode

// Model defines the interface for probability models used in arithmetic coding.
// A model provides the probability distribution for symbols in the data stream.
type Model interface {
	// SymbolCount returns the total number of possible symbols in this model.
	SymbolCount() int

	// Freq returns the cumulative frequency range [low, high) for the given symbol.
	// The range is relative to the total frequency returned by TotalFreq().
	// Returns (low, high) where 0 <= low < high <= TotalFreq().
	Freq(symbol int) (low, high uint64)

	// TotalFreq returns the sum of all symbol frequencies.
	TotalFreq() uint64

	// Find returns the symbol corresponding to the given cumulative frequency.
	// The cumFreq must be in range [0, TotalFreq()).
	Find(cumFreq uint64) int
}

// UniformModel implements a model where all symbols have equal probability.
type UniformModel struct {
	numSymbols int
}

// NewUniformModel creates a uniform probability model with the given number of symbols.
func NewUniformModel(numSymbols int) *UniformModel {
	if numSymbols <= 0 {
		panic("numSymbols must be positive")
	}
	return &UniformModel{numSymbols: numSymbols}
}

func (m *UniformModel) SymbolCount() int {
	return m.numSymbols
}

func (m *UniformModel) Freq(symbol int) (low, high uint64) {
	if symbol < 0 || symbol >= m.numSymbols {
		panic("symbol out of range")
	}
	return uint64(symbol), uint64(symbol + 1)
}

func (m *UniformModel) TotalFreq() uint64 {
	return uint64(m.numSymbols)
}

func (m *UniformModel) Find(cumFreq uint64) int {
	if cumFreq >= uint64(m.numSymbols) {
		panic("cumFreq out of range")
	}
	return int(cumFreq)
}

// FrequencyTable implements a model with custom symbol frequencies.
type FrequencyTable struct {
	cumFreqs []uint64 // Cumulative frequencies: cumFreqs[i] = sum of freqs[0..i-1]
	total    uint64   // Total of all frequencies
}

// NewFrequencyTable creates a model from the given symbol frequencies.
// The frequencies slice defines the frequency (probability weight) of each symbol.
func NewFrequencyTable(frequencies []uint64) *FrequencyTable {
	if len(frequencies) == 0 {
		panic("frequencies must not be empty")
	}

	cumFreqs := make([]uint64, len(frequencies)+1)
	cumFreqs[0] = 0

	var total uint64
	for i, freq := range frequencies {
		if freq == 0 {
			panic("frequency must be positive")
		}
		total += freq
		cumFreqs[i+1] = total
	}

	return &FrequencyTable{
		cumFreqs: cumFreqs,
		total:    total,
	}
}

func (ft *FrequencyTable) SymbolCount() int {
	return len(ft.cumFreqs) - 1
}

func (ft *FrequencyTable) Freq(symbol int) (low, high uint64) {
	if symbol < 0 || symbol >= ft.SymbolCount() {
		panic("symbol out of range")
	}
	return ft.cumFreqs[symbol], ft.cumFreqs[symbol+1]
}

func (ft *FrequencyTable) TotalFreq() uint64 {
	return ft.total
}

func (ft *FrequencyTable) Find(cumFreq uint64) int {
	if cumFreq >= ft.total {
		panic("cumFreq out of range")
	}

	// Binary search for the symbol
	left, right := 0, len(ft.cumFreqs)-1
	for left < right-1 {
		mid := (left + right) / 2
		if ft.cumFreqs[mid] <= cumFreq {
			left = mid
		} else {
			right = mid
		}
	}
	return left
}
