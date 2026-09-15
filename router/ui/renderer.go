package ui

import (
	"io"
	"strings"
	"html/template"

	"github.com/dustin/go-humanize"
	"github.com/hako/durafmt"
	"github.com/labstack/echo/v4"
	"github.com/swisscom/backman/config"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func (h *Handler) RegisterRenderer(e *echo.Echo) {
	funcMap := template.FuncMap{
		"ToLower":     strings.ToLower,
		"ServiceType": config.ParseServiceType,
		"Duration":    durafmt.Parse,
		"Bytes":       func(b int64) string { return humanize.Bytes(uint64(b)) },
		"Time":        humanize.Time,
		"Config":      config.Get,
	}
	renderer := &TemplateRenderer{
		templates: template.Must(template.New("backman").Funcs(funcMap).ParseGlob("public/*.html")),
	}
	e.Renderer = renderer
}
