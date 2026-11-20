package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const maxResponseSize = 64 << 10

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func (c *Client) GetRate(ctx context.Context, base, quote string) (decimal.Decimal, time.Time, error) {
	endpoint := c.baseURL + "/rate/" + url.PathEscape(base) + "/" + url.PathEscape(quote)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return decimal.Zero, time.Time{}, fmt.Errorf("build exchange request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return decimal.Zero, time.Time{}, fmt.Errorf("fetch exchange rate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decimal.Zero, time.Time{}, fmt.Errorf("exchange API returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return decimal.Zero, time.Time{}, fmt.Errorf("read exchange response: %w", err)
	}
	if len(body) > maxResponseSize {
		return decimal.Zero, time.Time{}, fmt.Errorf("exchange response exceeds size limit")
	}

	var result struct {
		Base  string           `json:"base"`
		Quote string           `json:"quote"`
		Date  string           `json:"date"`
		Rate  *decimal.Decimal `json:"rate"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return decimal.Zero, time.Time{}, fmt.Errorf("decode exchange response: %w", err)
	}

	if result.Base != base || result.Quote != quote || result.Rate == nil || !result.Rate.IsPositive() {
		return decimal.Zero, time.Time{}, fmt.Errorf("exchange response has no positive rate for %s/%s", base, quote)
	}

	updatedAt := time.Now().UTC()
	if result.Date != "" {
		updatedAt, err = time.Parse(time.DateOnly, result.Date)
		if err != nil {
			return decimal.Zero, time.Time{}, fmt.Errorf("parse exchange rate date: %w", err)
		}
	}

	return *result.Rate, updatedAt, nil
}
