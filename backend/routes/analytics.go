package routes

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterAnalyticsRoutes registers all analytics-related endpoints.
func RegisterAnalyticsRoutes(g *echo.Group, reg *services.Registry) {
	g.GET("/analytics/composition", analyticsCompositionHandler(reg))
	g.GET("/analytics/quality", analyticsQualityHandler(reg))
	g.GET("/analytics/bloat", analyticsBloatHandler(reg))
	g.GET("/analytics/dead-content", analyticsDeadContentHandler(reg))
	g.GET("/analytics/stale-content", analyticsStaleContentHandler(reg))
	g.GET("/analytics/popularity", analyticsPopularityHandler(reg))
	g.GET("/analytics/request-fulfillment", analyticsRequestFulfillmentHandler(reg))
}

func analyticsCompositionHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.Analytics.GetComposition()
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsQualityHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.Analytics.GetQualityDistribution()
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsBloatHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.Analytics.GetSizeAnomalies()
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsDeadContentHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		minDays := 90
		if v := c.QueryParam("minDays"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				minDays = parsed
			}
		}
		data := reg.WatchAnalytics.GetDeadContent(minDays)
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsStaleContentHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		staleDays := 180
		if v := c.QueryParam("staleDays"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				staleDays = parsed
			}
		}
		data := reg.WatchAnalytics.GetStaleContent(staleDays)
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsPopularityHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.WatchAnalytics.GetPopularity()
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsRequestFulfillmentHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.WatchAnalytics.GetRequestFulfillment()
		return c.JSON(http.StatusOK, data)
	}
}
