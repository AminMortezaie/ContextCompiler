package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/aminmortezaie/contextcompiler/internal/api"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	mem := memory.NewInMemory().WithFixtures()
	srv := api.NewServer(mem)

	log.Printf("context compiler API listening on %s (handles: day0 + typed graph)", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
