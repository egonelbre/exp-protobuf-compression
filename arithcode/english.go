package arithcode

import (
	"io"
)

// EnglishModel is a specialized model for compressing English text.
// It uses character frequency statistics typical of English text
// to achieve better compression than a uniform model.
type EnglishModel struct {
	charToSymbol map[rune]int
	symbolToChar []rune
	freqTable    *FrequencyTable
}

// NewEnglishModel creates a model optimized for English text compression.
// The model includes common English characters with frequencies based on
// typical English language statistics.
func NewEnglishModel() *EnglishModel {
	// Character frequencies based on English text analysis
	// Ordered by approximate frequency: space, e, t, a, o, i, n, s, h, r, etc.
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

	// Approximate frequencies based on English text statistics
	// Higher values = more common characters
	freqs := []uint64{
		1300, 1270, 906, 817, 751, 697, 675, 633, 609, 599, // space, e, t, a, o, i, n, s, h, r
		425, 403, 278, 276, 241, 236, 223, 202, 197, 193, // d, l, c, u, m, w, f, g, y, p
		149, 98, 77, 15, 15, 10, 7, // b, v, k, j, x, q, z
		50, 50, 45, 40, 40, 35, 35, 30, 30, 25, // uppercase E-D
		25, 20, 20, 15, 15, 15, 15, 10, 10, 10, // uppercase L-B
		8, 5, 5, 3, 2, 2, // uppercase V-Z
		100, 80, 20, 15, 10, 8, 50, 30, 40, 15, // punctuation . , ! ? ; : - ' " (
		15, 10, 10, 5, 5, 80, 20, 5, // ) [ ] { } \n \t \r
		50, 50, 50, 50, 50, 50, 50, 50, 50, 50, // digits 0-9
		20, 5, 5, 5, 10, 5, 10, 10, 15, 15, // @ # $ % & * + = / \
		30, 5, 8, 8, 3, 3, // _ | < > ~ `
	}

	// Add a special "other" symbol for characters not in our table
	// Use a rune value that won't appear in normal text (private use area)
	const otherCharPlaceholder = rune(0xE000)
	chars = append(chars, otherCharPlaceholder)
	freqs = append(freqs, 100) // Give it reasonable frequency

	charToSymbol := make(map[rune]int, len(chars))
	for i, ch := range chars {
		charToSymbol[ch] = i
	}

	return &EnglishModel{
		charToSymbol: charToSymbol,
		symbolToChar: chars,
		freqTable:    NewFrequencyTable(freqs),
	}
}

func (em *EnglishModel) SymbolCount() int {
	return em.freqTable.SymbolCount()
}

func (em *EnglishModel) Freq(symbol int) (low, high uint64) {
	return em.freqTable.Freq(symbol)
}

func (em *EnglishModel) TotalFreq() uint64 {
	return em.freqTable.TotalFreq()
}

func (em *EnglishModel) Find(cumFreq uint64) int {
	return em.freqTable.Find(cumFreq)
}

// EncodeString encodes a string using the English model.
func EncodeString(s string, w io.Writer) error {
	enc := NewEncoder(w)
	model := NewEnglishModel()
	otherSymbol := len(model.symbolToChar) - 1 // Last symbol is "other"
	byteModel := NewUniformModel(256)

	// Convert string to runes to properly count characters
	runes := []rune(s)
	length := len(runes)

	// Encode length as variable-length quantity (up to 4 bytes)
	tempLen := length
	for i := 0; i < 4; i++ {
		b := byte(tempLen & 0x7F)
		tempLen >>= 7
		if tempLen > 0 {
			b |= 0x80 // More bytes to come
		}
		if err := enc.Encode(int(b), byteModel); err != nil {
			return err
		}
		if tempLen == 0 {
			break
		}
	}

	// Encode each character
	for _, ch := range runes {
		symbol, ok := model.charToSymbol[ch]
		if !ok {
			// Character not in our table, encode as "other" followed by raw UTF-8
			if err := enc.Encode(otherSymbol, model); err != nil {
				return err
			}
			// Encode the actual rune as UTF-8 bytes
			utf8Bytes := []byte(string(ch))
			// Encode number of UTF-8 bytes
			if err := enc.Encode(len(utf8Bytes), NewUniformModel(5)); err != nil { // Max 4 bytes for UTF-8
				return err
			}
			for _, b := range utf8Bytes {
				if err := enc.Encode(int(b), byteModel); err != nil {
					return err
				}
			}
		} else {
			if err := enc.Encode(symbol, model); err != nil {
				return err
			}
		}
	}

	return enc.Close()
}

// DecodeString decodes a string using the English model.
func DecodeString(r io.Reader) (string, error) {
	dec, err := NewDecoder(r)
	if err != nil {
		return "", err
	}

	model := NewEnglishModel()
	otherSymbol := len(model.symbolToChar) - 1
	byteModel := NewUniformModel(256)

	// Decode the length
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

	// Decode characters
	result := make([]rune, 0, length)
	for len(result) < length {
		symbol, err := dec.Decode(model)
		if err != nil {
			return "", err
		}

		if symbol == otherSymbol {
			// Decode UTF-8 bytes for unknown character
			// Read the number of UTF-8 bytes
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

			// Convert UTF-8 bytes to rune
			runes := []rune(string(utf8Bytes))
			if len(runes) > 0 {
				result = append(result, runes[0])
			}
		} else {
			result = append(result, model.symbolToChar[symbol])
		}
	}

	return string(result), nil
}
