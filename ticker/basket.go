package ticker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TickerBasket represents a collection of related ticker symbols
type TickerBasket struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Symbols     []string `json:"symbols"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	CreatedBy   string   `json:"created_by,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsActive    bool     `json:"is_active"`
	Category    string   `json:"category,omitempty"`
}

// BasketManager manages ticker baskets
type BasketManager struct {
	dataDir string
	baskets map[string]*TickerBasket
	mutex   sync.RWMutex
}

// NewBasketManager creates a new basket manager
func NewBasketManager(dataDir string) (*BasketManager, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	manager := &BasketManager{
		dataDir: dataDir,
		baskets: make(map[string]*TickerBasket),
	}

	// Load existing baskets
	if err := manager.loadBaskets(); err != nil {
		log.Printf("Warning: Failed to load baskets: %v", err)
	}

	return manager, nil
}

// loadBaskets loads all basket files from the data directory
func (m *BasketManager) loadBaskets() error {
	basketDir := filepath.Join(m.dataDir, "baskets")
	if err := os.MkdirAll(basketDir, 0755); err != nil {
		return fmt.Errorf("failed to create baskets directory: %w", err)
	}

	files, err := ioutil.ReadDir(basketDir)
	if err != nil {
		return fmt.Errorf("failed to read baskets directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(basketDir, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Printf("Warning: Failed to read basket file %s: %v", filePath, err)
			continue
		}

		var basket TickerBasket
		if err := json.Unmarshal(data, &basket); err != nil {
			log.Printf("Warning: Failed to parse basket file %s: %v", filePath, err)
			continue
		}

		if basket.ID == "" {
			log.Printf("Warning: Basket file %s has no ID", filePath)
			continue
		}

		m.mutex.Lock()
		m.baskets[basket.ID] = &basket
		m.mutex.Unlock()
	}

	log.Printf("Loaded %d ticker baskets from %s", len(m.baskets), basketDir)
	return nil
}

// SaveBasket saves a ticker basket to disk
func (m *BasketManager) SaveBasket(basket *TickerBasket) error {
	// Validate basket
	if basket.ID == "" {
		basket.ID = generateID()
	}
	if basket.CreatedAt == "" {
		basket.CreatedAt = time.Now().Format(time.RFC3339)
	}
	basket.UpdatedAt = time.Now().Format(time.RFC3339)

	// Add to memory cache
	m.mutex.Lock()
	m.baskets[basket.ID] = basket
	m.mutex.Unlock()

	// Save to disk
	basketDir := filepath.Join(m.dataDir, "baskets")
	if err := os.MkdirAll(basketDir, 0755); err != nil {
		return fmt.Errorf("failed to create baskets directory: %w", err)
	}

	filePath := filepath.Join(basketDir, fmt.Sprintf("%s.json", basket.ID))
	data, err := json.MarshalIndent(basket, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal basket: %w", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write basket file: %w", err)
	}

	log.Printf("Saved ticker basket %s (%s) with %d symbols", 
		basket.ID, basket.Name, len(basket.Symbols))
	return nil
}

// GetBasket retrieves a ticker basket by ID
func (m *BasketManager) GetBasket(id string) (*TickerBasket, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	basket, exists := m.baskets[id]
	if !exists {
		return nil, fmt.Errorf("basket not found: %s", id)
	}

	// Return a copy to avoid race conditions
	basketCopy := *basket
	return &basketCopy, nil
}

// DeleteBasket deletes a ticker basket
func (m *BasketManager) DeleteBasket(id string) error {
	// Check if basket exists
	m.mutex.RLock()
	_, exists := m.baskets[id]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("basket not found: %s", id)
	}

	// Delete from memory cache
	m.mutex.Lock()
	delete(m.baskets, id)
	m.mutex.Unlock()

	// Delete from disk
	basketDir := filepath.Join(m.dataDir, "baskets")
	filePath := filepath.Join(basketDir, fmt.Sprintf("%s.json", id))
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete basket file: %w", err)
	}

	log.Printf("Deleted ticker basket %s", id)
	return nil
}

// ListBaskets returns a list of all ticker baskets
func (m *BasketManager) ListBaskets() []TickerBasket {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	baskets := make([]TickerBasket, 0, len(m.baskets))
	for _, basket := range m.baskets {
		baskets = append(baskets, *basket)
	}

	return baskets
}

// AddSymbolToBasket adds a symbol to a ticker basket
func (m *BasketManager) AddSymbolToBasket(basketID, symbol string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	basket, exists := m.baskets[basketID]
	if !exists {
		return fmt.Errorf("basket not found: %s", basketID)
	}

	// Check if symbol already exists
	for _, s := range basket.Symbols {
		if s == symbol {
			// Symbol already exists, no need to add
			return nil
		}
	}

	// Add symbol
	basket.Symbols = append(basket.Symbols, symbol)
	basket.UpdatedAt = time.Now().Format(time.RFC3339)

	// Save to disk
	basketDir := filepath.Join(m.dataDir, "baskets")
	filePath := filepath.Join(basketDir, fmt.Sprintf("%s.json", basketID))
	data, err := json.MarshalIndent(basket, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal basket: %w", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write basket file: %w", err)
	}

	log.Printf("Added symbol %s to basket %s (%s)", symbol, basketID, basket.Name)
	return nil
}

// RemoveSymbolFromBasket removes a symbol from a ticker basket
func (m *BasketManager) RemoveSymbolFromBasket(basketID, symbol string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	basket, exists := m.baskets[basketID]
	if !exists {
		return fmt.Errorf("basket not found: %s", basketID)
	}

	// Find and remove symbol
	for i, s := range basket.Symbols {
		if s == symbol {
			// Remove symbol
			basket.Symbols = append(basket.Symbols[:i], basket.Symbols[i+1:]...)
			basket.UpdatedAt = time.Now().Format(time.RFC3339)

			// Save to disk
			basketDir := filepath.Join(m.dataDir, "baskets")
			filePath := filepath.Join(basketDir, fmt.Sprintf("%s.json", basketID))
			data, err := json.MarshalIndent(basket, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal basket: %w", err)
			}

			if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
				return fmt.Errorf("failed to write basket file: %w", err)
			}

			log.Printf("Removed symbol %s from basket %s (%s)", symbol, basketID, basket.Name)
			return nil
		}
	}

	// Symbol not found
	return fmt.Errorf("symbol not found in basket: %s", symbol)
}

// GetBasketsBySymbol returns a list of baskets containing a specific symbol
func (m *BasketManager) GetBasketsBySymbol(symbol string) []TickerBasket {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var baskets []TickerBasket
	for _, basket := range m.baskets {
		for _, s := range basket.Symbols {
			if s == symbol {
				baskets = append(baskets, *basket)
				break
			}
		}
	}

	return baskets
}

// GetSymbolsFromBaskets returns a unique list of all symbols from a set of basket IDs
func (m *BasketManager) GetSymbolsFromBaskets(basketIDs []string) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Use a map for deduplication
	symbolMap := make(map[string]bool)

	for _, basketID := range basketIDs {
		basket, exists := m.baskets[basketID]
		if !exists {
			continue
		}

		for _, symbol := range basket.Symbols {
			symbolMap[symbol] = true
		}
	}

	// Convert map keys to slice
	symbols := make([]string, 0, len(symbolMap))
	for symbol := range symbolMap {
		symbols = append(symbols, symbol)
	}

	return symbols
}

// Helper function to generate a simple ID
func generateID() string {
	return fmt.Sprintf("basket_%d", time.Now().UnixNano())
}