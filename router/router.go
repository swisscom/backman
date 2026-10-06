package router

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/swisscom/backman/config"
	"github.com/swisscom/backman/router/api"
	"github.com/swisscom/backman/router/health"
	"github.com/swisscom/backman/router/metrics"
	"github.com/swisscom/backman/router/ui"
)

type Router struct {
	echo    *echo.Echo
	health  *health.Handler
	metrics *metrics.Handler
	api     *api.Handler
	ui      *ui.Handler
}

func New() *Router {
	// setup basic echo configuration
	e := echo.New()
	e.DisableHTTP2 = true
	e.HideBanner = true
	e.HidePort = true

	// middlewares
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Secure())
	//e.Use(middleware.Recover()) // don't recover, let platform deal with panics
	e.Use(middleware.Static("static"))

	// setup router
	r := &Router{
		echo:    e,
		health:  health.New(),
		metrics: metrics.New(),
		api:     api.New(),
		ui:      ui.New(),
	}

	// setup health route
	r.health.RegisterRoutes(r.echo)

	if !config.Get().DisableMetrics {
		// setup metrics route
		r.metrics.RegisterRoutes(r.echo)
	}

	if !config.Get().DisableWeb {
		// setup API routes
		r.api.RegisterRoutes(r.echo)

		if !config.Get().DisableUI {
			// setup Web-UI routes
			r.ui.RegisterRoutes(r.echo)
			// setup Web-UI rendering
			r.ui.RegisterRenderer(r.echo)
		}
	}

	return r
}

func (r *Router) Start() error {
	return r.echo.Start(fmt.Sprintf(":%d", config.Get().Port))
}
