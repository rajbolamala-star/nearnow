package aggregator

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/rajbolamala-star/nearnow/internal/geo"
	"github.com/rajbolamala-star/nearnow/internal/models"
)

type Aggregator struct {
	client           *http.Client
	ticketmasterKey  string
	eventbriteKey    string
}

func New() *Aggregator {
	return &Aggregator{
		client:          &http.Client{Timeout: 10 * time.Second},
		ticketmasterKey: os.Getenv("TICKETMASTER_KEY"),
		eventbriteKey:   os.Getenv("EVENTBRITE_KEY"),
	}
}

// Search fetches events from all sources and merges them
func (a *Aggregator) Search(params models.SearchParams) ([]models.Event, error) {
	results := make(chan []models.Event, 3)
	errors := make(chan error, 3)

	// Fan out — fetch from all sources concurrently
	go func() {
		events, err := a.fetchTicketmaster(params)
		if err != nil {
			log.Printf("ticketmaster error: %v", err)
			results <- []models.Event{}
			return
		}
		results <- events
	}()

	go func() {
		events, err := a.fetchEventbrite(params)
		if err != nil {
			log.Printf("eventbrite error: %v", err)
			results <- []models.Event{}
			return
		}
		results <- events
	}()

	go func() {
		// Mock data as fallback / demo
		results <- a.mockEvents(params)
		errors <- nil
	}()

	// Fan in — collect all results
	var allEvents []models.Event
	for i := 0; i < 3; i++ {
		events := <-results
		allEvents = append(allEvents, events...)
	}

	// Filter by radius
	filtered := make([]models.Event, 0)
	for _, e := range allEvents {
		dist := geo.Distance(params.Lat, params.Lng, e.Venue.Lat, e.Venue.Lng)
		if dist <= params.Radius {
			e.Venue.Distance = dist
			if params.FreeOnly && !e.Price.Free {
				continue
			}
			if params.Category != "" && e.Category != params.Category {
				continue
			}
			filtered = append(filtered, e)
		}
	}

	// Sort by distance
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Venue.Distance < filtered[j].Venue.Distance
	})

	// Mark hot events (high attendance)
	for i := range filtered {
		if filtered[i].Attending > 100 {
			filtered[i].Hot = true
		}
	}

	return filtered, nil
}

// fetchTicketmaster fetches from Ticketmaster API
func (a *Aggregator) fetchTicketmaster(params models.SearchParams) ([]models.Event, error) {
	if a.ticketmasterKey == "" {
		return []models.Event{}, nil
	}

	url := fmt.Sprintf(
		"https://app.ticketmaster.com/discovery/v2/events.json?apikey=%s&latlong=%f,%f&radius=%d&unit=miles&size=20",
		a.ticketmasterKey, params.Lat, params.Lng, int(params.Radius),
	)

	resp, err := a.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Embedded struct {
			Events []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				URL  string `json:"url"`
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
				Dates struct {
					Start struct {
						DateTime string `json:"dateTime"`
					} `json:"start"`
				} `json:"dates"`
				Classifications []struct {
					Segment struct {
						Name string `json:"name"`
					} `json:"segment"`
				} `json:"classifications"`
				Embedded struct {
					Venues []struct {
						Name    string `json:"name"`
						Address struct {
							Line1 string `json:"line1"`
						} `json:"address"`
						City  struct{ Name string `json:"name"` } `json:"city"`
						State struct{ Name string `json:"name"` } `json:"stateCode"`
						Location struct {
							Lat string `json:"latitude"`
							Lng string `json:"longitude"`
						} `json:"location"`
					} `json:"venues"`
				} `json:"_embedded"`
				PriceRanges []struct {
					Min float64 `json:"min"`
					Max float64 `json:"max"`
				} `json:"priceRanges"`
			} `json:"events"`
		} `json:"_embedded"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	events := make([]models.Event, 0)
	for _, e := range result.Embedded.Events {
		event := models.Event{
			ID:     e.ID,
			Title:  e.Name,
			URL:    e.URL,
			Source: "ticketmaster",
		}
		if len(e.Images) > 0 {
			event.ImageURL = e.Images[0].URL
		}
		if len(e.Classifications) > 0 {
			event.Category = e.Classifications[0].Segment.Name
		}
		if len(e.PriceRanges) > 0 {
			event.Price = models.Price{
				Min:      e.PriceRanges[0].Min,
				Max:      e.PriceRanges[0].Max,
				Currency: "USD",
				Free:     e.PriceRanges[0].Min == 0,
			}
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchEventbrite fetches from Eventbrite API
func (a *Aggregator) fetchEventbrite(params models.SearchParams) ([]models.Event, error) {
	if a.eventbriteKey == "" {
		return []models.Event{}, nil
	}

	url := fmt.Sprintf(
		"https://www.eventbriteapi.com/v3/events/search/?location.latitude=%f&location.longitude=%f&location.within=%dmi&expand=venue,ticket_availability",
		params.Lat, params.Lng, int(params.Radius),
	)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+a.eventbriteKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Events []struct {
			ID          string `json:"id"`
			Name        struct{ Text string `json:"text"` } `json:"name"`
			Description struct{ Text string `json:"text"` } `json:"description"`
			URL         string `json:"url"`
			Start       struct{ UTC string `json:"utc"` } `json:"start"`
			End         struct{ UTC string `json:"utc"` } `json:"end"`
			IsFree      bool   `json:"is_free"`
			Logo        struct{ URL string `json:"url"` } `json:"logo"`
			Venue       struct {
				Name    string `json:"name"`
				Address struct {
					LocalizedAreaDisplay string  `json:"localized_area_display"`
					City                 string  `json:"city"`
					Region               string  `json:"region"`
					Latitude             float64 `json:"latitude"`
					Longitude            float64 `json:"longitude"`
				} `json:"address"`
			} `json:"venue"`
		} `json:"events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	events := make([]models.Event, 0)
	for _, e := range result.Events {
		startTime, _ := time.Parse(time.RFC3339, e.Start.UTC)
		endTime, _ := time.Parse(time.RFC3339, e.End.UTC)

		events = append(events, models.Event{
			ID:          e.ID,
			Title:       e.Name.Text,
			Description: e.Description.Text,
			StartTime:   startTime,
			EndTime:     endTime,
			URL:         e.URL,
			ImageURL:    e.Logo.URL,
			Source:      "eventbrite",
			Price:       models.Price{Free: e.IsFree, Currency: "USD"},
			Venue: models.Venue{
				Name:  e.Venue.Name,
				City:  e.Venue.Address.City,
				State: e.Venue.Address.Region,
				Lat:   e.Venue.Address.Latitude,
				Lng:   e.Venue.Address.Longitude,
			},
		})
	}

	return events, nil
}

