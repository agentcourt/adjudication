package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"adjudication/adc/runtime/service"
)

func RunService(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("service", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc service [options]\n\n")
		fs.PrintDefaults()
	})
	listenAddr := fs.String("listen", service.DefaultListenAddr, "Service listen address")
	outputRoot := fs.String("output-root", "out/adc-service", "Directory containing service-created case output directories")
	adcBin := fs.String("adc-bin", ".bin/adc", "ADC binary used to start child cases")
	enginePath := fs.String("engine", defaultEngineCommand(), "Lean engine command passed to child cases")
	bearerToken := fs.String("bearer-token", "", "Optional bearer token required from service clients")
	startupWaitSeconds := fs.Int("startup-wait-seconds", int(service.DefaultCaseStartupWait/time.Second), "Seconds to wait for a child case API to become healthy")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	_ = stdout
	return service.Run(context.Background(), service.Config{
		ListenAddr:  strings.TrimSpace(*listenAddr),
		OutputRoot:  strings.TrimSpace(*outputRoot),
		ADCBin:      strings.TrimSpace(*adcBin),
		EnginePath:  strings.TrimSpace(*enginePath),
		BearerToken: strings.TrimSpace(*bearerToken),
		StartupWait: time.Duration(*startupWaitSeconds) * time.Second,
		Log:         stderr,
	})
}
