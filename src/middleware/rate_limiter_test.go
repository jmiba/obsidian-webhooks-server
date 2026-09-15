package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthRateLimiterAllowsConfiguredBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthRateLimitMiddleware(RateLimitConfig{
		RequestsPerMinute: 3,
		Burst:             3,
	}))
	router.POST("/auth/request-login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for requestNumber := 1; requestNumber <= 3; requestNumber++ {
		request := httptest.NewRequest(http.MethodPost, "/auth/request-login", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Equal(t, http.StatusOK, response.Code, "request %d", requestNumber)
	}

	request := httptest.NewRequest(http.MethodPost, "/auth/request-login", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusTooManyRequests, response.Code)
	assert.Equal(t, "20", response.Header().Get("Retry-After"))

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "20s", body["retry_after"])
}

func TestAuthRateLimiterSeparatesClientIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthRateLimitMiddleware(RateLimitConfig{
		RequestsPerMinute: 1,
		Burst:             1,
	}))
	router.POST("/auth/request-login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, remoteAddress := range []string{"192.0.2.1:12345", "192.0.2.2:12345"} {
		request := httptest.NewRequest(http.MethodPost, "/auth/request-login", nil)
		request.RemoteAddr = remoteAddress
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Equal(t, http.StatusOK, response.Code)
	}
}
