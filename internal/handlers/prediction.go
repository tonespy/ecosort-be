package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tonespy/ecosort_be/config"
	predictionService "github.com/tonespy/ecosort_be/internal/services/prediction"
	"github.com/tonespy/ecosort_be/pkg/logger"
)

type PredictionHandler struct {
	PredictionService predictionService.PredictionService
}

func (h *PredictionHandler) PredictImage(c *gin.Context) {
	// Log header and multipart form data
	// Debug: Log all form fields
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form", "details": err.Error()})
		return
	}

	// Log all form fields
	for key, values := range form.Value {
		h.PredictionService.Logger.Info("Form field", map[string]interface{}{"key": key, "values": values})
	}

	// Log all uploaded files
	for key, files := range form.File {
		for _, file := range files {
			h.PredictionService.Logger.Info("File field", map[string]interface{}{"key": key, "file": file})
		}
	}

	// Get the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get file"})
		return
	}

	// Validate the file
	tempFile, err := h.PredictionService.ValidateAndGetTemp(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save temporarily
	err = c.SaveUploadedFile(file, tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// Predict the image
	prediction, err := h.PredictionService.PredictImage(tempFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to predict image", "details": err.Error()})
		return
	}

	// Return the prediction
	c.JSON(http.StatusOK, gin.H{"prediction": prediction})
}

func (h *PredictionHandler) GetConfig(c *gin.Context) {
	versions := h.PredictionService.GetModelVersions()
	classes := h.PredictionService.GetSupportedClasses()
	availableGroupings := h.PredictionService.GetAvailableGroups()
	response := gin.H{
		"versions": versions,
		"classes":  classes,
		"groups":   availableGroupings,
	}
	c.JSON(http.StatusOK, response)
}

func BuildPredictionHandler(config *config.Config, logger *logger.Logger) *PredictionHandler {
	predictionService := predictionService.PredictionService{
		Config: config,
		Logger: logger,
	}
	return &PredictionHandler{
		PredictionService: predictionService,
	}
}

func (h *PredictionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/predict", h.PredictImage)
	router.GET("/predict/config", h.GetConfig)
}
