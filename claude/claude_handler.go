package claude

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// ClaudeHandler handles WebSocket connections for Claude AI trading signals
type ClaudeHandler struct {
	claudeClient *Client
	activeConns  map[*websocket.Conn]string // Maps connections to symbols
	connMutex    sync.RWMutex
	upgrader     websocket.Upgrader
}

// StreamMessage represents a message sent over the WebSocket
type StreamMessage struct {
	Type string      `json:"type"` // stream, signal, error
	Data interface{} `json:"data"`
}

// NewClaudeHandler creates a new Claude handler
func NewClaudeHandler(claudeClient *Client) *ClaudeHandler {
	return &ClaudeHandler{
		claudeClient: claudeClient,
		activeConns:  make(map[*websocket.Conn]string),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins in development
				return true
			},
		},
	}
}

// HandleWebSocket handles WebSocket connections for Claude trading signals
func (h *ClaudeHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("HandleWebSocket called with URL: %s", r.URL.String())
	// Get symbol from query parameter
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "Symbol parameter is required", http.StatusBadRequest)
		log.Printf("WebSocket connection rejected: missing symbol parameter")
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		log.Printf("Connection headers: %v", r.Header)
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}

	// Register the connection
	h.connMutex.Lock()
	h.activeConns[conn] = symbol
	h.connMutex.Unlock()

	log.Printf("New WebSocket connection for %s", symbol)

	// Handle the connection
	go h.handleConnection(conn, symbol)
}

// handleConnection manages a WebSocket connection
func (h *ClaudeHandler) handleConnection(conn *websocket.Conn, symbol string) {
	defer func() {
		// Unregister connection on close
		h.connMutex.Lock()
		delete(h.activeConns, conn)
		h.connMutex.Unlock()
		conn.Close()
		log.Printf("WebSocket connection closed for %s", symbol)
	}()

	for {
		// Read message from client
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse the message
		var request map[string]interface{}
		if err := json.Unmarshal(message, &request); err != nil {
			log.Printf("Error parsing WebSocket message: %v", err)
			h.sendError(conn, "Invalid message format")
			continue
		}

		log.Printf("Received WebSocket message: %s", string(message))
		action, ok := request["action"].(string)
		if !ok {
			h.sendError(conn, "Missing action field")
			continue
		}

		// Handle different actions
		switch action {
		case "request_signal":
			// Get the symbol from the message
			reqSymbol, ok := request["symbol"].(string)
			if !ok || reqSymbol == "" {
				h.sendError(conn, "Missing or invalid symbol field")
				continue
			}

			// Generate a signal
			go h.generateAndSendSignal(conn, reqSymbol)

		default:
			h.sendError(conn, fmt.Sprintf("Unknown action: %s", action))
		}
	}
}

// generateAndSendSignal generates a trading signal and sends it to the client
func (h *ClaudeHandler) generateAndSendSignal(conn *websocket.Conn, symbol string) {
	log.Printf("Generating signal for %s", symbol)

	h.sendStream(conn, "Analyzing market data with active trading algorithms...")

	// Get a signal from Claude
	signal, err := h.claudeClient.GenerateSignal(symbol)
	if err != nil {
		h.sendError(conn, fmt.Sprintf("Failed to generate signal: %v", err))
		return
	}

	log.Printf("Generated signal for %s: %+v", symbol, signal)

	// Send the signal
	h.sendSignal(conn, signal)
}

// sendStream sends a streaming message to the client
func (h *ClaudeHandler) sendStream(conn *websocket.Conn, message string) {
	msg := StreamMessage{
		Type: "stream",
		Data: message,
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending stream message: %v", err)
	}
}

// sendSignal sends a signal to the client
func (h *ClaudeHandler) sendSignal(conn *websocket.Conn, signal *TradeSignal) {
	msg := StreamMessage{
		Type: "signal",
 
		Data: signal,
 
	}

	jsonData, _ := json.Marshal(msg)
	log.Printf("Sending signal message: %s", string(jsonData))
	log.Printf("Signal field specifically: %s", signal.Signal)
	
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending signal: %v", err)
	}
}

// sendError sends an error message to the client
func (h *ClaudeHandler) sendError(conn *websocket.Conn, message string) {
	msg := StreamMessage{
		Type: "error",
		Data: message,
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending error message: %v", err)
	}
}

// RegisterRoutes registers routes with the provided HTTP mux
func (h *ClaudeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ws/claude", h.HandleWebSocket)
}
