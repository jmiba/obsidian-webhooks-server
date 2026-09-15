package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSetAuthCookieAllowsLocalHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	setAuthCookie(context, "session_token", "token", 3600, false)

	header := recorder.Header().Get("Set-Cookie")
	assert.Contains(t, header, "HttpOnly")
	assert.Contains(t, header, "SameSite=Lax")
	assert.False(t, strings.Contains(header, "; Secure"))
}

func TestSetAuthCookieRequiresHTTPSWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	setAuthCookie(context, "session_token", "token", 3600, true)

	header := recorder.Header().Get("Set-Cookie")
	assert.Contains(t, header, "; Secure")
	assert.Contains(t, header, "HttpOnly")
	assert.Contains(t, header, "SameSite=Lax")
}
