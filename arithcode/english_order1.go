package arithcode

import "io"

// EnglishOrder1Model is an order-1 model for English text compression.
// It uses the previous character as context to predict the next character,
// achieving better compression than order-0 models.
type EnglishOrder1Model struct {
	charToSymbol map[rune]int
	symbolToChar []rune

	// Context-dependent frequency tables
	// contextModels[prevChar] gives the frequency table for the next character
	contextModels map[int]*FrequencyTable

	// Default model for when we don't have context
	defaultModel *FrequencyTable

	// Special symbols
	otherSymbol int // For characters not in our table
}

// NewEnglishOrder1Model creates an order-1 model for English text.
func NewEnglishOrder1Model() *EnglishOrder1Model {
	// Common English characters
	chars := []rune{
		' ', 'e', 't', 'a', 'o', 'i', 'n', 's', 'h', 'r',
		'd', 'l', 'c', 'u', 'm', 'w', 'f', 'g', 'y', 'p',
		'b', 'v', 'k', 'j', 'x', 'q', 'z',
		'E', 'T', 'A', 'O', 'I', 'N', 'S', 'H', 'R', 'D',
		'L', 'C', 'U', 'M', 'W', 'F', 'G', 'Y', 'P', 'B',
		'V', 'K', 'J', 'X', 'Q', 'Z',
		'.', ',', '!', '?', ';', ':', '-', '\'', '"', '(',
		')', '[', ']', '{', '}', '\n', '\t', '\r',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'@', '#', '$', '%', '&', '*', '+', '=', '/', '\\',
		'_', '|', '<', '>', '~', '`',
	}

	// Add special "other" symbol
	const otherCharPlaceholder = rune(0xE000)
	chars = append(chars, otherCharPlaceholder)

	charToSymbol := make(map[rune]int, len(chars))
	for i, ch := range chars {
		charToSymbol[ch] = i
	}

	model := &EnglishOrder1Model{
		charToSymbol:  charToSymbol,
		symbolToChar:  chars,
		contextModels: make(map[int]*FrequencyTable),
		otherSymbol:   len(chars) - 1,
	}

	// Build default model (order-0 frequencies)
	model.defaultModel = model.createDefaultModel()

	// Build context-specific models for common characters
	model.buildContextModels()

	return model
}

// createDefaultModel creates the default order-0 frequency table.
func (em *EnglishOrder1Model) createDefaultModel() *FrequencyTable {
	numSymbols := len(em.symbolToChar)
	freqs := make([]uint64, numSymbols)

	// Default frequencies (order-0)
	baseFreqs := []uint64{
		1300, 1270, 906, 817, 751, 697, 675, 633, 609, 599, // space, e, t, a, o, i, n, s, h, r
		425, 403, 278, 276, 241, 236, 223, 202, 197, 193, // d, l, c, u, m, w, f, g, y, p
		149, 98, 77, 15, 15, 10, 7, // b, v, k, j, x, q, z
		50, 50, 45, 40, 40, 35, 35, 30, 30, 25, // uppercase
		25, 20, 20, 15, 15, 15, 15, 10, 10, 10,
		8, 5, 5, 3, 2, 2,
		100, 80, 20, 15, 10, 8, 50, 30, 40, 15, // punctuation
		15, 10, 10, 5, 5, 80, 20, 5,
		50, 50, 50, 50, 50, 50, 50, 50, 50, 50, // digits
		20, 5, 5, 5, 10, 5, 10, 10, 15, 15,
		30, 5, 8, 8, 3, 3,
	}

	copy(freqs, baseFreqs)
	freqs[em.otherSymbol] = 100 // "other" symbol

	return NewFrequencyTable(freqs)
}

