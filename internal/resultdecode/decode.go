package resultdecode

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strings"
)

// Decode converts a completed extraction value into a fresh destination value.
// The destination is assigned only after the complete conversion succeeds.
func Decode(source map[string]any, destination any) error {
	if destination == nil {
		return fmt.Errorf("destination must be a non-nil pointer to a struct or map")
	}
	target := reflect.ValueOf(destination)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer to a struct or map")
	}
	if target.Elem().Kind() != reflect.Struct && target.Elem().Kind() != reflect.Map {
		return fmt.Errorf("destination must point to a struct or map, got %s", target.Elem().Type())
	}
	decoded := reflect.New(target.Elem().Type()).Elem()
	if err := decodeValue(source, decoded, "value"); err != nil {
		return err
	}
	target.Elem().Set(decoded)
	return nil
}

func decodeValue(source any, destination reflect.Value, path string) error {
	if source == nil {
		if nullable(destination.Type()) {
			destination.SetZero()
			return nil
		}
		return decodeError(path, "null cannot be decoded into %s", destination.Type())
	}

	if destination.Kind() == reflect.Pointer {
		value := reflect.New(destination.Type().Elem())
		if err := decodeValue(source, value.Elem(), path); err != nil {
			return err
		}
		destination.Set(value)
		return nil
	}

	switch destination.Kind() {
	case reflect.Struct:
		return decodeStruct(source, destination, path)
	case reflect.Map:
		return decodeMap(source, destination, path)
	case reflect.Slice:
		return decodeSlice(source, destination, path)
	case reflect.Array:
		return decodeArray(source, destination, path)
	case reflect.Interface:
		if destination.Type().NumMethod() != 0 {
			return decodeError(path, "destination interface %s has methods and is not supported", destination.Type())
		}
		value, err := cloneJSONValue(source, path)
		if err != nil {
			return err
		}
		destination.Set(reflect.ValueOf(value))
		return nil
	case reflect.String:
		value, ok := source.(string)
		if !ok {
			return typeError(path, source, destination.Type())
		}
		destination.SetString(value)
		return nil
	case reflect.Bool:
		value, ok := source.(bool)
		if !ok {
			return typeError(path, source, destination.Type())
		}
		destination.SetBool(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return decodeSignedInteger(source, destination, path)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return decodeUnsignedInteger(source, destination, path)
	case reflect.Float32, reflect.Float64:
		return decodeFloat(source, destination, path)
	default:
		return decodeError(path, "destination type %s is not supported", destination.Type())
	}
}

func decodeStruct(source any, destination reflect.Value, path string) error {
	object, ok := source.(map[string]any)
	if !ok {
		return typeError(path, source, destination.Type())
	}
	fields, err := destinationFields(destination.Type(), path)
	if err != nil {
		return err
	}
	for _, field := range fields {
		value, exists := object[field.name]
		if !exists {
			if nullable(field.field.Type) {
				continue
			}
			return decodeError(fieldPath(path, field.name), "required destination field is missing")
		}
		if err := decodeValue(value, destination.Field(field.index), fieldPath(path, field.name)); err != nil {
			return err
		}
	}
	known := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		known[field.name] = struct{}{}
	}
	unknown := make([]string, 0)
	for name := range object {
		if _, ok := known[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return decodeError(fieldPath(path, unknown[0]), "source field has no destination field")
	}
	return nil
}

type destinationField struct {
	name  string
	index int
	field reflect.StructField
}

func destinationFields(destination reflect.Type, path string) ([]destinationField, error) {
	fields := make([]destinationField, 0, destination.NumField())
	seen := map[string]struct{}{}
	for index := 0; index < destination.NumField(); index++ {
		field := destination.Field(index)
		if !field.IsExported() {
			continue
		}
		name, skip := fieldName(field)
		if skip {
			continue
		}
		if _, exists := seen[name]; exists {
			return nil, decodeError(path, "destination has duplicate field name %q", name)
		}
		seen[name] = struct{}{}
		fields = append(fields, destinationField{name: name, index: index, field: field})
	}
	return fields, nil
}

func fieldName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		name = field.Name
	}
	return name, false
}

func decodeMap(source any, destination reflect.Value, path string) error {
	object, ok := source.(map[string]any)
	if !ok {
		return typeError(path, source, destination.Type())
	}
	if destination.Type().Key().Kind() != reflect.String {
		return decodeError(path, "destination map key must be a string type, got %s", destination.Type().Key())
	}
	names := make([]string, 0, len(object))
	for name := range object {
		names = append(names, name)
	}
	sort.Strings(names)
	result := reflect.MakeMapWithSize(destination.Type(), len(object))
	for _, name := range names {
		value := reflect.New(destination.Type().Elem()).Elem()
		if err := decodeValue(object[name], value, fieldPath(path, name)); err != nil {
			return err
		}
		key := reflect.New(destination.Type().Key()).Elem()
		key.SetString(name)
		result.SetMapIndex(key, value)
	}
	destination.Set(result)
	return nil
}

