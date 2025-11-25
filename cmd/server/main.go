package main

import (
	"context"
	"distributed-task-queue/internal/api"
	"distributed-task-queue/internal/database"
	"distributed-task-queue/internal/websocket"
	"distributed-task-queue/internal/worker"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Open database
	db, err := database.New("./jobs.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Initialize database schema
	if err := db.InitSchema(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	log.Println("[INIT] Database initialized")

	// Create WebSocket manager
	wsManager := websocket.New(db)

	// Create context for workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start workers
	numWorkers := 3
	pollInterval := 2 * time.Second

	for i := 1; i <= numWorkers; i++ {
		w := worker.New(i, db, pollInterval, ctx, wsManager.Broadcast)
		go w.Start()
	}
	log.Printf("[INIT] Started %d workers", numWorkers)

	// Create API server
	apiServer := api.NewServer(db, wsManager)

	// Setup routes
	mux := http.NewServeMux()
	apiServer.SetupRoutes(mux)

	// Start HTTP server
	port := ":8080"
	log.Printf("[INIT] Server starting on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

