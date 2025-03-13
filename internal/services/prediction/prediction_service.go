package prediction

import (
	"fmt"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tonespy/ecosort_be/config"
	"github.com/tonespy/ecosort_be/pkg/logger"

	"github.com/nfnt/resize"
	tf "github.com/wamuir/graft/tensorflow"
)

type PredictionService struct {
	Config *config.Config
	Logger *logger.Logger
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
		ext := filepath.Ext(file.Filename)
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

// loadKerasModel loads the Keras model from the given path
func (p *PredictionService) loadKerasModel(path string) (*tf.SavedModel, error) {
	// Load the TensorFlow SavedModel
	model, err := tf.LoadSavedModel(path, []string{"serve"}, nil)
	if err != nil {
		return nil, err
	}

	return model, nil
}

func (p *PredictionService) loadKerasModelFromVersion() (*tf.SavedModel, error) {
	// Latest model version
	latestVersion := p.Config.ModelVersions[len(p.Config.ModelVersions)-1].Version
	// Retrieve model path
	modelPath := filepath.Join(p.Config.RootDir, "tmp", latestVersion+".keras")
	return p.loadKerasModel(modelPath)
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

	filePath := fmt.Sprintf("%s/%s", tmpDir, file.Filename)

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

func (p *PredictionService) PredictImage(filePath string) (string, error) {
	// Retrieve the model
	model, err := p.loadKerasModelFromVersion()
	if err != nil {
		return "", fmt.Errorf("failed to load model: %v", err)
	}
	defer model.Session.Close()

	// Defer deleting the tmp directory
	defer os.RemoveAll(filepath.Dir(filePath))

	// Preprocess the image
	tensorData, err := preprocessImage(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to preprocess image: %v", err)
	}

	// Debug: Log model operations
	// for _, op := range model.Graph.Operations() {
	// 	fmt.Println("Operation:", op.Name())
	// }

	// Create a tensor from the image data
	fmt.Println("Tensor data length: ", len(tensorData))
	// Reshape tensor to batch format [1, 256, 256, 3]
	batchTensor := [][][][]float32{tensorData}
	tensor, err := tf.NewTensor(batchTensor)
	if err != nil {
		return "", fmt.Errorf("failed to create tensor: %v", err)
	}
	fmt.Printf("Tensor Shape: %v\n", tensor.Shape())

	// Run the model
	fmt.Println("Running model...")
	result, err := model.Session.Run(map[tf.Output]*tf.Tensor{
		model.Graph.Operation("serve_eco_sort_static_input_layer").Output(0): tensor,
	}, []tf.Output{
		model.Graph.Operation("StatefulPartitionedCall").Output(0),
	}, nil)
	if err != nil {
		fmt.Println("Error running model: ", err)
		return "", fmt.Errorf("failed to run model: %v", err)
	}

	// Get the predicted class
	probabilities := result[0].Value().([][]float32)[0]
	predictedClass := getPredictedClass(probabilities)

	// Get the class name from Index
	filtered := filterClassName(p.Config.SupportedClasses, func(i int) bool {
		return i == predictedClass
	})
	if len(filtered) == 0 {
		return "", fmt.Errorf("class not found")
	}

	return filtered[0].Name, nil
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
