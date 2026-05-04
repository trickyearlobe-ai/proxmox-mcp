package install

import (
	"strings"
)

// StripJSONC removes comments and trailing commas from JSONC content.
// Handles line comments (//), block comments (/* */), and trailing commas.
// Preserves content inside quoted strings correctly.
func StripJSONC(input string) string {
	var out strings.Builder
	out.Grow(len(input))

	i := 0
	n := len(input)

	for i < n {
		ch := input[i]

		// Inside a string literal — copy verbatim until closing quote
		if ch == '"' {
			out.WriteByte(ch)
			i++
			for i < n {
				sch := input[i]
				out.WriteByte(sch)
				i++
				if sch == '\\' && i < n {
					out.WriteByte(input[i])
					i++
				} else if sch == '"' {
					break
				}
			}
			continue
		}

		// Line comment: // → skip to end of line
		if ch == '/' && i+1 < n && input[i+1] == '/' {
			i += 2
			for i < n && input[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment: /* → skip to closing */
		if ch == '/' && i+1 < n && input[i+1] == '*' {
			i += 2
			for i+1 < n {
				if input[i] == '*' && input[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		out.WriteByte(ch)
		i++
	}

	// Remove trailing commas before } or ]
	result := out.String()
	result = removeTrailingCommas(result)
	return result
}

func removeTrailingCommas(s string) string {
	var out strings.Builder
	out.Grow(len(s))

	runes := []rune(s)
	n := len(runes)

	for i := 0; i < n; i++ {
		ch := runes[i]

		if ch == '"' {
			out.WriteRune(ch)
			i++
			for i < n {
				out.WriteRune(runes[i])
				if runes[i] == '\\' && i+1 < n {
					i++
					out.WriteRune(runes[i])
				} else if runes[i] == '"' {
					break
				}
				i++
			}
			continue
		}

		if ch == ',' {
			// Look ahead past whitespace for } or ]
			j := i + 1
			for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
				j++
			}
			if j < n && (runes[j] == '}' || runes[j] == ']') {
				// Skip the trailing comma
				continue
			}
		}

		out.WriteRune(ch)
	}

	return out.String()
}

// SplitPreamble separates leading non-JSON content (comments before {) from the JSON body.
func SplitPreamble(content string) (preamble string, jsonBody string) {
	idx := strings.Index(content, "{")
	if idx < 0 {
		return content, "{}"
	}
	return content[:idx], content[idx:]
}
