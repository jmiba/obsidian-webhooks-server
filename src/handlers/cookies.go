package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func setAuthCookie(c *gin.Context, name, value string, maxAge int, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", "", secure, true)
}
