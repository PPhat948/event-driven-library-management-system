package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	BookURL         string
	InventoryURL    string
	NotificationURL string
	Mode            string
}

type Book struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	ISBN           string `json:"isbn"`
	TotalQuantity  int    `json:"total_quantity"`
	AvailableCount int    `json:"available_count"`
	Status         string `json:"status"`
}

type InventoryItem struct {
	ID             string `json:"id"`
	BookID         string `json:"book_id"`
	Title          string `json:"title"`
	TotalQuantity  int    `json:"total_quantity"`
	AvailableCount int    `json:"available_count"`
	BorrowedCount  int    `json:"borrowed_count"`
	Status         string `json:"status"`
}

type NotificationItem struct {
	ID        string `json:"id"`
	EventType string `json:"event_type"`
	BookID    string `json:"book_id,omitempty"`
	Message   string `json:"message"`
}

func detectEnvironment(client *http.Client) Config {
	// Try Nginx Gateway first (Port 80)
	resp, err := client.Get("http://localhost:80/api/books/health")
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return Config{
			BookURL:         "http://localhost:80/api/books",
			InventoryURL:    "http://localhost:80/api/inventory",
			NotificationURL: "http://localhost:80/api/notifications",
			Mode:            "Nginx Gateway (Port 80)",
		}
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Fallback to direct ports (Local Dev)
	resp, err = client.Get("http://localhost:8001/health")
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return Config{
			BookURL:         "http://localhost:8001/books",
			InventoryURL:    "http://localhost:8002/inventory",
			NotificationURL: "http://localhost:8003/notifications",
			Mode:            "Direct Ports (Local Dev: :8001, :8002, :8003)",
		}
	}
	if resp != nil {
		resp.Body.Close()
	}

	fmt.Println("  [FAIL] Error: Neither Docker (make up) nor Local Dev services are running!")
	fmt.Println("  [INFO] Fix: Run 'make up' (Docker) OR start 'make infra-up' + Go services.")
	os.Exit(1)
	return Config{}
}