// mockEvents generates realistic demo events near the user
func (a *Aggregator) mockEvents(params models.SearchParams) []models.Event {
	now := time.Now()
	categories := []string{"Music", "Food & Drink", "Tech", "Sports", "Arts", "Networking", "Comedy", "Festival"}
	venues := []string{"Downtown Arena", "Riverside Park", "The Grand Hall", "City Center", "Rooftop Bar", "Community Center", "Jazz Club", "Art Gallery"}

	events := make([]models.Event, 0, 15)
	for i := 0; i < 15; i++ {
		latOffset := (rand.Float64() - 0.5) * 0.15
		lngOffset := (rand.Float64() - 0.5) * 0.15
		attending := rand.Intn(300) + 10
		isFree := rand.Float64() < 0.3
		price := models.Price{Free: isFree, Currency: "USD"}
		if !isFree {
			price.Min = float64(rand.Intn(50) + 5)
			price.Max = price.Min + float64(rand.Intn(50))
		}

		category := categories[rand.Intn(len(categories))]
		venueName := venues[rand.Intn(len(venues))]
		startTime := now.Add(time.Duration(rand.Intn(48)) * time.Hour)

		events = append(events, models.Event{
			ID:          fmt.Sprintf("mock-%d", i),
			Title:       mockTitle(category, i),
			Description: mockDescription(category),
			StartTime:   startTime,
			EndTime:     startTime.Add(3 * time.Hour),
			Category:    category,
			Source:      "nearnow",
			URL:         "#",
			ImageURL:    fmt.Sprintf("https://picsum.photos/seed/%d/400/200", i+1),
			Attending:   attending,
			Price:       price,
			Venue: models.Venue{
				Name:    venueName,
				Address: fmt.Sprintf("%d Main St", rand.Intn(999)+1),
				City:    "Miami",
				State:   "FL",
				Lat:     params.Lat + latOffset,
				Lng:     params.Lng + lngOffset,
			},
		})
	}
	return events
}

func mockTitle(category string, i int) string {
	titles := map[string][]string{
		"Music":       {"Live Jazz Night", "Indie Rock Showcase", "DJ Set & Dance", "Open Mic Night", "Symphony Under Stars"},
		"Food & Drink": {"Craft Beer Festival", "Food Truck Rally", "Wine Tasting Evening", "Brunch Pop-Up", "Street Food Market"},
		"Tech":        {"Go Meetup Miami", "Startup Pitch Night", "AI & Future of Work", "Web3 Workshop", "Hackathon 2026"},
		"Sports":      {"5K Fun Run", "Beach Volleyball Tournament", "Yoga in the Park", "Cycling Club Ride", "Pickup Basketball"},
		"Arts":        {"Gallery Opening Night", "Street Art Tour", "Photography Workshop", "Pottery Class", "Film Screening"},
		"Networking":  {"Young Professionals Mixer", "Founders Dinner", "Creative Industries Meetup", "Tech Career Fair", "Women in Tech"},
		"Comedy":      {"Stand-up Comedy Night", "Improv Show", "Open Mic Comedy", "Comedy Club Showcase", "Roast Battle"},
		"Festival":    {"Cultural Heritage Festival", "Music & Arts Festival", "Night Market", "Block Party", "Summer Carnival"},
	}
	list := titles[category]
	return list[i%len(list)]
}

func mockDescription(category string) string {
	return fmt.Sprintf("Join us for an amazing %s event! Connect with locals, enjoy great vibes, and make memories. All are welcome.", category)
}
