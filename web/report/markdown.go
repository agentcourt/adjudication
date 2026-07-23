package report

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
)

// RenderMarkdown converts a limited markdown subset to HTML: ATX
// headings, paragraphs, unordered and ordered lists, blockquotes,
// fenced code blocks, horizontal rules, inline code, bold, single-star
// emphasis, and bracket links with http, https, mailto, or relative
// targets.  All input text is HTML-escaped, and unhandled constructs
// pass through as literal text.  Run artifacts contain markdown written
// by models, so faithful literal fallback beats guessing.
func RenderMarkdown(src []byte) template.HTML {
	// NUL bytes would collide with the placeholder scheme in inline().
	text := strings.NewReplacer("\r\n", "\n", "\x00", "").Replace(string(src))
	lines := strings.Split(text, "\n")
	var b strings.Builder
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			i++
		case strings.HasPrefix(trimmed, "```"):
			i = renderFence(&b, lines, i)
		case headingLevel(trimmed) > 0:
			level := headingLevel(trimmed)
			text := strings.TrimSpace(trimmed[level:])
			fmt.Fprintf(&b, "<h%d>%s</h%d>\n", level, inline(text), level)
			i++
		case isRule(trimmed):
			b.WriteString("<hr>\n")
			i++
		case bulletText(trimmed) != "":
			i = renderList(&b, lines, i, "ul", bulletText)
		case orderedText(trimmed) != "":
			i = renderList(&b, lines, i, "ol", orderedText)
		case strings.HasPrefix(trimmed, ">"):
			i = renderQuote(&b, lines, i)
		default:
			i = renderParagraph(&b, lines, i)
		}
	}
	return template.HTML(b.String())
}

func headingLevel(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n == len(s) || s[n] != ' ' {
		return 0
	}
	return n
}

func isRule(s string) bool {
	if len(s) < 3 {
		return false
	}
	c := s[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	for i := range s {
		if s[i] != c {
			return false
		}
	}
	return true
}

// bulletText returns the item text of an unordered list line, or "".
func bulletText(s string) string {
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return strings.TrimSpace(s[2:])
	}
	return ""
}

var orderedPrefix = regexp.MustCompile(`^(\d{1,9})[.)] +`)

// orderedText returns the item text of an ordered list line, or "".
func orderedText(s string) string {
	m := orderedPrefix.FindString(s)
	if m == "" {
		return ""
	}
	return strings.TrimSpace(s[len(m):])
}

func orderedStart(s string) string {
	m := orderedPrefix.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

func renderFence(b *strings.Builder, lines []string, i int) int {
	i++
	var code []string
	for i < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			i++
			break
		}
		code = append(code, lines[i])
		i++
	}
	for len(code) > 0 && strings.TrimSpace(code[len(code)-1]) == "" {
		code = code[:len(code)-1]
	}
	b.WriteString("<pre><code>")
	b.WriteString(html.EscapeString(strings.Join(code, "\n")))
	b.WriteString("</code></pre>\n")
	return i
}

// renderList consumes consecutive list lines.  A line indented at least
// two spaces continues the current item.  A nested marker becomes a
// sibling item.
func renderList(b *strings.Builder, lines []string, i int, tag string, itemText func(string) string) int {
	open := "<" + tag + ">"
	if tag == "ol" {
		if start := orderedStart(strings.TrimSpace(lines[i])); start != "" && start != "1" {
			open = `<ol start="` + start + `">`
		}
	}
	b.WriteString(open + "\n")
	var item string
	flush := func() {
		if item != "" {
			b.WriteString("<li>" + inline(item) + "</li>\n")
			item = ""
		}
	}
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		text := itemText(trimmed)
		switch {
		case text != "":
			flush()
			item = text
		case trimmed == "":
			// A blank line ends the list unless another item follows.
			if next := peekListItem(lines, i+1, itemText); !next {
				flush()
				b.WriteString("</" + tag + ">\n")
				return i
			}
		case strings.HasPrefix(lines[i], "  ") && item != "":
			// Continuation of the current item, including markers of the
			// other list kind nested under this item.
			item += " " + trimmed
		default:
			flush()
			b.WriteString("</" + tag + ">\n")
			return i
		}
		i++
	}
	flush()
	b.WriteString("</" + tag + ">\n")
	return i
}

func peekListItem(lines []string, i int, itemText func(string) string) bool {
	for ; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		return itemText(trimmed) != ""
	}
	return false
}

func renderQuote(b *strings.Builder, lines []string, i int) int {
	var parts []string
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, ">") {
			break
		}
		parts = append(parts, strings.TrimSpace(strings.TrimPrefix(trimmed, ">")))
		i++
	}
	b.WriteString("<blockquote><p>")
	b.WriteString(inline(strings.Join(parts, " ")))
	b.WriteString("</p></blockquote>\n")
	return i
}

func renderParagraph(b *strings.Builder, lines []string, i int) int {
	var parts []string
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || headingLevel(trimmed) > 0 || strings.HasPrefix(trimmed, "```") ||
			isRule(trimmed) || bulletText(trimmed) != "" || orderedText(trimmed) != "" ||
			strings.HasPrefix(trimmed, ">") {
			break
		}
		parts = append(parts, trimmed)
		i++
	}
	b.WriteString("<p>")
	b.WriteString(inline(strings.Join(parts, " ")))
	b.WriteString("</p>\n")
	return i
}

var (
	codeSpanRe = regexp.MustCompile("`([^`]+)`")
	linkRe     = regexp.MustCompile(`\[([^\[\]]+)\]\(([^()\s]+)\)`)
	boldRe     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	emRe       = regexp.MustCompile(`\*([^*\s](?:[^*]*[^*\s])?)\*`)
)

// inline escapes text and applies inline markdown.  Code spans are
// extracted first so their contents stay literal, and placeholders keep
// them out of the later replacements.
func inline(s string) string {
	esc := html.EscapeString(s)
	var spans []string
	esc = codeSpanRe.ReplaceAllStringFunc(esc, func(m string) string {
		content := codeSpanRe.FindStringSubmatch(m)[1]
		spans = append(spans, "<code>"+content+"</code>")
		return fmt.Sprintf("\x00%d\x00", len(spans)-1)
	})
	esc = linkRe.ReplaceAllStringFunc(esc, func(m string) string {
		parts := linkRe.FindStringSubmatch(m)
		if !safeLinkTarget(parts[2]) {
			return m
		}
		return `<a href="` + parts[2] + `">` + parts[1] + `</a>`
	})
	esc = boldRe.ReplaceAllString(esc, "<strong>$1</strong>")
	esc = emRe.ReplaceAllString(esc, "<em>$1</em>")
	for i, span := range spans {
		esc = strings.Replace(esc, fmt.Sprintf("\x00%d\x00", i), span, 1)
	}
	return esc
}

// safeLinkTarget accepts http, https, and mailto targets plus targets
// with no scheme.  The target is already HTML-escaped.
func safeLinkTarget(target string) bool {
	lower := strings.ToLower(target)
	for _, p := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	colon := strings.IndexByte(target, ':')
	slash := strings.IndexByte(target, '/')
	return colon < 0 || (slash >= 0 && slash < colon)
}
