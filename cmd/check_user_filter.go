//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func main() {
	token := strings.TrimSpace(os.Getenv("COPREM_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GH_TOKEN"))
	}
	if token == "" {
		fmt.Println("NO TOKEN")
		os.Exit(1)
	}
	ctx := context.Background()

	do := func(endpoint string) (int, int) {
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", "coprem-test")
		resp, _ := http.DefaultClient.Do(req)
		defer resp.Body.Close()
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		items, _ := body["usageItems"].([]any)
		users := map[string]bool{}
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				if u, ok := m["user"].(string); ok && u != "" {
					users[u] = true
				}
			}
		}
		return len(items), len(users)
	}

	// Test 1 — no filter
	allItems, allUsers := do("https://api.github.com/enterprises/moka-united/settings/billing/usage?year=2026&month=6")
	fmt.Printf("NO FILTER: %d items, %d unique users\n", allItems, allUsers)

	// Test 2 — with user=baris-dogu_MOKA
	filtItems, filtUsers := do("https://api.github.com/enterprises/moka-united/settings/billing/usage?year=2026&month=6&user=baris-dogu_MOKA")
	fmt.Printf("WITH user=baris-dogu_MOKA: %d items, %d unique users\n", filtItems, filtUsers)

	if filtItems == allItems {
		fmt.Println(">>> user filter IS BROKEN — same count with or without filter")
	} else if filtUsers == 1 {
		fmt.Println(">>> user filter WORKS")
	}
}
