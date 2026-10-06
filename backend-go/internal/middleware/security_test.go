package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSafeRecoveryDoesNotLeakCredentials(t *testing.T) {
	var captured bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&captured)
	defer log.SetOutput(previousOutput)
	router := gin.New()
	router.Use(SafeAccessLogger(), SafeRecovery())
	router.POST("/panic", func(c *gin.Context) { panic("panic-secret-test") })
	request := httptest.NewRequest(http.MethodPost, "/panic?secret=query-secret-test", strings.NewReader("body-secret-test"))
	request.Header.Set("Cookie", "auth_token=cookie-secret-test")
	request.Header.Set("X-Agent-Secret", "agent-secret-test")
	request.Header.Set("Authorization", "Bearer bearer-secret-test")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("panic returned status %d", response.Code)
	}
	combined := captured.String() + response.Body.String()
	for _, sensitive := range []string{"panic-secret-test", "query-secret-test", "body-secret-test", "cookie-secret-test", "agent-secret-test", "bearer-secret-test"} {
		if strings.Contains(combined, sensitive) {
			t.Fatalf("sensitive request data leaked: %s", sensitive)
		}
	}
	if !strings.Contains(captured.String(), "/panic") {
		t.Fatal("recovery did not identify the failing route")
	}
}

func TestAccessLogsExcludeRawQueryAndPathParameters(t *testing.T) {
	var captured bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&captured)
	defer log.SetOutput(previousOutput)
	router := gin.New()
	router.Use(SafeAccessLogger(), SafeRecovery())
	router.GET("/item/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, path := range []string{"/item/private-path-value?token=private-query-value", "/private-unmatched-value?token=private-query-value"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	if strings.Contains(captured.String(), "private-") || !strings.Contains(captured.String(), "/item/:id") {
		t.Fatal("access log exposed user input or lost route template")
	}
}

func TestCSRFProtectionAndBearerCompatibility(t *testing.T) {
	for _, test := range []struct {
		name, method, cookie, header string
		want                         int
	}{
		{"missing CSRF", "POST", "auth_token=session", "", 403},
		{"mismatched CSRF", "DELETE", "auth_token=session; csrf_token=expected", "wrong", 403},
		{"valid CSRF", "PUT", "auth_token=session; csrf_token=expected", "expected", 204},
		{"safe read", "GET", "auth_token=session", "", 204},
		{"bearer only", "POST", "", "", 204},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(CSRFMiddleware())
			router.Any("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(test.method, "/protected", nil)
			request.Header.Set("Authorization", "Bearer ignored-by-CSRF")
			request.Header.Set("Cookie", test.cookie)
			request.Header.Set("X-CSRF-Token", test.header)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}
