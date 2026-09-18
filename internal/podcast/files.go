package podcast

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func FileSHA256(path string) (string, int64, error) {
	// #nosec G304 -- callers supply explicit CLI or repository artifact paths.
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open audio: %w", err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash audio: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
