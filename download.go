package shotgun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Download fetches the signed download URL for field on the given entity and
// copies the file contents to w.
func (c *Client) Download(ctx context.Context, entityType string, id int, field string, w io.Writer) error {
	path := fmt.Sprintf("/api/v1.1/entity/%s/%d/%s", entityType, id, field)

	resp, err := c.get(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get download URL failed with status %d", resp.StatusCode)
	}

	// data can be a plain string URL or an object with a "url" field.
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("failed to decode download URL response: %w", err)
	}

	var downloadURL string
	var obj struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(envelope.Data, &obj); err == nil && obj.URL != "" {
		downloadURL = obj.URL
	} else {
		json.Unmarshal(envelope.Data, &downloadURL)
	}

	if downloadURL == "" {
		return fmt.Errorf("no download URL returned for %s/%d/%s", entityType, id, field)
	}

	// The download URL is pre-signed — no auth header needed.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}

	dlResp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", dlResp.StatusCode)
	}

	_, err = io.Copy(w, dlResp.Body)
	return err
}

// DownloadFile creates the file at filePath and downloads field into it.
// Use Download directly to stream to an existing io.Writer.
func (c *Client) DownloadFile(ctx context.Context, entityType string, id int, field, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", filePath, err)
	}
	defer f.Close()
	return c.Download(ctx, entityType, id, field, f)
}

// DownloadImage downloads the image field for the given entity and copies it to w.
// Uses the same plural snake_case entity type names as the rest of the API (e.g. "shots", "assets").
//
// Note: the thumbnail parameter is reserved for future use — Shotgun does not
// expose a reliable API for selecting thumbnail vs original via the field
// download endpoint, so the full image is always returned.
func (c *Client) DownloadImage(ctx context.Context, entityType string, id int, thumbnail bool, w io.Writer) error {
	return c.Download(ctx, entityType, id, "image", w)
}

// DownloadImageFile creates the file at filePath and downloads the image into it.
// Use DownloadImage directly to stream to an existing io.Writer.
func (c *Client) DownloadImageFile(ctx context.Context, entityType string, id int, thumbnail bool, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", filePath, err)
	}
	defer f.Close()
	return c.DownloadImage(ctx, entityType, id, thumbnail, f)
}
