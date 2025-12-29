package arithcode

import "io"

// EnglishOrder2Model is an order-2 model for English text compression.
// It uses the previous 2 characters as context to predict the next character,
// achieving better compression than order-0 or order-1 models.
type EnglishOrder2Model struct {
	charToSymbol map[rune]int
	symbolToChar []rune
	
	// Context-dependent frequency tables
	// contextModels[ctx] gives the frequency table for the next character
	// where ctx is a hash of the previous 2 characters
	contextModels map[string]*FrequencyTable
	
	// Default models for when we don't have enough context
	order1Model   *EnglishOrder1Model
	defaultModel  *FrequencyTable
	
	// Special symbols
	otherSymbol int
}

// NewEnglishOrder2Model creates an order-2 model for English text.
func NewEnglishOrder2Model() *EnglishOrder2Model {
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

	model := &EnglishOrder2Model{
		charToSymbol:  charToSymbol,
		symbolToChar:  chars,
		contextModels: make(map[string]*FrequencyTable),
		order1Model:   NewEnglishOrder1Model(),
		otherSymbol:   len(chars) - 1,
	}

	// Build default model (order-0 frequencies)
	model.defaultModel = model.createDefaultModel()

	// Build context-specific models for common bigrams
	model.buildContextModels()

	return model
}

// createDefaultModel creates the default order-0 frequency table.
func (em *EnglishOrder2Model) createDefaultModel() *FrequencyTable {
	numSymbols := len(em.symbolToChar)
	freqs := make([]uint64, numSymbols)
	
	// Default frequencies
	baseFreqs := []uint64{
		1300, 1270, 906, 817, 751, 697, 675, 633, 609, 599,
		425, 403, 278, 276, 241, 236, 223, 202, 197, 193,
		149, 98, 77, 15, 15, 10, 7,
		50, 50, 45, 40, 40, 35, 35, 30, 30, 25,
		25, 20, 20, 15, 15, 15, 15, 10, 10, 10,
		8, 5, 5, 3, 2, 2,
		100, 80, 20, 15, 10, 8, 50, 30, 40, 15,
		15, 10, 10, 5, 5, 80, 20, 5,
		50, 50, 50, 50, 50, 50, 50, 50, 50, 50,
		20, 5, 5, 5, 10, 5, 10, 10, 15, 15,
		30, 5, 8, 8, 3, 3,
	}
	
	copy(freqs, baseFreqs)
	freqs[em.otherSymbol] = 100
	
	return NewFrequencyTable(freqs)
}

