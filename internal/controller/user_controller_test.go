package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"tinyauth/internal/config"
	"tinyauth/internal/controller"
	"tinyauth/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"gotest.tools/v3/assert"
)

var cookieValue string
var totpSecret = "6WFZXPEZRK5MZHHYAFW4DAOUYQMCASBJ"

func setupUserController(t *testing.T, middlewares *[]gin.HandlerFunc) (*gin.Engine, *httptest.ResponseRecorder) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	if middlewares != nil {
		for _, m := range *middlewares {
			router.Use(m)
		}
	}

	group := router.Group("/api")
	recorder := httptest.NewRecorder()

	// Database
	databaseService := service.NewDatabaseService(service.DatabaseServiceConfig{
		DatabasePath: "/tmp/tinyauth_test.db",
	})

	assert.NilError(t, databaseService.Init())

	database := databaseService.GetDatabase()

	// Auth service
	authService := service.NewAuthService(service.AuthServiceConfig{
		Users: []config.User{
			{
				Username: "testuser",
				Password: "$2a$10$ne6z693sTgzT3ePoQ05PgOecUHnBjM7sSNj6M.l5CLUP.f6NyCnt.", // test
			},
			{
				Username:   "totpuser",
				Password:   "$2a$10$ne6z693sTgzT3ePoQ05PgOecUHnBjM7sSNj6M.l5CLUP.f6NyCnt.", // test
				TotpSecret: totpSecret,
			},
		},
		OauthWhitelist:    "",
		SessionExpiry:     3600,
		SecureCookie:      false,
		RootDomain:        "localhost",
		LoginTimeout:      300,
		LoginMaxRetries:   3,
		SessionCookieName: "tinyauth-session",
	}, nil, nil, database)

	// Controller
	ctrl := controller.NewUserController(controller.UserControllerConfig{
		RootDomain: "localhost",
	}, group, authService)
	ctrl.SetupRoutes()

	return router, recorder
}

func TestLoginHandler(t *testing.T) {
	// Setup
	router, recorder := setupUserController(t, nil)

	loginReq := controller.LoginRequest{
		Username: "testuser",
		Password: "test",
	}

	loginReqJson, err := json.Marshal(loginReq)
	assert.NilError(t, err)

	// Test
	req := httptest.NewRequest("POST", "/api/user/login", strings.NewReader(string(loginReqJson)))
	router.ServeHTTP(recorder, req)

	assert.Equal(t, 200, recorder.Code)

	cookie := recorder.Result().Cookies()[0]

	assert.Equal(t, "tinyauth-session", cookie.Name)
	assert.Assert(t, cookie.Value != "")

	cookieValue = cookie.Value

	// Test invalid credentials
	loginReq = controller.LoginRequest{
		Username: "testuser",
		Password: "invalid",
	}

	loginReqJson, err = json.Marshal(loginReq)
	assert.NilError(t, err)

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/user/login", strings.NewReader(string(loginReqJson)))
	router.ServeHTTP(recorder, req)

	assert.Equal(t, 401, recorder.Code)

	// Test totp required
	loginReq = controller.LoginRequest{
		Username: "totpuser",
		Password: "test",
	}

	loginReqJson, err = json.Marshal(loginReq)
	assert.NilError(t, err)

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/user/login", strings.NewReader(string(loginReqJson)))
	router.ServeHTTP(recorder, req)

	assert.Equal(t, 200, recorder.Code)

	loginResJson, err := json.Marshal(map[string]any{
		"message":     "TOTP required",
		"status":      200,
		"totpPending": true,
	})

	assert.NilError(t, err)
	assert.Equal(t, string(loginResJson), recorder.Body.String())

	// Test rate limiting
	loginReq = controller.LoginRequest{
		Username: "testuser",
		Password: "invalid",
	}

	loginReqJson, err = json.Marshal(loginReq)
	assert.NilError(t, err)

	for range 5 {
		recorder = httptest.NewRecorder()
		req = httptest.NewRequest("POST", "/api/user/login", strings.NewReader(string(loginReqJson)))
		router.ServeHTTP(recorder, req)
	}

	assert.Equal(t, 429, recorder.Code)
}

