package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

var (
	port  = flag.Int("port", 9000, "port to listen on")
	name  = flag.String("name", "", "server name (defaults to the OS hostname)")
	delay = flag.Duration("delay", 0, "artificial delay for responses")
)

var reqCount atomic.Int64

type Response struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Requests int64  `json:"requests"`
	Hostname string `json:"hostname"`
}

func main() {
	flag.Parse()

	serverName := *name
	if serverName == "" {
		hostname, _ := os.Hostname()
		serverName = fmt.Sprintf("%s:%d", hostname, *port)
	}

	// health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// wait for delay to elapse
		if *delay > 0 {
			time.Sleep(*delay)
		}

		// record request count
		count := reqCount.Add(1)
		hostname, _ := os.Hostname()

		resp := Response{
			Server:   serverName,
			Port:     *port,
			Requests: count,
			Hostname: hostname,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	})

	// start server
	addr := fmt.Sprintf("localhost:%d", *port)
	log.Printf("Server '%s' listening on %s", serverName, addr)
	if *delay > 0 {
		log.Printf("Server will process all request with a delay of: %s", *delay)
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}
