package config

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type GroupConfig struct {
	Name        string          `json:"name"`
	GroupConfig []ClassGrouping `json:"group_config"`
}

type ClassGrouping struct {
	Name    string    `json:"name"`
	Classes []Classes `json:"classes"`
}

type Classes struct {
	Index        int    `json:"index"`
	Name         string `json:"name"`
	ReadableName string `json:"readable_name"`
	Description  string `json:"description"`
}

type ModelInfo struct {
	Version         string `json:"version"`
	Date            string `json:"date"`
	SavedModel      string `json:"url"`
	SavedModelSize  string `json:"model_size"`
	TFLiteModel     string `json:"tflite_url"`
	TFLiteModelSize string `json:"tflite_size"`
	Accuracy        string `json:"accuracy"`
}

type Config struct {
	Port             string
	GinMode          string
	ModelPath        string
	RootDir          string
	SupportedClasses []Classes
	ModelVersions    []ModelInfo
	ModelAPIKey      string
	APIKey           string
	ModelGrouping    []GroupConfig
}

// GetBaseWorkingDirectory returns the base project directory
func getBaseWorkingDirectory() (string, error) {
	// Start from the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Traverse upwards to find the base directory
	for currentDir != "/" { // Stop at the root directory
		// Check for a file or folder that signifies the base directory
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir, nil
		}
		if _, err := os.Stat(filepath.Join(currentDir, ".git")); err == nil {
			return currentDir, nil
		}

		// Move up one level
		currentDir = filepath.Dir(currentDir)
	}

	return "", fmt.Errorf("could not find the project base directory")
}

// Convert bytes to MB: 59098816 bytes
// 59098816 / 1024 / 1024 = ~56.36 MB
// 59098816 / 1024 / 1024 / 1024 = ~0.055 GB