func main() {
	fmt.Println("=========================================================")
	fmt.Println("Starting E2E System Integration Test")
	fmt.Println("=========================================================")

	client := &http.Client{Timeout: 5 * time.Second}

	// ── Step 1: Detect Environment & Health Check ────────────────────────
	fmt.Println("\n[1/6] Auto-detecting Environment & System Health...")
	cfg := detectEnvironment(client)
	fmt.Printf("  [OK] Environment Detected: %s\n", cfg.Mode)

	// ── Step 2: Create Book ───────────────────────────────────────────────
	fmt.Println("\n[2/6] Creating test book via Book Service...")
	isbn := fmt.Sprintf("978-%d", time.Now().UnixNano()%10000000000)
	createReq := map[string]any{
		"title":          "E2E Integration Book",
		"author":         "Go E2E Runner",
		"isbn":           isbn,
		"total_quantity": 4,
	}

	book := createBook(client, cfg.BookURL, createReq)
	fmt.Printf("  [OK] Created Book ID: %s (Title: %q, Total Qty: 4)\n", book.ID, book.Title)

	// ── Step 3: Verify Inventory Sync ─────────────────────────────────────
	fmt.Println("\n[3/6] Verifying async inventory sync (SQS consumer)...")
	var invItem InventoryItem
	err := pollWithRetry(func() bool {
		invItem = getInventory(client, cfg.InventoryURL, book.ID)
		return invItem.ID != "" && invItem.AvailableCount == 4 && invItem.Status == "available"
	}, 6*time.Second, 500*time.Millisecond)

	if err != nil {
		fmt.Printf("  [FAIL] Inventory sync assertion failed: %v (Got item: %+v)\n", err, invItem)
		os.Exit(1)
	}
	fmt.Printf("  [OK] Inventory Synced: AvailableCount = %d, Status = %s\n", invItem.AvailableCount, invItem.Status)

	// ── Step 4: Borrow Book & Trigger Low Stock Alert ─────────────────────
	fmt.Println("\n[4/6] Borrowing book 2 times to trigger Low Stock Alert (Avail 4 -> 2 <= threshold 2)...")
	borrowBook(client, cfg.BookURL, book.ID, "member-e2e-1", "Alice E2E")
	borrowBook(client, cfg.BookURL, book.ID, "member-e2e-2", "Bob E2E")

	fmt.Println("  [WAIT] Waiting for async low_stock alert processing...")
	var lowStockItems []InventoryItem
	err = pollWithRetry(func() bool {
		lowStockItems = getLowStock(client, cfg.InventoryURL)
		for _, item := range lowStockItems {
			if item.BookID == book.ID {
				return true
			}
		}
		return false
	}, 6*time.Second, 500*time.Millisecond)

	if err != nil {
		fmt.Printf("  [FAIL] Book not found in low-stock list: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  [OK] Book correctly flagged in low-stock list")

	// ── Step 5: Verify Notification Feed ──────────────────────────────────
	fmt.Println("\n[5/6] Verifying Notification Feed (book.borrowed & inventory.low_stock)...")
	var foundBorrowed, foundLowStock bool
	err = pollWithRetry(func() bool {
		notifications := getNotifications(client, cfg.NotificationURL)
		for _, n := range notifications {
			if n.BookID == book.ID || strings.Contains(n.Message, "E2E Integration Book") {
				if strings.Contains(n.Message, "Alice E2E borrowed") || strings.Contains(n.Message, "borrowed") {
					foundBorrowed = true
				}
				if n.EventType == "inventory.low_stock" || strings.Contains(n.Message, "Low stock alert") {
					foundLowStock = true
				}
			}
		}
		return foundBorrowed && foundLowStock
	}, 6*time.Second, 500*time.Millisecond)

	if err != nil {
		fmt.Printf("  [FAIL] Notifications assertion failed (foundBorrowed=%v, foundLowStock=%v): %v\n", foundBorrowed, foundLowStock, err)
		os.Exit(1)
	}
	fmt.Println("  [OK] Notifications verified: Both 'borrowed' and 'low_stock alert' logged")

	// ── Step 6: Return Book & Teardown ────────────────────────────────────
	fmt.Println("\n[6/6] Returning book and performing teardown...")
	returnBook(client, cfg.BookURL, book.ID, "member-e2e-1")

	err = pollWithRetry(func() bool {
		invItem = getInventory(client, cfg.InventoryURL, book.ID)
		return invItem.AvailableCount == 3 && invItem.Status == "available"
	}, 6*time.Second, 500*time.Millisecond)

	if err != nil {
		fmt.Printf("  [FAIL] Inventory status recovery assertion failed: %v (Got item: %+v)\n", err, invItem)
		os.Exit(1)
	}
	fmt.Println("  [OK] Inventory status recovered: AvailableCount = 3, Status = available")

	// Cleanup
	deleteBook(client, cfg.BookURL, book.ID)
	fmt.Println("  [OK] Test book deleted from catalog")

	fmt.Println("\n=========================================================")
	fmt.Println("ALL E2E INTEGRATION TESTS PASSED SUCCESSFULLY!")
	fmt.Println("=========================================================")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func createBook(client *http.Client, bookURL string, reqBody map[string]any) Book {
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(bookURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("  [FAIL] Create book failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("  [FAIL] Create book returned status %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var book Book
	_ = json.NewDecoder(resp.Body).Decode(&book)
	return book
}

func getInventory(client *http.Client, inventoryURL, bookID string) InventoryItem {
	resp, err := client.Get(inventoryURL + "/" + bookID)
	if err != nil {
		return InventoryItem{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return InventoryItem{}
	}

	var item InventoryItem
	_ = json.NewDecoder(resp.Body).Decode(&item)
	return item
}

func borrowBook(client *http.Client, bookURL, bookID, memberID, memberName string) {
	reqBody := map[string]any{
		"member_id":   memberID,
		"member_name": memberName,
		"due_date":    time.Now().Add(14 * 24 * time.Hour).Format(time.RFC3339),
	}
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(bookURL+"/"+bookID+"/borrow", "application/json", bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("  [FAIL] Borrow book failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("  [FAIL] Borrow book status = %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
}

func getLowStock(client *http.Client, inventoryURL string) []InventoryItem {
	resp, err := client.Get(inventoryURL + "/low-stock")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Items []InventoryItem `json:"items"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result.Items
}

func getNotifications(client *http.Client, notificationURL string) []NotificationItem {
	resp, err := client.Get(notificationURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Notifications []NotificationItem `json:"notifications"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result.Notifications
}

func returnBook(client *http.Client, bookURL, bookID, memberID string) {
	reqBody := map[string]any{
		"member_id": memberID,
	}
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(bookURL+"/"+bookID+"/return", "application/json", bytes.NewBuffer(b))
	if err != nil {
		fmt.Printf("  [FAIL] Return book failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("  [FAIL] Return book returned status %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
}

func deleteBook(client *http.Client, bookURL, bookID string) {
	req, _ := http.NewRequest(http.MethodDelete, bookURL+"/"+bookID, nil)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func pollWithRetry(fn func() bool, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timeout after %v", timeout)
}
