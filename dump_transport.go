package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const dumpChunkSize = 4 << 20

type dumpTarget struct {
	TargetType string
	TargetID   string
	TenantID   string
}

func (t dumpTarget) query() string {
	q := url.Values{}
	if t.TargetType != "" {
		q.Set("target_type", t.TargetType)
	}
	if t.TargetID != "" {
		q.Set("target_id", t.TargetID)
	}
	if t.TenantID != "" {
		q.Set("tenant_id", t.TenantID)
	}
	enc := q.Encode()
	if enc == "" {
		return ""
	}
	return "?" + enc
}

func (c *SyncGraphQLClient) dumpHTTP(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		return &http.Client{}
	}
	return &http.Client{Timeout: timeout}
}

func (c *SyncGraphQLClient) DumpPreflight(t dumpTarget) (dumpPreflightResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL()+"/system/v1/dump/preflight"+t.query(), nil)
	if err != nil {
		return dumpPreflightResponse{}, err
	}
	c.setAuthHeaders(req)
	resp, err := c.dumpHTTP(c.timeout).Do(req)
	if err != nil {
		return dumpPreflightResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return dumpPreflightResponse{}, err
	}
	if resp.StatusCode >= 400 {
		return dumpPreflightResponse{}, dumpHTTPError(resp.StatusCode, raw)
	}
	var out dumpPreflightResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return dumpPreflightResponse{}, err
	}
	return out, nil
}

func (c *SyncGraphQLClient) DownloadDump(t dumpTarget, destPath string) error {
	var existing int64
	if st, err := os.Stat(destPath); err == nil {
		existing = st.Size()
	}
	req, err := http.NewRequest(http.MethodGet, c.baseURL()+"/system/v1/dump/export"+t.query(), nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)
	if existing > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
	}
	resp, err := c.dumpHTTP(0).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return dumpHTTPError(resp.StatusCode, raw)
	}

	wantSHA := strings.ToLower(strings.TrimSpace(resp.Header.Get("X-Apito-Dump-Sha256")))
	flag := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent && existing > 0 {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
		existing = 0
	}
	f, err := os.OpenFile(destPath, flag, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if wantSHA == "" {
		return nil
	}
	got, _, err := hashDumpFile(destPath)
	if err != nil {
		return err
	}
	if got != wantSHA {
		return fmt.Errorf("download sha256 mismatch: got %s want %s", got, wantSHA)
	}
	return nil
}

func (c *SyncGraphQLClient) UploadDump(t dumpTarget, tarPath, confirm string) error {
	st, err := os.Stat(tarPath)
	if err != nil {
		return err
	}
	sha, _, err := hashDumpFile(tarPath)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"target_type": t.TargetType,
		"target_id":   t.TargetID,
		"tenant_id":   t.TenantID,
		"size_bytes":  st.Size(),
		"sha256":      sha,
		"confirm":     confirm,
	})
	req, err := http.NewRequest(http.MethodPost, c.baseURL()+"/system/v1/dump/import/init", bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.dumpHTTP(c.timeout).Do(req)
	if err != nil {
		return err
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return dumpHTTPError(resp.StatusCode, raw)
	}
	var initOut struct {
		UploadID string `json:"upload_id"`
	}
	if err := json.Unmarshal(raw, &initOut); err != nil {
		return err
	}
	if initOut.UploadID == "" {
		return fmt.Errorf("import init returned empty upload_id")
	}

	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, dumpChunkSize)
	var offset int64
	for {
		n, rerr := io.ReadFull(f, buf)
		if n == 0 && rerr == io.EOF {
			break
		}
		if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {
			return rerr
		}
		chunk := buf[:n]
		sum := sha256.Sum256(chunk)
		chunkSHA := hex.EncodeToString(sum[:])
		u := fmt.Sprintf("%s/system/v1/dump/import/%s/chunk?offset=%d&sha256=%s",
			c.baseURL(), url.PathEscape(initOut.UploadID), offset, chunkSHA)
		creq, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(chunk))
		if err != nil {
			return err
		}
		c.setAuthHeaders(creq)
		creq.Header.Set("Content-Type", "application/octet-stream")
		creq.Header.Set("X-Apito-Dump-Chunk-Sha256", chunkSHA)
		cresp, err := c.dumpHTTP(2 * time.Minute).Do(creq)
		if err != nil {
			return fmt.Errorf("chunk offset %d: %w", offset, err)
		}
		craw, _ := io.ReadAll(cresp.Body)
		_ = cresp.Body.Close()
		if cresp.StatusCode >= 400 {
			return fmt.Errorf("chunk offset %d: %w", offset, dumpHTTPError(cresp.StatusCode, craw))
		}
		offset += int64(n)
		if rerr == io.EOF || rerr == io.ErrUnexpectedEOF {
			break
		}
	}

	creq, err := http.NewRequest(http.MethodPost, c.baseURL()+"/system/v1/dump/import/"+url.PathEscape(initOut.UploadID)+"/commit", nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(creq)
	cresp, err := c.dumpHTTP(10 * time.Minute).Do(creq)
	if err != nil {
		return err
	}
	craw, _ := io.ReadAll(cresp.Body)
	_ = cresp.Body.Close()
	if cresp.StatusCode >= 400 {
		return dumpHTTPError(cresp.StatusCode, craw)
	}
	return nil
}

func hashDumpFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", n, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func dumpHTTPError(status int, raw []byte) error {
	var wrapped struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &wrapped) == nil {
		msg := strings.TrimSpace(wrapped.Error)
		if msg == "" {
			msg = strings.TrimSpace(wrapped.Message)
		}
		if msg != "" {
			return fmt.Errorf("dump HTTP %d: %s", status, msg)
		}
	}
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return fmt.Errorf("dump HTTP %d: %s", status, msg)
}
