package prediction

import (
	"fmt"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tonespy/ecosort_be/config"
	"github.com/tonespy/ecosort_be/pkg/logger"

	"github.com/nfnt/resize"
	tf "github.com/wamuir/graft/tensorflow"
)

type PredictionService struct {
	Config       *config.Config
	Logger       *logger.Logger
	model        *tf.SavedModel
	sessionMutex sync.Mutex
}

// Allowed MIME types for images and videos
var allowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"video/mp4":  true,
	"video/avi":  true,
	"video/mpeg": true,
}

// jobProgressMap is an in-memory "database" for job progress.
var jobProgressMap = struct {
	sync.RWMutex
	Data map[string]JobProgress
}{
	Data: make(map[string]JobProgress),
}

var (
	// wsConnections stores active WebSocket connections keyed by job ID.
	wsConnections = struct {
		sync.RWMutex
		Connections map[string]*websocket.Conn
	}{Connections: make(map[string]*websocket.Conn)}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type JobImagePrediction struct {
	JobID      string         `json:"jobID"`
	Prediction config.Classes `json:"prediction"`
	ImageName  string         `json:"imageName"`
	Status     string         `json:"status,omitempty"`
}

type JobProgress struct {
	Progress    int                  `json:"progress"`              // Percentage progress (0 to 100)
	Status      string               `json:"status"`                // e.g. "running", "completed", "stopped"
	Predictions []JobImagePrediction `json:"predictions,omitempty"` // Batch predictions (e.g., filenames or other result strings)
}

// InitModel loads the TensorFlow model once and stores it for reuse.
func (p *PredictionService) InitModel() error {
	// Use the latest model version from configuration.
	latestVersion := p.Config.ModelVersions[len(p.Config.ModelVersions)-1].Version
	modelPath := filepath.Join(p.Config.RootDir, "tmp", latestVersion+".keras")
	model, err := tf.LoadSavedModel(modelPath, []string{"serve"}, nil)
	if err != nil {
		return fmt.Errorf("failed to load model: %v", err)
	}
	p.model = model
	p.Logger.Info("Model loaded from "+modelPath, nil)
	return nil
}

// getWebSocketConnection retrieves the WebSocket connection for a given jobID.
func getWebSocketConnection(jobID string) (*websocket.Conn, bool) {
	wsConnections.RLock()
	defer wsConnections.RUnlock()
	conn, ok := wsConnections.Connections[jobID]
	return conn, ok
}

func (p *PredictionService) GetUpgrader() websocket.Upgrader {
	return upgrader
}

func (p *PredictionService) GetWsConnections() *struct {
	sync.RWMutex
	Connections map[string]*websocket.Conn
} {
	return &wsConnections
}

func (p *PredictionService) GetJobProgressMap() *struct {
	sync.RWMutex
	Data map[string]JobProgress
} {
	return &jobProgressMap
}

// processPredictions simulates batched prediction processing.
func (p *PredictionService) ProcessPredictions(jobID string, files []*multipart.FileHeader, jobDir string) {
	batchSize := 10
	total := len(files)
	for i := 0; i < total; i += batchSize {
		end := min(i+batchSize, total)

		var predictions []JobImagePrediction
		for j := i; j < end; j++ {
			predictionResult, err := p.PredictImage(filepath.Join(jobDir, getJpgFileName(files[j])))
			statusInfo := "Completed"
			if err != nil {
				statusInfo = "Failed"
			}
			resultInfo := config.Classes{}
			if predictionResult != nil {
				resultInfo = *predictionResult
			}
			prediction := JobImagePrediction{
				JobID:      jobID,
				Prediction: resultInfo,
				ImageName:  getJpgFileName(files[j]),
				Status:     statusInfo,
			}
			predictions = append(predictions, prediction)
		}

		update := JobProgress{
			Progress:    (end * 100) / total,
			Status:      "running",
			Predictions: predictions,
		}

		// Update the in-memory database.
		jobProgressMap.Lock()
		// If JobID exists, update the predictions, status and progress. Otherwise, create a new entry.
		if _, ok := jobProgressMap.Data[jobID]; !ok {
			jobProgressMap.Data[jobID] = update
		} else {
			previousPredictions := jobProgressMap.Data[jobID]
			update.Predictions = append(previousPredictions.Predictions, update.Predictions...)
			jobProgressMap.Data[jobID] = update
		}
		jobProgressMap.Unlock()

		// If a WebSocket connection exists, send the update.
		if ws, ok := getWebSocketConnection(jobID); ok {
			ws.WriteJSON(update)
		}

		time.Sleep(1 * time.Second) // Simulate processing delay.
	}

	// Final update: mark as completed.
	finalUpdate := JobProgress{
		Progress: 100,
		Status:   "completed",
	}
	if _, ok := jobProgressMap.Data[jobID]; !ok {
		jobProgressMap.Data[jobID] = finalUpdate
	} else {
		previousPredictions := jobProgressMap.Data[jobID]
		finalUpdate.Predictions = previousPredictions.Predictions
		jobProgressMap.Data[jobID] = finalUpdate
	}
	jobProgressMap.Lock()
	jobProgressMap.Data[jobID] = finalUpdate
	jobProgressMap.Unlock()
	if ws, ok := getWebSocketConnection(jobID); ok {
		result := map[string]any{"jobID": jobID, "message": "Job completed", "update": finalUpdate}
		ws.WriteJSON(result)
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Job completed"))
		ws.Close()
		os.RemoveAll(filepath.Join("wsjobs", jobID))
	}
}

// validateFile checks the file type and size
func validateFile(file *multipart.FileHeader) error {
	// Open the file to check its MIME type
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer src.Close()

	// Check the file MIME type
	buffer := make([]byte, 512) // Read the first 512 bytes for MIME detection
	if _, err := src.Read(buffer); err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}
	mimeType := http.DetectContentType(buffer)

	// Log detected MIME type for debugging
	fmt.Printf("Detected MIME type: %s\n", mimeType)

	// Fallback to file extension if MIME detection fails
	if mimeType == "application/octet-stream" {
		ext := filepath.Ext(getJpgFileName(file))
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".png":
			mimeType = "image/png"
		case ".gif":
			mimeType = "image/gif"
		case ".mp4":
			mimeType = "video/mp4"
		case ".avi":
			mimeType = "video/avi"
		case ".mpeg":
			mimeType = "video/mpeg"
		default:
			return fmt.Errorf("unsupported file type: %s", mimeType)
		}
	}

	// Validate against allowed MIME types
	if !allowedMIMETypes[mimeType] {
		return fmt.Errorf("unsupported file type: %s", mimeType)
	}

	// Check file size: max 50 MB
	const maxFileSize = 50 << 20 // 50 MB
	if file.Size > maxFileSize {
		return fmt.Errorf("file is too large: %d bytes", file.Size)
	}

	return nil
}

