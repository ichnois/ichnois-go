<p align="center">
  <h1 align="center">Official Ichnois SDK for Go</h1>
  <p align="center">Real-time error tracking and telemetry monitoring for Go applications powered by <a href="https://ichnois-h2jw.onrender.com">Ichnois</a>.</p>
</p>

---

[![Go Reference](https://pkg.go.dev/badge/github.com/ichnois/ichnois-go.svg)](https://pkg.go.dev/github.com/ichnois/ichnois-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

`ichnois-go` is the official Go SDK for **[Ichnois](https://ichnois-h2jw.onrender.com)**. It captures unhandled panics, application errors, and performance traces, seamlessly streaming NDJSON envelopes to the Ichnois telemetry ingestion gateway.

## Features

- **Zero-Friction Ingestion**: Native path handling that resolves directly to `/api/{project_id}/envelope/`.
- **Pure Ichnois Experience**: Modern `ichnois` package namespace with zero visible third-party dependencies required in your code.
- **Dual-Wire Authentication**: Sends `X-Ichnois-Auth` and `X-Sentry-Auth` for maximum gateway compatibility.
- **Environment First**: Automatic configuration via `ICHNOIS_DSN`, `ICHNOIS_ENVIRONMENT`, and `ICHNOIS_RELEASE`.
- **Framework Ecosystem**: Built-in middleware for Gin, Echo, Fiber, Net/HTTP, Logrus, Zap, Slog, Zerolog, and Cron.
- **100% Drop-In Compatibility**: Supports alias importing (`import sentry "github.com/ichnois/ichnois-go"`) for existing codebases.

## Requirements

- Go 1.20 or newer.

## Installation

```bash
go get github.com/ichnois/ichnois-go@latest
```

## Quick Start

Initialize the SDK as early as possible in your `main()` function:

```go
package main

import (
	"context"
	"errors"
	"time"

	"github.com/ichnois/ichnois-go"
)

func main() {
	err := ichnois.Init(ichnois.ClientOptions{
		Dsn: "https://your-key@ichnois-h2jw.onrender.com/api/my-service",
		// Environment: "production",
		// Release:     "v1.0.0",
		// Debug:       false,
	})
	if err != nil {
		panic(err)
	}

	// Flush buffered events before exit
	defer ichnois.Flush(2 * time.Second)

	// Capture an arbitrary message
	ichnois.CaptureMessage(context.Background(), "Application started successfully")

	// Capture an error
	if err := doRiskyOperation(); err != nil {
		ichnois.CaptureException(context.Background(), err)
	}
}

func doRiskyOperation() error {
	return errors.New("database connection failed")
}
```

## Panic Recovery

Protect goroutines or HTTP handlers using `ichnois.Recover()`:

```go
go func() {
	defer ichnois.Recover(context.Background(), nil)
	// Code that may panic
}()
```

## Environment Variables

If not explicitly defined in `ClientOptions`, the SDK automatically resolves:

| Variable | Description | Fallback |
| :--- | :--- | :--- |
| `ICHNOIS_DSN` | Ingestion endpoint URL | `SENTRY_DSN` |
| `ICHNOIS_ENVIRONMENT` | Target environment (e.g. `production`) | `SENTRY_ENVIRONMENT` |
| `ICHNOIS_RELEASE` | Application release version / commit SHA | `SENTRY_RELEASE` |
| `ICHNOISGODEBUG` | Comma-separated debug options (`httpdump`, etc.) | `SENTRYGODEBUG` |

## Framework Middleware

`ichnois-go` includes first-class integrations for popular Go frameworks:

### Gin
```go
import (
	"github.com/gin-gonic/gin"
	ichnoisgin "github.com/ichnois/ichnois-go/gin"
)

r := gin.Default()
r.Use(ichnoisgin.New(ichnoisgin.Options{
	Repanic: true,
}))
```

### Echo
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
```go
import (
	"github.com/gofiber/fiber/v2"
	ichnoisfiber "github.com/ichnois/ichnois-go/fiber"
)

app := fiber.New()
app.Use(ichnoisfiber.New(ichnoisfiber.Options{}))
```

### Standard `net/http`
```go
import (
	"net/http"
	ichnoishttp "github.com/ichnois/ichnois-go/http"
)

handler := ichnoishttp.New(ichnoishttp.Options{}).Handle(myHandler)
http.ListenAndServe(":8080", handler)
```

## Migrating from Sentry

Existing projects can switch to Ichnois with a single import alias change:

```go
// Replace:
// import "github.com/getsentry/sentry-go"
// With:
import sentry "github.com/ichnois/ichnois-go"
```

All existing calls (`sentry.Init`, `sentry.CaptureException`, `sentry.WithScope`, etc.) work unchanged.

## Documentation

For the complete technical reference, framework guides, structured logging (Zap, Slog, Zerolog), goroutine panic recovery, and automated release instructions, see:
- 📖 [Complete Integration & Automation Guide](DOCUMENTATION.md)
- ⚙️ [Docs Folder Reference](docs/GUIDE.md)

## License

Licensed under the MIT License. See [LICENSE](LICENSE) for details.
