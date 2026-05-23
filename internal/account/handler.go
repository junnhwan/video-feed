package account

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register godoc
// @Summary      用户注册
// @Description  根据用户名和密码创建新账号
// @Tags         account
// @Accept       json
// @Produce      json
// @Param        body  body      registerRequest  true  "注册参数"
// @Success      200   {object}  accountResponse
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string  "username taken"
// @Router       /account/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account, err := h.service.Register(c.Request.Context(), RegisterInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrUsernameTaken) {
			status = http.StatusConflict
		}
		if errors.Is(err, ErrInvalidInput) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, accountResponse{ID: account.ID, Username: account.Username})
}

// Login godoc
// @Summary      用户登录
// @Description  用户名密码登录,返回 access token 与 refresh token
// @Tags         account
// @Accept       json
// @Produce      json
// @Param        body  body      loginRequest  true  "登录参数"
// @Success      200   {object}  loginResponse
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string  "invalid credentials"
// @Router       /account/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	login, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, loginResponse{
		Token:        login.Token,
		RefreshToken: login.RefreshToken,
		AccountID:    login.Account.ID,
		Username:     login.Account.Username,
	})
}

func (h *Handler) FindByID(c *gin.Context) {
	var req findByIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account, err := h.service.FindByID(c.Request.Context(), req.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, accountResponse{ID: account.ID, Username: account.Username})
}

func (h *Handler) FindByUsername(c *gin.Context) {
	var req findByUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account, err := h.service.FindByUsername(c.Request.Context(), req.Username)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, accountResponse{
		ID:        account.ID,
		Username:  account.Username,
		AvatarURL: account.AvatarURL,
		Bio:       account.Bio,
	})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), req.Username, req.OldPassword, req.NewPassword); err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, ErrInvalidCredentials) {
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "successfully password changed"})
}

func (h *Handler) Logout(c *gin.Context) {
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Logout(c.Request.Context(), accountID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "account logged out"})
}

func (h *Handler) Rename(c *gin.Context) {
	var req renameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	token, err := h.service.Rename(c.Request.Context(), accountID, req.NewUsername)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrNewUsernameRequired) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	login, err := h.service.RefreshAccessToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loginResponse{
		Token:        login.Token,
		RefreshToken: login.RefreshToken,
		AccountID:    login.Account.ID,
		Username:     login.Account.Username,
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.UpdateProfile(c.Request.Context(), accountID, req.AvatarURL, req.Bio); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidInput) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "profile updated"})
}

func (h *Handler) UploadAvatar(c *gin.Context) {
	accountID, err := getAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	const maxAvatarSize = 10 << 20
	if file.Size <= 0 || file.Size > maxAvatarSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file size"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .jpg/.jpeg/.png/.webp allowed"})
		return
	}
	dir := filepath.Join(".run", "uploads", "avatars", strconv.FormatUint(uint64(accountID), 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filename, err := randHex(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filename += ext
	uploadPath := filepath.Join(dir, filename)
	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	urlPath := path.Join("/static", "avatars", strconv.FormatUint(uint64(accountID), 10), filename)
	avatarURL := buildAbsoluteURL(c, urlPath)
	if err := h.service.UpdateAvatar(c.Request.Context(), accountID, avatarURL); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidInput) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"avatar_url": avatarURL})
}

func getAccountID(c *gin.Context) (uint, error) {
	value, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}
	accountID, ok := value.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}
	return accountID, nil
}

func randHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random filename: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func buildAbsoluteURL(c *gin.Context, urlPath string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := c.GetHeader("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, urlPath)
}
