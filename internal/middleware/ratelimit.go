package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

func (r *RateLimiter) getLimiter(key string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[key]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if limiter, exists = r.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(r.rate, r.burst)
	r.limiters[key] = limiter
	return limiter
}

func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()

		limiter := r.getLimiter(key)
		if !limiter.Allow() {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMITED",
					"message": "Too many requests, please try again later",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func CleanupRateLimiters(rl *RateLimiter, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for key, limiter := range rl.limiters {
			if !limiter.AllowN(time.Now(), 1) {
				delete(rl.limiters, key)
			}
		}
		rl.mu.Unlock()
	}
}
