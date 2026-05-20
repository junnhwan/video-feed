package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"video-feed/internal/account"
	"video-feed/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestRouter(t *testing.T) (*gin.Engine, *account.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&account.Account{}); err != nil {
		t.Fatalf("migrate account: %v", err)
	}
	repo := account.NewRepository(database)
	service := account.NewService(repo)

	router := gin.New()
	router.GET("/protected", JWTAuth(repo), func(c *gin.Context) {
		accountID, err := GetAccountID(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		username, err := GetUsername(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"account_id": accountID, "username": username})
	})

	return router, service
}

func TestJWTAuthAcceptsCurrentStoredToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	router, service := newTestRouter(t)
	if _, err := service.Register(t.Context(), account.RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := service.Login(t.Context(), "alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestJWTAuthRejectsRevokedToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	router, service := newTestRouter(t)
	if _, err := service.Register(t.Context(), account.RegisterInput{Username: "alice", Password: "secret123"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := service.Login(t.Context(), "alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := service.Logout(t.Context(), login.Account.ID); err != nil {
		t.Fatalf("logout: %v", err)
	}
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSoftJWTAuthAllowsAnonymousRequest(t *testing.T) {
	repo := account.NewRepository(nil)
	router := gin.New()
	router.GET("/feed", SoftJWTAuth(repo), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest("GET", "/feed", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestJWTAuthRejectsTokenNotStoredOnAccount(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	router, service := newTestRouter(t)
	created, err := service.Register(t.Context(), account.RegisterInput{Username: "alice", Password: "secret123"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	token, err := auth.GenerateToken(created.ID, created.Username)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}
