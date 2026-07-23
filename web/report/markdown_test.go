package report

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "headings",
			src:  "# Title\n\n### Deep\n",
			want: []string{"<h1>Title</h1>", "<h3>Deep</h3>"},
		},
		{
			name: "paragraph joins lines",
			src:  "one\ntwo\n\nthree\n",
			want: []string{"<p>one two</p>", "<p>three</p>"},
		},
		{
			name: "unordered list",
			src:  "- a\n- b\n",
			want: []string{"<ul>", "<li>a</li>", "<li>b</li>", "</ul>"},
		},
		{
			name: "ordered list with start",
			src:  "3. c\n4. d\n",
			want: []string{`<ol start="3">`, "<li>c</li>", "<li>d</li>"},
		},
		{
			name: "list item continuation",
			src:  "- a\n  continued\n- b\n",
			want: []string{"<li>a continued</li>", "<li>b</li>"},
		},
		{
			name: "list survives blank line before next item",
			src:  "- a\n\n- b\n",
			want: []string{"<li>a</li>", "<li>b</li>"},
		},
		{
			name: "fenced code stays literal",
			src:  "```\n# not a heading\n<b>\n```\n",
			want: []string{"<pre><code># not a heading\n&lt;b&gt;</code></pre>"},
		},
		{
			name: "blockquote",
			src:  "> quoted\n> text\n",
			want: []string{"<blockquote><p>quoted text</p></blockquote>"},
		},
		{
			name: "horizontal rule",
			src:  "---\n",
			want: []string{"<hr>"},
		},
		{
			name: "inline code and bold",
			src:  "use `x < y` and **now**\n",
			want: []string{"<code>x &lt; y</code>", "<strong>now</strong>"},
		},
		{
			name: "emphasis",
			src:  "an *important* point\n",
			want: []string{"<em>important</em>"},
		},
		{
			name: "snake case untouched",
			src:  "not_demonstrated and file_name stay plain\n",
			want: []string{"not_demonstrated and file_name stay plain"},
		},
		{
			name: "link",
			src:  "see [docs](https://example.com/a?b=1)\n",
			want: []string{`<a href="https://example.com/a?b=1">docs</a>`},
		},
		{
			name: "unsafe link stays literal",
			src:  "see [x](javascript:alert(1))\n",
			want: []string{"[x](javascript:alert(1))"},
		},
		{
			name: "html escaped",
			src:  "<script>alert(1)</script>\n",
			want: []string{"&lt;script&gt;alert(1)&lt;/script&gt;"},
		},
		{
			name: "unclosed fence runs to end",
			src:  "```\ncode\n",
			want: []string{"<pre><code>code</code></pre>"},
		},
		{
			name: "nul bytes stripped before placeholder use",
			src:  "a\x00b \x000\x00 `c`\n",
			want: []string{"<p>ab 0 <code>c</code></p>"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(RenderMarkdown([]byte(tc.src)))
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("output missing %q:\n%s", w, got)
				}
			}
			if strings.Contains(got, "<script>") {
				t.Errorf("unescaped script tag:\n%s", got)
			}
		})
	}
}
