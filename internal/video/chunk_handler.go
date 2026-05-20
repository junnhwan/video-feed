package video

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	authjwt "video-feed/internal/middleware/jwt"
	rediscache "video-feed/internal/middleware/redis"

	"github.com/gin-gonic/gin"
)

const chunkSessionTTL = 24 * time.Hour

var ErrChunkUploadCacheUnavailable = errors.New("chunk upload cache unavailable")

type ChunkUploadHandler struct {
	cache *rediscache.Client
}

func NewChunkUploadHandler(cache *rediscache.Client) *ChunkUploadHandler {
	return &ChunkUploadHandler{cache: cache}
}

func (h *ChunkUploadHandler) InitChunkUpload(c *gin.Context) {
	if !h.ensureCache(c) {
		return
	}
	var req InitChunkUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if req.FileSize > 200<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file size exceeds 200MB limit"})
		return
	}

	hashKey := h.hashKey(accountID, req.FileHash)
	if existingID, err := h.cache.GetBytes(c.Request.Context(), hashKey); err == nil && len(existingID) > 0 {
		session, err := h.getSession(c, string(existingID))
		if err == nil && session.AccountID == accountID {
			_ = h.cache.SetBytes(c.Request.Context(), hashKey, existingID, chunkSessionTTL)
			_ = h.saveSession(c, session)
			c.JSON(http.StatusOK, gin.H{"upload_id": session.UploadID, "uploaded_chunks": session.UploadedChunks()})
			return
		}
	}

	randomID, err := randHex(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	session := &ChunkUploadSession{
		UploadID:     randomID + fmt.Sprintf("%d", time.Now().UnixNano()),
		AccountID:    accountID,
		Filename:     req.Filename,
		FileSize:     req.FileSize,
		ChunkSize:    req.ChunkSize,
		TotalChunks:  req.TotalChunks,
		FileHash:     req.FileHash,
		UploadedBits: make([]bool, req.TotalChunks),
	}
	if err := h.saveSession(c, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	if err := h.cache.SetBytes(c.Request.Context(), hashKey, []byte(session.UploadID), chunkSessionTTL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"upload_id": session.UploadID, "uploaded_chunks": []int{}})
}

func (h *ChunkUploadHandler) UploadChunk(c *gin.Context) {
	if !h.ensureCache(c) {
		return
	}
	var req UploadChunkRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	session, ok := h.loadOwnedSession(c, req.UploadID)
	if !ok {
		return
	}
	if req.ChunkIndex < 0 || req.ChunkIndex >= session.TotalChunks {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk_index"})
		return
	}
	if session.UploadedBits[req.ChunkIndex] {
		c.JSON(http.StatusOK, gin.H{"chunk_index": req.ChunkIndex})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read chunk"})
		return
	}
	defer src.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash chunk"})
		return
	}
	actualHash := fmt.Sprintf("%x", hash.Sum(nil))
	if actualHash != req.ChunkHash {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk hash mismatch", "expected": req.ChunkHash, "actual": actualHash})
		return
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read chunk"})
		return
	}

	tmpDir := filepath.Join(".run", "uploads", "tmp", req.UploadID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp dir"})
		return
	}
	dst, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf("%d", req.ChunkIndex)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save chunk"})
		return
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save chunk"})
		return
	}
	if err := dst.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save chunk"})
		return
	}

	session.UploadedBits[req.ChunkIndex] = true
	if err := h.saveSession(c, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"chunk_index": req.ChunkIndex})
}

func (h *ChunkUploadHandler) ChunkStatus(c *gin.Context) {
	if !h.ensureCache(c) {
		return
	}
	var req ChunkStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	session, ok := h.loadOwnedSession(c, req.UploadID)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"upload_id":       session.UploadID,
		"uploaded_chunks": session.UploadedChunks(),
		"total_chunks":    session.TotalChunks,
	})
}

