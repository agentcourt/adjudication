package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"adjudication/adc/runtime/mcp"
)

func RunMCP(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("mcp", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc mcp --caseapi-base <url> [options]\n\n")
		fs.PrintDefaults()
	})
	listenAddr := fs.String("listen", mcp.DefaultListenAddr, "MCP listen address")
	caseAPIBase := fs.String("caseapi-base", "", "Base URL for the ADC case API")
	bearerToken := fs.String("bearer-token", "", "Optional bearer token required from MCP clients")
	apiBearerToken := fs.String("api-bearer-token", "", "Optional bearer token sent to the case API")
	disableSessionExpiry := fs.Bool("disable-session-expiry", false, "Disable idle MCP session expiry")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*caseAPIBase) == "" {
		return fmt.Errorf("--caseapi-base is required")
	}
	_ = stdout
	return mcp.Run(context.Background(), mcp.Options{
		ListenAddr:           strings.TrimSpace(*listenAddr),
		CaseAPIBase:          strings.TrimSpace(*caseAPIBase),
		BearerToken:          strings.TrimSpace(*bearerToken),
		APIBearerToken:       strings.TrimSpace(*apiBearerToken),
		DisableSessionExpiry: *disableSessionExpiry,
		Log:                  stderr,
	})
}
