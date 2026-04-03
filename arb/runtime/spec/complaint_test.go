package spec

import "testing"

func TestParseComplaintMarkdown(t *testing.T) {
	raw := `# Proposition

The claimant will argue that the proposition is substantially true.
`
	got, err := ParseComplaintMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseComplaintMarkdown error = %v", err)
	}
	if got.Proposition == "" {
		t.Fatalf("unexpected parsed complaint: %#v", got)
	}
}

func TestComplaintMarkdownRoundTrip(t *testing.T) {
	in := Complaint{
		Proposition: "P",
	}
	got, err := ParseComplaintMarkdown(ComplaintMarkdown(in))
	if err != nil {
		t.Fatalf("round trip parse error = %v", err)
	}
	if got != in {
		t.Fatalf("round trip mismatch: got %#v want %#v", got, in)
	}
}
