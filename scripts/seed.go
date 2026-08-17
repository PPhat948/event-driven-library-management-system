package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type BookReq struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn"`
	TotalQuantity int    `json:"total_quantity"`
}

type BorrowReq struct {
	MemberID   string `json:"member_id"`
	MemberName string `json:"member_name"`
	DueDate    string `json:"due_date"`
}

type Book struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	ISBN  string `json:"isbn"`
}

type ListResp struct {
	Books []Book `json:"books"`
}

func main() {
	api := os.Getenv("BOOK_API")
	if api == "" {
		api = "http://localhost:8001"
	}

	fmt.Printf("seeding book data -> %s\n\n", api)

	_ = getOrCreateBook(api, BookReq{
		Title:         "Clean Code",
		Author:        "Robert C. Martin",
		ISBN:          "9780132350884",
		TotalQuantity: 5,
	})

	goBook := getOrCreateBook(api, BookReq{
		Title:         "The Go Programming Language",
		Author:        "Alan Donovan",
		ISBN:          "9780134190440",
		TotalQuantity: 3,
	})

	ddia := getOrCreateBook(api, BookReq{
		Title:         "Designing Data-Intensive Applications",
		Author:        "Martin Kleppmann",
		ISBN:          "9781449373320",
		TotalQuantity: 1,
	})

	borrowBook(api, goBook.ID, "member-001", "Alice")
	borrowBook(api, goBook.ID, "member-002", "Bob")
	borrowBook(api, ddia.ID, "member-003", "Charlie")

	fmt.Println("\ndone.")
	fmt.Println("  Clean Code                             available    (5/5)")
	fmt.Println("  The Go Programming Language            low stock    (1/3)")
	fmt.Println("  Designing Data-Intensive Applications  out of stock (0/1)")
}

func getOrCreateBook(api string, req BookReq) Book {
	body, _ := json.Marshal(req)
	resp, err := http.Post(api+"/books", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return findByISBN(api, req.ISBN)
	}

	if resp.StatusCode >= 400 {
		fmt.Printf("failed to create %s (status: %v)\n", req.Title, resp.Status)
		os.Exit(1)
	}

	var b Book
	json.NewDecoder(resp.Body).Decode(&b)
	fmt.Printf("  created: %s\n", b.Title)
	return b
}

func findByISBN(api, isbn string) Book {
	resp, err := http.Get(api + "/books")
	if err != nil || resp.StatusCode >= 400 {
		fmt.Printf("failed to fetch books list: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var list ListResp
	json.NewDecoder(resp.Body).Decode(&list)
	for _, b := range list.Books {
		if b.ISBN == isbn {
			fmt.Printf("  exists: %s\n", b.Title)
			return b
		}
	}

	fmt.Printf("book with isbn %s marked conflict but not found\n", isbn)
	os.Exit(1)
	return Book{}
}

func borrowBook(api, bookID, memberID, memberName string) {
	body, _ := json.Marshal(BorrowReq{
		MemberID:   memberID,
		MemberName: memberName,
		DueDate:    "2027-06-01T00:00:00Z",
	})
	resp, err := http.Post(fmt.Sprintf("%s/books/%s/borrow", api, bookID), "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("failed to borrow %s: %v\n", bookID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		fmt.Printf("  already out of stock / borrowed: %s\n", memberName)
		return
	}

	if resp.StatusCode >= 400 {
		fmt.Printf("  borrow skipped/failed (%s): %s\n", memberName, resp.Status)
		return
	}

	fmt.Printf("  borrowed by %s\n", memberName)
}
