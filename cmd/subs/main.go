package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wucai/subs/internal/secrets"
	"github.com/wucai/subs/internal/server"
	"github.com/wucai/subs/internal/store"
)

func main() {
	dataDir := flag.String("data-dir", env("SUBS_DATA_DIR", "./data"), "data directory")
	addr := flag.String("listen", env("SUBS_LISTEN", "127.0.0.1:8787"), "HTTP listen address")
	initToken := flag.String("init-token", os.Getenv("SUBS_INIT_TOKEN"), "one-time initialization token")
	flag.Parse()
	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Fatalf("create data directory: %v", err)
	}
	dbPath := filepath.Join(*dataDir, "subs.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		f, e := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if e != nil {
			log.Fatalf("create database: %v", e)
		}
		_ = f.Close()
	}
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer db.Close()
	box, err := secrets.LoadOrCreateBox(*dataDir)
	if err != nil {
		log.Fatalf("load secrets: %v", err)
	}
	srv := &server.Server{Store: db, InitToken: *initToken, Box: box, DataDir: *dataDir, DBPath: dbPath}
	log.Printf("subs listening on %s", *addr)
	if err = http.ListenAndServe(*addr, srv.Router()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
