package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()
	// get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081" // default port
	}
	mux.HandleFunc("/", handler)
	log.Println("Service A listening on :8081")
	log.Fatal(http.ListenAndServe(":"+port, mux))

}

func handler(w http.ResponseWriter, r *http.Request) {
	svcName := os.Getenv("SERVICE_NAME")
	if svcName == "" {
		svcName = "Service Name Not Set" // default value
	}
	// read the environment variable " SERVICE_INSTANCE" and return it as a response
	svcInstanceName := os.Getenv("SERVICE_INSTANCE")
	if svcInstanceName == "" {
		svcInstanceName = ""
	}
	response := map[string]string{"service": svcName, "instance": svcInstanceName}
	json.NewEncoder(w).Encode(response)
}
