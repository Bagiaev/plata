package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"

	"plata/internal/apperror"
	"plata/internal/model"
	"plata/internal/service"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	service *service.Service
	db      Pinger
}

func NewHandler(service *service.Service, db Pinger) *Handler {
	return &Handler{service: service, db: db}
}

type createRequest struct {
	Base  string `json:"base_currency"`
	Quote string `json:"quote_currency"`
}

type createResponse struct {
	ID     uuid.UUID    `json:"update_id"`
	Status model.Status `json:"status"`
}

type updateResponse struct {
	ID        uuid.UUID        `json:"update_id"`
	Base      string           `json:"base_currency"`
	Quote     string           `json:"quote_currency"`
	Status    model.Status     `json:"status"`
	Rate      *decimal.Decimal `json:"rate"`
	UpdatedAt *time.Time       `json:"updated_at"`
	CreatedAt time.Time        `json:"created_at"`
	Error     *string          `json:"error"`
}

type quoteResponse struct {
	Base      string          `json:"base_currency"`
	Quote     string          `json:"quote_currency"`
	Rate      decimal.Decimal `json:"rate"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type healthResponse struct {
	Status string `json:"status"`
}

func (h *Handler) create(c echo.Context) error {
	req := c.Request()

	mediaType, _, err := mime.ParseMediaType(req.Header.Get(echo.HeaderContentType))
	if err != nil || mediaType != echo.MIMEApplicationJSON {
		return echo.ErrUnsupportedMediaType
	}

	var body *createRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&body); err != nil {
		return bodyError(err)
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return bodyError(err)
	}
	if body == nil {
		return bodyError(nil)
	}

	var key *string
	keys := req.Header.Values("Idempotency-Key")

	switch len(keys) {
	case 0:
	case 1:
		key = &keys[0]
	default:
		return apperror.ErrInvalidKey
	}

	update, err := h.service.Create(req.Context(), body.Base, body.Quote, key)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusAccepted, createResponse{ID: update.ID, Status: update.Status})
}

func bodyError(err error) error {
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}

	return echo.ErrBadRequest
}

func (h *Handler) get(c echo.Context) error {
	update, err := h.service.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}

	response := updateResponse{
		ID:        update.ID,
		Base:      update.Pair.Base,
		Quote:     update.Pair.Quote,
		Status:    update.Status,
		Rate:      update.Rate,
		UpdatedAt: update.UpdatedAt,
		CreatedAt: update.CreatedAt,
	}

	if update.Status == model.StatusFailed {
		message := "could not refresh quote"
		response.Error = &message
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) latest(c echo.Context) error {
	quote, err := h.service.Latest(c.Request().Context(), c.QueryParam("base"), c.QueryParam("quote"))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, quoteResponse{
		Base:      quote.Pair.Base,
		Quote:     quote.Pair.Quote,
		Rate:      quote.Rate,
		UpdatedAt: quote.UpdatedAt,
	})
}

func (h *Handler) live(c echo.Context) error {
	return c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

func (h *Handler) ready(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable).SetInternal(err)
	}

	return c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}
