package podcast

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// DecodeYAMLFile accepts exactly one YAML document and rejects fields not in
// the destination contract. This is the repository's YAML trust boundary.
func DecodeYAMLFile(path string, target any) error {
	// #nosec G304 -- callers supply explicit CLI or repository artifact paths.
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("expected one YAML document")
		}
		return err
	}
	return nil
}

func LoadConfig(path string) (ShowConfig, error) {
	var config ShowConfig
	if err := DecodeYAMLFile(path, &config); err != nil {
		return ShowConfig{}, fmt.Errorf("load show configuration: %w", err)
	}
	return config, nil
}

func LoadEpisode(path string) (Episode, error) {
	var episode Episode
	if err := DecodeYAMLFile(path, &episode); err != nil {
		return Episode{}, fmt.Errorf("load episode manifest %q: %w", path, err)
	}
	return episode, nil
}
