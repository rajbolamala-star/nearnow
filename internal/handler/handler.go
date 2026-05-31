package handler

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rajbolamala-star/nearnow/internal/aggregator"
	"github.com/rajbolamala-star/nearnow/internal/cache"
	"github.com/rajbolamala-star/nearnow/internal/models"
)

type Handler struct {
	tmpl       *template.Template
	aggregator *aggregator.Aggregator
	cache      *cache.Cache
}

func New(templatesGlob string) (*Handler, error) {
	tmpl, err := template.ParseGlob(templatesGlob)
	if err != nil {
		return nil, err
	}
	return &Handler{
		tmpl:       tmpl,
		aggregator: aggregator.New(),
		cache:      cache.New(),
	}, nil
}

// Home serves the main UI
func (h *Handler) Home(c *gin.Context) {
	c.Status(http.StatusOK)
	if err := h.tmpl.ExecuteTemplate(c.Writer, "index.html", nil); err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
	}
}

// Events returns events as JSON
func (h *Handler) Events(c *gin.Context) {
	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil || lat == 0 {
		lat = 25.7617 // Miami default
	}
	lng, err := strconv.ParseFloat(c.Query("lng"), 64)
	if err != nil || lng == 0 {
		lng = -80.1918
	}
	radius, err := strconv.ParseFloat(c.Query("radius"), 64)
	if err != nil || radius == 0 {
		radius = 10
	}

	params := models.SearchParams{
		Lat:      lat,
		Lng:      lng,
		Radius:   radius,
		Category: c.Query("category"),
		FreeOnly: c.Query("free") == "true",
		Keyword:  c.Query("q"),
	}

	// Check cache first
	if events, hit := h.cache.Get(params); hit {
		c.JSON(http.StatusOK, models.EventsResponse{
			Events:   events,
			Total:    len(events),
			Source:   "cache",
			CacheHit: true,
		})
		return
	}

	// Fetch from sources
	events, err := h.aggregator.Search(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store in cache
	h.cache.Set(params, events)

	c.JSON(http.StatusOK, models.EventsResponse{
		Events:   events,
		Total:    len(events),
		Source:   "live",
		CacheHit: false,
	})
}

// Health returns liveness check
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
