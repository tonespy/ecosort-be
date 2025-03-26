package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tonespy/ecosort_be/config"
	predictionService "github.com/tonespy/ecosort_be/internal/services/prediction"
	"github.com/tonespy/ecosort_be/pkg/logger"
)

type PredictionHandler struct {
	PredictionService *predictionService.PredictionService
}

func (h *PredictionHandler) BatchPredict(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid multipart form"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	// Generate a unique job ID.
	jobID := uuid.New().String()
	// Create a job-specific directory.
	jobDir := filepath.Join("wsjobs", jobID)
	if err := os.MkdirAll(jobDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job directory"})
		return
	}

	// Save each file.
	for _, file := range files {
		savePath := filepath.Join(jobDir, file.Filename)
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save file %s", file.Filename)})
			return
		}
	}

	// Start background processing.
	go h.PredictionService.ProcessPredictions(jobID, files, jobDir)

	// Return the job ID to the client.
	c.JSON(http.StatusOK, gin.H{"jobID": jobID, "message": "Files uploaded successfully"})
}

// PredictionsWebSocketHandler upgrades the connection and registers it.
func (h *PredictionHandler) PredictionsWebSocketHandler(c *gin.Context) {
	jobID := c.Query("jobID")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing jobID query parameter"})
		return
	}

	upgrader := h.PredictionService.GetUpgrader()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	wsConnections := h.PredictionService.GetWsConnections()

	// Register connection.
	wsConnections.Lock()
	wsConnections.Connections[jobID] = conn
	wsConnections.Unlock()

	// Send current progress immediately (if available).
	jobProgressMap := h.PredictionService.GetJobProgressMap()
	jobProgressMap.RLock()
	if progress, ok := jobProgressMap.Data[jobID]; ok {
		conn.WriteJSON(progress)
		if progress.Progress == 100 {
			delete(jobProgressMap.Data, jobID)
			wsConnections.Connections[jobID].WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Job completed"))
			wsConnections.Connections[jobID].Close()
			os.RemoveAll(filepath.Join("wsjobs", jobID))
		}
	}
	jobProgressMap.RUnlock()

	// Keep connection alive by reading (to detect disconnect).
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	// Remove connection when closed.
	wsConnections.Lock()
	delete(wsConnections.Connections, jobID)
	wsConnections.Unlock()
}

// JobProgressHandler returns the current progress for a given jobID.
func (h *PredictionHandler) JobProgressHandler(c *gin.Context) {
	jobID := c.Query("jobID")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing jobID"})
		return
	}

	jobProgressMap := h.PredictionService.GetJobProgressMap()
	jobProgressMap.RLock()
	progress, ok := jobProgressMap.Data[jobID]
	jobProgressMap.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}
	c.JSON(http.StatusOK, progress)
}

func (h *PredictionHandler) PredictImage(c *gin.Context) {
	// Log header and multipart form data
	// Debug: Log all form fields
	_, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form", "details": err.Error()})
		return
	}

	// Log all form fields
	// for key, values := range form.Value {
	// 	h.PredictionService.Logger.Info("Form field", map[string]interface{}{"key": key, "values": values})
	// }

	// // Log all uploaded files
	// for key, files := range form.File {
	// 	for _, file := range files {
	// 		h.PredictionService.Logger.Info("File field", map[string]interface{}{"key": key, "file": file})
	// 	}
	// }

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
	predictionService := &predictionService.PredictionService{
		Config: config,
		Logger: logger,
	}

	// Initialize the shared TensorFlow model.
	if err := predictionService.InitModel(); err != nil {
		log.Fatalf("Model initialization failed: %v", err)
	}

	return &PredictionHandler{
		PredictionService: predictionService,
	}
}

func (h *PredictionHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/predict", h.PredictImage)
	router.POST("/predict/batch", h.BatchPredict)
	router.GET("/predict/websocket", h.PredictionsWebSocketHandler)
	router.GET("/predict/config", h.GetConfig)
}
