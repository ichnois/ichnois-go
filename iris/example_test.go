package sentryiris_test

import (
	"context"
	"net/http"

	sentry "github.com/ichnois/ichnois-go"
	sentryiris "github.com/ichnois/ichnois-go/iris"
	"github.com/kataras/iris/v12"
)

func Example() {
	app := iris.New()
	app.Use(sentryiris.New(sentryiris.Options{}))
	app.Get("/", func(ctx iris.Context) {
		expensiveThing := func(ctx context.Context) {
			span := sentry.StartSpan(ctx, "expensive_thing")
			defer span.Finish()
			// do resource intensive thing
		}

		expensiveThing(ctx.Request().Context())

		ctx.StatusCode(http.StatusOK)
	})
}