// Helper function to convert bytes to human readable format
func bytesToHumanReadable(bytes int) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func PrepareConfig() (*Config, error) {
	// Get default model path from <root directory>/tmp folder
	// Get root directory
	rootDir, err := getBaseWorkingDirectory()
	if err != nil {
		return nil, err
	}
	modelPath := filepath.Join(rootDir, "tmp", "1.0.1.keras")
	supportedClasses := []Classes{
		{
			0,
			"battery",
			"Battery",
			"Used batteries, including rechargeable batteries, contain heavy metals and other toxic substances. They should not be disposed of in the trash. Instead, they should be taken to a recycling center or a hazardous waste facility.",
		},
		{
			1,
			"biological",
			"Biological",
			"Biological waste is any waste that is generated from the human body or from plants or animals. This includes things like food scraps, yard waste, and other organic materials.",
		},
		{
			2,
			"brown-glass",
			"Brown Glass",
			"Brown glass is used to make beer and liquor bottles. It is 100% recyclable and can be recycled endlessly without loss in quality or purity.",
		},
		{
			3,
			"cardboard",
			"Cardboard",
			"Cardboard is a heavy type of paper that is used to make boxes and other types of packaging. It is recyclable and can be used to make new cardboard boxes, paper towels, and other paper products.",
		},
		{
			4,
			"clothes",
			"Clothes",
			"Textiles, including clothes, and linens, can be recycled or donated to charity. If they are no longer wearable, they can be repurposed into rags or other items.",
		},
		{
			5,
			"green-glass",
			"Green Glass",
			"Green glass is used to make wine bottles and other types of containers. It is 100% recyclable and can be recycled endlessly without loss in quality or purity.",
		},
		{
			6,
			"metal",
			"Metal",
			"Metal is a valuable material that can be recycled over and over again without losing its properties. It can be used to make new metal products, such as cans, appliances, and building materials.",
		},
		{
			7,
			"paper",
			"Paper",
			"Paper is a versatile material that can be recycled into new paper products, such as newspapers, magazines, and packaging materials. It is important to recycle paper to save trees and reduce waste.",
		},
		{
			8,
			"plastic",
			"Plastic",
			"Plastic is a synthetic material that is used to make a wide range of products, including bottles, containers, and packaging materials. It is important to recycle plastic to reduce waste and protect the environment.",
		},
		{
			9,
			"shoes",
			"Shoes",
			"Shoes can be recycled or donated to charity. If they are no longer wearable, they can be repurposed into new products, such as playground surfaces or athletic fields.",
		},
		{
			10,
			"trash",
			"Trash",
			"Trash is any waste that cannot be recycled or composted. It includes things like plastic bags, styrofoam, and other non-recyclable materials.",
		},
		{
			11,
			"white-glass",
			"White Glass",
			"White glass is used to make clear glass containers, such as jars and bottles. It is 100% recyclable and can be recycled endlessly without loss in quality or purity.",
		},
	}

	defaultGrouping := []ClassGrouping{
		{
			"Glass",
			[]Classes{
				{
					2,
					"brown-glass",
					"Brown Glass",
					"Brown glass is used to make beer and liquor bottles. It is 100% recyclable and can be recycled endlessly without loss in quality or purity.",
				},
				{
					5,
					"green-glass",
					"Green Glass",
					"Green glass is used to make wine bottles and other types of containers. It is 100% recyclable and can be recycled endlessly without loss in quality or purity.",
				},
				{
					11,
					"white-glass",
					"White Glass",
					"White glass is used to make clear glass containers, such as jars and bottles. It is 100% recyclable and can be recycled endlessly without loss in quality or purity.",
				},
			},
		},
		{
			"Papers",
			[]Classes{
				{
					3,
					"cardboard",
					"Cardboard",
					"Cardboard is a heavy type of paper that is used to make boxes and other types of packaging. It is recyclable and can be used to make new cardboard boxes, paper towels, and other paper products.",
				},
				{
					7,
					"paper",
					"Paper",
					"Paper is a versatile material that can be recycled into new paper products, such as newspapers, magazines, and packaging materials. It is important to recycle paper to save trees and reduce waste.",
				},
			},
		},
		{
			"Food",
			[]Classes{
				{
					1,
					"biological",
					"Biological",
					"Biological waste is any waste that is generated from the human body or from plants or animals. This includes things like food scraps, yard waste, and other organic materials.",
				},
			},
		},
		{
			"Trash",
			[]Classes{
				{
					0,
					"battery",
					"Battery",
					"Used batteries, including rechargeable batteries, contain heavy metals and other toxic substances. They should not be disposed of in the trash. Instead, they should be taken to a recycling center or a hazardous waste facility.",
				},
				{
					4,
					"clothes",
					"Clothes",
					"Textiles, including clothes, and linens, can be recycled or donated to charity. If they are no longer wearable, they can be repurposed into rags or other items.",
				},
				{
					6,
					"metal",
					"Metal",
					"Metal is a valuable material that can be recycled over and over again without losing its properties. It can be used to make new metal products, such as cans, appliances, and building materials.",
				},
				{
					8,
					"plastic",
					"Plastic",
					"Plastic is a synthetic material that is used to make a wide range of products, including bottles, containers, and packaging materials. It is important to recycle plastic to reduce waste and protect the environment.",
				},
				{
					9,
					"shoes",
					"Shoes",
					"Shoes can be recycled or donated to charity. If they are no longer wearable, they can be repurposed into new products, such as playground surfaces or athletic fields.",
				},
				{
					10,
					"trash",
					"Trash",
					"Trash is any waste that cannot be recycled or composted. It includes things like plastic bags, styrofoam, and other non-recyclable materials.",
				},
			},
		},
	}

	availableGroups := []GroupConfig{
		{
			"Default",
			defaultGrouping,
		},
	}

	err = godotenv.Load()
	if err != nil {
		fmt.Printf("No .env file found. Assuming environment variables are set by the system.")
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.TestMode
	}
	versions := []ModelInfo{
		{
			"1.0.0",
			"2024-12-17",
			"https://api.github.com/repos/tonespy/uol_bsc/releases/assets/226864547",
			bytesToHumanReadable(439050819),
			"https://api.github.com/repos/tonespy/uol_bsc/releases/assets/229632801",
			bytesToHumanReadable(59098816),
			"73%",
		},
		{
			"1.0.1",
			"2024-12-18",
			"https://api.github.com/repos/tonespy/uol_bsc/releases/assets/229632683",
			bytesToHumanReadable(439193592),
			"https://api.github.com/repos/tonespy/uol_bsc/releases/assets/229632373",
			bytesToHumanReadable(59098816),
			"79%",
		},
	}

	modelAPIKey := os.Getenv("MODEL_RELEASE_API_KEY")
	if modelAPIKey == "" {
		return nil, fmt.Errorf("MODEL_RELEASE_API_KEY is not set")
	}

	apiKey := os.Getenv("API_REQ_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("API_REQ_KEY is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "5500"
	}

	return &Config{
		Port:             ":" + port,
		GinMode:          ginMode,
		ModelPath:        modelPath,
		RootDir:          rootDir,
		SupportedClasses: supportedClasses,
		ModelVersions:    versions,
		ModelAPIKey:      modelAPIKey,
		APIKey:           apiKey,
		ModelGrouping:    availableGroups,
	}, nil
}

