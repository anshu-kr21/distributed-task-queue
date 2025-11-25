package websocket

import (
	"distributed-task-queue/internal/database"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Manager manages WebSocket connections and broadcasts
type Manager struct {
	clients   map[*websocket.Conn]bool
	clientsMu sync.Mutex
	db        *database.DB
}

// New creates a new WebSocket manager
func New(db *database.DB) *Manager {
	return &Manager{
		clients: make(map[*websocket.Conn]bool),
		db:      db,
	}
}

// AddClient adds a new WebSocket client
func (m *Manager) AddClient(conn *websocket.Conn) {
	m.clientsMu.Lock()
	m.clients[conn] = true
	m.clientsMu.Unlock()

	log.Printf("[WEBSOCKET] New client connected. Total clients: %d", len(m.clients))

	// Send initial data
	m.SendUpdateToClient(conn)

	// Handle disconnection
	go func() {
		defer func() {
			m.clientsMu.Lock()
			delete(m.clients, conn)
			m.clientsMu.Unlock()
			conn.Close()
			log.Printf("[WEBSOCKET] Client disconnected. Total clients: %d", len(m.clients))
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// Broadcast sends updates to all connected clients
func (m *Manager) Broadcast() {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	for client := range m.clients {
		go m.SendUpdateToClient(client)
	}
}

// SendUpdateToClient sends current state to a specific client
func (m *Manager) SendUpdateToClient(conn *websocket.Conn) {
	jobs, _ := m.db.GetAllJobs()
	metrics, _ := m.db.GetMetrics()

	update := map[string]interface{}{
		"jobs":    jobs,
		"metrics": metrics,
	}

	if err := conn.WriteJSON(update); err != nil {
		log.Printf("[ERROR] Failed to send WebSocket update: %v", err)
	}
}

// ClientCount returns the number of connected clients
func (m *Manager) ClientCount() int {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	return len(m.clients)
}

