// Package site owns static output concerns. It depends only on podcast-domain
// values; release verification deliberately remains outside this package.
package site

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func RejectUnsafeOutput(path string) error {
	clean := filepath.Clean(path)
	if clean == "." || clean == string(filepath.Separator) || clean == "" {
		return fmt.Errorf("refusing unsafe output directory %q", path)
	}
	return nil
}

func WriteFile(path string, data []byte) error {
	// #nosec G301 -- generated public-site directories must be traversable by the hosting server.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	// #nosec G306,G703 -- path is derived from a validated build root; generated site files are intentionally public-readable.
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func CopyStaticAssets(outDir string) error {
	for _, name := range []string{"favicon.png", "pplsg-cover-840.webp"} {
		// #nosec G304 -- name comes from the fixed static-asset allowlist above.
		data, err := os.ReadFile(filepath.Join("static", name))
		if err != nil {
			return fmt.Errorf("read static asset %q: %w", name, err)
		}
		if err := WriteFile(filepath.Join(outDir, name), data); err != nil {
			return err
		}
	}
	return nil
}

func CopyCoverArt(outDir, source string) error {
	if source == "" {
		return nil
	}
	// #nosec G304 -- podcast.ValidateConfig requires a safe repository-relative cover-art source.
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open cover art for build: %w", err)
	}
	defer func() { _ = input.Close() }()
	destination := filepath.Join(outDir, filepath.Clean(source))
	// #nosec G301 -- generated public-site directories must be traversable by the hosting server.
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create cover art output directory: %w", err)
	}
	// #nosec G302,G304 -- destination is beneath the validated build root and is intentionally public-readable.
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create cover art output: %w", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("copy cover art: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close cover art output: %w", err)
	}
	return nil
}
