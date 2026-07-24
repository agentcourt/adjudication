// Package manage serves a control UI for the ARB service: it starts,
// monitors, and stops clerk, direct, and attested cases through the
// service HTTP API and links to the report UI for reading run output.
package manage

import "embed"

//go:embed templates/*.html static/*.css static/*.js
var files embed.FS
