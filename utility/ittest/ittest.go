// Package ittest gives integration tests the local connection settings
// without any password in the source: they come from the environment, or
// from the repository's .env (the same file docker compose reads).
//
//	GOFRAMESHOP_IT=1 go test ./... -run Integration
package ittest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MySQL is the link for database db as the local "shop" account.
func MySQL(t *testing.T, db string) string {
	t.Helper()
	return fmt.Sprintf("mysql:shop:%s@tcp(127.0.0.1:3306)/%s?loc=Local&parseTime=true", Secret(t, "MYSQL_PASSWORD"), db)
}

// RabbitMQ is the local broker URL as the "shop" account.
func RabbitMQ(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("amqp://shop:%s@127.0.0.1:5672/", Secret(t, "RABBITMQ_PASSWORD"))
}

// Secret reads key from the environment, else from .env in the repository
// root (found by walking up to go.mod). A missing value fails the test with
// a pointer to .env.example rather than guessing a default.
func Secret(t *testing.T, key string) string {
	t.Helper()
	if v := os.Getenv(key); v != "" {
		return v
	}
	if v := fromDotEnv(key); v != "" {
		return v
	}
	t.Fatalf("%s is not set: export it, or copy .env.example to .env and fill it in", key)
	return ""
}

func fromDotEnv(key string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	f, err := os.Open(filepath.Join(dir, ".env"))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		name, value, ok := strings.Cut(line, "=")
		if ok && !strings.HasPrefix(line, "#") && strings.TrimSpace(name) == key {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}
