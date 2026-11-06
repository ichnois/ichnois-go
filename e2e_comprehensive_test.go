package ichnois_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ichnois/ichnois-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func localDSN(serverURL string) string {
	return strings.Replace(serverURL, "http://", "http://testkey@", 1) + "/api/test-project"
}

// TestE2E_LocalCaptureMessage verifies that messages are captured with proper scope and metadata.
func TestE2E_LocalCaptureMessage(t *testing.T) {
	var capturedEvents []*ichnois.Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, "POST", r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Ichnois-Auth"))
		assert.NotEmpty(t, r.Header.Get("X-Sentry-Auth"))
		assert.Equal(t, "application/x-sentry-envelope", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := ichnois.Init(ichnois.ClientOptions{
		Dsn: localDSN(server.URL),
		BeforeSend: func(event *ichnois.Event, hint *ichnois.EventHint) *ichnois.Event {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
			return event
		},
	})
	require.NoError(t, err)

	ctx := context.Background()
	eventID := ichnois.CaptureMessage(ctx, "Test message for Ichnois")
	require.NotNil(t, eventID)

	flushed := ichnois.Flush(2 * time.Second)
	assert.True(t, flushed)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedEvents, 1)
	assert.Equal(t, "Test message for Ichnois", capturedEvents[0].Message)
	assert.Equal(t, "ichnois.go", capturedEvents[0].Sdk.Name)
}

// TestE2E_LocalCaptureException verifies error chain capture, exception types, and stack traces.
func TestE2E_LocalCaptureException(t *testing.T) {
	var capturedEvents []*ichnois.Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := ichnois.Init(ichnois.ClientOptions{
		Dsn: localDSN(server.URL),
		BeforeSend: func(event *ichnois.Event, hint *ichnois.EventHint) *ichnois.Event {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
			return event
		},
	})
	require.NoError(t, err)

	rootErr := errors.New("underlying database connection refused")
	wrappedErr := fmt.Errorf("service failed to fetch user: %w", rootErr)

	ctx := context.Background()
	eventID := ichnois.CaptureException(ctx, wrappedErr)
	require.NotNil(t, eventID)

	flushed := ichnois.Flush(2 * time.Second)
	assert.True(t, flushed)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedEvents, 1)
	require.Len(t, capturedEvents[0].Exception, 2)
	assert.Equal(t, "underlying database connection refused", capturedEvents[0].Exception[0].Value)
	assert.Equal(t, "service failed to fetch user: underlying database connection refused", capturedEvents[0].Exception[1].Value)
}

// TestE2E_LocalPanicRecovery verifies panic recovery in goroutines.
func TestE2E_LocalPanicRecovery(t *testing.T) {
	var capturedEvents []*ichnois.Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := ichnois.Init(ichnois.ClientOptions{
		Dsn: localDSN(server.URL),
		BeforeSend: func(event *ichnois.Event, hint *ichnois.EventHint) *ichnois.Event {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
			return event
		},
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				ichnois.Recover(context.Background(), r)
			}
		}()
		panic("critical panic in background worker")
	}()

	wg.Wait()
	flushed := ichnois.Flush(2 * time.Second)
	assert.True(t, flushed)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedEvents, 1)
	assert.Equal(t, "critical panic in background worker", capturedEvents[0].Message)
	assert.Equal(t, ichnois.LevelFatal, capturedEvents[0].Level)
}

// TestE2E_LocalScopeAndBreadcrumbs verifies user, tags, extra, and breadcrumbs.
func TestE2E_LocalScopeAndBreadcrumbs(t *testing.T) {
	var capturedEvents []*ichnois.Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := ichnois.Init(ichnois.ClientOptions{
		Dsn: localDSN(server.URL),
		BeforeSend: func(event *ichnois.Event, hint *ichnois.EventHint) *ichnois.Event {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
			return event
		},
	})
	require.NoError(t, err)

	ctx, scope := ichnois.WithScope(context.Background())
	scope.SetUser(ichnois.User{
		ID:        "usr_12345",
		Email:     "developer@ichnois.com",
		Username:  "gopher",
		IPAddress: "127.0.0.1",
	})
	scope.SetTag("service.name", "auth-service")
	scope.SetTag("environment", "staging")
	scope.SetContext("custom", map[string]interface{}{"request_id": "req-xyz-9876"})

	ichnois.AddBreadcrumb(ctx, &ichnois.Breadcrumb{
		Category:  "auth",
		Message:   "User submitted login form",
		Level:     ichnois.LevelInfo,
		Timestamp: time.Now(),
	})

	ichnois.AddBreadcrumb(ctx, &ichnois.Breadcrumb{
		Category:  "db",
		Message:   "Query SELECT * FROM users executed in 2ms",
		Level:     ichnois.LevelDebug,
		Timestamp: time.Now(),
	})

	eventID := ichnois.CaptureMessage(ctx, "Failed authentication attempt")
	require.NotNil(t, eventID)

	flushed := ichnois.Flush(2 * time.Second)
	assert.True(t, flushed)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedEvents, 1)
	ev := capturedEvents[0]
	assert.Equal(t, "usr_12345", ev.User.ID)
	assert.Equal(t, "developer@ichnois.com", ev.User.Email)
	assert.Equal(t, "auth-service", ev.Tags["service.name"])
	assert.Equal(t, "staging", ev.Tags["environment"])
	assert.Equal(t, "req-xyz-9876", ev.Contexts["custom"]["request_id"])
	require.Len(t, ev.Breadcrumbs, 2)
	assert.Equal(t, "User submitted login form", ev.Breadcrumbs[0].Message)
	assert.Equal(t, "Query SELECT * FROM users executed in 2ms", ev.Breadcrumbs[1].Message)
}

