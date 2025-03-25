package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tonespy/ecosort_be/config"
	"github.com/tonespy/ecosort_be/internal/handlers"
	"github.com/tonespy/ecosort_be/internal/middleware"
	"github.com/tonespy/ecosort_be/pkg/logger"
)

type Server struct {
	Logger *logger.Logger
	Config *config.Config
}

func (s *Server) NewRouter() *gin.Engine {
	router := gin.Default()
	gin.SetMode(s.Config.GinMode)

	// Create handlers
	predictionHandler := handlers.BuildPredictionHandler(s.Config, s.Logger)

	// Apply middleware
	router.Use(middleware.DefaultClientAuth(s.Config.APIKey))

	// No route handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{})
		c.Abort()
	})

	// Define routes
	groupV1 := router.Group("/v1")

	// Define prediction routes
	predictionHandler.RegisterRoutes(groupV1)
	return router
}
