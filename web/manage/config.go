package manage

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	DefaultListen = "127.0.0.1:9091"
	DefaultARBURL = "http://127.0.0.1:19770"
)

// ReportRoot maps an absolute filesystem prefix to a report-server root
// name, so a case's out_dir can link to its report run page.
type ReportRoot struct {
	Name string
	Path string
}

type Config struct {
	Listen      string
	ARBURL      string
	ARBToken    string
	ReportURL   string
	ReportRoots []ReportRoot
}

// Finish validates and normalizes the configuration.
func (c *Config) Finish() error {
	if c.Listen == "" {
		c.Listen = DefaultListen
	}
	if c.ARBURL == "" {
		c.ARBURL = DefaultARBURL
	}
	c.ARBURL = strings.TrimRight(c.ARBURL, "/")
	c.ReportURL = strings.TrimRight(c.ReportURL, "/")
	for i := range c.ReportRoots {
		if c.ReportRoots[i].Name == "" || c.ReportRoots[i].Path == "" {
			return fmt.Errorf("report root needs name and path: %+v", c.ReportRoots[i])
		}
		abs, err := filepath.Abs(c.ReportRoots[i].Path)
		if err != nil {
			return fmt.Errorf("report root %s: %w", c.ReportRoots[i].Path, err)
		}
		c.ReportRoots[i].Path = abs
	}
	if len(c.ReportRoots) > 0 && c.ReportURL == "" {
		return fmt.Errorf("report roots need --report-url")
	}
	return nil
}

// ParseReportRootArg parses a --report-root argument of the form
// name=path, matching the names configured on the report server.
func ParseReportRootArg(arg string) (ReportRoot, error) {
	name, path, ok := strings.Cut(arg, "=")
	if !ok || name == "" || path == "" {
		return ReportRoot{}, fmt.Errorf("report root %q must be name=path", arg)
	}
	return ReportRoot{Name: name, Path: path}, nil
}

// ReportLink returns the report run page URL for an absolute out_dir,
// or "" when no configured report root contains it.
func (c *Config) ReportLink(outDir string) string {
	if c.ReportURL == "" || !filepath.IsAbs(outDir) {
		return ""
	}
	clean := filepath.Clean(outDir)
	for _, rt := range c.ReportRoots {
		if clean == rt.Path {
			return c.ReportURL + "/run/" + rt.Name + "/."
		}
		if strings.HasPrefix(clean, rt.Path+string(filepath.Separator)) {
			rel, err := filepath.Rel(rt.Path, clean)
			if err != nil {
				continue
			}
			return c.ReportURL + "/run/" + rt.Name + "/" + filepath.ToSlash(rel)
		}
	}
	return ""
}
