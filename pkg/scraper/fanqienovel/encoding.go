package fanqienovel

// TextDecodeResult contains the decoded text and the selected encoding
// rotation. Rotation is empty when the input did not contain encoded runes.
type TextDecodeResult struct {
	Text         string
	Rotation     string
	Replacements int
}

type text_encoding_rotation struct {
	name          string
	character_map map[rune]rune
}

// Add future Fanqie encoding tables here. DecodeText chooses the rotation with
// the most matching encoded runes, so different rotations must not be applied
// sequentially to the same chapter.
var text_encoding_rotations = []text_encoding_rotation{
	new_text_encoding_rotation("legacy_v1", encoded_characters, decoded_characters),
}

func new_text_encoding_rotation(name string, encoded string, decoded string) text_encoding_rotation {
	encoded_runes := []rune(encoded)
	decoded_runes := []rune(decoded)
	character_count := len(encoded_runes)
	if len(decoded_runes) < character_count {
		character_count = len(decoded_runes)
	}
	character_map := make(map[rune]rune, character_count)
	for character_index := 0; character_index < character_count; character_index++ {
		character_map[encoded_runes[character_index]] = decoded_runes[character_index]
	}
	return text_encoding_rotation{name: name, character_map: character_map}
}

// DecodeText automatically selects the encoding rotation that matches the
// largest number of runes in value and decodes with only that rotation.
func DecodeText(value string) TextDecodeResult {
	return decode_text_with_rotations(value, text_encoding_rotations)
}

func decode_text_with_rotations(value string, rotations []text_encoding_rotation) TextDecodeResult {
	selected_index := -1
	selected_matches := 0
	for rotation_index, rotation := range rotations {
		matches := 0
		for _, character := range value {
			if _, exists := rotation.character_map[character]; exists {
				matches++
			}
		}
		if matches > selected_matches {
			selected_index = rotation_index
			selected_matches = matches
		}
	}
	if selected_index < 0 {
		return TextDecodeResult{Text: value}
	}

	selected_rotation := rotations[selected_index]
	decoded_runes := []rune(value)
	for character_index, character := range decoded_runes {
		if decoded_character, exists := selected_rotation.character_map[character]; exists {
			decoded_runes[character_index] = decoded_character
		}
	}
	return TextDecodeResult{
		Text:         string(decoded_runes),
		Rotation:     selected_rotation.name,
		Replacements: selected_matches,
	}
}
