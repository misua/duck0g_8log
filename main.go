package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var requestTimestamps = make(map[string]map[string]interface{})
var defaultLimit = 2
var defaultWindow = 60

func isRateLimited(ipAddress string, limit int, window int) bool {
	currentTime := time.Now()

	if _, ok := requestTimestamps[ipAddress]; !ok {
		requestTimestamps[ipAddress] = make(map[string]interface{})
		requestTimestamps[ipAddress]["timestamps"] = []time.Time{}
		return false
	}

	if timestamps, ok := requestTimestamps[ipAddress]["timestamps"].([]time.Time); ok {
		// Remove timestamps that are outside the time window
		var newTimestamps []time.Time
		for _, t := range timestamps {
			if currentTime.Sub(t) <= time.Second*time.Duration(window) {
				newTimestamps = append(newTimestamps, t)
			}
		}
		requestTimestamps[ipAddress]["timestamps"] = newTimestamps

		if len(requestTimestamps[ipAddress]["timestamps"].([]time.Time)) < limit {
			requestTimestamps[ipAddress]["timestamps"] = append(timestamps, currentTime)
			return false
		}
	}
	return true
}

func getLimitAndWindow(ipAddress string) (int, int) {
	limit, ok := requestTimestamps[ipAddress]["limit"].(int)
	if !ok {
		limit = defaultLimit
	}

	window, ok := requestTimestamps[ipAddress]["window"].(int)
	if !ok {
		window = defaultWindow
	}

	return limit, window
}

func getResource(c *gin.Context) {
	clientIP := c.ClientIP()
	limit, window := getLimitAndWindow(clientIP)

	if !isRateLimited(clientIP, limit, window) {
		c.JSON(http.StatusOK, gin.H{"data": "This is a rate-limited resource."})
	} else {
		log.Println("You have reached a limit")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded."})
	}
}

func changeLimit(c *gin.Context) {
	clientIP := c.ClientIP()
	newLimit := c.PostForm("limit")
	newWindow := c.PostForm("window")

	if _, ok := requestTimestamps[clientIP]; !ok {
		requestTimestamps[clientIP] = make(map[string]interface{})
		requestTimestamps[clientIP]["timestamps"] = []time.Time{}
	}

	if newLimit != "" && newWindow != "" {
		requestTimestamps[clientIP]["limit"] = newLimit
		requestTimestamps[clientIP]["window"] = newWindow
		requestTimestamps[clientIP]["timestamps"] = []time.Time{}
		c.JSON(http.StatusOK, gin.H{"data": "Rate limit has been changed."})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body."})
	}
}

func main() {
	router := gin.Default()

	router.GET("/api/resource", getResource)
	router.POST("/api/limit", changeLimit)

	if err := router.Run(":5000"); err != nil {
		fmt.Println(err)
	}
}
