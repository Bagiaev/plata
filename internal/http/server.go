package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"plata/internal/apperror"
)

func NewServer(handler *Handler, logger *slog.Logger) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = errorHandler(logger)

	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:    true,
		LogRoutePath: true,
		LogStatus:    true,
		LogRequestID: true,
		LogLatency:   true,
		HandleError:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logger.InfoContext(c.Request().Context(), "HTTP request", "method", v.Method, "route", v.RoutePath,
				"status", v.Status, "request_id", v.RequestID, "latency", v.Latency)

			return nil
		},
	}))
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{DisablePrintStack: true}))
	e.Use(middleware.BodyLimit("4K"))
	e.Use(middleware.ContextTimeout(5 * time.Second))

	quotes := e.Group("/api/v1/quotes")
	quotes.POST("/updates", handler.create)
	quotes.GET("/updates/:id", handler.get)
	quotes.GET("/latest", handler.latest)

	e.GET("/health/live", handler.live)
	e.GET("/health/ready", handler.ready)

	return e
}

type errorResponse struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		status, detail := classifyError(err)
		ctx := c.Request().Context()
		requestID := c.Response().Header().Get(echo.HeaderXRequestID)

		if status >= 500 {
			logger.ErrorContext(ctx, "HTTP request failed", "request_id", requestID, "error", err)
		}

		if err := c.JSON(status, errorResponse{Error: detail}); err != nil {
			logger.WarnContext(ctx, "could not write HTTP error", "request_id", requestID, "error", err)
		}
	}
}

func classifyError(err error) (int, errorDetails) {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, apperror.ErrUpdateNotFound), errors.Is(err, apperror.ErrQuoteNotFound):
			status = http.StatusNotFound
		case errors.Is(err, apperror.ErrKeyConflict):
			status = http.StatusConflict
		}

		return status, errorDetails{appErr.Code, appErr.Message}
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Code {
		case http.StatusBadRequest:
			return httpErr.Code, errorDetails{"invalid_body", "request body must be a JSON object with currency codes"}
		case http.StatusNotFound:
			return httpErr.Code, errorDetails{"not_found", "route not found"}
		case http.StatusMethodNotAllowed:
			return httpErr.Code, errorDetails{"method_not_allowed", "method not allowed"}
		case http.StatusRequestEntityTooLarge:
			return httpErr.Code, errorDetails{"body_too_large", "request body exceeds 4 KiB"}
		case http.StatusUnsupportedMediaType:
			return httpErr.Code, errorDetails{"unsupported_media_type", "Content-Type must be application/json"}
		case http.StatusServiceUnavailable:
			return httpErr.Code, errorDetails{"database_unavailable", "database unavailable"}
		}
	}

	return http.StatusInternalServerError, errorDetails{"internal_error", "internal server error"}
}
