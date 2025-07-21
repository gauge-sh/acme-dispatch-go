package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/acme-dispatch/dispatch/api/internal/server"
	"github.com/acme-dispatch/dispatch/core/store"
)

func main() {
	addr := flag.String("addr", ":4400", "listen address")
	dbPath := flag.String("db", defaultDB(), "path to the sqlite database")
	seed := flag.Bool("seed", false, "load the fixed development dataset and exit")
	flag.Parse()

	s, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open %s: %v", *dbPath, err)
	}
	defer s.Close()

	if *seed {
		if err := s.Seed(); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("seeded %s", *dbPath)
		return
	}

	log.Printf("api listening on %s (db %s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, server.New(s)); err != nil {
		log.Fatal(err)
	}
}

func defaultDB() string {
	if path := os.Getenv("DISPATCH_DB"); path != "" {
		return path
	}
	return "dispatch.db"
}
