package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"log"

	"github.com/go-redis/redis/v8"
	"github.com/grafana/loki/pkg/client/simpleclient"
)

const (
	defaultRateLimit = 10                                       // Default rate limit per IP address per minute
	rateLimitTTL     = 60 * time.Second                         // Rate limit expiration time
	lokiURL          = "http://localhost:3100/loki/api/v1/push" // Loki URL
)

type rateLimiter struct {
	sync.Mutex
	redisClient *redis.Client
	lokiClient  simpleclient.Client
	rateLimit   map[string]int
}

func newRateLimiter(redisClient *redis.Client, lokiClient simpleclient.Client) *rateLimiter {
	return &rateLimiter{
		redisClient: redisClient,
		lokiClient:  lokiClient,
		rateLimit:   make(map[string]int),
	}
}

func (rl *rateLimiter) checkAndConsumeRequest(ip string) bool {
	rl.Lock()
	defer rl.Unlock()

	// Check if the request count for the IP address exceeds the rate limit
	requestCount, err := rl.redisClient.Get(fmt.Sprintf("ip_requests:%s", ip)).Int()
	if err != nil && err != redis.Nil {
		log.Printf("Error checking request count for IP %s: %v", ip, err)
		return false
	}

	if requestCount >= defaultRateLimit {
		log.Printf("Request from IP %s exceeded rate limit", ip)
		return false
	}

	// Increment the request count for the IP address
	err = rl.redisClient.Incr(fmt.Sprintf("ip_requests:%s", ip)).Err()
	if err != nil {
		log.Printf("Error incrementing request count for IP %s: %v", ip, err)
		return false
	}

	// Set the expiration time for the request count
	err = rl.redisClient.Expire(fmt.Sprintf("ip_requests:%s", ip), rateLimitTTL).Err()
	if err != nil {
		log.Printf("Error setting expiration for request count for IP %s: %v", ip, err)
		return false
	}

	return true
}

func (rl *rateLimiter) updateRateLimit(ip string, newRateLimit int) error {
	rl.Lock()
	defer rl.Unlock()

	// Update the rate limit for the IP address
	rl.rateLimit[ip] = newRateLimit
	log.Printf("Rate limit updated to %d for IP %s", newRateLimit, ip)

	return nil
}

func handleDataRequest(rl *rateLimiter, w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr

	// Check if the request exceeds the rate limit
	if !rl.checkAndConsumeRequest(ip) {
		http.Error(w, "Too many requests from your IP address. Please try again later.", http.StatusTooManyRequests)
		return
	}

	// Process the data request and return the response
	fmt.Fprintf(w, "Data response for IP address: %s", ip)

	// Send a log entry to Loki
	entry := simpleclient.Entry{
		Labels:    "app=rate-limiter",
		Line:      fmt.Sprintf("Data request processed for IP %s", ip),
		Timestamp: time.Now(),
	}

	err := rl.lokiClient.Push(entry)
	if err != nil {
		log.Printf("Error sending log entry to Loki: %v", err)
	}
}

func handleRateLimitRequest(rl *rateLimiter, w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr
	newRateLimit := r.URL.Query().Get("rate_limit")

	// Convert newRateLimit to integer
	newRateLimitInt, err := strconv.Atoi(newRateLimit)
	if err != nil {
		http.Error(w, "Invalid rate limit value", http.StatusBadRequest)
		return
	}

	// Update the rate limit for the IP address
	err = rl.updateRateLimit(ip, newRateLimitInt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Rate limit update to %d for IP %s", newRateLimitInt, ip)
}

func main() {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Initialize Loki client
	lokiClient, err := simpleclient.New(lokiURL, "rate-limiter")
	if err != nil {
		log.Fatal("Error initializing Loki client:", err)
	}

	// Create rate limiter
	rateLimiter := newRateLimiter(redisClient, lokiClient)

	// Define HTTP routes
	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {

		handleDataRequest(rateLimiter, w, r)

	})

	http.HandleFunc("/data/rate_limit", func(w http.ResponseWriter, r *http.Request) {
		handleRateLimitRequest(rateLimiter, w, r)
	})

	// Start the HTTP server
	fmt.Println("Starting HTTP server on port 8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
