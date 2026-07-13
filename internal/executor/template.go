package executor

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/ir"
)

func expandTemplate(template ir.Template, inputs map[string]any) (string, error) {
	var builder strings.Builder
	for _, segment := range template.Segments {
		switch typed := segment.(type) {
		case ir.LiteralTemplateSegment:
			builder.WriteString(typed.Value)
		case ir.InputTemplateSegment:
			value, ok := inputs[typed.Name]
			if !ok {
				return "", &ExecutionError{Code: "E_INPUT_REQUIRED", Message: fmt.Sprintf("URL template input %q is missing", typed.Name), Path: "input." + typed.Name}
			}
			text, err := inputString(value)
			if err != nil {
				return "", &ExecutionError{Code: "E_INPUT_TYPE", Message: err.Error(), Path: "input." + typed.Name, Cause: err}
			}
			builder.WriteString(percentEncode(text))
		default:
			return "", &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown URL template segment %T", segment)}
		}
	}
	expanded := builder.String()
	parsed, err := url.Parse(expanded)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", &ExecutionError{Code: "E_URL_INVALID", Message: fmt.Sprintf("expanded URL is not an absolute HTTP(S) URL: %q", expanded), Cause: err}
	}
	return expanded, nil
}

func percentEncode(value string) string {
	const hexadecimal = "0123456789ABCDEF"
	var builder strings.Builder
	for _, b := range []byte(value) {
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '.' || b == '_' || b == '~' {
			builder.WriteByte(b)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hexadecimal[b>>4])
		builder.WriteByte(hexadecimal[b&15])
	}
	return builder.String()
}
