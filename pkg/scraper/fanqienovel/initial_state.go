package fanqienovel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

var initialStateMarker = []byte("window.__INITIAL_STATE__")

func ExtractInitialStateJSON(body []byte) (json.RawMessage, error) {
	idx := bytes.Index(body, initialStateMarker)
	if idx < 0 {
		return nil, fmt.Errorf("missing fanqienovel initial state")
	}
	rest := body[idx+len(initialStateMarker):]
	eq := bytes.IndexByte(rest, '=')
	if eq < 0 {
		return nil, fmt.Errorf("missing fanqienovel initial state assignment")
	}
	rest = rest[eq+1:]
	open := bytes.IndexByte(rest, '{')
	if open < 0 {
		return nil, fmt.Errorf("missing fanqienovel initial state object")
	}
	raw, err := extractBalancedObject(rest[open:])
	if err != nil {
		return nil, err
	}
	jsonBytes := normalizeJSObjectLiteral(raw)
	if !json.Valid(jsonBytes) {
		var payload any
		if err := json.Unmarshal(jsonBytes, &payload); err != nil {
			return nil, fmt.Errorf("invalid fanqienovel initial state json: %w", err)
		}
		return nil, fmt.Errorf("invalid fanqienovel initial state json")
	}
	return json.RawMessage(append([]byte(nil), jsonBytes...)), nil
}

func ParseInitialState(body []byte) (*InitialState, error) {
	raw, err := ExtractInitialStateJSON(body)
	if err != nil {
		return nil, err
	}
	var state InitialState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("parse fanqienovel initial state: %w", err)
	}
	state.Raw = raw
	return &state, nil
}

func extractBalancedObject(input []byte) ([]byte, error) {
	depth := 0
	inString := byte(0)
	escaped := false
	for i, b := range input {
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == inString {
				inString = 0
			}
			continue
		}
		switch b {
		case '"', '\'', '`':
			inString = b
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return append([]byte(nil), input[:i+1]...), nil
			}
			if depth < 0 {
				return nil, fmt.Errorf("invalid fanqienovel initial state object")
			}
		}
	}
	return nil, fmt.Errorf("unterminated fanqienovel initial state object")
}

func normalizeJSObjectLiteral(input []byte) []byte {
	out := make([]byte, 0, len(input))
	inString := byte(0)
	escaped := false
	for i := 0; i < len(input); i++ {
		b := input[i]
		if inString != 0 {
			out = append(out, b)
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == inString {
				inString = 0
			}
			continue
		}
		switch b {
		case '"', '\'', '`':
			inString = b
			out = append(out, b)
		case ',':
			next := nextNonSpace(input, i+1)
			if next == '}' || next == ']' {
				continue
			}
			out = append(out, b)
		default:
			if bytes.HasPrefix(input[i:], []byte("undefined")) && isIdentifierBoundary(input, i-1) && isIdentifierBoundary(input, i+len("undefined")) {
				out = append(out, "null"...)
				i += len("undefined") - 1
				continue
			}
			out = append(out, b)
		}
	}
	return []byte(strings.TrimSpace(string(out)))
}

func nextNonSpace(input []byte, start int) byte {
	for i := start; i < len(input); i++ {
		switch input[i] {
		case ' ', '\n', '\r', '\t':
			continue
		default:
			return input[i]
		}
	}
	return 0
}

func isIdentifierBoundary(input []byte, idx int) bool {
	if idx < 0 || idx >= len(input) {
		return true
	}
	b := input[idx]
	return !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '$')
}
