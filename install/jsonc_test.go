package install

import (
	"testing"
)

func TestStripJSONC_LineComments(t *testing.T) {
	input := `{
  // this is a comment
  "key": "value" // inline comment
}`
	want := `{
  
  "key": "value" 
}`
	got := StripJSONC(input)
	if got != want {
		t.Errorf("StripJSONC line comments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_BlockComments(t *testing.T) {
	input := `{
  /* block comment */
  "key": "value"
}`
	want := `{
  
  "key": "value"
}`
	got := StripJSONC(input)
	if got != want {
		t.Errorf("StripJSONC block comments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_MultilineBlockComment(t *testing.T) {
	input := `{
  /* this
     spans
     lines */
  "key": "value"
}`
	want := `{
  
  "key": "value"
}`
	got := StripJSONC(input)
	if got != want {
		t.Errorf("StripJSONC multiline block:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_TrailingCommaObject(t *testing.T) {
	input := `{"a": 1, "b": 2,}`
	want := `{"a": 1, "b": 2}`
	got := StripJSONC(input)
	if got != want {
		t.Errorf("trailing comma object:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_TrailingCommaArray(t *testing.T) {
	input := `[1, 2, 3,]`
	want := `[1, 2, 3]`
	got := StripJSONC(input)
	if got != want {
		t.Errorf("trailing comma array:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_CommentsInsideStrings(t *testing.T) {
	input := `{"url": "http://example.com", "desc": "use // for comments"}`
	want := input // no change — comments inside strings are preserved
	got := StripJSONC(input)
	if got != want {
		t.Errorf("comments inside strings should be preserved:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_BlockCommentInsideString(t *testing.T) {
	input := `{"note": "/* not a comment */"}`
	want := input
	got := StripJSONC(input)
	if got != want {
		t.Errorf("block comment inside string should be preserved:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_EscapedQuotes(t *testing.T) {
	input := `{"key": "value with \" escaped // quote"}`
	want := input
	got := StripJSONC(input)
	if got != want {
		t.Errorf("escaped quotes:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripJSONC_CommentAndTrailingComma(t *testing.T) {
	input := `{
  "a": 1, // comment
  "b": 2, // another comment
}`
	got := StripJSONC(input)
	// Should be valid JSON after stripping
	if got[len(got)-1] != '}' {
		t.Errorf("result should end with }, got: %q", got)
	}
	// Should not contain //
	for i := 0; i < len(got)-1; i++ {
		if got[i] == '/' && got[i+1] == '/' {
			// Check we're not inside a string
			t.Errorf("found // outside string at position %d in: %q", i, got)
			break
		}
	}
}

func TestStripJSONC_Empty(t *testing.T) {
	if got := StripJSONC(""); got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestStripJSONC_NoComments(t *testing.T) {
	input := `{"key": "value"}`
	if got := StripJSONC(input); got != input {
		t.Errorf("no-op case changed: got %q", got)
	}
}

func TestStripJSONC_ZedStyle(t *testing.T) {
	input := `// Zed settings
// See https://zed.dev/docs/configuring-zed
{
  "theme": "One Dark",
  "context_servers": {
    "my-server": {
      "command": "/usr/local/bin/my-server",
      "args": [],
      "env": {},
    },
  },
}`
	got := StripJSONC(input)
	// Should be parseable as JSON (after preamble stripping)
	_, jsonBody := SplitPreamble(got)
	if jsonBody[0] != '{' {
		t.Errorf("json body should start with {, got: %q", jsonBody[:20])
	}
}

func TestSplitPreamble_WithComments(t *testing.T) {
	input := `// Zed settings
// More comments
{
  "key": "value"
}`
	preamble, body := SplitPreamble(input)
	if preamble != "// Zed settings\n// More comments\n" {
		t.Errorf("preamble = %q", preamble)
	}
	if body[0] != '{' {
		t.Errorf("body should start with {, got %q", body[:10])
	}
}

func TestSplitPreamble_NoPreamble(t *testing.T) {
	input := `{"key": "value"}`
	preamble, body := SplitPreamble(input)
	if preamble != "" {
		t.Errorf("preamble should be empty, got %q", preamble)
	}
	if body != input {
		t.Errorf("body = %q", body)
	}
}

func TestSplitPreamble_NoBrace(t *testing.T) {
	input := "no json here"
	preamble, body := SplitPreamble(input)
	if preamble != input {
		t.Errorf("preamble should be entire input, got %q", preamble)
	}
	if body != "{}" {
		t.Errorf("body should be {}, got %q", body)
	}
}
