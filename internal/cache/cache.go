package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rajbolamala-star/nearnow/internal/models"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

func New() *Cache {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("cache: invalid redis url, running without cache: %v", err)
		return &Cache{}
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("cache: redis unavailable, running without cache: %v", err)
		return &Cache{}
	}

	log.Println("cache: connected to redis")
	return &Cache{client: client, ttl: 5 * time.Minute}
}

func (c *Cache) Get(params models.SearchParams) ([]models.Event, bool) {
	if c.client == nil {
		return nil, false
	}

	key := c.key(params)
	ctx := context.Background()

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var events []models.Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, false
	}

	log.Printf("cache: hit for key %s", key)
	return events, true
}

func (c *Cache) Set(params models.SearchParams, events []models.Event) {
	if c.client == nil {
		return
	}

	key := c.key(params)
	ctx := context.Background()

	data, err := json.Marshal(events)
	if err != nil {
		return
	}

	c.client.Set(ctx, key, data, c.ttl)
	log.Printf("cache: set key %s (%d events)", key, len(events))
}

func (c *Cache) key(params models.SearchParams) string {
	return fmt.Sprintf("nearnow:%.4f:%.4f:%.1f:%s:%v",
		params.Lat, params.Lng, params.Radius, params.Category, params.FreeOnly)
}
