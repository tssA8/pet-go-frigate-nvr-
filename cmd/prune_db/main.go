package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 1. Open DB
	homeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(homeDir, "Library/Mobile Documents/com~apple~CloudDocs/NVR/nvr.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// 2. Query all recordings
	rows, err := db.Query("SELECT id, path FROM recordings")
	if err != nil {
		log.Fatalf("Failed to query recordings: %v", err)
	}
	defer rows.Close()

	// 3. Check file existence
	var toDelete []int64
	dataDir := filepath.Join(homeDir, "Library/Mobile Documents/com~apple~CloudDocs/NVR")

	for rows.Next() {
		var id int64
		var relPath string
		if err := rows.Scan(&id, &relPath); err != nil {
			log.Println("Scan error:", err)
			continue
		}

		fullPath := filepath.Join(dataDir, relPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			log.Printf("File missing: %s (ID: %d)", fullPath, id)
			toDelete = append(toDelete, id)
		}
	}

	// 4. Delete missing entries
	if len(toDelete) > 0 {
		tx, err := db.Begin()
		if err != nil {
			log.Fatal(err)
		}

		stmt, err := tx.Prepare("DELETE FROM recordings WHERE id = ?")
		if err != nil {
			log.Fatal(err)
		}
		defer stmt.Close()

		for _, id := range toDelete {
			if _, err := stmt.Exec(id); err != nil {
				log.Printf("Failed to delete ID %d: %v", id, err)
			}
		}

		if err := tx.Commit(); err != nil {
			log.Fatal("Commit failed:", err)
		}
		log.Printf("Deleted %d orphaned recordings from DB", len(toDelete))
	} else {
		log.Println("No orphaned recordings found.")
	}
}
