package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client calls the BKI and EGRUL services with a per-attempt timeout and retries.
type Client struct {
	bkiURL, egrulURL string
	http             *http.Client
	Timeout          time.Duration // per attempt
	Attempts         int
	Backoff          time.Duration // base pause, doubles each attempt
}

func NewClient(bkiURL, egrulURL string) *Client {
	return &Client{bkiURL: bkiURL, egrulURL: egrulURL, http: &http.Client{}, Timeout: 2 * time.Second, Attempts: 3, Backoff: 200 * time.Millisecond}
}

func (c *Client) BKI(ctx context.Context, inn string) (*BKIReport, error) {
	var rep BKIReport
	if err := c.get(ctx, c.bkiURL+"/bki/"+url.PathEscape(inn), &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

func (c *Client) EGRUL(ctx context.Context, inn string) (EGRULRecord, error) {
	var rec EGRULRecord
	err := c.get(ctx, c.egrulURL+"/egrul/"+url.PathEscape(inn), &rec)
	return rec, err
}

func (c *Client) get(ctx context.Context, u string, out any) error {
	var last error
	for attempt := 0; attempt < c.Attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(c.Backoff << (attempt - 1)):
			case <-ctx.Done():
				return fmt.Errorf("%w: %v", ErrUnavailable, ctx.Err())
			}
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		retry, err := c.once(ctx, u, out)
		if err == nil {
			return nil
		}
		if !retry {
			return err
		}
		last = err
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, last)
}

// once makes one attempt; retry=false means the error is final.
func (c *Client) once(ctx context.Context, u string, out any) (retry bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return false, ErrNotFound
	case resp.StatusCode >= 500:
		return true, fmt.Errorf("status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return true, fmt.Errorf("decode: %w", err)
	}
	return false, nil
}