// buildContextModels creates frequency tables for different contexts.
func (em *EnglishOrder1Model) buildContextModels() {
	numSymbols := len(em.symbolToChar)

	// Helper to create a frequency table with biases
	createBiasedModel := func(biases map[rune]uint64) *FrequencyTable {
		freqs := make([]uint64, numSymbols)
		// Start with small base frequency for all symbols
		for i := range freqs {
			freqs[i] = 10
		}
		// Apply biases
		for ch, freq := range biases {
			if idx, ok := em.charToSymbol[ch]; ok {
				freqs[idx] = freq
			}
		}
		return NewFrequencyTable(freqs)
	}

	// Space typically followed by uppercase or common starting letters
	em.contextModels[em.charToSymbol[' ']] = createBiasedModel(map[rune]uint64{
		't': 800, 'a': 700, 'o': 500, 'i': 450, 'w': 400, 's': 380, 'b': 300, 'c': 280,
		'h': 250, 'm': 220, 'f': 200, 'p': 180, 'd': 170, 'n': 150,
		'T': 100, 'I': 90, 'A': 80, 'W': 70, 'H': 60, 'S': 50,
	})

	// 'e' often followed by space, d, r, s, n
	em.contextModels[em.charToSymbol['e']] = createBiasedModel(map[rune]uint64{
		' ': 900, 'd': 600, 'r': 550, 's': 500, 'n': 400, 't': 300, 'a': 250, 'l': 200, 'c': 150,
	})

	// 't' often followed by h, e, i, o
	em.contextModels[em.charToSymbol['t']] = createBiasedModel(map[rune]uint64{
		'h': 800, 'e': 500, 'i': 400, 'o': 350, ' ': 300, 'a': 200, 'r': 180, 's': 150, 'y': 120,
	})

	// 'h' often followed by e, a, i, o
	em.contextModels[em.charToSymbol['h']] = createBiasedModel(map[rune]uint64{
		'e': 700, 'a': 400, 'i': 350, 'o': 300, ' ': 200, 't': 150, 'r': 100,
	})

	// 'a' often followed by t, n, r, l, s
	em.contextModels[em.charToSymbol['a']] = createBiasedModel(map[rune]uint64{
		't': 600, 'n': 550, 'r': 500, 'l': 450, 's': 400, ' ': 350, 'd': 300, 'i': 250, 'c': 200,
	})

	// 'n' often followed by space, d, t, g, e
	em.contextModels[em.charToSymbol['n']] = createBiasedModel(map[rune]uint64{
		' ': 700, 'd': 500, 't': 450, 'g': 400, 'e': 350, 's': 300, 'c': 200, 'o': 180,
	})

	// 'o' often followed by n, f, r, u, m
	em.contextModels[em.charToSymbol['o']] = createBiasedModel(map[rune]uint64{
		'n': 600, 'f': 400, 'r': 380, 'u': 350, 'm': 300, ' ': 280, 'w': 250, 'p': 200, 't': 180,
	})

	// 'r' often followed by e, s, t, i, o
	em.contextModels[em.charToSymbol['r']] = createBiasedModel(map[rune]uint64{
		'e': 600, 's': 400, 't': 350, 'i': 300, 'o': 280, ' ': 250, 'a': 200, 'y': 150,
	})

	// 'i' often followed by n, t, o, s, c
	em.contextModels[em.charToSymbol['i']] = createBiasedModel(map[rune]uint64{
		'n': 600, 't': 500, 'o': 400, 's': 350, 'c': 300, 'e': 250, ' ': 200, 'a': 150, 'l': 140,
	})

	// 's' often followed by space, t, e, i, h
	em.contextModels[em.charToSymbol['s']] = createBiasedModel(map[rune]uint64{
		' ': 700, 't': 500, 'e': 450, 'i': 350, 'h': 300, 'o': 250, 's': 200, 'a': 180, 'u': 150,
	})

	// Common punctuation contexts
	em.contextModels[em.charToSymbol['.']] = createBiasedModel(map[rune]uint64{
		' ': 800, '\n': 150,
	})

	em.contextModels[em.charToSymbol[',']] = createBiasedModel(map[rune]uint64{
		' ': 900,
	})

	// Digits often followed by other digits, space, or punctuation
	for _, digit := range "0123456789" {
		em.contextModels[em.charToSymbol[digit]] = createBiasedModel(map[rune]uint64{
			'0': 200, '1': 200, '2': 200, '3': 200, '4': 200,
			'5': 200, '6': 200, '7': 200, '8': 200, '9': 200,
			' ': 300, '.': 150, ',': 100,
		})
	}
}

