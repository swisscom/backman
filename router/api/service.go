package api

import (
	"fmt"
	"net/http"
	"net/url"

	echo "github.com/labstack/echo/v4"
	"github.com/swisscom/backman/log"
)

// swagger:route GET /api/v1/services service listServices
// Lists all services managed by backman.
//
// produces:
// - application/json
//
// schemes: http, https
//
// responses:
//   200: services
func (h *Handler) ListServices(c echo.Context) error {
	serviceType := c.QueryParam("service_type")
	serviceName, err := url.QueryUnescape(c.Param("service_name"))
	if err != nil {
		log.Errorf("%v", err)
		return c.JSON(http.StatusBadRequest, fmt.Sprintf("invalid service name: %v", err))
	}

	return c.JSON(http.StatusOK, h.Service.GetServices(serviceType, serviceName))
}
