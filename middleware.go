package cli

type MiddlewareFunc func(next func() error) error

func Use(middleware ...MiddlewareFunc) {
	app.middleware = append(app.middleware, middleware...)
}
