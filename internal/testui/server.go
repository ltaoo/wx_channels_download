package testui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// Server wraps the test UI HTTP server.
type Server struct {
	engine  *gin.Engine
	disc    *discoverer
	run     *runner
	modDir  string
	addr    string
}

// NewServer creates a new test UI server.
// modDir is the Go module root (where go.mod lives).
// addr is the listen address, e.g. "127.0.0.1:2024".
func NewServer(modDir, addr string) *Server {
	s := &Server{
		disc:   newDiscoverer(modDir),
		run:    newRunner(modDir, 5*time.Minute),
		modDir: modDir,
		addr:   addr,
	}

	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()
	s.engine.Use(gin.Recovery())
	s.engine.Use(corsMiddleware())

	s.engine.GET("/", s.handleIndex)
	s.engine.GET("/api/tests", s.handleListTests)
	s.engine.POST("/api/tests/run", s.handleRunTest)
	s.engine.GET("/api/tests/run/:runId", s.handleRunResult)

	// Start preloading the test cache in background
	go s.disc.Discover()

	return s
}

// Addr returns the listen address.
func (s *Server) Addr() string {
	return s.addr
}

// Handler returns the HTTP handler for use with http.Server.
func (s *Server) Handler() http.Handler {
	return s.engine
}

// Close cleans up resources.
func (s *Server) Close() {}

// --- Handlers ---

func (s *Server) handleIndex(ctx *gin.Context) {
	html, err := TestUIPage()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "failed to load UI: %v", err)
		return
	}
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

type listTestsResponse struct {
	Packages []PackageInfo `json:"packages"`
	ModDir   string        `json:"mod_dir"`
}

func (s *Server) handleListTests(ctx *gin.Context) {
	packages, err := s.disc.Discover()
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"code": 1,
			"msg":  fmt.Sprintf("failed to discover tests: %v", err),
		})
		return
	}
	if packages == nil {
		packages = []PackageInfo{}
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": listTestsResponse{
			Packages: packages,
			ModDir:   s.modDir,
		},
	})
}

type runTestRequest struct {
	Pkg  string `json:"pkg" form:"pkg"`
	Test string `json:"test" form:"test"`
}

func (s *Server) handleRunTest(ctx *gin.Context) {
	var req runTestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// try query params for GET-style
		req.Pkg = ctx.Query("pkg")
		req.Test = ctx.Query("test")
	}
	if req.Pkg == "" {
		ctx.JSON(http.StatusOK, gin.H{
			"code": 400,
			"msg":  "pkg is required",
		})
		return
	}

	runID := s.run.StartRun(req.Pkg, req.Test)
	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"run_id": runID,
		},
	})
}

func (s *Server) handleRunResult(ctx *gin.Context) {
	runID := ctx.Param("runId")
	result := s.run.GetResult(runID)
	if result == nil {
		ctx.JSON(http.StatusOK, gin.H{
			"code": 404,
			"msg":  "run not found",
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": result,
	})
}

// --- Helpers ---

func corsMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

// ResolveModDir finds the Go module root by looking for go.mod
// starting from dir and walking up. If not found, returns dir.
func ResolveModDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	current := abs
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			// reached filesystem root without finding go.mod
			return abs
		}
		current = parent
	}
}
