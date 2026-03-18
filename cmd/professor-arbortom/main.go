package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pokemondata "github.com/TravisPenn/professor-arbortom/data"
	"github.com/TravisPenn/professor-arbortom/internal/db"
	"github.com/TravisPenn/professor-arbortom/internal/handlers"
	"github.com/TravisPenn/professor-arbortom/internal/pokeapi"
	"github.com/TravisPenn/professor-arbortom/internal/services"
	pokestatic "github.com/TravisPenn/professor-arbortom/static"
	poketemplates "github.com/TravisPenn/professor-arbortom/templates"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// Version is injected at build time via -ldflags "-X main.Version=<sha>"
var Version = "dev"

func main() {
	// Load .env (ignore error — env may be set by systemd EnvironmentFile)
	_ = godotenv.Load()

	dbPath := os.Getenv("POKEMON_DB_PATH")
	if dbPath == "" {
		log.Fatal("POKEMON_DB_PATH is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	// Open + migrate database
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	// Apply pre-seeded reference data if the DB is fresh.
	// Prefers seeds.sql adjacent to the DB file; falls back to the copy embedded
	// in the binary (data/seeds.sql) so fresh Docker volumes hydrate instantly.
	if err := db.ApplySeedsIfEmpty(database, dbPath, pokemondata.SeedsSQL); err != nil {
		log.Printf("seeds: %v", err) // non-fatal
	}

	// PokeAPI client
	pokeClient := pokeapi.New(database, dbPath)

	// AI Coach client
	coachHost := os.Getenv("COACH_HOST")
	coachModel := os.Getenv("COACH_MODEL")
	if coachHost != "" && coachModel == "" {
		coachModel = "qwen2.5:3b"
	}
	coachPrompt := os.Getenv("COACH_SYSTEM_PROMPT")
	zc := services.NewCoachClient(coachHost, coachModel, coachPrompt)

	// SEC-005: Validate AI Coach config at startup to prevent SSRF.
	if err := zc.ValidateConfig(); err != nil {
		log.Fatalf("coach config: %v", err)
	}

	// Parse templates
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
		"mkrange": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"toJSON": func(v any) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(b), nil //nolint:gosec // data is server-controlled, not user input
		},
		// evoMethod formats an evolution trigger+conditions map into a short
		// human-readable label, e.g. "Lv 36", "use Fire Stone", "trade".
		"evoMethod": func(trigger string, conds map[string]interface{}) string {
			toInt := func(v interface{}) int {
				switch t := v.(type) {
				case float64:
					return int(t)
				case int:
					return t
				}
				return 0
			}
			switch trigger {
			case "level-up":
				if v, ok := conds["min_level"]; ok {
					if lvl := toInt(v); lvl > 0 {
						return fmt.Sprintf("Lv %d", lvl)
					}
				}
				if _, ok := conds["friendship"]; ok {
					return "high friendship"
				}
				if _, ok := conds["held_item_id"]; ok {
					return "level-up (held item)"
				}
				return "level-up"
			case "use-item":
				return "use item"
			case "trade":
				if ts, ok := conds["trade_species"]; ok {
					return fmt.Sprintf("trade for %v", ts)
				}
				return "trade"
			default:
				if trigger != "" {
					return trigger
				}
				return "?"
			}
		},
		"lower": strings.ToLower,
		"title": strings.Title, //nolint:staticcheck // acceptable for display
		"join":  strings.Join,
	}
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(poketemplates.FS, "*.html")
	if err != nil {
		log.Fatalf("template parse: %v", err)
	}

	// SEC-007: Set Gin mode from GIN_MODE env var; default to release for production.
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Gin router
	r := gin.Default()
	r.SetHTMLTemplate(tmpl)

	// SEC-010: Disable trusting all proxies. If behind a reverse proxy, set
	// the proxy IP explicitly instead of nil.
	r.SetTrustedProxies(nil) //nolint:errcheck

	// SEC-003: Security headers on all responses.
	r.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'")
		c.Next()
	})

	// Static files
	r.StaticFS("/static", http.FS(pokestatic.FS))

	// Routes
	r.GET("/", handlers.RedirectToRuns)
	r.GET("/health", handlers.Health(database, zc, Version))

	runs := r.Group("/runs")
	{
		runs.GET("", handlers.ListRuns(database))
		runs.POST("", handlers.CreateRun(database, pokeClient))

		run := runs.Group("/:run_id", handlers.RunContextMiddleware(database))
		{
			run.GET("", handlers.ShowRun)
			run.GET("/overview", handlers.ShowOverview(database, zc))
			run.POST("/archive", handlers.ArchiveRun(database))
			run.POST("/unarchive", handlers.UnarchiveRun(database))
			run.GET("/progress", handlers.ShowProgress(database, pokeClient))
			run.POST("/progress", handlers.UpdateProgress(database, pokeClient))
			run.GET("/progress/hydration", handlers.HydrationStatus(database))
			run.GET("/team", handlers.ShowTeam(database))
			run.GET("/team/:slot", handlers.ShowTeamSlot(database))
			run.POST("/team", handlers.UpdateTeam(database))
			run.GET("/box", handlers.ShowBox(database))
			run.POST("/box/:entry_id/faint", handlers.MarkFainted(database))
			run.POST("/box/:entry_id/revive", handlers.MarkRevived(database))
			run.POST("/box/:entry_id/evolve", handlers.EvolveBox(database, pokeClient))
			run.GET("/routes", handlers.ShowRoutes(database, pokeClient))
			run.POST("/routes", handlers.LogEncounter(database, pokeClient))
			run.GET("/rules", handlers.ShowRules(database))
			run.POST("/rules", handlers.UpdateRules(database))
			run.GET("/coach", handlers.ShowCoach(database, pokeClient, zc))
			run.POST("/coach", handlers.QueryCoach(database, pokeClient, zc))
		}
	}

	api := r.Group("/api")
	{
		legal := api.Group("/legal")
		{
			legal.GET("/acquisitions/:run_id", handlers.APILegalAcquisitions(database))
			legal.GET("/moves/:run_id/:form_id", handlers.APILegalMoves(database))
			legal.GET("/items/:run_id", handlers.APILegalItems(database))
			legal.GET("/evolutions/:run_id/:form_id", handlers.APILegalEvolutions(database))
		}
	}

	log.Printf("PokemonProfessor %s listening on :%s", Version, port)

	// SEC-015: Graceful shutdown — drain in-flight requests before closing DB.
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// SEC-011: Wait for background PokeAPI goroutines before closing DB.
	pokeClient.Wait()

	log.Println("Server exited")
}
