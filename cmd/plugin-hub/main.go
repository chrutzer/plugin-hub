package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/chrutzer/plugin-hub/internal/api"
	"github.com/chrutzer/plugin-hub/internal/config"
	"github.com/chrutzer/plugin-hub/internal/registry"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	reg, err := registry.New(cfg)
	if err != nil {
		log.Fatalf("init registry: %v", err)
	}
	reg.Reload()

	srv := api.New(reg)

	log.Printf("plugin-hub listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
