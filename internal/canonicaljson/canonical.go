package canonicaljson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Canonicalize(input []byte) ([]byte, error) {
	if !utf8.Valid(input) {
		return nil, fmt.Errorf("canonical JSON requires valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("canonical JSON has trailing token %v", token)
		}
		return nil, fmt.Errorf("canonical JSON trailing data: %w", err)
	}
	return Marshal(value)
}

func Marshal(value any) ([]byte, error) {
	var output bytes.Buffer
	if err := appendValue(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode canonical JSON: %w", err)
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("decode canonical JSON object key: %w", err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("canonical JSON object key is %T", keyToken)
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("canonical JSON object contains duplicate key %q", key)
			}
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
			return nil, fmt.Errorf("decode canonical JSON object close: %v, %w", closing, err)
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if closing, err := decoder.Token(); err != nil || closing != json.Delim(']') {
			return nil, fmt.Errorf("decode canonical JSON array close: %v, %w", closing, err)
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected canonical JSON delimiter %q", delimiter)
	}
}

func appendValue(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		if typed {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case string:
		if err := appendString(output, typed); err != nil {
			return err
		}
	case json.Number:
		number, err := canonicalNumber(typed.String())
		if err != nil {
			return err
		}
		output.WriteString(number)
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return fmt.Errorf("canonical JSON rejects non-finite number %v", typed)
		}
		number, err := canonicalNumber(strconv.FormatFloat(typed, 'g', -1, 64))
		if err != nil {
			return err
		}
		output.WriteString(number)
	case []any:
		output.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := appendValue(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := appendString(output, key); err != nil {
				return err
			}
			output.WriteByte(':')
			if err := appendValue(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Errorf("marshal canonical JSON value %T: %w", value, err)
		}
		canonical, err := Canonicalize(encoded)
		if err != nil {
			return err
		}
		output.Write(canonical)
	}
	return nil
}

func appendString(output *bytes.Buffer, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("canonical JSON string requires valid UTF-8")
	}
	const hexadecimal = "0123456789abcdef"
	output.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteRune(character)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character < 0x20 {
				output.WriteString(`\u00`)
				output.WriteByte(hexadecimal[character>>4])
				output.WriteByte(hexadecimal[character&0xf])
				continue
			}
			output.WriteRune(character)
		}
	}
	output.WriteByte('"')
	return nil
}

func canonicalNumber(raw string) (string, error) {
	if !strings.ContainsAny(raw, ".eE") {
		integer := new(big.Int)
		if _, ok := integer.SetString(raw, 10); !ok {
			return "", fmt.Errorf("invalid canonical JSON integer %q", raw)
		}
		return integer.String(), nil
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return "", fmt.Errorf("canonical JSON number %q is outside finite binary64", raw)
	}
	if parsed == 0 {
		return "0", nil
	}

	sign := ""
	unsigned := raw
	if strings.HasPrefix(unsigned, "-") {
		sign = "-"
		unsigned = unsigned[1:]
	}
	mantissa, exponentText := unsigned, ""
	if index := strings.IndexAny(unsigned, "eE"); index >= 0 {
		mantissa, exponentText = unsigned[:index], unsigned[index+1:]
	}
	exponent := 0
	if exponentText != "" {
		parsedExponent, err := strconv.ParseInt(exponentText, 10, 32)
		if err != nil {
			return "", fmt.Errorf("invalid canonical JSON exponent %q", exponentText)
		}
		exponent = int(parsedExponent)
	}
	fractionDigits := 0
	if index := strings.IndexByte(mantissa, '.'); index >= 0 {
		fractionDigits = len(mantissa) - index - 1
		mantissa = mantissa[:index] + mantissa[index+1:]
	}
	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return "0", nil
	}
	exponent -= fractionDigits
	for strings.HasSuffix(digits, "0") {
		digits = strings.TrimSuffix(digits, "0")
		exponent++
	}
	if exponent >= 0 {
		return sign + digits + strings.Repeat("0", exponent), nil
	}
	decimalAt := len(digits) + exponent
	if decimalAt > 0 {
		return sign + digits[:decimalAt] + "." + digits[decimalAt:], nil
	}
	return sign + "0." + strings.Repeat("0", -decimalAt) + digits, nil
}
