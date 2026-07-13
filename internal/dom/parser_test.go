package dom

import (
	"strings"
	"testing"
)

func mustParseHTML(t *testing.T, source string) *Node {
	t.Helper()
	document, err := ParseHTML(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func mustSelector(t *testing.T, source string) Selector {
	t.Helper()
	selector, err := ParseSelector(source)
	if err != nil {
		t.Fatal(err)
	}
	return selector
}

func TestParseHTMLRawText(t *testing.T) {
	document := mustParseHTML(t, `<script>if (a < b && c > d) { x("&copy;") }</script><style>.x > .y {}</style><h1>ok</h1>`)
	scripts := QueryAll(document, mustSelector(t, "script"))
	if len(scripts) != 1 || scripts[0].TextContent() != `if (a < b && c > d) { x("&copy;") }` {
		t.Fatalf("script = %#v", scripts)
	}
	styles := QueryAll(document, mustSelector(t, "style"))
	if len(styles) != 1 || styles[0].TextContent() != `.x > .y {}` {
		t.Fatalf("style = %#v", styles)
	}
	if got := document.InnerHTML(); !strings.Contains(got, `<script>if (a < b && c > d) { x("&copy;") }</script>`) {
		t.Fatalf("serialized raw text = %q", got)
	}
}

func TestParseHTMLRCDATA(t *testing.T) {
	document := mustParseHTML(t, `<title>A &amp; B < C</title><textarea>x &lt; y</textarea>`)
	titles := QueryAll(document, mustSelector(t, "title"))
	if len(titles) != 1 || titles[0].TextContent() != `A & B < C` {
		t.Fatalf("title = %#v", titles)
	}
	areas := QueryAll(document, mustSelector(t, "textarea"))
	if len(areas) != 1 || areas[0].TextContent() != `x < y` {
		t.Fatalf("textarea = %#v", areas)
	}
}

func TestProtectRawTextPreservesInvalidUTF8Offsets(t *testing.T) {
	source := "<sCript>\xd6\xd6\xd6\xd6\xd6</sCript"
	if got := protectRawText(source); got != source {
		t.Fatalf("protectRawText = %q, want %q", got, source)
	}
	if got := asciiLower("A\xd6Zé"); got != "a\xd6zé" || len(got) != len("A\xd6Zé") {
		t.Fatalf("asciiLower = %q (%d bytes)", got, len(got))
	}
	_, _ = ParseHTML(strings.NewReader(source))
}

func TestParseHTMLRecoversTruncatedDocument(t *testing.T) {
	document := mustParseHTML(t, `<main><input disabled><p>first<p>second<ul><li>a<li>b`)
	inputs := QueryAll(document, mustSelector(t, "input[disabled]"))
	if len(inputs) != 1 {
		t.Fatalf("inputs = %d", len(inputs))
	}
	paragraphs := QueryAll(document, mustSelector(t, "main > p"))
	if len(paragraphs) != 2 || paragraphs[0].TextContent() != "first" || paragraphs[1].TextContent() != "second" {
		t.Fatalf("paragraphs = %#v", paragraphs)
	}
	items := QueryAll(document, mustSelector(t, "ul > li"))
	if len(items) != 2 || items[0].TextContent() != "a" || items[1].TextContent() != "b" {
		t.Fatalf("items = %#v", items)
	}
}
