package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

func main() {
	dbURL := "postgres://postgres.kcxsqfbepqrcfmrefqlt:MHWDUdklbdFnU4Xw@aws-1-eu-west-2.pooler.supabase.com:5432/postgres?sslmode=require"
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("Failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Printf("Failed to ping DB: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connected to DB successfully.")

	migrationsDir := filepath.Join("..", "..", "infra", "migrations")
	files, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		fmt.Printf("Failed to read migrations directory: %v\n", err)
		os.Exit(1)
	}

	var sqlFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, f := range sqlFiles {
		fmt.Printf("Executing migration: %s... ", f)
		path := filepath.Join(migrationsDir, f)
		content, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			continue
		}

		_, err = db.Exec(string(content))
		if err != nil {
			fmt.Printf("FAILED\nError: %v\n", err)
		} else {
			fmt.Printf("SUCCESS\n")
		}
	}
	fmt.Println("All migrations processed.")
}
