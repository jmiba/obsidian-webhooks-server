package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/database"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/models"
	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/services"
)

func TestNormalizeWriteMode(t *testing.T) {
	tests := []struct {
		input string
		want  string
		valid bool
	}{
		{input: "", want: "", valid: true},
		{input: "create", want: "create", valid: true},
		{input: " APPEND ", want: "append", valid: true},
		{input: "overwrite", want: "overwrite", valid: true},
		{input: "frontmatter", want: "frontmatter", valid: true},
		{input: "delete", want: "", valid: false},
	}

	for _, tt := range tests {
		got, valid := normalizeWriteMode(tt.input)
		if got != tt.want || valid != tt.valid {
			t.Errorf("normalizeWriteMode(%q) = (%q, %v), want (%q, %v)",
				tt.input, got, valid, tt.want, tt.valid)
		}
	}
}

func TestFormatEventForSSEIncludesWriteMode(t *testing.T) {
	event := &models.Event{
		ID:        uuid.New(),
		Path:      "notes/test.md",
		Data:      []byte("status: done"),
		WriteMode: "frontmatter",
		CreatedAt: time.Now(),
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(formatEventForSSE(event)), &payload); err != nil {
		t.Fatalf("failed to parse event JSON: %v", err)
	}
	if payload["mode"] != "frontmatter" {
		t.Fatalf("expected frontmatter mode, got %v", payload["mode"])
	}
}

func TestHandleWebhook_Success(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		// Create test webhook key
		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		reqBody := []byte(`{"test": "data"}`)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path=/test/path&mode=frontmatter", bytes.NewReader(reqBody))
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["status"] != "ok" {
			t.Errorf("expected status 'ok', got %v", response["status"])
		}

		if response["event_id"] == nil || response["event_id"] == "" {
			t.Error("expected event_id to be set")
		}
		if response["mode"] != "frontmatter" {
			t.Errorf("expected mode frontmatter, got %v", response["mode"])
		}

		eventID, err := uuid.Parse(response["event_id"].(string))
		if err != nil {
			t.Fatalf("failed to parse event ID: %v", err)
		}
		storedEvent, err := eventService.GetEventByID(c.Request.Context(), eventID)
		if err != nil {
			t.Fatalf("failed to load stored event: %v", err)
		}
		if storedEvent.WriteMode != "frontmatter" {
			t.Errorf("expected stored mode frontmatter, got %q", storedEvent.WriteMode)
		}
	})
}

func TestHandleWebhook_MissingPath(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey, nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "path query parameter is required" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_PathTooLong(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		// Path longer than 512 characters
		longPath := strings.Repeat("a", 513)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path="+longPath, nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "path too long (max 512 characters)" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_PathTraversal(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path=../../etc/passwd", nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid path (path traversal not allowed)" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_InvalidWebhookKey(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/wh_invalid?path=/test", nil)
		c.Params = gin.Params{
			{Key: "webhook_key", Value: "wh_invalid"},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "invalid webhook key" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}

func TestHandleWebhook_PayloadTooLarge(t *testing.T) {
	database.WithTestDB(t, func(tdb *database.TestDB) {
		gin.SetMode(gin.TestMode)
		db := database.NewDatabaseFromPool(tdb.Pool)

		_, _, webhookKey, _, err := tdb.CreateTestKeyPair(123456, "testuser")
		if err != nil {
			t.Fatalf("failed to create test key pair: %v", err)
		}

		keyService := services.NewKeyService(db.GetPool())
		eventService := services.NewEventService(db.GetPool())
		handler := NewWebhookHandler(keyService, eventService, nil)

		// Create payload larger than 10MB
		largePayload := bytes.Repeat([]byte("a"), 11*1024*1024)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/webhook/"+webhookKey+"?path=/test", bytes.NewReader(largePayload))
		c.Params = gin.Params{
			{Key: "webhook_key", Value: webhookKey},
		}

		handler.HandleWebhook(c)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected status 413, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["error"] != "payload too large (max 10MB)" {
			t.Errorf("unexpected error: %v", response["error"])
		}
	})
}
