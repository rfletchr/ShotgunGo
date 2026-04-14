package shotgun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

const (
	// multipartThreshold is the file size above which multipart upload is used.
	// The API requires all parts except the last to be at least 5MB.
	multipartThreshold = 5 * 1024 * 1024 // 5 MB

	// uploadChunkSize is the size of each part in a multipart upload.
	uploadChunkSize = 10 * 1024 * 1024 // 10 MB
)

type uploadLinks struct {
	Upload         string `json:"upload"`
	GetNextPart    string `json:"get_next_part"`
	CompleteUpload string `json:"complete_upload"`
}

type uploadMetadata struct {
	Data  json.RawMessage `json:"data"`
	Links uploadLinks     `json:"links"`
}

// getUploadMetadata requests upload URLs from Shotgun for the given entity and field.
// If field is empty the file is uploaded as an Attachment linked to the entity rather
// than stored in a specific field.
func (c *Client) getUploadMetadata(ctx context.Context, entityType string, id int, field, filename string, multipart bool) (*uploadMetadata, error) {
	params := url.Values{}
	params.Set("filename", filename)
	if multipart {
		params.Set("multipart_upload", "true")
	}

	var path string
	if field != "" {
		path = fmt.Sprintf("/api/v1.1/entity/%s/%d/%s/_upload?%s", entityType, id, field, params.Encode())
	} else {
		path = fmt.Sprintf("/api/v1.1/entity/%s/%d/_upload?%s", entityType, id, params.Encode())
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get upload metadata failed with status %d", resp.StatusCode)
	}

	var meta uploadMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode upload metadata: %w", err)
	}
	return &meta, nil
}

// putExternal uploads data to a pre-signed external URL (S3 or SG storage).
// No Shotgun auth headers are sent. Returns the ETag header and the raw response body.
func (c *Client) putExternal(ctx context.Context, uploadURL, contentType string, data []byte) (etag string, body json.RawMessage, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(data))

	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("upload to storage failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return "", nil, fmt.Errorf("upload to storage failed with status %d", resp.StatusCode)
	}

	etag = resp.Header.Get("ETag")
	json.NewDecoder(resp.Body).Decode(&body) // empty for S3, upload_id for SG storage
	return etag, body, nil
}

// completeUpload notifies Shotgun that all data has been uploaded and links it to the entity.
func (c *Client) completeUpload(ctx context.Context, meta *uploadMetadata, uploadInfo map[string]any, filename string) error {
	body := map[string]any{
		"upload_info": uploadInfo,
		"upload_data": map[string]any{
			"display_name": filename,
		},
	}

	resp, err := c.post(ctx, meta.Links.CompleteUpload, "application/json", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("complete upload failed with status %d", resp.StatusCode)
	}
	return nil
}

// abortMultipartUpload cancels an in-progress multipart upload. Errors are
// swallowed since this is only called while already handling another error.
func (c *Client) abortMultipartUpload(ctx context.Context, meta *uploadMetadata) {
	abortURL := meta.Links.CompleteUpload + "/multipart_abort"
	resp, err := c.post(ctx, abortURL, "application/json", map[string]any{})
	if err == nil {
		resp.Body.Close()
	}
}

func (c *Client) uploadSingle(ctx context.Context, meta *uploadMetadata, filename, contentType string, r io.Reader, size int64) error {
	data, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return fmt.Errorf("failed to read upload data: %w", err)
	}

	_, respBody, err := c.putExternal(ctx, meta.Links.Upload, contentType, data)
	if err != nil {
		return err
	}

	var uploadInfo map[string]any
	if err := json.Unmarshal(meta.Data, &uploadInfo); err != nil {
		return fmt.Errorf("failed to decode upload info: %w", err)
	}

	// SG storage returns an upload_id in the PUT response body.
	if uploadInfo["storage_service"] == "sg" {
		var sgResp struct {
			Data struct {
				UploadID string `json:"upload_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &sgResp); err == nil && sgResp.Data.UploadID != "" {
			uploadInfo["upload_id"] = sgResp.Data.UploadID
		}
	}

	return c.completeUpload(ctx, meta, uploadInfo, filename)
}

func (c *Client) uploadMultipart(ctx context.Context, meta *uploadMetadata, filename, contentType string, r io.Reader) error {
	var uploadInfo map[string]any
	if err := json.Unmarshal(meta.Data, &uploadInfo); err != nil {
		return fmt.Errorf("failed to decode upload info: %w", err)
	}

	currentLinks := meta.Links
	var etags []string
	buf := make([]byte, uploadChunkSize)

	for {
		n, readErr := io.ReadFull(r, buf)
		if n == 0 {
			break
		}

		etag, _, err := c.putExternal(ctx, currentLinks.Upload, contentType, buf[:n])
		if err != nil {
			c.abortMultipartUpload(ctx, meta)
			return err
		}
		etags = append(etags, etag)

		if readErr == io.ErrUnexpectedEOF || readErr == io.EOF {
			break // last chunk
		}
		if readErr != nil {
			c.abortMultipartUpload(ctx, meta)
			return readErr
		}

		// Fetch the upload URL for the next part.
		resp, err := c.get(ctx, currentLinks.GetNextPart)
		if err != nil {
			c.abortMultipartUpload(ctx, meta)
			return err
		}
		var nextMeta struct {
			Links uploadLinks `json:"links"`
		}
		err = json.NewDecoder(resp.Body).Decode(&nextMeta)
		resp.Body.Close()
		if err != nil {
			c.abortMultipartUpload(ctx, meta)
			return fmt.Errorf("failed to decode next part metadata: %w", err)
		}
		currentLinks = nextMeta.Links
	}

	uploadInfo["etags"] = etags
	if err := c.completeUpload(ctx, meta, uploadInfo, filename); err != nil {
		c.abortMultipartUpload(ctx, meta)
		return err
	}
	return nil
}

// Upload uploads r to field on the given entity record.
// For files larger than 5MB, multipart upload is used automatically.
// Set field to "" to upload as an Attachment linked to the entity instead of a specific field.
func (c *Client) Upload(ctx context.Context, entityType string, id int, field, filename, contentType string, r io.Reader, size int64) error {
	useMultipart := size > multipartThreshold

	meta, err := c.getUploadMetadata(ctx, entityType, id, field, filename, useMultipart)
	if err != nil {
		return err
	}

	// Fall back to single upload if the server doesn't support multipart (non-S3 storage).
	if useMultipart && meta.Links.GetNextPart == "" {
		useMultipart = false
	}

	if useMultipart {
		return c.uploadMultipart(ctx, meta, filename, contentType, r)
	}
	return c.uploadSingle(ctx, meta, filename, contentType, r, size)
}

// UploadFile opens the file at filePath and uploads it to field on the given entity record.
// The filename is taken from the path. Use Upload directly for other io.Reader sources.
func (c *Client) UploadFile(ctx context.Context, entityType string, id int, field, filePath, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filePath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", filePath, err)
	}

	return c.Upload(ctx, entityType, id, field, filepath.Base(filePath), contentType, f, info.Size())
}
