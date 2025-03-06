package notification

import (
	"strings"
	"sync"
	"time"
)

// NotificationPriority defines the priority level of a notification
type NotificationPriority string

const (
	// Priority levels
	PriorityLow    NotificationPriority = "low"
	PriorityMedium NotificationPriority = "medium"
	PriorityHigh   NotificationPriority = "high"
)

// NotificationType defines the type of notification
type NotificationType string

const (
	// Notification types
	TypeSignalGenerated NotificationType = "signal_generated"
	TypeOrderExecuted   NotificationType = "order_executed"
	TypeMarketEvent     NotificationType = "market_event"
	TypeSystemAlert     NotificationType = "system_alert"
)

// Notification represents a notification to be displayed to the user
type Notification struct {
	ID        string              `json:"id"`
	Type      NotificationType    `json:"type"`
	Title     string              `json:"title"`
	Message   string              `json:"message"`
	Priority  NotificationPriority `json:"priority"`
	Timestamp time.Time           `json:"timestamp"`
	Read      bool                `json:"read"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationManager manages notifications
type NotificationManager struct {
	notifications   []Notification
	maxNotifications int
	mutex           sync.RWMutex
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(maxNotifications int) *NotificationManager {
	return &NotificationManager{
		notifications:   []Notification{},
		maxNotifications: maxNotifications,
	}
}

// AddNotification adds a notification to the manager
func (nm *NotificationManager) AddNotification(notification Notification) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Add timestamp if not already set
	if notification.Timestamp.IsZero() {
		notification.Timestamp = time.Now()
	}

	// Add to the beginning of the list for reverse chronological order
	nm.notifications = append([]Notification{notification}, nm.notifications...)

	// Trim if exceeding max notifications
	if len(nm.notifications) > nm.maxNotifications {
		nm.notifications = nm.notifications[:nm.maxNotifications]
	}
}

// GetNotifications returns all notifications
func (nm *NotificationManager) GetNotifications() []Notification {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	// Return a copy to avoid race conditions
	notifications := make([]Notification, len(nm.notifications))
	copy(notifications, nm.notifications)

	return notifications
}

// GetUnreadNotifications returns all unread notifications
func (nm *NotificationManager) GetUnreadNotifications() []Notification {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	var unread []Notification
	for _, notification := range nm.notifications {
		if !notification.Read {
			unread = append(unread, notification)
		}
	}

	return unread
}

// GetNotificationsByType returns notifications of a specific type
func (nm *NotificationManager) GetNotificationsByType(notificationType NotificationType) []Notification {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	var filtered []Notification
	for _, notification := range nm.notifications {
		if notification.Type == notificationType {
			filtered = append(filtered, notification)
		}
	}

	return filtered
}

// GetNotificationsBySymbol returns notifications related to a specific symbol
func (nm *NotificationManager) GetNotificationsBySymbol(symbol string) []Notification {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	var filtered []Notification
	for _, notification := range nm.notifications {
		// Check if metadata map exists and contains the symbol
		if notification.Metadata != nil {
			if sym, exists := notification.Metadata["symbol"]; exists {
				if symbolStr, ok := sym.(string); ok && symbolStr == symbol {
					filtered = append(filtered, notification)
					continue
				}
			}
		}
		
		// Also check if the symbol appears in the title or message
		if strings.Contains(notification.Title, symbol) || strings.Contains(notification.Message, symbol) {
			filtered = append(filtered, notification)
		}
	}

	return filtered
}

// DeleteNotification deletes a notification by ID and returns whether it was found
func (nm *NotificationManager) DeleteNotification(id string) bool {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	for i, notification := range nm.notifications {
		if notification.ID == id {
			// Remove the notification (preserve order)
			nm.notifications = append(nm.notifications[:i], nm.notifications[i+1:]...)
			return true
		}
	}
	
	// Notification not found
	return false
}

// MarkAsRead marks a notification as read and returns whether it was found
func (nm *NotificationManager) MarkAsRead(id string) bool {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	for i := range nm.notifications {
		if nm.notifications[i].ID == id {
			nm.notifications[i].Read = true
			return true
		}
	}
	
	return false
}

// MarkAllAsRead marks all notifications as read
func (nm *NotificationManager) MarkAllAsRead() {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	for i := range nm.notifications {
		nm.notifications[i].Read = true
	}
}

// ClearNotifications removes all notifications
func (nm *NotificationManager) ClearNotifications() {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	nm.notifications = []Notification{}
}

// Helper functions to create different types of notifications

// CreateSignalGeneratedNotification creates a notification for a generated trading signal
// Note: By including the symbol in the metadata, GetNotificationsBySymbol can find it
// even if the symbol is not in the title/message (although it will be in this case)
func CreateSignalGeneratedNotification(symbol, signal, reasoning string, priority NotificationPriority, metadata map[string]interface{}) Notification {
	// If metadata wasn't provided, create it
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	// Ensure symbol is in metadata
	metadata["symbol"] = symbol
	metadata["signal"] = signal
	
	return Notification{
		ID:        generateID(),
		Type:      TypeSignalGenerated,
		Title:     symbol + " Trading Signal",
		Message:   "Generated " + signal + " signal for " + symbol + ": " + reasoning,
		Priority:  priority,
		Timestamp: time.Now(),
		Read:      false,
		Metadata:  metadata,
	}
}

// CreateOrderExecutedNotification creates a notification for an executed order
func CreateOrderExecutedNotification(symbol, orderType string, quantity float64, price float64, metadata map[string]interface{}) Notification {
	// If metadata wasn't provided, create it
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	// Ensure symbol is in metadata
	metadata["symbol"] = symbol
	metadata["order_type"] = orderType
	metadata["quantity"] = quantity
	metadata["price"] = price
	
	return Notification{
		ID:        generateID(),
		Type:      TypeOrderExecuted,
		Title:     symbol + " Order Executed",
		Message:   formatOrderMessage(symbol, orderType, quantity, price),
		Priority:  PriorityHigh,
		Timestamp: time.Now(),
		Read:      false,
		Metadata:  metadata,
	}
}

// CreateMarketEventNotification creates a notification for a market event
func CreateMarketEventNotification(symbol, eventType, message string) Notification {
	metadata := map[string]interface{}{
		"symbol":     symbol,
		"event_type": eventType,
	}
	
	return Notification{
		ID:        generateID(),
		Type:      TypeMarketEvent,
		Title:     eventType + ": " + symbol,
		Message:   message,
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Read:      false,
		Metadata:  metadata,
	}
}

// CreateSystemAlertNotification creates a notification for a system alert
func CreateSystemAlertNotification(title, message string, metadata map[string]interface{}) Notification {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	return Notification{
		ID:        generateID(),
		Type:      TypeSystemAlert,
		Title:     title,
		Message:   message,
		Priority:  PriorityHigh,
		Timestamp: time.Now(),
		Read:      false,
		Metadata:  metadata,
	}
}

// Helper function to format order messages
func formatOrderMessage(symbol, orderType string, quantity, price float64) string {
	action := "Bought"
	if quantity < 0 {
		action = "Sold"
		quantity = -quantity // Make positive for display
	}

	return action + " " + formatFloat(quantity) + " shares of " + symbol + " at $" + formatFloat(price) + " (" + orderType + ")"
}

// Helper function to format floats nicely
func formatFloat(value float64) string {
	if value == float64(int(value)) {
		return formatInt(int(value))
	}
	return formatInt(int(value)) + "." + formatFraction(value-float64(int(value)))
}

// Helper function to format integers with commas
func formatInt(value int) string {
	if value < 1000 {
		return time.Now().Format("20060102150405") // Just convert to string
	}
	return formatInt(value/1000) + "," + formatInt(value%1000)
}

// Helper function to format fractional part
func formatFraction(value float64) string {
	value *= 100
	if value == float64(int(value)) {
		if int(value) < 10 {
			return "0" + time.Now().Format("20060102150405")[:1]
		}
		return time.Now().Format("20060102150405")[:2]
	}
	return time.Now().Format("20060102150405")[:2]
}

// generateID generates a unique ID for notifications
func generateID() string {
	return time.Now().Format("20060102150405") + time.Now().Format("999")
}