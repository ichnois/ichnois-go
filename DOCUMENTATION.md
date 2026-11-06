# Ichnois Go SDK — Complete Integration & Automation Guide

Official developer and operations documentation for **[Ichnois Go SDK](https://github.com/ichnois/ichnois-go)**.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Package Distribution & Go Module Proxy](#2-package-distribution--go-module-proxy)
3. [Automated Releases & CI/CD Pipeline](#3-automated-releases--cicd-pipeline)
4. [Connecting to Ichnois](#4-connecting-to-ichnois)
5. [Core Telemetry API](#5-core-telemetry-api)
   - [Capturing Messages & Exceptions](#capturing-messages--exceptions)
   - [Go 1.13+ Error Chains](#go-113-error-chains)
   - [Goroutine Panic Recovery](#goroutine-panic-recovery)
   - [Scope & Metadata (User, Tags, Context, Breadcrumbs)](#scope--metadata)
   - [Distributed Tracing & Custom Spans](#distributed-tracing--custom-spans)
6. [Framework Middleware](#6-framework-middleware)
   - [Gin](#gin)
   - [Echo](#echo)
   - [Fiber](#fiber)
   - [Standard net/http](#standard-nethttp)
   - [gRPC](#grpc)
7. [Structured Logging Integrations](#7-structured-logging-integrations)
   - [log/slog](#logslog)
   - [Uber Zap](#uber-zap)
   - [Zerolog](#zerolog)
   - [Logrus](#logrus)
8. [Migration from Sentry (1-Line Alias)](#8-migration-from-sentry-1-line-alias)
9. [Wire Protocol & Gateway Specification](#9-wire-protocol--gateway-specification)

---

## 1. Architecture Overview

`ichnois-go` connects Go services to the Ichnois ingestion gateway (`https://ichnois-h2jw.onrender.com`). It handles envelope packaging, automatic retry buffering, rate-limiting backoffs, stack trace unwinding, and distributed trace propagation.

```
┌────────────────────────────────────────────────────────┐
│                   Go Application                       │
│  (ichnois.CaptureException, Panic Recovery, Spans)     │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────┐
│                   ichnois-go Client                    │
│   - Scope resolution (User, Tags, Breadcrumbs)         │
│   - Event serialization (NDJSON Envelope)              │
│   - RingBuffer queue & flush synchronization           │
└──────────────────────────┬─────────────────────────────┘
                           │ POST /api/{project_id}/envelope/
                           │ Headers:
                           │   X-Ichnois-Auth: Ichnois ichnois_version=7...
                           │   X-Sentry-Auth: Sentry sentry_version=7...
                           │   Content-Type: application/x-sentry-envelope
                           ▼
┌────────────────────────────────────────────────────────┐
│              Ichnois Ingestion Gateway                 │
│         (https://ichnois-h2jw.onrender.com)            │
└────────────────────────────────────────────────────────┘
```

---

## 2. Package Distribution & Go Module Proxy

In the Go ecosystem, packages do not use manual upload forms. Instead, Go modules are **decentralized and Git-native**:

1. **Git Host**: [github.com/ichnois/ichnois-go](https://github.com/ichnois/ichnois-go)
2. **Go Module Proxy**: Google's global mirror `proxy.golang.org`
3. **Documentation Registry**: [pkg.go.dev/github.com/ichnois/ichnois-go](https://pkg.go.dev/github.com/ichnois/ichnois-go)

### Installing as a Developer
```bash
go get github.com/ichnois/ichnois-go@latest
```
or pin a specific release:
```bash
go get github.com/ichnois/ichnois-go@v1.0.0
```

---

## 3. Automated Releases & CI/CD Pipeline

The repository includes an automated GitHub Actions pipeline in `.github/workflows/release.yml`.

### How Releases are Automated:
Whenever a new Git tag following Semantic Versioning (e.g. `v1.0.1`, `v1.1.0`) is pushed to GitHub:

1. **Test Runner**: Runs all unit, integration, and E2E tests across root and all submodules.
2. **GitHub Release**: Automatically drafts and publishes a GitHub Release with auto-generated release notes and commit summaries.
3. **Go Proxy Warming**: Pings `https://proxy.golang.org` to immediately index and cache the new version worldwide.

### Releasing a New Version (Step-by-Step):

```bash
# 1. Ensure you are on main and all changes are committed
git checkout main
git pull origin main

# 2. Create an annotated git tag
git tag -a v1.0.1 -m "Release v1.0.1: Added feature X"

# 3. Push main and the new tag
git push origin main --tags
```

The GitHub Actions workflow will execute automatically. You can monitor the progress under the **Actions** tab on GitHub:
`https://github.com/ichnois/ichnois-go/actions`

---

## 4. Connecting to Ichnois

### Direct Initialization
```go
package main

import (
	"context"
	"time"

	"github.com/ichnois/ichnois-go"
)

func main() {
	err := ichnois.Init(ichnois.ClientOptions{
		Dsn:              "https://your-key@ichnois-h2jw.onrender.com/api/my-service",
		Environment:      "production",
		Release:          "v1.0.0",
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		Debug:            false, // Set to true for verbose [Ichnois] logging
	})
	if err != nil {
		panic(err)
	}

	// ALWAYS call Flush before terminating your application
	defer ichnois.Flush(2 * time.Second)
}
```

### Zero-Config via Environment Variables
If deployed on Docker, Kubernetes, or Render, set the environment variables:

| Variable | Description | Example |
| :--- | :--- | :--- |
| `ICHNOIS_DSN` | Ingestion endpoint URL | `https://key@ichnois-h2jw.onrender.com/api/my-service` |
| `ICHNOIS_ENVIRONMENT` | Environment tag | `production`, `staging` |
| `ICHNOIS_RELEASE` | Git commit or release tag | `v1.0.0`, `git rev-parse HEAD` |
| `ICHNOISGODEBUG` | Optional debug flags | `httpdump=1` |

Then call:
```go
ichnois.Init(ichnois.ClientOptions{})
```

---

## 5. Core Telemetry API

### Capturing Messages & Exceptions
```go
ctx := context.Background()

// 1. Capture Message
ichnois.CaptureMessage(ctx, "Cron job completed successfully")

// 2. Capture Error
if err := processOrder(); err != nil {
	ichnois.CaptureException(ctx, err)
}
```

### Go 1.13+ Error Chains
Nested errors wrapped using `fmt.Errorf("...: %w", err)` are fully unwrapped into a hierarchical exception chain with stack traces:

```go
rootErr := errors.New("connection reset by peer")
dbErr := fmt.Errorf("database query failed: %w", rootErr)
serviceErr := fmt.Errorf("user authentication failed: %w", dbErr)

ichnois.CaptureException(ctx, serviceErr)
```

### Goroutine Panic Recovery
Standard goroutines crash the entire Go runtime if a panic is unhandled. Intercept and capture panics using `ichnois.Recover()`:

```go
go func() {
	defer func() {
		if r := recover(); r != nil {
			ichnois.Recover(context.Background(), r)
		}
	}()

	// Background worker logic that might panic
	riskyCalculation()
}()
```

### Scope & Metadata
```go
ctx, scope := ichnois.WithScope(context.Background())

// User context
scope.SetUser(ichnois.User{
	ID:        "usr_1002",
	Email:     "developer@example.com",
	Username:  "dev1002",
	IPAddress: "10.0.0.1",
})

// Searchable tags
scope.SetTag("tier", "enterprise")
scope.SetTag("server_cluster", "us-east-cluster-3")

// Structured custom context
scope.SetContext("order_details", map[string]interface{}{
	"order_id":  "ord_9981",
	"items":     3,
	"total_usd": 89.50,
})

// Chronological breadcrumbs
ichnois.AddBreadcrumb(ctx, &ichnois.Breadcrumb{
	Category:  "payment",
	Message:   "Payment token generated",
	Level:     ichnois.LevelInfo,
	Timestamp: time.Now(),
})

// All subsequent captures within this ctx will include the above metadata
ichnois.CaptureException(ctx, err)
```

### Distributed Tracing & Custom Spans
```go
// 1. Root Transaction Span
rootSpan := ichnois.StartSpan(context.Background(), "http.server",
	ichnois.WithTransactionName("POST /api/v1/checkout"),
)
defer rootSpan.Finish()

// 2. Nested Child Span
dbSpan := ichnois.StartSpan(rootSpan.Context(), "db.query")
dbSpan.Description = "INSERT INTO orders (user_id, amount) VALUES (?, ?)"
time.Sleep(10 * time.Millisecond) // Simulated query
dbSpan.Status = ichnois.SpanStatusOK
dbSpan.Finish()
```

---

## 6. Framework Middleware

### Gin
```bash
go get github.com/ichnois/ichnois-go/gin@latest
```
```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/ichnois/ichnois-go"
	ichnoisgin "github.com/ichnois/ichnois-go/gin"
)

func main() {
	ichnois.Init(ichnois.ClientOptions{
		Dsn: "https://key@ichnois-h2jw.onrender.com/api/my-service",
	})

	r := gin.Default()
	r.Use(ichnoisgin.New(ichnoisgin.Options{
		Repanic: true,
	}))

	r.GET("/api/users", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.Run(":8080")
}
```

### Echo
```bash
go get github.com/ichnois/ichnois-go/echo@latest
```
```go
import (
	"github.com/labstack/echo/v4"
	ichnoisecho "github.com/ichnois/ichnois-go/echo"
)

e := echo.New()
e.Use(ichnoisecho.New(ichnoisecho.Options{
	Repanic: true,
}))
```

### Fiber
```bash
go get github.com/ichnois/ichnois-go/fiber@latest
```
```go
import (
	"github.com/gofiber/fiber/v2"
	ichnoisfiber "github.com/ichnois/ichnois-go/fiber"
)

app := fiber.New()
app.Use(ichnoisfiber.New(ichnoisfiber.Options{}))
```

### Standard `net/http`
```bash
go get github.com/ichnois/ichnois-go/http@latest
```
```go
import (
	"net/http"
	ichnoishttp "github.com/ichnois/ichnois-go/http"
)

handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello from Ichnois"))
})

http.ListenAndServe(":8080", ichnoishttp.New(ichnoishttp.Options{}).Handle(handler))
```

---

## 7. Structured Logging Integrations

### `log/slog` (Go Standard Library)
```bash
go get github.com/ichnois/ichnois-go/slog@latest
```
```go
import (
	"log/slog"
	ichnoisslog "github.com/ichnois/ichnois-go/slog"
)

logger := slog.New(ichnoisslog.NewHandler(slog.Default().Handler()))
slog.SetDefault(logger)

slog.Error("Database query timed out", "query_id", "q-4412", "duration_ms", 502)
```

### Uber Zap
```bash
go get github.com/ichnois/ichnois-go/zap@latest
```
```go
import (
	"go.uber.org/zap"
	ichnoiszap "github.com/ichnois/ichnois-go/zap"
)

core := ichnoiszap.NewCore(zap.ErrorLevel)
logger := zap.New(core)
logger.Error("Failed to dispatch webhook")
```

---

## 8. Migration from Sentry (1-Line Alias)

Existing Go applications using `getsentry/sentry-go` can migrate in seconds without rewriting any function calls:

```go
// Replace:
// import "github.com/getsentry/sentry-go"
// With:
import sentry "github.com/ichnois/ichnois-go"
```

All functions (`sentry.Init`, `sentry.CaptureException`, `sentry.CaptureMessage`, `sentry.WithScope`, `sentry.Flush`) work identically.

---

## 9. Wire Protocol & Gateway Specification

When transmitting telemetry, `ichnois-go` sends newline-delimited JSON (NDJSON) envelopes to:

```
POST https://ichnois-h2jw.onrender.com/api/{project_id}/envelope/
```

### HTTP Headers Emitted:
```http
POST /api/{project_id}/envelope/ HTTP/1.1
Host: ichnois-h2jw.onrender.com
User-Agent: ichnois.go/1.0.0
Content-Type: application/x-sentry-envelope
X-Ichnois-Auth: Ichnois ichnois_version=7, ichnois_client=ichnois.go/1.0.0, ichnois_key={PUBLIC_KEY}
X-Sentry-Auth: Sentry sentry_version=7, sentry_client=ichnois.go/1.0.0, sentry_key={PUBLIC_KEY}
```

### Envelope Payload Structure:
```
{"event_id":"3ede7d7d65064c15810dc8a3fdd36395","sent_at":"2026-11-06T10:01:39Z","sdk":{"name":"ichnois.go","version":"1.0.0"}}
{"type":"event","length":842}
{"message":"Hello from Ichnois","level":"info","platform":"go","timestamp":1793962899.12}
```

---

## 10. License

Licensed under the [MIT License](../LICENSE).
