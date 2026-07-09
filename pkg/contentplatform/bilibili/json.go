package bilibili

import (
	"encoding/json"
	"io"
)

func decodeJSONWithRaw(r io.Reader, out any) (json.RawMessage, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return raw, err
	}
	return json.RawMessage(raw), nil
}
