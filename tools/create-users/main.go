// Command create-users creates authentik users in bulk from a CSV file.
//
// Usage:
//
//	go run . -csv users.csv
//
// AUTHENTIK_TOKEN (required) and AUTHENTIK_URL are read from the environment
// or from a .env file in the working directory. See users.example.csv for the
// CSV format. Existing usernames are skipped, missing groups are created, and
// rows without a password get a random one, printed in the result table.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Loaded here, not in run, so tests never pick up a real token from .env.
	_ = godotenv.Load() // .env is optional; real env vars take precedence
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the tool and returns the exit code: 0 when every row succeeded,
// 1 when any row failed, 2 for a usage, configuration or CSV error.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("create-users", flag.ContinueOnError)
	fs.SetOutput(stderr)
	csvPath := fs.String("csv", "users.csv", "CSV file of users to create")
	baseURL := fs.String("url", envOr("AUTHENTIK_URL", "http://localhost:9000"), "authentik base URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	token := os.Getenv("AUTHENTIK_TOKEN")
	if token == "" {
		fmt.Fprintln(stderr, "AUTHENTIK_TOKEN is not set; put it in .env or the environment")
		return 2
	}

	f, err := os.Open(*csvPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer f.Close()

	rows, err := ParseUsers(f)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", *csvPath, err)
		return 2
	}

	client := NewClient(*baseURL, token, &http.Client{Timeout: 10 * time.Second})
	results := NewCreator(client, rand.Text).CreateAll(context.Background(), rows)
	if PrintResults(stdout, results) > 0 {
		return 1
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
