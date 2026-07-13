package executor

import (
	"bytes"
	"fmt"
	"mime"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var metaCharsetPattern = regexp.MustCompile(`(?is)<meta\s+[^>]*(?:charset\s*=\s*["']?\s*([^\s"'/>;]+)|content\s*=\s*["'][^"']*charset\s*=\s*([^\s"'/>;]+))`)

func decodeHTML(body []byte, contentType string) (string, error) {
	return decodeHTMLWithFallback(body, contentType, nil)
}

func decodeHTMLWithFallback(body []byte, contentType string, fallback CharsetDecoder) (string, error) {
	charset := charsetFromContentType(contentType)
	if charset == "" {
		charset = sniffMetaCharset(body)
	}
	if len(body) >= 3 && bytes.Equal(body[:3], []byte{0xef, 0xbb, 0xbf}) {
		body = body[3:]
		charset = "utf-8"
	} else if len(body) >= 2 && bytes.Equal(body[:2], []byte{0xff, 0xfe}) {
		body = body[2:]
		charset = "utf-16le"
	} else if len(body) >= 2 && bytes.Equal(body[:2], []byte{0xfe, 0xff}) {
		body = body[2:]
		charset = "utf-16be"
	}
	if charset == "" {
		charset = "utf-8"
	}
	switch normalizeCharset(charset) {
	case "utf-8", "us-ascii":
		if !utf8.Valid(body) {
			return "", &ExecutionError{Code: "E_HTML_DECODE", Message: "response is not valid UTF-8"}
		}
		return string(body), nil
	case "iso-8859-1":
		runes := make([]rune, len(body))
		for index, value := range body {
			runes[index] = rune(value)
		}
		return string(runes), nil
	case "windows-1252":
		return decodeWindows1252(body), nil
	case "utf-16le":
		return decodeUTF16(body, true)
	case "utf-16be":
		return decodeUTF16(body, false)
	default:
		if fallback != nil {
			decoded, err := fallback(body, charset)
			if err != nil {
				return "", &ExecutionError{Code: "E_HTML_DECODE", Message: fmt.Sprintf("decode charset %q: %v", charset, err), Cause: err}
			}
			return decoded, nil
		}
		return "", &ExecutionError{Code: "E_HTML_CHARSET_UNSUPPORTED", Message: fmt.Sprintf("unsupported HTML charset %q; configure Options.CharsetDecoder", charset)}
	}
}

func charsetFromContentType(contentType string) string {
	_, parameters, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return parameters["charset"]
}

func sniffMetaCharset(body []byte) string {
	limit := len(body)
	if limit > 4096 {
		limit = 4096
	}
	match := metaCharsetPattern.FindSubmatch(body[:limit])
	if len(match) == 0 {
		return ""
	}
	for _, candidate := range match[1:] {
		if len(candidate) > 0 {
			return string(candidate)
		}
	}
	return ""
}

func normalizeCharset(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "utf8":
		return "utf-8"
	case "ascii":
		return "us-ascii"
	case "latin1", "latin-1", "iso8859-1":
		return "iso-8859-1"
	case "cp1252":
		return "windows-1252"
	case "utf16", "utf_16":
		return "utf-16le"
	case "utf16le", "utf_16le":
		return "utf-16le"
	case "utf16be", "utf_16be":
		return "utf-16be"
	default:
		return value
	}
}

func decodeUTF16(body []byte, littleEndian bool) (string, error) {
	if len(body)%2 != 0 {
		return "", &ExecutionError{Code: "E_HTML_DECODE", Message: "UTF-16 body has odd byte length"}
	}
	units := make([]uint16, len(body)/2)
	for index := range units {
		if littleEndian {
			units[index] = uint16(body[index*2]) | uint16(body[index*2+1])<<8
		} else {
			units[index] = uint16(body[index*2])<<8 | uint16(body[index*2+1])
		}
	}
	return string(utf16.Decode(units)), nil
}

var windows1252 = [32]rune{
	0x20ac, 0x0081, 0x201a, 0x0192, 0x201e, 0x2026, 0x2020, 0x2021,
	0x02c6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008d, 0x017d, 0x008f,
	0x0090, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014,
	0x02dc, 0x2122, 0x0161, 0x203a, 0x0153, 0x009d, 0x017e, 0x0178,
}

func decodeWindows1252(body []byte) string {
	runes := make([]rune, len(body))
	for index, value := range body {
		if value >= 0x80 && value <= 0x9f {
			runes[index] = windows1252[value-0x80]
		} else {
			runes[index] = rune(value)
		}
	}
	return string(runes)
}
