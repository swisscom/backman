package router

import (
	"crypto/subtle"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swisscom/backman/config"
	"github.com/swisscom/backman/router/api"
	"github.com/swisscom/backman/router/ui"
	"go.elastic.co/apm/module/apmechov4"
)

type Router struct {
	echo *echo.Echo
	api  *api.Handler
	ui   *ui.Handler
}

func New() *Router {
	// setup basic echo configuration
	e := echo.New()
	e.DisableHTTP2 = true
	e.HideBanner = true
	e.HidePort = true

	// middlewares
	if config.Get().EnableAPM == true {
		e.Use(apmechov4.Middleware())
	}
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Secure())

	// don't show timestamp unless specifically configured
	format := `remote_ip="${remote_ip}", host="${host}", method=${method}, uri=${uri}, user_agent="${user_agent}", ` +
		`status=${status}, error="${error}", latency_human="${latency_human}", bytes_out=${bytes_out}` + "\n"
	if config.Get().LoggingTimestamp {
		format = `time="${time_rfc3339}", ` + format
	}
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: format,
	}))

	//e.Use(middleware.Recover()) // don't recover, let platform deal with panics
	e.Use(middleware.Static("static"))

	// secure whole app with HTTP BasicAuth
	username := config.Get().Username
	password := config.Get().Password
	e.Use(middleware.BasicAuth(func(u, p string, c echo.Context) (bool, error) {
		if subtle.ConstantTimeCompare([]byte(u), []byte(username)) == 1 && subtle.ConstantTimeCompare([]byte(p), []byte(password)) == 1 {
			return true, nil
		}
		return false, nil
	}))

	// setup router
	r := &Router{
		echo: e,
		api:  api.New(),
		ui:   ui.New(),
	}

	if !config.Get().DisableMetrics {
		// setup Prometheus endpoint
		r.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	}
	if !config.Get().DisableWeb {
		// setup API routes
		r.api.RegisterRoutes(r.echo)
		// setup Web-UI routes
		r.ui.RegisterRoutes(r.echo)
		// setup Web-UI rendering
		r.ui.RegisterRenderer(r.echo)
	}

	return r
}

func (r *Router) Start() error {
	return r.echo.Start(fmt.Sprintf(":%d", r.api.App.Port))
}
