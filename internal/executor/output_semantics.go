package executor

import (
	"fmt"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func handleMissingOutput(field ir.Field, path, message string) (any, error) {
	if field.Required {
		return nil, &ExecutionError{Code: "E_REQUIRED_VALUE_MISSING", Message: message, Path: path}
	}
	if field.Default != nil {
		return decodeFieldDefault(field, path)
	}
	return nil, nil
}

func recoverOutputField(partial *bool, warnings *[]Warning, field ir.Field, path string, cause error) (any, error) {
	policy := field.OnError
	if policy == "" {
		if field.Required {
			policy = "fail"
		} else {
			policy = "null"
		}
	}
	switch policy {
	case "fail":
		if execution, ok := cause.(*ExecutionError); ok {
			return nil, execution
		}
		return nil, &ExecutionError{Code: "E_FIELD_EXECUTION", Message: cause.Error(), Path: path, Cause: cause}
	case "null":
		*partial = true
		return nil, nil
	case "warn":
		*partial = true
		*warnings = append(*warnings, Warning{Code: "W_ERROR_RECOVERED", Message: cause.Error(), Path: path})
		return nil, nil
	case "default":
		value, err := decodeFieldDefault(field, path)
		if err != nil {
			return nil, err
		}
		*partial = true
		return value, nil
	default:
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown on-error policy %q", policy), Path: path}
	}
}

func validateCollectionMinimum(required bool, minItems, count int, path string) error {
	minimum := minItems
	if required && minimum < 1 {
		minimum = 1
	}
	if count < minimum {
		return &ExecutionError{Code: "E_COLLECTION_CARDINALITY", Message: fmt.Sprintf("collection has %d rows after recovery; minimum is %d", count, minimum), Path: path}
	}
	return nil
}

func validateCollectionMaximum(maxItems *int, count int, path string) error {
	if maxItems != nil && count > *maxItems {
		return &ExecutionError{Code: "E_COLLECTION_CARDINALITY", Message: fmt.Sprintf("collection has %d rows; maximum is %d", count, *maxItems), Path: path}
	}
	return nil
}
