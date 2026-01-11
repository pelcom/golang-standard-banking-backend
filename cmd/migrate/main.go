package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"banking/internal/config"
	"banking/internal/db"
)

func main() {
	cfg := config.Load()
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (filename text primary key, applied_at timestamptz default now())`); err != nil {
		log.Fatalf("failed to ensure schema_migrations: %v", err)
	}

	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		log.Fatalf("failed to read migrations: %v", err)
	}
	sort.Strings(files)

	for _, file := range files {
		filename := filepath.Base(file)
		var exists bool
		if err := database.Get(&exists, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, filename); err != nil {
			log.Fatalf("failed to read migration state: %v", err)
		}
		if exists {
			continue
		}
		if err := applyFile(database, file); err != nil {
			log.Fatalf("failed to apply %s: %v", filename, err)
		}
		if _, err := database.Exec(`INSERT INTO schema_migrations (filename) VALUES ($1)`, filename); err != nil {
			log.Fatalf("failed to record migration %s: %v", filename, err)
		}
		fmt.Printf("applied %s\n", filename)
	}
}

func applyFile(db execer, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sections := strings.Split(string(content), "-- +migrate Down")
	if len(sections) == 0 {
		return nil
	}
	up := sections[0]
	statements := splitSQL(up)
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitSQL(sqlText string) []string {
	var statements []string
	var current strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(sqlText))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		current.WriteString(line)
		current.WriteRune('\n')
		if strings.Contains(line, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}
	if strings.TrimSpace(current.String()) != "" {
		statements = append(statements, current.String())
	}
	return statements
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}
