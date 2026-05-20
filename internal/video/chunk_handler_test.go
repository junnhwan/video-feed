package video

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	rediscache "video-feed/internal/middleware/redis"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
)

const chunkTestAccountID uint = 1

func TestFullChunkUploadFlow(t *testing.T) {
	handler, cleanup := setupChunkTestEnv(t)
	defer cleanup()

	chunkSize := 128
	totalChunks := 3
	chunks, chunkHashes, fileHash := makeTestChunks(t, totalChunks, chunkSize)

	uploadID := initChunkUpload(t, handler, "clip.mp4", int64(totalChunks*chunkSize), int64(chunkSize), totalChunks, fileHash)
	for i := range chunks {
		uploadChunk(t, handler, uploadID, i, chunkHashes[i], chunks[i])
	}

	c, rec := newChunkJSONContext(t, "/video/chunk/complete", CompleteChunkUploadRequest{UploadID: uploadID})
	handler.CompleteChunkUpload(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	resp := parseChunkJSON(t, rec)
	merged := readMergedChunkFile(t, resp)
	var expected bytes.Buffer
	for _, chunk := range chunks {
		expected.Write(chunk)
	}
	if !bytes.Equal(merged, expected.Bytes()) {
		t.Fatalf("merged content mismatch: got %d bytes, want %d", len(merged), expected.Len())
	}
	if _, err := os.Stat(filepath.Join(".run", "uploads", "tmp", uploadID)); !os.IsNotExist(err) {
		t.Fatalf("temp dir should be removed after complete")
	}
}

func TestChunkUploadResumeAndIdempotentUpload(t *testing.T) {
	handler, cleanup := setupChunkTestEnv(t)
	defer cleanup()

	chunkSize := 64
	totalChunks := 4
	chunks, chunkHashes, fileHash := makeTestChunks(t, totalChunks, chunkSize)
	uploadID := initChunkUpload(t, handler, "resume.mp4", int64(totalChunks*chunkSize), int64(chunkSize), totalChunks, fileHash)

	uploadChunk(t, handler, uploadID, 0, chunkHashes[0], chunks[0])
	uploadChunk(t, handler, uploadID, 0, chunkHashes[0], chunks[0])
	uploadChunk(t, handler, uploadID, 1, chunkHashes[1], chunks[1])

	c, rec := newChunkJSONContext(t, "/video/chunk/init", InitChunkUploadRequest{
		Filename:    "resume.mp4",
		FileSize:    int64(totalChunks * chunkSize),
		ChunkSize:   int64(chunkSize),
		TotalChunks: totalChunks,
		FileHash:    fileHash,
	})
	handler.InitChunkUpload(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("resume init expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	resp := parseChunkJSON(t, rec)
	if got := resp["upload_id"].(string); got != uploadID {
		t.Fatalf("expected resume upload_id %s, got %s", uploadID, got)
	}
	if got := len(resp["uploaded_chunks"].([]any)); got != 2 {
		t.Fatalf("expected 2 uploaded chunks, got %d", got)
	}

	c, rec = newChunkJSONContext(t, "/video/chunk/status", ChunkStatusRequest{UploadID: uploadID})
	handler.ChunkStatus(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	status := parseChunkJSON(t, rec)
	if got := len(status["uploaded_chunks"].([]any)); got != 2 {
		t.Fatalf("expected status with 2 chunks, got %d", got)
	}
}

func TestChunkUploadRejectsHashMismatchAndIncompleteMerge(t *testing.T) {
	handler, cleanup := setupChunkTestEnv(t)
	defer cleanup()

	chunks, chunkHashes, fileHash := makeTestChunks(t, 2, 32)
	uploadID := initChunkUpload(t, handler, "bad.mp4", int64(64), int64(32), 2, fileHash)

	c, rec := newChunkMultipartContext(t, "/video/chunk/upload", map[string]string{
		"upload_id":   uploadID,
		"chunk_index": "0",
		"chunk_hash":  "deadbeef000000000000000000000000",
	}, chunks[0])
	handler.UploadChunk(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("hash mismatch expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	uploadChunk(t, handler, uploadID, 0, chunkHashes[0], chunks[0])
	c, rec = newChunkJSONContext(t, "/video/chunk/complete", CompleteChunkUploadRequest{UploadID: uploadID})
	handler.CompleteChunkUpload(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("incomplete merge expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func setupChunkTestEnv(t *testing.T) (*ChunkUploadHandler, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	client := rediscache.NewClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()}), "")
	handler := NewChunkUploadHandler(client)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return handler, func() {
		_ = os.Chdir(originalDir)
		_ = client.Close()
		mr.Close()
	}
}

func newChunkJSONContext(t *testing.T, path string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.com"+path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("accountID", chunkTestAccountID)
	return c, rec
}

func newChunkMultipartContext(t *testing.T, path string, fields map[string]string, content []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", "chunk.bin")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.com"+path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("accountID", chunkTestAccountID)
	return c, rec
}

func initChunkUpload(t *testing.T, handler *ChunkUploadHandler, filename string, fileSize int64, chunkSize int64, totalChunks int, fileHash string) string {
	t.Helper()
	c, rec := newChunkJSONContext(t, "/video/chunk/init", InitChunkUploadRequest{
		Filename:    filename,
		FileSize:    fileSize,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		FileHash:    fileHash,
	})
	handler.InitChunkUpload(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("init expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	resp := parseChunkJSON(t, rec)
	return resp["upload_id"].(string)
}

func uploadChunk(t *testing.T, handler *ChunkUploadHandler, uploadID string, index int, hash string, content []byte) {
	t.Helper()
	c, rec := newChunkMultipartContext(t, "/video/chunk/upload", map[string]string{
		"upload_id":   uploadID,
		"chunk_index": fmt.Sprintf("%d", index),
		"chunk_hash":  hash,
	}, content)
	handler.UploadChunk(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload chunk %d expected 200, got %d body=%s", index, rec.Code, rec.Body.String())
	}
}

func makeTestChunks(t *testing.T, totalChunks int, chunkSize int) ([][]byte, []string, string) {
	t.Helper()
	chunks := make([][]byte, totalChunks)
	hashes := make([]string, totalChunks)
	var full bytes.Buffer
	for i := 0; i < totalChunks; i++ {
		chunk := make([]byte, chunkSize)
		if _, err := rand.Read(chunk); err != nil {
			t.Fatalf("rand read: %v", err)
		}
		chunks[i] = chunk
		hashes[i] = md5Hex(chunk)
		full.Write(chunk)
	}
	return chunks, hashes, md5Hex(full.Bytes())
}

func md5Hex(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func parseChunkJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return response
}

func readMergedChunkFile(t *testing.T, resp map[string]any) []byte {
	t.Helper()
	url := resp["url"].(string)
	prefix := "http://example.com/static/"
	if len(url) <= len(prefix) || url[:len(prefix)] != prefix {
		t.Fatalf("unexpected url %q", url)
	}
	data, err := os.ReadFile(filepath.Join(".run", "uploads", url[len(prefix):]))
	if err != nil {
		t.Fatalf("read merged file: %v", err)
	}
	return data
}
