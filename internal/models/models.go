package models

import "time"

// Event represents a unified event from any source
type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Venue       Venue     `json:"venue"`
	Category    string    `json:"category"`
	Price       Price     `json:"price"`
	Source      string    `json:"source"` // "eventbrite", "ticketmaster", "meetup"
	URL         string    `json:"url"`
	ImageURL    string    `json:"image_url"`
	Attending   int       `json:"attending"`
	Hot         bool      `json:"hot"` // trending event
}

// Venue holds location data
type Venue struct {
	Name    string  `json:"name"`
	Address string  `json:"address"`
	City    string  `json:"city"`
	State   string  `json:"state"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Distance float64 `json:"distance_miles"`
}

// Price holds pricing info
type Price struct {
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Currency string  `json:"currency"`
	Free     bool    `json:"free"`
}

// SearchParams holds search filters
type SearchParams struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Radius   float64 `json:"radius"`   // miles
	Category string  `json:"category"`
	FreeOnly bool    `json:"free_only"`
	Keyword  string  `json:"keyword"`
}

// EventsResponse is the API response
type EventsResponse struct {
	Events  []Event `json:"events"`
	Total   int     `json:"total"`
	Source  string  `json:"source"`
	CacheHit bool   `json:"cache_hit"`
}
