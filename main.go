package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/neerajhoney2-code/billionaire/api"
	"github.com/neerajhoney2-code/billionaire/contracts"
	"github.com/neerajhoney2-code/billionaire/core"
	"github.com/neerajhoney2-code/billionaire/network"
)

func main() {
	p2pPort := flag.Int("port", 3000, "P2P TCP port")
	apiPort := flag.Int("api-port", 8080, "REST API / UI port")
	peersFlag := flag.String("peers", "", "Comma-separated bootstrap peer addresses (host:port)")
	flag.Parse()

	p2pAddr := fmt.Sprintf(":%d", *p2pPort)
	apiAddr := fmt.Sprintf(":%d", *apiPort)
	host := fmt.Sprintf("localhost%s", apiAddr)

	log.Printf("Starting Billionaire PoS Blockchain node")
	log.Printf("  P2P:  %s", p2pAddr)
	log.Printf("  API:  http://localhost%s", apiAddr)

	// Initialise core components.
	bc := core.New()
	cr := contracts.NewRegistry()
	node := network.NewNode(p2pAddr, bc)

	// Start P2P server.
	if err := node.Start(); err != nil {
		log.Fatalf("P2P start: %v", err)
	}

	// Connect to bootstrap peers.
	if *peersFlag != "" {
		for _, peer := range strings.Split(*peersFlag, ",") {
			peer = strings.TrimSpace(peer)
			if peer != "" {
				go node.Connect(peer)
			}
		}
	}

	// Set up HTTP server.
	mux := http.NewServeMux()
	srv := api.NewServer(bc, node, cr, host)
	srv.RegisterRoutes(mux)

	log.Printf("Dashboard: http://localhost%s", apiAddr)
	log.Printf("QR Scanner: http://localhost%s/scanner.html", apiAddr)

	if err := http.ListenAndServe(apiAddr, mux); err != nil {
		log.Fatalf("API server: %v", err)
	}
}
