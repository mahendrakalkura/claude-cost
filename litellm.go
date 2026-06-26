package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const litellmURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

func fetchLive() (map[string]ModelPrice, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(litellmURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing fetch: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	prices, err := parsePrices(data)
	if err != nil {
		return nil, err
	}
	cp := cachePath()
	if cp != "" {
		if err := os.WriteFile(cp, data, 0644); err != nil {
			return nil, err
		}
	}
	return prices, nil
}
