package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	log.Println("Service A listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))

}

func handler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{"service": "Service A"}
	json.NewEncoder(w).Encode(response)
}
