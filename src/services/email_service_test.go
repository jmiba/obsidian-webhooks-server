package services

import (
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMessageCreatesMultipartAlternative(t *testing.T) {
	service := NewEmailService(SMTPConfig{
		FromEmail: "noreply@example.com",
		FromName:  "Obsidian Webhooks",
	})

	raw, err := service.buildMessage(
		"person@example.com",
		"Your login link — Obsidian Webhooks",
		"Plain login link",
		"<p>HTML login link</p>",
	)
	require.NoError(t, err)

	message, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)
	to, err := mail.ParseAddress(message.Header.Get("To"))
	require.NoError(t, err)
	assert.Equal(t, "person@example.com", to.Address)
	assert.Contains(t, message.Header.Get("From"), "noreply@example.com")
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	require.NoError(t, err)
	assert.Equal(t, "Your login link — Obsidian Webhooks", subject)

	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, "multipart/alternative", mediaType)

	reader := multipart.NewReader(message.Body, params["boundary"])
	var bodies []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		bodies = append(bodies, string(body))
	}
	require.Len(t, bodies, 2)
	assert.Contains(t, bodies[0], "Plain login link")
	assert.Contains(t, bodies[1], "<p>HTML login link</p>")
}

func TestBuildMessageRejectsHeaderInjection(t *testing.T) {
	service := NewEmailService(SMTPConfig{FromEmail: "noreply@example.com"})

	_, err := service.buildMessage("person@example.com\r\nBcc: attacker@example.com", "subject", "text", "html")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recipient email")
}

func TestEmailURLsUseConfiguredBaseURL(t *testing.T) {
	service := NewEmailService(SMTPConfig{BaseURL: "http://localhost:8070/"})
	config, err := templates.LoadEmailConfig("en")
	require.NoError(t, err)

	service.applyPublicURLs(config)

	assert.Equal(t, "http://localhost:8070", config.Branding.Website)
	assert.Equal(t, "http://localhost:8070/dashboard", config.Branding.DashboardURL)
	assert.Equal(t, "http://localhost:8070/guides/", config.Branding.DocsURL)
	assert.Contains(t, config.Welcome.HelpText, "http://localhost:8070/guides/")
	assert.Contains(t, config.Welcome.Steps[len(config.Welcome.Steps)-1], "http://localhost:8070/guides/")
	assert.NotContains(t, config.Welcome.HelpText, defaultPublicBaseURL)
}

func TestFallbackWelcomeEmailUsesConfiguredDashboard(t *testing.T) {
	service := NewEmailService(SMTPConfig{BaseURL: "http://localhost:8070"})

	assert.Contains(t, service.getWelcomeHTMLTemplate("there"), `href="http://localhost:8070/dashboard"`)
	assert.Contains(t, service.getWelcomePlainTextTemplate("there"), "Open your dashboard: http://localhost:8070/dashboard")
}
