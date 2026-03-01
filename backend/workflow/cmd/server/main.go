package main

import (
	"log"
	"net/http"

	_ "github.com/joho/godotenv/autoload"
	"github.com/example/business-automation/backend/workflow/internal/api"
)

func main() {
	srv := api.NewServer()
	addr := ":8085"
	log.Printf("workflow server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