// GetModel returns the appropriate model for the given context.
func (em *EnglishOrder1Model) GetModel(prevSymbol int) Model {
	if prevSymbol < 0 || prevSymbol >= len(em.symbolToChar) {
		return em.defaultModel
	}

	if model, ok := em.contextModels[prevSymbol]; ok {
		return model
	}

	return em.defaultModel
}

// EncodeStringOrder1 encodes a string using the order-1 English model.
func EncodeStringOrder1(s string, w io.Writer) error {
	enc := NewEncoder(w)
	model := NewEnglishOrder1Model()
	byteModel := NewUniformModel(256)

	runes := []rune(s)
	length := len(runes)

	// Encode length as varint
	tempLen := length
	for i := 0; i < 4; i++ {
		b := byte(tempLen & 0x7F)
		tempLen >>= 7
		if tempLen > 0 {
			b |= 0x80
		}
		if err := enc.Encode(int(b), byteModel); err != nil {
			return err
		}
		if tempLen == 0 {
			break
		}
	}

	// Track previous symbol for context
	prevSymbol := -1

	// Encode each character
	for _, ch := range runes {
		symbol, ok := model.charToSymbol[ch]
		if !ok {
			// Character not in table, encode as "other"
			contextModel := model.GetModel(prevSymbol)
			if err := enc.Encode(model.otherSymbol, contextModel); err != nil {
				return err
			}

			// Encode the actual rune as UTF-8 bytes
			utf8Bytes := []byte(string(ch))
			if err := enc.Encode(len(utf8Bytes), NewUniformModel(5)); err != nil {
				return err
			}
			for _, b := range utf8Bytes {
				if err := enc.Encode(int(b), byteModel); err != nil {
					return err
				}
			}

			prevSymbol = model.otherSymbol
		} else {
			// Use context-specific model
			contextModel := model.GetModel(prevSymbol)
			if err := enc.Encode(symbol, contextModel); err != nil {
				return err
			}
			prevSymbol = symbol
		}
	}

	return enc.Close()
}

// DecodeStringOrder1 decodes a string using the order-1 English model.
func DecodeStringOrder1(r io.Reader) (string, error) {
	dec, err := NewDecoder(r)
	if err != nil {
		return "", err
	}

	model := NewEnglishOrder1Model()
	byteModel := NewUniformModel(256)

	// Decode length
	var length int
	for i := 0; i < 4; i++ {
		symbol, err := dec.Decode(byteModel)
		if err != nil {
			return "", err
		}
		b := byte(symbol)
		length |= int(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			break
		}
	}

	// Track previous symbol for context
	prevSymbol := -1

	// Decode characters
	result := make([]rune, 0, length)
	for len(result) < length {
		contextModel := model.GetModel(prevSymbol)
		symbol, err := dec.Decode(contextModel)
		if err != nil {
			return "", err
		}

		if symbol == model.otherSymbol {
			// Decode UTF-8 bytes for unknown character
			numBytes, err := dec.Decode(NewUniformModel(5))
			if err != nil {
				return "", err
			}

			utf8Bytes := make([]byte, numBytes)
			for i := 0; i < numBytes; i++ {
				b, err := dec.Decode(byteModel)
				if err != nil {
					return "", err
				}
				utf8Bytes[i] = byte(b)
			}

			runes := []rune(string(utf8Bytes))
			if len(runes) > 0 {
				result = append(result, runes[0])
			}

			prevSymbol = model.otherSymbol
		} else {
			result = append(result, model.symbolToChar[symbol])
			prevSymbol = symbol
		}
	}

	return string(result), nil
}
