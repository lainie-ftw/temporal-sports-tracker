package main

import (
	"log"
	"net/http"
	"os"
	"temporal-sports-tracker/web"

	"go.temporal.io/sdk/client"
)

func main() {
	// Create Temporal client
	var temporalClient client.Client
	var err error
	
	temporalClient, err = client.Dial(client.Options{})
	if err != nil {
		log.Printf("Warning: Unable to create Temporal client: %v", err)
		log.Printf("The UI will work but workflow operations will be limited")
		temporalClient = nil
	} else {
		defer temporalClient.Close()
		log.Printf("Successfully connected to Temporal server")
	}

	// Create web handlers with Temporal client (can be nil)
	handlers := web.NewHandlers(temporalClient)

	// Serve static files
	staticDir := "web/static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// If running from different directory, try relative path
		staticDir = "../../web/static"
	}
	
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/", fs)

	// API routes
	http.HandleFunc("/api/sports", handlers.GetSports)
	http.HandleFunc("/api/leagues/", handlers.GetLeagues)
	http.HandleFunc("/api/teams/", handlers.GetTeams)
	http.HandleFunc("/api/conferences/", handlers.GetConferences)
	http.HandleFunc("/api/track", handlers.StartTracking)
	http.HandleFunc("/api/workflows", handlers.GetWorkflows)
	http.HandleFunc("/api/workflows/", handlers.ManageWorkflow)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting web server on port %s", port)
	log.Printf("Open http://localhost:%s in your browser", port)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalln("Server failed to start:", err)
	}
}
