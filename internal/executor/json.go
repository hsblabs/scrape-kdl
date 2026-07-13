package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func decodeJSON(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode IR literal: %w", err)
	}
	return value, nil
}

func namedArgument(arguments map[string]json.RawMessage, name string) (any, bool, error) {
	raw, ok := arguments[name]
	if !ok {
		return nil, false, nil
	}
	value, err := decodeJSON(raw)
	return value, true, err
}