// buildContextModels creates frequency tables for common bigram contexts.
func (em *EnglishOrder2Model) buildContextModels() {
	numSymbols := len(em.symbolToChar)
	
	// Helper to create a frequency table with biases
	createBiasedModel := func(biases map[rune]uint64) *FrequencyTable {
		freqs := make([]uint64, numSymbols)
		// Start with small base frequency
		for i := range freqs {
			freqs[i] = 5
		}
		// Apply biases
		for ch, freq := range biases {
			if idx, ok := em.charToSymbol[ch]; ok {
				freqs[idx] = freq
			}
		}
		return NewFrequencyTable(freqs)
	}
	
	// Common bigram patterns in English
	// "th" → e, a, i, o, er
	em.contextModels["th"] = createBiasedModel(map[rune]uint64{
		'e': 900, 'a': 400, 'i': 350, 'o': 300, 'r': 250, ' ': 200, 'y': 150,
	})
	
	// "he" → r, n, space, d, y
	em.contextModels["he"] = createBiasedModel(map[rune]uint64{
		'r': 700, ' ': 500, 'n': 400, 'd': 300, 'y': 250, 's': 200, 'a': 150,
	})
	
	// "in" → g, space, t, e, d
	em.contextModels["in"] = createBiasedModel(map[rune]uint64{
		'g': 800, ' ': 600, 't': 400, 'e': 300, 'd': 250, 's': 200, 'k': 150,
	})
	
	// "er" → space, s, e, i, a
	em.contextModels["er"] = createBiasedModel(map[rune]uint64{
		' ': 700, 's': 500, 'e': 300, 'i': 250, 'a': 200, 'y': 180, 't': 150,
	})
	
	// "an" → d, t, space, c, y
	em.contextModels["an"] = createBiasedModel(map[rune]uint64{
		'd': 600, 't': 500, ' ': 450, 'c': 300, 'y': 250, 'g': 200, 's': 150,
	})
	
	// "re" → space, d, s, a, n
	em.contextModels["re"] = createBiasedModel(map[rune]uint64{
		' ': 600, 'd': 500, 's': 450, 'a': 350, 'n': 300, 't': 250, 'e': 200,
	})
	
	// "nd" → space, e, a, i
	em.contextModels["nd"] = createBiasedModel(map[rune]uint64{
		' ': 800, 'e': 400, 'a': 200, 'i': 150, 's': 100,
	})
	
	// "on" → space, g, e, t, a
	em.contextModels["on"] = createBiasedModel(map[rune]uint64{
		' ': 600, 'g': 500, 'e': 350, 't': 300, 'a': 200, 's': 180, 'd': 150,
	})
	
	// "nt" → space, e, i, s, a
	em.contextModels["nt"] = createBiasedModel(map[rune]uint64{
		' ': 700, 'e': 400, 'i': 300, 's': 250, 'a': 200, 'o': 150, 'r': 120,
	})
	
	// "ha" → t, v, n, s, d
	em.contextModels["ha"] = createBiasedModel(map[rune]uint64{
		't': 700, 'v': 500, 'n': 350, 's': 300, 'd': 250, 'r': 200, 'l': 150,
	})
	
	// "en" → t, d, space, c, s
	em.contextModels["en"] = createBiasedModel(map[rune]uint64{
		't': 600, 'd': 400, ' ': 350, 'c': 300, 's': 250, 'e': 200, 'a': 150,
	})
	
	// "ed" → space, (punctuation)
	em.contextModels["ed"] = createBiasedModel(map[rune]uint64{
		' ': 900, '.': 150, ',': 100, '!': 50, '?': 40,
	})
	
	// "to" → space, r, n, o, w
	em.contextModels["to"] = createBiasedModel(map[rune]uint64{
		' ': 700, 'r': 400, 'n': 300, 'o': 250, 'w': 200, 'p': 150, 'm': 120,
	})
	
	// "it" → space, y, h, e, i
	em.contextModels["it"] = createBiasedModel(map[rune]uint64{
		' ': 600, 'y': 400, 'h': 350, 'e': 300, 'i': 250, 's': 200, 't': 150,
	})
	
	// "st" → space, a, e, i, r
	em.contextModels["st"] = createBiasedModel(map[rune]uint64{
		' ': 500, 'a': 400, 'e': 350, 'i': 300, 'r': 250, 'o': 200, 'u': 150,
	})
	
	// "io" → n, ns
	em.contextModels["io"] = createBiasedModel(map[rune]uint64{
		'n': 900, 'u': 150, 's': 100,
	})
	
	// "le" → space, d, s, r, a
	em.contextModels["le"] = createBiasedModel(map[rune]uint64{
		' ': 600, 'd': 350, 's': 300, 'r': 250, 'a': 200, 't': 180, 'n': 150,
	})
	
	// "ar" → e, d, y, t, s
	em.contextModels["ar"] = createBiasedModel(map[rune]uint64{
		'e': 500, 'd': 400, 'y': 350, 't': 300, 's': 250, 'i': 200, 'k': 150,
	})
	
	// "te" → space, d, r, s, n
	em.contextModels["te"] = createBiasedModel(map[rune]uint64{
		' ': 500, 'd': 450, 'r': 400, 's': 350, 'n': 300, 'm': 250, 'l': 200,
	})
	
	// "co" → n, m, u, l, r
	em.contextModels["co"] = createBiasedModel(map[rune]uint64{
		'n': 600, 'm': 500, 'u': 400, 'l': 300, 'r': 250, 'v': 200, 'p': 150,
	})
	
	// "or" → space, e, t, d, y
	em.contextModels["or"] = createBiasedModel(map[rune]uint64{
		' ': 600, 'e': 400, 't': 350, 'd': 300, 'y': 250, 's': 200, 'i': 150,
	})
	
	// "at" → space, e, i, ion, h
	em.contextModels["at"] = createBiasedModel(map[rune]uint64{
		' ': 500, 'e': 450, 'i': 400, 'h': 300, 't': 250, 'u': 200, 'o': 150,
	})
	
	// "ou" → t, r, n, s, l
	em.contextModels["ou"] = createBiasedModel(map[rune]uint64{
		't': 600, 'r': 500, 'n': 400, 's': 350, 'l': 300, 'p': 200, 'g': 150,
	})
	
	// Space + common starting letters
	em.contextModels[" t"] = createBiasedModel(map[rune]uint64{
		'h': 900, 'o': 400, 'i': 300, 'a': 250, 'e': 200, 'r': 150, 'w': 120,
	})
	
	em.contextModels[" a"] = createBiasedModel(map[rune]uint64{
		'n': 600, ' ': 400, 't': 350, 'l': 300, 's': 250, 'r': 200, 'b': 150,
	})
	
	em.contextModels[" i"] = createBiasedModel(map[rune]uint64{
		'n': 700, 's': 500, 't': 400, 'f': 200, ' ': 150,
	})
	
	em.contextModels[" w"] = createBiasedModel(map[rune]uint64{
		'h': 600, 'a': 400, 'i': 350, 'e': 300, 'o': 250, 'r': 200,
	})
	
	em.contextModels[" h"] = createBiasedModel(map[rune]uint64{
		'e': 700, 'a': 500, 'i': 350, 'o': 300, 'u': 200, 'y': 150,
	})
	
	// Common endings
	em.contextModels["ly"] = createBiasedModel(map[rune]uint64{
		' ': 900, '.': 100, ',': 80,
	})
	
	em.contextModels["ng"] = createBiasedModel(map[rune]uint64{
		' ': 700, 's': 300, 'e': 200, '.': 100,
	})
}

