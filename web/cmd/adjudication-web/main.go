package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"adjudication/web/console"
)

func main() {
	cfg := console.DefaultConfig()
	var requestTimeout time.Duration
	adc := cfg.Systems["adc"]
	arb := cfg.Systems["arb"]
	arbd := cfg.Systems["arbd"]
	flag.StringVar(&cfg.ListenAddr, "listen", envDefault("ADJUDICATION_WEB_LISTEN", cfg.ListenAddr), "web listen address")
	flag.StringVar(&cfg.WebBearerToken, "bearer-token", os.Getenv("ADJUDICATION_WEB_BEARER_TOKEN"), "optional bearer token for the web console")
	flag.StringVar(&adc.BaseURL, "adc-url", envDefault("ADC_SERVICE_URL", adc.BaseURL), "ADC service base URL")
	flag.StringVar(&adc.BearerToken, "adc-token", os.Getenv("ADC_SERVICE_TOKEN"), "ADC service bearer token")
	flag.StringVar(&arb.BaseURL, "arb-url", envDefault("ARB_SERVICE_URL", arb.BaseURL), "ARB service base URL")
	flag.StringVar(&arb.BearerToken, "arb-token", os.Getenv("ARB_SERVICE_TOKEN"), "ARB service bearer token")
	flag.StringVar(&arbd.BaseURL, "arbd-url", envDefault("ARBD_SERVICE_URL", arbd.BaseURL), "AARD service base URL")
	flag.StringVar(&arbd.BearerToken, "arbd-token", os.Getenv("ARBD_SERVICE_TOKEN"), "AARD service bearer token")
	flag.DurationVar(&requestTimeout, "request-timeout", 30*time.Second, "service request timeout")
	flag.Parse()
	cfg.Systems["adc"] = adc
	cfg.Systems["arb"] = arb
	cfg.Systems["arbd"] = arbd
	cfg.RequestTimeout = requestTimeout

	app, err := console.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(os.Stderr, "adjudication web listening on http://%s\n", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
