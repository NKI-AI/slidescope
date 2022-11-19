package utils

// From https://github.com/koddr/example-go-config-yaml/
import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
)

// Config struct for webapp config
type Config struct {
	// TODO: Validate these inputs (sum 256, or format = png or jpg)
	DeepZoom struct {
		TileSize    int    `yaml:"tile_size"`
		TileOverlap int    `yaml:"tile_overlap"`
		Format      string `yaml:"format"`
	}

	Sqlite struct {
		Filename string `yaml:"filename"`
	}

	Server struct {
		// Port is the local machine TCP Port to bind the HTTP Server to
		Port string `yaml:"port"`
	} `yaml:"server"`
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

// ParseFlags will create and parse the CLI flags
// and return the path to be used elsewhere
func ParseFlags() (string, bool, error) {
	// String that contains the configured configuration path
	var configPath string
	var debugMode bool

	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configPath, "config", "./config.yaml", "path to config file")
	flag.BoolVar(&debugMode, "debug", false, "GIN debug mode")

	// Actually parse the flags
	flag.Parse()

	// Validate the path first
	if err := ValidateConfigPath(configPath); err != nil {
		return "", true, err
	}

	// Return the configuration path
	return configPath, debugMode, nil
}