// GetModel returns the appropriate model for the given context.
func (em *EnglishOrder2Model) GetModel(prev1, prev2 int) Model {
	// Try order-2 context (previous 2 chars)
	if prev1 >= 0 && prev1 < len(em.symbolToChar) && prev2 >= 0 && prev2 < len(em.symbolToChar) {
		ctx := string([]rune{em.symbolToChar[prev2], em.symbolToChar[prev1]})
		if model, ok := em.contextModels[ctx]; ok {
			return model
		}
	}
	
	// Fall back to order-1 context
	if prev1 >= 0 && prev1 < len(em.symbolToChar) {
		return em.order1Model.GetModel(prev1)
	}
	
	// Fall back to order-0
	return em.defaultModel
}

// EncodeStringOrder2 encodes a string using the order-2 English model.
func EncodeStringOrder2(s string, w io.Writer) error {
	enc := NewEncoder(w)
	model := NewEnglishOrder2Model()
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
	
	// Track previous 2 symbols for context
	prevSymbol1 := -1 // most recent
	prevSymbol2 := -1 // second most recent
	
	// Encode each character
	for _, ch := range runes {
		symbol, ok := model.charToSymbol[ch]
		if !ok {
			// Character not in table, encode as "other"
			contextModel := model.GetModel(prevSymbol1, prevSymbol2)
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
			
			prevSymbol2 = prevSymbol1
			prevSymbol1 = model.otherSymbol
		} else {
			// Use context-specific model
			contextModel := model.GetModel(prevSymbol1, prevSymbol2)
			if err := enc.Encode(symbol, contextModel); err != nil {
				return err
			}
			prevSymbol2 = prevSymbol1
			prevSymbol1 = symbol
		}
	}
	
	return enc.Close()
}

// DecodeStringOrder2 decodes a string using the order-2 English model.
func DecodeStringOrder2(r io.Reader) (string, error) {
	dec, err := NewDecoder(r)
	if err != nil {
		return "", err
	}
	
	model := NewEnglishOrder2Model()
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
	
	// Track previous 2 symbols for context
	prevSymbol1 := -1
	prevSymbol2 := -1
	
	// Decode characters
	result := make([]rune, 0, length)
	for len(result) < length {
		contextModel := model.GetModel(prevSymbol1, prevSymbol2)
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
			
			prevSymbol2 = prevSymbol1
			prevSymbol1 = model.otherSymbol
		} else {
			result = append(result, model.symbolToChar[symbol])
			prevSymbol2 = prevSymbol1
			prevSymbol1 = symbol
		}
	}
	
	return string(result), nil
}
