package executor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/japanese"
)

func TestDecodeHTMLSniffsMetaCharset(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "charset attribute",
			body: append([]byte(`<meta charset="windows-1252"><p>`), 0x80),
		},
		{
			name: "content attribute",
			body: append([]byte(`<meta http-equiv="content-type" content="text/html; charset=cp1252"><p>`), 0x80),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeHTML(tt.body, "")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(decoded, "<p>€") {
				t.Fatalf("decoded = %q", decoded)
			}
		})
	}
}

func TestDecodeHTMLMetaSniffIsBounded(t *testing.T) {
	body := append([]byte(strings.Repeat(" ", 4096)+`<meta charset="windows-1252">`), 0x80)
	_, err := decodeHTML(body, "")
	assertExecutionErrorCode(t, err, "E_HTML_DECODE")
}

func TestDecodeHTMLUTF16(t *testing.T) {
	const input = "A😀"
	for _, tt := range []struct {
		name         string
		contentType  string
		littleEndian bool
	}{
		{name: "little endian", contentType: "text/html; charset=utf-16le", littleEndian: true},
		{name: "big endian", contentType: "text/html; charset=utf-16be", littleEndian: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeHTML(encodeUTF16(input, tt.littleEndian), tt.contentType)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != input {
				t.Fatalf("decoded = %q, want %q", decoded, input)
			}
		})
	}
}

func TestDecodeHTMLBOMOverridesDeclaredCharset(t *testing.T) {
	body := append([]byte{0xff, 0xfe}, encodeUTF16("BOM wins", true)...)
	decoded, err := decodeHTML(body, "text/html; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "BOM wins" {
		t.Fatalf("decoded = %q", decoded)
	}
}

func TestDecodeHTMLASCIIRejectsNonASCII(t *testing.T) {
	decoded, err := decodeHTML([]byte("ASCII\n"), "text/html; charset=us-ascii")
	if err != nil || decoded != "ASCII\n" {
		t.Fatalf("ASCII decode = %q, %v", decoded, err)
	}
	_, err = decodeHTML([]byte("é"), "text/html; charset=ascii")
	assertExecutionErrorCode(t, err, "E_HTML_DECODE")
}

func TestDecodeHTMLIgnoresInvalidHTTPMetadataBeforeMeta(t *testing.T) {
	body := append([]byte(`<meta charset="windows-1252"><p>`), 0x80)
	decoded, err := decodeHTML(body, "text/html; charset")
	if err != nil || !strings.HasSuffix(decoded, "<p>€") {
		t.Fatalf("decoded = %q, error = %v", decoded, err)
	}
}

func TestDecodeHTMLFallbackReceivesDeclaredLabelAndBytes(t *testing.T) {
	body := []byte{0x81, 0x82}
	called := 0
	decoded, err := decodeHTMLWithFallback(body, `text/html; charset="X-Custom"`, func(gotBody []byte, charset string) (string, error) {
		called++
		if !bytes.Equal(gotBody, body) || charset != "X-Custom" {
			t.Fatalf("fallback input = %v, %q", gotBody, charset)
		}
		return "decoded", nil
	})
	if err != nil || decoded != "decoded" || called != 1 {
		t.Fatalf("fallback result = %q, error = %v, calls = %d", decoded, err, called)
	}
}

func TestNormalizeCharsetAliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: " UTF8 ", want: "utf-8"},
		{input: "ASCII", want: "us-ascii"},
		{input: "latin1", want: "iso-8859-1"},
		{input: "latin-1", want: "iso-8859-1"},
		{input: "iso8859-1", want: "iso-8859-1"},
		{input: "CP1252", want: "windows-1252"},
		{input: "utf16", want: "utf-16le"},
		{input: "utf_16le", want: "utf-16le"},
		{input: "UTF16BE", want: "utf-16be"},
		{input: "x-custom", want: "x-custom"},
	}
	for _, tt := range tests {
		if got := normalizeCharset(tt.input); got != tt.want {
			t.Fatalf("normalizeCharset(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDecodeHTMLReportsStableFailures(t *testing.T) {
	t.Run("invalid UTF-8", func(t *testing.T) {
		_, err := decodeHTML([]byte{0xff}, "text/html; charset=utf-8")
		assertExecutionErrorCode(t, err, "E_HTML_DECODE")
	})
	t.Run("odd UTF-16 length", func(t *testing.T) {
		_, err := decodeHTML([]byte{0x41}, "text/html; charset=utf-16le")
		assertExecutionErrorCode(t, err, "E_HTML_DECODE")
	})
	t.Run("unsupported charset", func(t *testing.T) {
		_, err := decodeHTML([]byte("text"), "text/html; charset=x-nonexistent")
		assertExecutionErrorCode(t, err, "E_HTML_CHARSET_UNSUPPORTED")
	})
	t.Run("replacement encoding label", func(t *testing.T) {
		_, err := decodeHTML([]byte("text"), "text/html; charset=iso-2022-kr")
		assertExecutionErrorCode(t, err, "E_HTML_CHARSET_UNSUPPORTED")
	})
	t.Run("fallback failure", func(t *testing.T) {
		wantErr := errors.New("decoder failed")
		_, err := decodeHTMLWithFallback([]byte("text"), "text/html; charset=shift_jis", func([]byte, string) (string, error) {
			return "", wantErr
		})
		assertExecutionErrorCode(t, err, "E_HTML_DECODE")
		if !errors.Is(err, wantErr) {
			t.Fatalf("error %v does not wrap fallback error", err)
		}
		var execution *ExecutionError
		if !errors.As(err, &execution) || execution.Cause != wantErr || !strings.Contains(execution.Message, `charset "shift_jis"`) {
			t.Fatalf("fallback execution error = %#v", err)
		}
	})
}

func encodeUTF16(value string, littleEndian bool) []byte {
	units := utf16.Encode([]rune(value))
	encoded := make([]byte, len(units)*2)
	for index, unit := range units {
		if littleEndian {
			encoded[index*2] = byte(unit)
			encoded[index*2+1] = byte(unit >> 8)
		} else {
			encoded[index*2] = byte(unit >> 8)
			encoded[index*2+1] = byte(unit)
		}
	}
	return encoded
}

func assertExecutionErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var executionError *ExecutionError
	if !errors.As(err, &executionError) || executionError.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func TestDecodeHTMLWHATWGFallbackEncodings(t *testing.T) {
	eucJP, err := japanese.EUCJP.NewEncoder().String("<p>日本語</p>")
	if err != nil {
		t.Fatal(err)
	}
	shiftJIS, err := japanese.ShiftJIS.NewEncoder().String("<meta charset=\"shift_jis\"><p>競馬</p>")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("EUC-JP from Content-Type", func(t *testing.T) {
		decoded, err := decodeHTML([]byte(eucJP), "text/html; charset=euc-jp")
		if err != nil || decoded != "<p>日本語</p>" {
			t.Fatalf("decoded = %q, error = %v", decoded, err)
		}
	})
	t.Run("Shift_JIS from meta sniff", func(t *testing.T) {
		decoded, err := decodeHTML([]byte(shiftJIS), "text/html")
		if err != nil || !strings.HasSuffix(decoded, "<p>競馬</p>") {
			t.Fatalf("decoded = %q, error = %v", decoded, err)
		}
	})
	t.Run("explicit CharsetDecoder still overrides", func(t *testing.T) {
		decoded, err := decodeHTMLWithFallback([]byte(eucJP), "text/html; charset=euc-jp", func([]byte, string) (string, error) {
			return "override", nil
		})
		if err != nil || decoded != "override" {
			t.Fatalf("decoded = %q, error = %v", decoded, err)
		}
	})
	t.Run("invalid bytes decode to replacement runes", func(t *testing.T) {
		decoded, err := decodeHTML([]byte{0x41, 0xff, 0xff, 0x42}, "text/html; charset=euc-jp")
		if err != nil || !strings.Contains(decoded, "�") || !strings.Contains(decoded, "B") {
			t.Fatalf("decoded = %q, error = %v", decoded, err)
		}
	})
}