func (h *ChunkUploadHandler) CompleteChunkUpload(c *gin.Context) {
	if !h.ensureCache(c) {
		return
	}
	var req CompleteChunkUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	session, ok := h.loadOwnedSession(c, req.UploadID)
	if !ok {
		return
	}
	if !session.IsComplete() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "not all chunks uploaded",
			"missing":   missingChunkCount(session),
			"completed": len(session.UploadedChunks()),
			"total":     session.TotalChunks,
		})
		return
	}

	date := time.Now().Format("20060102")
	relDir := filepath.Join("videos", fmt.Sprintf("%d", session.AccountID), date)
	absDir := filepath.Join(".run", "uploads", relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create output dir"})
		return
	}
	filename, err := randHex(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate filename"})
		return
	}
	finalPath := filepath.Join(absDir, filename+".mp4")
	if err := h.mergeChunks(req.UploadID, session.TotalChunks, finalPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tmpDir := filepath.Join(".run", "uploads", "tmp", req.UploadID)
	_ = os.RemoveAll(tmpDir)
	_ = h.cache.Del(c.Request.Context(), h.sessionKey(req.UploadID))
	_ = h.cache.Del(c.Request.Context(), h.hashKey(session.AccountID, session.FileHash))

	urlPath := fmt.Sprintf("/static/videos/%d/%s/%s.mp4", session.AccountID, date, filename)
	playURL := buildAbsoluteURL(c, urlPath)
	c.JSON(http.StatusOK, gin.H{"url": playURL, "play_url": playURL})
}

func (h *ChunkUploadHandler) loadOwnedSession(c *gin.Context, uploadID string) (*ChunkUploadSession, bool) {
	session, err := h.getSession(c, uploadID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return nil, false
	}
	accountID, err := authjwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return nil, false
	}
	if session.AccountID != accountID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	return session, true
}

func (h *ChunkUploadHandler) getSession(c *gin.Context, uploadID string) (*ChunkUploadSession, error) {
	payload, err := h.cache.GetBytes(c.Request.Context(), h.sessionKey(uploadID))
	if err != nil {
		return nil, errors.New("upload session not found")
	}
	var session ChunkUploadSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, errors.New("invalid session data")
	}
	return &session, nil
}

func (h *ChunkUploadHandler) saveSession(c *gin.Context, session *ChunkUploadSession) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return h.cache.SetBytes(c.Request.Context(), h.sessionKey(session.UploadID), payload, chunkSessionTTL)
}

func (h *ChunkUploadHandler) mergeChunks(uploadID string, totalChunks int, finalPath string) error {
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return errors.New("failed to create final file")
	}
	defer finalFile.Close()

	tmpDir := filepath.Join(".run", "uploads", "tmp", uploadID)
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			_ = finalFile.Close()
			_ = os.Remove(finalPath)
			return fmt.Errorf("chunk %d missing", i)
		}
		_, copyErr := io.Copy(finalFile, chunkFile)
		_ = chunkFile.Close()
		if copyErr != nil {
			_ = finalFile.Close()
			_ = os.Remove(finalPath)
			return errors.New("failed to merge chunks")
		}
	}
	return nil
}

func (h *ChunkUploadHandler) sessionKey(uploadID string) string {
	return h.cache.Key("chunk_upload:%s", uploadID)
}

func (h *ChunkUploadHandler) hashKey(accountID uint, fileHash string) string {
	return h.cache.Key("chunk_upload_hash:%d:%s", accountID, fileHash)
}

func (h *ChunkUploadHandler) ensureCache(c *gin.Context) bool {
	if h == nil || h.cache == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": ErrChunkUploadCacheUnavailable.Error()})
		return false
	}
	return true
}

func missingChunkCount(session *ChunkUploadSession) int {
	missing := 0
	for _, uploaded := range session.UploadedBits {
		if !uploaded {
			missing++
		}
	}
	return missing
}