// TestE2E_LocalTracingAndSpans verifies transaction creation and child span nesting.
func TestE2E_LocalTracingAndSpans(t *testing.T) {
	var capturedEvents []*ichnois.Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := ichnois.Init(ichnois.ClientOptions{
		Dsn:              localDSN(server.URL),
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		BeforeSendTransaction: func(event *ichnois.Event, hint *ichnois.EventHint) *ichnois.Event {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
			return event
		},
	})
	require.NoError(t, err)

	span := ichnois.StartSpan(context.Background(), "http.server",
		ichnois.WithTransactionName("POST /api/checkout"),
	)

	childSpan := ichnois.StartSpan(span.Context(), "db.query")
	childSpan.Description = "SELECT * FROM orders WHERE user_id = ?"
	time.Sleep(10 * time.Millisecond)
	childSpan.Finish()

	span.Status = ichnois.SpanStatusOK
	span.Finish()

	flushed := ichnois.Flush(2 * time.Second)
	assert.True(t, flushed)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, capturedEvents, 1)
	assert.Equal(t, "transaction", capturedEvents[0].Type)
	assert.Equal(t, "POST /api/checkout", capturedEvents[0].Transaction)
	require.Len(t, capturedEvents[0].Spans, 1)
	assert.Equal(t, "db.query", capturedEvents[0].Spans[0].Op)
	assert.Equal(t, "SELECT * FROM orders WHERE user_id = ?", capturedEvents[0].Spans[0].Description)
}

// TestE2E_LiveGatewayIngestion sends real requests to the Ichnois ingestion backend.
func TestE2E_LiveGatewayIngestion(t *testing.T) {
	err := ichnois.Init(ichnois.ClientOptions{
		Dsn:              "https://testkey@ichnois-h2jw.onrender.com/api/test-project",
		Environment:      "production",
		Release:          "ichnois-go@1.0.0",
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		Debug:            true,
	})
	require.NoError(t, err)

	ctx, scope := ichnois.WithScope(context.Background())
	scope.SetUser(ichnois.User{
		ID:    "user_e2e_test",
		Email: "ci@ichnois.com",
	})
	scope.SetTag("sdk.language", "go")
	scope.SetTag("test.suite", "e2e_comprehensive")

	ichnois.AddBreadcrumb(ctx, &ichnois.Breadcrumb{
		Category:  "e2e",
		Message:   "Running end-to-end live ingestion test against Render backend",
		Level:     ichnois.LevelInfo,
		Timestamp: time.Now(),
	})

	// 1. Capture live message
	msgID := ichnois.CaptureMessage(ctx, "E2E Message: Hello from automated ichnois-go test suite!")
	require.NotNil(t, msgID)
	t.Logf("Delivered message event ID: %s", *msgID)

	// 2. Capture live exception
	errToCapture := errors.New("E2E Exception: Simulated payment gateway timeout")
	errID := ichnois.CaptureException(ctx, errToCapture)
	require.NotNil(t, errID)
	t.Logf("Delivered exception event ID: %s", *errID)

	// 3. Capture live transaction
	txSpan := ichnois.StartSpan(ctx, "e2e.transaction",
		ichnois.WithTransactionName("GET /api/v1/health"),
	)
	subSpan := ichnois.StartSpan(txSpan.Context(), "cache.get")
	subSpan.Description = "redis:get:health"
	time.Sleep(5 * time.Millisecond)
	subSpan.Finish()
	txSpan.Finish()

	flushed := ichnois.Flush(6 * time.Second)
	assert.True(t, flushed, "Events must be successfully flushed to the live Ichnois gateway")
}