func DownloadModel(config Config) error {
	latestModel := config.ModelVersions[len(config.ModelVersions)-1]
	modelUrl := latestModel.SavedModel
	modelVersion := latestModel.Version
	apiKey := config.ModelAPIKey
	output_name := filepath.Join(config.RootDir, "tmp", modelVersion+".keras")
	output_zip := filepath.Join(config.RootDir, "tmp", modelVersion+".keras.zip")

	// Check if folder already exist and not empty
	if _, err := os.Stat(output_name); err == nil {
		fmt.Println("Model already downloaded")
		return nil
	}

	// Create the output file
	outputZipFile, err := os.Create(output_zip)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer outputZipFile.Close()

	// Construct request to download from git and extract to the tmp folder
	// Prepare the HTTP GET request.
	req, err := http.NewRequest("GET", modelUrl, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return err
	}

	// Add the required headers.
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Add("Accept", "application/octet-stream")

	// Send the request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status code.
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error: unexpected status code: %d\n", resp.StatusCode)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Write the response body to a outputZipFile.
	_, err = io.Copy(outputZipFile, resp.Body)
	if err != nil {
		fmt.Printf("Error writing response body: %v\n", err)
		return fmt.Errorf("error writing response body: %v", err)
	}

	// Unzip the file to tmp folder located in config.RootDir
	fmt.Println("Extracting to tmp folder")
	err = unzip(output_zip, filepath.Join(config.RootDir, "tmp"))
	if err != nil {
		fmt.Printf("Error extracting zip file: %v\n", err)
		return fmt.Errorf("error extracting zip file: %v", err)
	}

	// Clearn up the zip file
	err = os.Remove(output_zip)
	if err != nil {
		fmt.Printf("Error removing zip file: %v\n", err)
	}

	return nil
}

// unzip extracts a zip archive specified by src into a destination directory dest.
func unzip(src string, dest string) error {
	// Open the zip archive for reading.
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// Iterate through each file in the archive.
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Prevent ZipSlip (Directory traversal vulnerability)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		// Create directories if needed.
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		// Ensure the directory exists.
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// Create the file.
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		// Copy file content.
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// generateAPIKey generates a random API key of n bytes and returns it as a hex string.
func GenerateAPIKey(n int) (string, error) {
	bytes := make([]byte, n)
	// Read n random bytes from crypto/rand.
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	// Encode the random bytes as a hexadecimal string.
	return hex.EncodeToString(bytes), nil
}
