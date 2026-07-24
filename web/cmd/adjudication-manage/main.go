// Command adjudication-manage serves the ARB management UI: it starts,
// monitors, and stops clerk, direct, and attested cases through a
// configured aar service and links to the report UI for reading.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"adjudication/web/manage"
)

type rootList []manage.ReportRoot

func (r *rootList) String() string { return fmt.Sprintf("%v", []manage.ReportRoot(*r)) }

func (r *rootList) Set(arg string) error {
	root, err := manage.ParseReportRootArg(arg)
	if err != nil {
		return err
	}
	*r = append(*r, root)
	return nil
}

func main() {
	var cfg manage.Config
	var roots rootList
	flag.StringVar(&cfg.Listen, "listen", "", "listen address (default "+manage.DefaultListen+")")
	flag.StringVar(&cfg.ARBURL, "arb-url", "", "aar service base URL (default "+manage.DefaultARBURL+")")
	flag.StringVar(&cfg.ARBToken, "arb-token", "", "aar service bearer token")
	flag.StringVar(&cfg.ReportURL, "report-url", "", "report server base URL for read links")
	flag.Var(&roots, "report-root", "report root as name=path, matching the report server; repeatable")
	flag.Parse()
	cfg.ReportRoots = roots

	if err := cfg.Finish(); err != nil {
		log.Fatal(err)
	}
	srv, err := manage.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("arb service at %s", cfg.ARBURL)
	log.Printf("listening on %s", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, srv))
}
