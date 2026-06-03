package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"adjudication/arb/runtime/mcp"
)

type originList struct {
	values []string
}

func (l *originList) String() string {
	values := append([]string{}, l.values...)
	sort.Strings(values)
	return strings.Join(values, ",")
}

func (l *originList) Set(value string) error {
	origin := strings.TrimSpace(value)
	if origin == "" {
		return fmt.Errorf("origin must not be empty")
	}
	l.values = append(l.values, origin)
	return nil
}

func runMCP(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var origins originList
	listen := fs.String("listen", "127.0.0.1:19780", "Listen address")
	caseAPI := fs.String("caseapi-base", "", "Case API base URL, for example http://127.0.0.1:19770")
	bearerToken := fs.String("bearer-token", "", "Optional bearer token required for MCP requests")
	apiBearerToken := fs.String("api-bearer-token", "", "Optional bearer token sent to the Case API")
	sessionTTL := fs.Duration("session-ttl", 30*time.Minute, "Idle MCP session TTL; 0 disables expiry")
	sessionCleanupInterval := fs.Duration("session-cleanup-interval", time.Minute, "Interval for deleting expired MCP sessions")
	fs.Var(&origins, "allow-origin", "Allowed HTTP Origin; repeat when browser clients need non-localhost origins")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar mcp --caseapi-base URL [options]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	_ = stdout
	opts := mcp.Options{
		ListenAddr:             *listen,
		CaseAPIBase:            *caseAPI,
		BearerToken:            *bearerToken,
		APIBearerToken:         *apiBearerToken,
		SessionTTL:             *sessionTTL,
		DisableSessionExpiry:   *sessionTTL == 0,
		SessionCleanupInterval: *sessionCleanupInterval,
		AllowedOrigins:         origins.values,
		Log:                    stderr,
	}
	return mcp.Run(ctx, opts)
}
