package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/TravisPenn/professor-arbortom/internal/models"
)

// itoa is a convenience wrapper for int to string conversion.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// scanInt parses s as a base-10 integer, returning an error if it fails.
func scanInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.Atoi(s)
}

// expectsJSON reports whether the request prefers a JSON response.
func expectsJSON(c *gin.Context) bool {
	return c.GetHeader("Accept") == "application/json"
}

// respondError sends a 500 response (JSON or HTML) and aborts the request.
// SEC-006: In release mode, returns a generic message to avoid leaking
// internal details (SQL, file paths, driver errors). Full error is logged.
func respondError(c *gin.Context, err error) {
	log.Printf("ERROR [%s %s]: %v", c.Request.Method, c.Request.URL.Path, err)

	msg := err.Error()
	if gin.Mode() == gin.ReleaseMode {
		msg = "An internal error occurred"
	}

	if expectsJSON(c) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	} else {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"Message": msg})
	}
	c.Abort()
}

// respondNotFound sends a 404 response (JSON or HTML) and aborts the request.
func respondNotFound(c *gin.Context) {
	if expectsJSON(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
	} else {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Message": "Run not found"})
	}
	c.Abort()
}

// mustParamInt parses a URL path parameter as an integer. On failure it writes
// a 400 error response and returns (0, false) so callers can return immediately.
func mustParamInt(c *gin.Context, key string) (int, bool) {
	n, err := strconv.Atoi(c.Param(key))
	if err != nil {
		if expectsJSON(c) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + key})
		} else {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"Message": "invalid " + key})
		}
		return 0, false
	}
	return n, true
}

// formInt parses an optional POST form field as an integer. Returns def on failure.
func formInt(c *gin.Context, key string, def int) int {
	n, err := strconv.Atoi(c.PostForm(key))
	if err != nil {
		return def
	}
	return n
}

// isRuleEnabled reports whether the named rule is enabled in the given slice.
func isRuleEnabled(rules []models.ActiveRule, key string) bool {
	for _, r := range rules {
		if r.Key == key && r.Enabled {
			return true
		}
	}
	return false
}
