package controller

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/tinyauthapp/tinyauth/internal/test"
	"github.com/tinyauthapp/tinyauth/internal/utils/logger"
)

func TestOAuthController(t *testing.T) {
	log := logger.NewLogger().WithTestConfig()
	log.Init()

	cfg, runtime := test.CreateTestConfigs(t)

	type testCase struct {
		description string
		run         func(ctrl *OAuthController)
	}

	tests := []testCase{
		{
			description: "Test exact match of redirect URI",
			run: func(ctrl *OAuthController) {
				redirectUri := "https://tinyauth.example.com"
				assert.True(t, ctrl.isRedirectSafe(redirectUri))
			},
		},
		{
			description: "Test subdomain match of redirect URI",
			run: func(ctrl *OAuthController) {
				redirectUri := "https://sub.example.com"
				assert.True(t, ctrl.isRedirectSafe(redirectUri))
			},
		},
		{
			description: "Test different trusted domain",
			run: func(ctrl *OAuthController) {
				redirectUri := "https://app.foo.com"
				assert.True(t, ctrl.isRedirectSafe(redirectUri))
			},
		},
		{
			description: "Test invalid redirect URI",
			run: func(ctrl *OAuthController) {
				redirectUri := "https://malicious.com"
				assert.False(t, ctrl.isRedirectSafe(redirectUri))
			},
		},
		{
			description: "Test empty redirect URI",
			run: func(ctrl *OAuthController) {
				redirectUri := ""
				assert.False(t, ctrl.isRedirectSafe(redirectUri))
			},
		},
		{
			description: "Test redirect URI with different scheme",
			run: func(ctrl *OAuthController) {
				redirectUri := "http://tinyauth.example.com"
				assert.False(t, ctrl.isRedirectSafe(redirectUri))
			},
		},
	}

	// TODO: add auth service
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			router := gin.Default()
			group := router.Group("/api")
			gin.SetMode(gin.TestMode)
			ctrl := NewOAuthController(OAuthControllerInput{
				Log:           log,
				Config:        &cfg,
				RuntimeConfig: &runtime,
				RouterGroup:   group,
			})
			tc.run(ctrl)
		})
	}
}
