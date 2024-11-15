package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	log.Println("Service A listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))

}

func handler(w http.ResponseWriter, r *http.Request) {
	// read the environment variable " SERVICE_INSTANCE" and return it as a response
	svcInstanceName := os.Getenv("SERVICE_INSTANCE")
	if svcInstanceName == "" {
		svcInstanceName = "ServiceA"
	}
	response := map[string]string{"service": svcInstanceName}
	json.NewEncoder(w).Encode(response)
}
