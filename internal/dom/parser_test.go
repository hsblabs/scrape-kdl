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

func TestParseHTMLRawTextClosingTagBoundaries(t *testing.T) {
	document := mustParseHTML(t, `<script>before</scriptx><b>inside</b></ScRiPt ><p>after</p>`)
	scripts := QueryAll(document, mustSelector(t, "script"))
	if len(scripts) != 1 || scripts[0].TextContent() != `before</scriptx><b>inside</b>` {
		t.Fatalf("script = %#v", scripts)
	}
	if nested := QueryAll(document, mustSelector(t, "script b")); len(nested) != 0 {
		t.Fatalf("raw-text markup produced elements: %#v", nested)
	}
	paragraphs := QueryAll(document, mustSelector(t, "p"))
	if len(paragraphs) != 1 || paragraphs[0].TextContent() != "after" {
		t.Fatalf("paragraphs = %#v", paragraphs)
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
	want := "<script>\xd6\xd6\xd6\xd6\xd6</sCript"
	if got := protectRawText(source); got != want || len(got) != len(source) {
		t.Fatalf("protectRawText = %q (%d bytes), want %q (%d bytes)", got, len(got), want, len(source))
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

func TestParseHTMLOptionalTableEndTags(t *testing.T) {
	document := mustParseHTML(t, `<table><thead><tr><th>H<tbody><tr><td>A<td>B<tfoot><tr><td>F</table>`)
	assertText := func(selector string, want ...string) {
		t.Helper()
		nodes := QueryAll(document, mustSelector(t, selector))
		got := make([]string, 0, len(nodes))
		for _, node := range nodes {
			got = append(got, node.TextContent())
		}
		if len(got) != len(want) {
			t.Fatalf("%s text = %v, want %v", selector, got, want)
		}
		for index := range got {
			if got[index] != want[index] {
				t.Fatalf("%s text = %v, want %v", selector, got, want)
			}
		}
	}
	assertText("table > thead > tr > th", "H")
	assertText("table > tbody > tr > td", "A", "B")
	assertText("table > tfoot > tr > td", "F")
}

func TestParseHTMLOptionalEndTagFamilies(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		selector string
		want     []string
	}{
		{
			name: "description list", html: `<dl><dt>Term A<dd>Definition A<dt>Term B<dd>Definition B</dl>`,
			selector: "dl > dt, dl > dd", want: []string{"Term A", "Definition A", "Term B", "Definition B"},
		},
		{
			name: "ruby annotations", html: `<ruby>base<rp>(<rt>reading<rp>)</ruby>`,
			selector: "ruby > rp, ruby > rt", want: []string{"(", "reading", ")"},
		},
		{
			name: "select options", html: `<select><optgroup label="a"><option>A<option>B<optgroup label="b"><option>C</select>`,
			selector: "select > optgroup > option", want: []string{"A", "B", "C"},
		},
		{
			name: "paragraph block", html: `<main><p>paragraph<div>block</div></main>`,
			selector: "main > p, main > div", want: []string{"paragraph", "block"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := mustParseHTML(t, tt.html)
			nodes := QueryAll(document, mustSelector(t, tt.selector))
			got := make([]string, 0, len(nodes))
			for _, node := range nodes {
				got = append(got, node.TextContent())
			}
			if len(got) != len(tt.want) {
				t.Fatalf("text = %v, want %v", got, tt.want)
			}
			for index := range got {
				if got[index] != tt.want[index] {
					t.Fatalf("text = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
