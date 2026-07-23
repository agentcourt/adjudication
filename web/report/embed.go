// Package report serves a read-only web report over adjudication run
// output directories found under configured root directories.  It reads
// artifact files from the filesystem and renders server-side HTML.  It
// performs no writes and offers no control actions.
package report

import "embed"

//go:embed templates/*.html static/*.css static/*.js
var files embed.FS
