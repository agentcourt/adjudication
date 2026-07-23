// Command adjudication-report serves a read-only web report over
// adjudication run output directories found under configured roots.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"adjudication/web/report"
)

type rootList []report.Root

func (r *rootList) String() string { return fmt.Sprintf("%v", []report.Root(*r)) }

func (r *rootList) Set(arg string) error {
	root, err := report.ParseRootArg(arg)
	if err != nil {
		return err
	}
	*r = append(*r, root)
	return nil
}

func main() {
	var (
		configPath = flag.String("config", "", "path to a JSON config file with listen and roots")
		listen     = flag.String("listen", "", "listen address (default "+report.DefaultListen+")")
		roots      rootList
	)
	flag.Var(&roots, "root", "run tree root as path or name=path; repeatable")
	flag.Parse()

	cfg, err := report.LoadConfig(*configPath, roots, *listen)
	if err != nil {
		log.Fatal(err)
	}
	srv, err := report.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	for _, rt := range cfg.Roots {
		log.Printf("root %s: %s", rt.Name, rt.Path)
	}
	log.Printf("listening on %s", cfg.Listen)
	log.Fatal(http.ListenAndServe(cfg.Listen, srv))
}