func TestLogoutHandler(t *testing.T) {
	// Setup
	router, recorder := setupUserController(t, nil)

	// Test
	req := httptest.NewRequest("POST", "/api/user/logout", nil)

	req.AddCookie(&http.Cookie{
		Name:  "tinyauth-session",
		Value: cookieValue,
	})

	router.ServeHTTP(recorder, req)

	assert.Equal(t, 200, recorder.Code)

	cookie := recorder.Result().Cookies()[0]

	assert.Equal(t, "tinyauth-session", cookie.Name)
	assert.Equal(t, "", cookie.Value)
	assert.Equal(t, -1, cookie.MaxAge)
}

func TestTotpHandler(t *testing.T) {
	// Setup
	router, recorder := setupUserController(t, &[]gin.HandlerFunc{
		func(c *gin.Context) {
			c.Set("context", &config.UserContext{
				Username:    "totpuser",
				Name:        "totpuser",
				Email:       "totpuser@example.com",
				IsLoggedIn:  false,
				OAuth:       false,
				Provider:    "username",
				TotpPending: true,
				OAuthGroups: "",
				TotpEnabled: true,
			})
			c.Next()
		},
	})

	// Test
	code, err := totp.GenerateCode(totpSecret, time.Now())

	assert.NilError(t, err)

	totpReq := controller.TotpRequest{
		Code: code,
	}

	totpReqJson, err := json.Marshal(totpReq)
	assert.NilError(t, err)

	req := httptest.NewRequest("POST", "/api/user/totp", strings.NewReader(string(totpReqJson)))
	router.ServeHTTP(recorder, req)

	assert.Equal(t, 200, recorder.Code)

	cookie := recorder.Result().Cookies()[0]

	assert.Equal(t, "tinyauth-session", cookie.Name)
	assert.Assert(t, cookie.Value != "")

	// Test rate limiting
	totpReq = controller.TotpRequest{
		Code: "000000",
	}

	totpReqJson, err = json.Marshal(totpReq)
	assert.NilError(t, err)

	for range 5 {
		recorder = httptest.NewRecorder()
		req = httptest.NewRequest("POST", "/api/user/totp", strings.NewReader(string(totpReqJson)))
		router.ServeHTTP(recorder, req)
	}

	assert.Equal(t, 429, recorder.Code)

	// Test invalid code
	router, recorder = setupUserController(t, &[]gin.HandlerFunc{
		func(c *gin.Context) {
			c.Set("context", &config.UserContext{
				Username:    "totpuser",
				Name:        "totpuser",
				Email:       "totpuser@example.com",
				IsLoggedIn:  false,
				OAuth:       false,
				Provider:    "username",
				TotpPending: true,
				OAuthGroups: "",
				TotpEnabled: true,
			})
			c.Next()
		},
	})

	req = httptest.NewRequest("POST", "/api/user/totp", strings.NewReader(string(totpReqJson)))
	router.ServeHTTP(recorder, req)

	assert.Equal(t, 401, recorder.Code)

	// Test no totp pending
	router, recorder = setupUserController(t, &[]gin.HandlerFunc{
		func(c *gin.Context) {
			c.Set("context", &config.UserContext{
				Username:    "totpuser",
				Name:        "totpuser",
				Email:       "totpuser@example.com",
				IsLoggedIn:  false,
				OAuth:       false,
				Provider:    "username",
				TotpPending: false,
				OAuthGroups: "",
				TotpEnabled: false,
			})
			c.Next()
		},
	})

	req = httptest.NewRequest("POST", "/api/user/totp", strings.NewReader(string(totpReqJson)))
	router.ServeHTTP(recorder, req)

	assert.Equal(t, 401, recorder.Code)
}
