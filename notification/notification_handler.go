package notification

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// NotificationHandler implements HTTP handlers for notification API endpoints
type NotificationHandler struct {
	manager *NotificationManager
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(manager *NotificationManager) *NotificationHandler {
	return &NotificationHandler{
		manager: manager,
	}
}

// RegisterRoutes registers notification routes with the provided HTTP mux
func (h *NotificationHandler) RegisterRoutes(mux *http.ServeMux) {
	// GET /api/notifications - List all notifications
	// POST /api/notifications - Create a new notification
	mux.HandleFunc("/api/notifications", h.handleNotifications)

	// GET /api/notifications?unread=true - List unread notifications only
	// GET /api/notifications?type=market_event - List notifications by type
	// GET /api/notifications?symbol=AAPL - List notifications by symbol

	// POST /api/notifications/{id}/read - Mark a notification as read
	mux.HandleFunc("/api/notifications/", h.handleNotificationActions)

	// POST /api/notifications/read-all - Mark all notifications as read
	mux.HandleFunc("/api/notifications/read-all", h.handleReadAllNotifications)
}

// handleNotifications handles GET and POST requests to /api/notifications
func (h *NotificationHandler) handleNotifications(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodGet {
		// Get query parameters for filtering
		unreadOnly := r.URL.Query().Get("unread") == "true"
		notifType := r.URL.Query().Get("type")
		symbol := r.URL.Query().Get("symbol")

		var notifications []Notification

		// Apply filters
		if unreadOnly {
			notifications = h.manager.GetUnreadNotifications()
		} else if notifType != "" {
			// Convert string to NotificationType
			notifications = h.filterByType(NotificationType(notifType))
		} else if symbol != "" {
			notifications = h.manager.GetNotificationsBySymbol(symbol)
		} else {
			// If no filters, get all notifications
			notifications = h.manager.GetNotifications()
		}

		// Serialize and return
		if err := json.NewEncoder(w).Encode(notifications); err != nil {
			http.Error(w, "Failed to encode notifications", http.StatusInternalServerError)
			log.Printf("Error encoding notifications: %v", err)
			return
		}
		return
	}

	if r.Method == http.MethodPost {
		// Parse request body
		var notif Notification
		if err := json.NewDecoder(r.Body).Decode(&notif); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			log.Printf("Error decoding notification request: %v", err)
			return
		}

		// Set ID and timestamp if not provided
		if notif.ID == "" {
			notif.ID = generateID()
		}
		if notif.Timestamp.IsZero() {
			notif.Timestamp = time.Now()
		}

		// Add to manager
		h.manager.AddNotification(notif)

		// Return success response
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Notification created successfully",
			"id":      notif.ID,
		}); err != nil {
			log.Printf("Error encoding success response: %v", err)
		}
		return
	}

	// Method not allowed
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Filter notifications by type (helper method)
func (h *NotificationHandler) filterByType(notificationType NotificationType) []Notification {
	notifications := h.manager.GetNotifications()
	var filtered []Notification
	
	for _, notification := range notifications {
		if notification.Type == notificationType {
			filtered = append(filtered, notification)
		}
	}
	
	return filtered
}

// handleNotificationActions handles actions on individual notifications
func (h *NotificationHandler) handleNotificationActions(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract path components
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications/")
	pathParts := strings.Split(path, "/")

	if len(pathParts) < 1 {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}

	notificationID := pathParts[0]

	// Handle mark as read action
	if len(pathParts) >= 2 && pathParts[1] == "read" && r.Method == http.MethodPost {
		success := h.manager.MarkAsRead(notificationID)
		
		w.Header().Set("Content-Type", "application/json")
		if success {
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Notification marked as read",
				"id":      notificationID,
			})
		} else {
			http.Error(w, "Notification not found", http.StatusNotFound)
		}
		return
	}

	// Handle delete action
	if r.Method == http.MethodDelete {
		success := h.manager.DeleteNotification(notificationID)
		
		w.Header().Set("Content-Type", "application/json")
		if success {
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Notification deleted",
				"id":      notificationID,
			})
		} else {
			http.Error(w, "Notification not found", http.StatusNotFound)
		}
		return
	}

	// Method not allowed
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleReadAllNotifications handles marking all notifications as read
func (h *NotificationHandler) handleReadAllNotifications(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodPost {
		h.manager.MarkAllAsRead()
		if err := json.NewEncoder(w).Encode(map[string]string{
			"message": "All notifications marked as read",
		}); err != nil {
			log.Printf("Error encoding success response: %v", err)
		}
		return
	}

	// Method not allowed
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}