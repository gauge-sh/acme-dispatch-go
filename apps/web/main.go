package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/acme-dispatch/dispatch/core/store"
	"github.com/acme-dispatch/dispatch/web/internal/webui"
)

func main() {
	addr := flag.String("addr", ":4000", "listen address")
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

	app := webui.New(s, sessionSecret())
	log.Printf("web listening on %s (db %s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, app); err != nil {
		log.Fatal(err)
	}
}

func defaultDB() string {
	if path := os.Getenv("DISPATCH_DB"); path != "" {
		return path
	}
	return "dispatch.db"
}

// sessionSecret falls back to a fixed development key; set SESSION_SECRET
// for anything reachable by other people.
func sessionSecret() []byte {
	if secret := os.Getenv("SESSION_SECRET"); secret != "" {
		return []byte(secret)
	}
	return []byte("dispatch-dev-only-session-secret")
}
