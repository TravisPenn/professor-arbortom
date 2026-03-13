package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/TravisPenn/professor-arbortom/internal/legality"
	"github.com/TravisPenn/professor-arbortom/internal/services"
)

// apiErrorMsg returns err.Error() in debug mode, generic message in release.
// SEC-006: Prevents leaking internal details in JSON API error responses.
func apiErrorMsg(err error) string {
	if gin.Mode() == gin.ReleaseMode {
		return "An internal error occurred"
	}
	return err.Error()
}

// Health returns service status.
func Health(db *sql.DB, zc *services.ZeroClaw, version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbStatus := "ok"
		if err := db.Ping(); err != nil {
			dbStatus = "error"
		}

		zcStatus := "unavailable"
		if zc.IsAvailable() {
			zcStatus = "available"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"db":       dbStatus,
			"zeroclaw": zcStatus,
			"version":  version,
		})
	}
}

// APILegalAcquisitions handles GET /api/legal/acquisitions/:run_id
func APILegalAcquisitions(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		runID, err := strconv.Atoi(c.Param("run_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
			return
		}

		acqs, warns, err := legality.LegalAcquisitions(db, runID)
		if err != nil {
			log.Printf("ERROR [%s %s]: %v", c.Request.Method, c.Request.URL.Path, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": apiErrorMsg(err)})
			return
		}

		var badgeCount int
		db.QueryRow(`SELECT badge_count FROM run_progress WHERE run_id = ?`, runID).Scan(&badgeCount) //nolint:errcheck

		c.JSON(http.StatusOK, gin.H{
			"run_id":       runID,
			"badge_count":  badgeCount,
			"acquisitions": acqs,
			"warnings":     warns,
		})
	}
}

// APILegalMoves handles GET /api/legal/moves/:run_id/:form_id
func APILegalMoves(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		runID, err := strconv.Atoi(c.Param("run_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
			return
		}
		formID, err := strconv.Atoi(c.Param("form_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form_id"})
			return
		}

		moves, warns, err := legality.LegalMoves(db, runID, formID)
		if err != nil {
			log.Printf("ERROR [%s %s]: %v", c.Request.Method, c.Request.URL.Path, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": apiErrorMsg(err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"run_id":   runID,
			"form_id":  formID,
			"moves":    moves,
			"warnings": warns,
		})
	}
}

// APILegalItems handles GET /api/legal/items/:run_id
func APILegalItems(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		runID, err := strconv.Atoi(c.Param("run_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
			return
		}

		items, err := legality.LegalItems(db, runID)
		if err != nil {
			log.Printf("ERROR [%s %s]: %v", c.Request.Method, c.Request.URL.Path, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": apiErrorMsg(err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"run_id": runID,
			"items":  items,
		})
	}
}

// APILegalEvolutions handles GET /api/legal/evolutions/:run_id/:form_id
func APILegalEvolutions(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		runID, err := strconv.Atoi(c.Param("run_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
			return
		}
		formID, err := strconv.Atoi(c.Param("form_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form_id"})
			return
		}

		evos, err := legality.EvolutionOptions(db, runID, formID)
		if err != nil {
			log.Printf("ERROR [%s %s]: %v", c.Request.Method, c.Request.URL.Path, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": apiErrorMsg(err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"run_id":     runID,
			"form_id":    formID,
			"evolutions": evos,
		})
	}
}
