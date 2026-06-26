package main

import (
	_ "embed"
	"log"
)

//go:embed pricing_data.json
var pricingData []byte

func loadFallback() map[string]ModelPrice {
	prices, err := parsePrices(pricingData)
	if err != nil {
		log.Printf("fallback pricing parse error: %v", err)
		return map[string]ModelPrice{}
	}
	return prices
}