func preprocessImage(imagePath string) ([][][]float32, error) {
	// Open image
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		return nil, err
	}

	// Resize to model input size (256x256)
	resizedImg := resize.Resize(256, 256, img, resize.Lanczos3)

	// Convert to float32 3D tensor values
	bounds := resizedImg.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y
	tensorData := make([][][]float32, height)
	for y := 0; y < height; y++ {
		row := make([][]float32, width)
		for x := 0; x < width; x++ {
			r, g, b, _ := resizedImg.At(x, y).RGBA()
			// Normalize pixel values to range [0, 1]
			row[x] = []float32{
				float32(r>>8) / 255.0,
				float32(g>>8) / 255.0,
				float32(b>>8) / 255.0,
			}
		}
		tensorData[y] = row
	}

	return tensorData, nil
}

func getJpgFileName(file *multipart.FileHeader) string {
	fullFileName := file.Filename
	if filepath.Ext(fullFileName) != ".jpg" {
		fullFileName += ".jpg"
	}
	return fullFileName
}

func (p *PredictionService) ValidateAndGetTemp(file *multipart.FileHeader) (string, error) {
	// Validate the file
	if err := validateFile(file); err != nil {
		return "", err
	}

	// Create tmp directory for processing file
	tmpDir, err := os.MkdirTemp(p.Config.RootDir, "tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}

	filePath := fmt.Sprintf("%s/%s", tmpDir, getJpgFileName(file))

	return filePath, nil
}

// getPredictedClass returns the index of the class with the highest probability
func getPredictedClass(probabilities []float32) int {
	maxIdx := 0
	maxProb := probabilities[0]
	for i, prob := range probabilities {
		if prob > maxProb {
			maxIdx = i
			maxProb = prob
		}
	}
	return maxIdx
}

func filterClassName(input []config.Classes, predicate func(int) bool) []config.Classes {
	var result []config.Classes
	for _, value := range input {
		if predicate(value.Index) {
			result = append(result, value)
		}
	}
	return result
}

// predictFromImageTensor performs inference on preprocessed tensor data using the shared model.
// It locks the session to ensure concurrent calls are serialized.
func (p *PredictionService) predictFromImageTensor(tensorData [][][]float32) (*config.Classes, error) {
	// Reshape tensor to batch format: [1, 256, 256, 3]
	batchTensor := [][][][]float32{tensorData}
	tensor, err := tf.NewTensor(batchTensor)
	if err != nil {
		return nil, fmt.Errorf("failed to create tensor: %v", err)
	}

	// Lock the session for thread-safe access.
	p.sessionMutex.Lock()
	defer p.sessionMutex.Unlock()

	result, err := p.model.Session.Run(
		map[tf.Output]*tf.Tensor{
			p.model.Graph.Operation("serve_eco_sort_static_input_layer").Output(0): tensor,
		},
		[]tf.Output{
			p.model.Graph.Operation("StatefulPartitionedCall").Output(0),
		},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run model: %v", err)
	}

	// Extract probabilities and determine the predicted class.
	probabilities := result[0].Value().([][]float32)[0]
	predictedClass := getPredictedClass(probabilities)

	// Map the index to a class name using the supported classes.
	filtered := filterClassName(p.Config.SupportedClasses, func(i int) bool {
		return i == predictedClass
	})
	if len(filtered) == 0 {
		return nil, fmt.Errorf("class not found")
	}

	return &filtered[0], nil
}

// PredictImage handles a single-image prediction using the shared model.
// It validates and preprocesses the image, then calls predictFromImageTensor.
func (p *PredictionService) PredictImage(filePath string) (*config.Classes, error) {
	// Defer cleanup of temporary files.
	// defer os.RemoveAll(filepath.Dir(filePath))
	defer os.Remove(filePath)

	// Preprocess the image into a tensor.
	tensorData, err := preprocessImage(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess image: %v", err)
	}

	// Use the shared inference function.
	return p.predictFromImageTensor(tensorData)
}

func (p *PredictionService) GetModelVersions() []config.ModelInfo {
	return p.Config.ModelVersions
}

func (p *PredictionService) GetSupportedClasses() []config.Classes {
	return p.Config.SupportedClasses
}

func (p *PredictionService) GetAvailableGroups() []config.GroupConfig {
	return p.Config.ModelGrouping
}
