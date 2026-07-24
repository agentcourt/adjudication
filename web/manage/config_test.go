package manage

import "testing"

func TestReportLink(t *testing.T) {
	cfg := Config{
		ReportURL:   "http://127.0.0.1:9090/",
		ReportRoots: []ReportRoot{{Name: "svc", Path: "/srv/arb/out/service"}},
	}
	if err := cfg.Finish(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"/srv/arb/out/service/case-1":      "http://127.0.0.1:9090/run/svc/case-1",
		"/srv/arb/out/service/a/b":         "http://127.0.0.1:9090/run/svc/a/b",
		"/srv/arb/out/service":             "http://127.0.0.1:9090/run/svc/.",
		"/srv/arb/out/servicex/case-1":     "",
		"/elsewhere/case-1":                "",
		"relative/case-1":                  "",
		"/srv/arb/out/service/../evil/c":   "",
		"/srv/arb/out/service/case-1/deep": "http://127.0.0.1:9090/run/svc/case-1/deep",
	}
	for in, want := range cases {
		if got := cfg.ReportLink(in); got != want {
			t.Errorf("ReportLink(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFinishRequiresReportURL(t *testing.T) {
	cfg := Config{ReportRoots: []ReportRoot{{Name: "a", Path: "/x"}}}
	if err := cfg.Finish(); err == nil {
		t.Fatal("report roots without report URL accepted")
	}
}
