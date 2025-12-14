package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var serverPort string
var serverID string

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate variable processing time
	processingTime := time.Duration(rand.Intn(100)) * time.Millisecond
	time.Sleep(processingTime)

	response := map[string]interface{}{
		"server_id":       serverID,
		"port":            serverPort,
		"timestamp":       time.Now().Format(time.RFC3339),
		"processing_time": processingTime.Milliseconds(),
		"path":            r.URL.Path,
		"method":          r.Method,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func productsHandler(w http.ResponseWriter, r *http.Request) {
	processingTime := time.Duration(rand.Intn(150)) * time.Millisecond
	time.Sleep(processingTime)

	products := []map[string]interface{}{
		{"id": 1, "name": "Laptop", "price": 999.99},
		{"id": 2, "name": "Mouse", "price": 29.99},
		{"id": 3, "name": "Keyboard", "price": 79.99},
	}

	response := map[string]interface{}{
		"server_id":       serverID,
		"products":        products,
		"processing_time": processingTime.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	processingTime := time.Duration(rand.Intn(200)) * time.Millisecond
	time.Sleep(processingTime)

	orders := []map[string]interface{}{
		{"id": 101, "total": 1099.98, "status": "shipped"},
		{"id": 102, "total": 79.99, "status": "processing"},
	}

	response := map[string]interface{}{
		"server_id":       serverID,
		"orders":          orders,
		"processing_time": processingTime.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	processingTime := time.Duration(rand.Intn(80)) * time.Millisecond
	time.Sleep(processingTime)

	users := []map[string]interface{}{
		{"id": 1, "name": "John Doe", "email": "john@example.com"},
		{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
	}

	response := map[string]interface{}{
		"server_id":       serverID,
		"users":           users,
		"processing_time": processingTime.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run server.go <port>")
	}

	serverPort = os.Args[1]
	serverID = fmt.Sprintf("backend-%s", serverPort)

	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/api/products", productsHandler)
	http.HandleFunc("/api/orders", ordersHandler)
	http.HandleFunc("/api/users", usersHandler)

	addr := fmt.Sprintf(":%s", serverPort)
	log.Printf("Backend server [%s] starting on %s\n", serverID, addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