func decodeSlice(source any, destination reflect.Value, path string) error {
	items, ok := sourceSequence(source)
	if !ok {
		return typeError(path, source, destination.Type())
	}
	result := reflect.MakeSlice(destination.Type(), items.Len(), items.Len())
	for index := 0; index < items.Len(); index++ {
		if err := decodeValue(items.Index(index).Interface(), result.Index(index), indexPath(path, index)); err != nil {
			return err
		}
	}
	destination.Set(result)
	return nil
}

func decodeArray(source any, destination reflect.Value, path string) error {
	items, ok := sourceSequence(source)
	if !ok {
		return typeError(path, source, destination.Type())
	}
	if items.Len() != destination.Len() {
		return decodeError(path, "source has %d items, destination array requires %d", items.Len(), destination.Len())
	}
	for index := 0; index < items.Len(); index++ {
		if err := decodeValue(items.Index(index).Interface(), destination.Index(index), indexPath(path, index)); err != nil {
			return err
		}
	}
	return nil
}

func sourceSequence(source any) (reflect.Value, bool) {
	value := reflect.ValueOf(source)
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return reflect.Value{}, false
	}
	return value, true
}

func decodeSignedInteger(source any, destination reflect.Value, path string) error {
	integer, ok := exactInteger(source)
	if !ok || !integer.IsInt64() {
		return typeError(path, source, destination.Type())
	}
	value := integer.Int64()
	if destination.OverflowInt(value) {
		return decodeError(path, "integer %s overflows %s", integer.String(), destination.Type())
	}
	destination.SetInt(value)
	return nil
}

func decodeUnsignedInteger(source any, destination reflect.Value, path string) error {
	integer, ok := exactInteger(source)
	if !ok || !integer.IsUint64() {
		return typeError(path, source, destination.Type())
	}
	value := integer.Uint64()
	if destination.OverflowUint(value) {
		return decodeError(path, "integer %s overflows %s", integer.String(), destination.Type())
	}
	destination.SetUint(value)
	return nil
}

func exactInteger(source any) (*big.Int, bool) {
	switch value := source.(type) {
	case int:
		return big.NewInt(int64(value)), true
	case int8:
		return big.NewInt(int64(value)), true
	case int16:
		return big.NewInt(int64(value)), true
	case int32:
		return big.NewInt(int64(value)), true
	case int64:
		return big.NewInt(value), true
	case uint:
		return new(big.Int).SetUint64(uint64(value)), true
	case uint8:
		return new(big.Int).SetUint64(uint64(value)), true
	case uint16:
		return new(big.Int).SetUint64(uint64(value)), true
	case uint32:
		return new(big.Int).SetUint64(uint64(value)), true
	case uint64:
		return new(big.Int).SetUint64(value), true
	case json.Number:
		rational, ok := new(big.Rat).SetString(value.String())
		if !ok || !rational.IsInt() {
			return nil, false
		}
		return new(big.Int).Set(rational.Num()), true
	default:
		return nil, false
	}
}

func decodeFloat(source any, destination reflect.Value, path string) error {
	var value float64
	switch source := source.(type) {
	case float32:
		value = float64(source)
	case float64:
		value = source
	case json.Number:
		var err error
		value, err = source.Float64()
		if err != nil {
			return typeError(path, source, destination.Type())
		}
	default:
		return typeError(path, source, destination.Type())
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return decodeError(path, "non-finite number cannot be decoded into %s", destination.Type())
	}
	if destination.Kind() == reflect.Float32 {
		converted := float32(value)
		if math.IsInf(float64(converted), 0) || float64(converted) != value {
			return decodeError(path, "number %g cannot be represented exactly as %s", value, destination.Type())
		}
	}
	destination.SetFloat(value)
	return nil
}

func cloneJSONValue(source any, path string) (any, error) {
	switch value := source.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return value, nil
	case float32:
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, decodeError(path, "source contains a non-finite number")
		}
		return value, nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, decodeError(path, "source contains a non-finite number")
		}
		return value, nil
	case json.Number:
		parsed, err := value.Float64()
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, decodeError(path, "source contains an invalid JSON number")
		}
		return value, nil
	case []string:
		return append([]string(nil), value...), nil
	case []any:
		result := make([]any, len(value))
		for index, item := range value {
			cloned, err := cloneJSONValue(item, indexPath(path, index))
			if err != nil {
				return nil, err
			}
			result[index] = cloned
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(value))
		names := make([]string, 0, len(value))
		for name := range value {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			item := value[name]
			cloned, err := cloneJSONValue(item, fieldPath(path, name))
			if err != nil {
				return nil, err
			}
			result[name] = cloned
		}
		return result, nil
	default:
		return nil, decodeError(path, "source type %T is not JSON-compatible", source)
	}
}

func nullable(destination reflect.Type) bool {
	switch destination.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
		return true
	default:
		return false
	}
}

func typeError(path string, source any, destination reflect.Type) error {
	return decodeError(path, "source type %T cannot be decoded into %s", source, destination)
}

func decodeError(path, format string, args ...any) error {
	return fmt.Errorf("decode result at %s: %s", path, fmt.Sprintf(format, args...))
}

func fieldPath(path, name string) string {
	if name == "" {
		return path
	}
	return path + "." + name
}

func indexPath(path string, index int) string {
	return fmt.Sprintf("%s[%d]", path, index)
}
