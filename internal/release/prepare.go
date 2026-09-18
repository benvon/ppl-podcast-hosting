package release

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
	"gopkg.in/yaml.v3"
)

type PrepareOptions struct {
	SourceDir, AudioPath, OutputDir, StagingPrefix string
	RefreshExisting                                bool
}

type Prepared struct {
	ID, StagingKey, PublicKey, SHA256 string
	Bytes                             int64
}

func Prepare(options PrepareOptions) (Prepared, error) {
	if options.SourceDir == "" || options.AudioPath == "" {
		return Prepared{}, errors.New("--source and --audio are required")
	}
	if err := VerifyHandoff(options.SourceDir, options.AudioPath); err != nil {
		return Prepared{}, err
	}
	if err := rejectUnsafeOutput(options.OutputDir); err != nil {
		return Prepared{}, err
	}
	input, err := podcast.LoadEpisode(filepath.Join(options.SourceDir, "episode.yaml"))
	if err != nil {
		return Prepared{}, err
	}
	seal, err := ReadHandoffSeal(options.SourceDir)
	if err != nil {
		return Prepared{}, err
	}
	if _, err := TagFor(input, seal.Payload.Episode.Version); err != nil {
		return Prepared{}, fmt.Errorf("validate sealed public release identity: %w", err)
	}
	if !podcast.ValidEpisodeID(input.ID) {
		return Prepared{}, fmt.Errorf("episode id must match %s", podcast.EpisodeIDPattern())
	}
	notes, err := os.ReadFile(filepath.Join(options.SourceDir, "show-notes.md"))
	if err != nil {
		return Prepared{}, fmt.Errorf("read show notes: %w", err)
	}
	sum, size, err := podcast.FileSHA256(options.AudioPath)
	if err != nil {
		return Prepared{}, err
	}
	if strings.ToLower(filepath.Ext(options.AudioPath)) != ".mp3" {
		return Prepared{}, errors.New("audio file must have a .mp3 extension")
	}
	prefix := strings.Trim(strings.TrimSpace(options.StagingPrefix), "/")
	if prefix == "" || strings.Contains(prefix, "..") {
		return Prepared{}, errors.New("staging prefix must be a simple object-key prefix")
	}
	input.Audio = podcast.Audio{StagingKey: fmt.Sprintf("%s/%s/%s.mp3", prefix, input.ID, sum), PublicKey: fmt.Sprintf("audio/%s-%s.mp3", input.ID, sum[:16]), SHA256: sum, Bytes: size, ContentType: "audio/mpeg"}
	sealHash, _, err := podcast.FileSHA256(filepath.Join(options.SourceDir, "source-release-seal.yaml"))
	if err != nil {
		return Prepared{}, err
	}
	input.SourceReleaseSealSHA256 = sealHash
	if len(input.Chapters) > 0 && input.ChaptersAudioSHA256 != sum {
		return Prepared{}, fmt.Errorf("episode %q chapter markers are not bound to the supplied MP3", input.ID)
	}
	destination := filepath.Join(options.OutputDir, input.ID)
	if _, err := os.Stat(destination); err == nil {
		if !options.RefreshExisting {
			return Prepared{}, fmt.Errorf("refusing to replace existing release directory %q", destination)
		}
		if err := verifyRefreshable(destination, input, notes); err != nil {
			return Prepared{}, fmt.Errorf("refusing to refresh existing release directory %q: %w", destination, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Prepared{}, fmt.Errorf("inspect output directory: %w", err)
	}
	serialized, err := yaml.Marshal(input)
	if err != nil {
		return Prepared{}, fmt.Errorf("serialize episode manifest: %w", err)
	}
	// #nosec G301 -- hosted release packages are reviewable publication artifacts, not private data.
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return Prepared{}, fmt.Errorf("create release directory: %w", err)
	}
	for _, file := range []struct {
		name string
		data []byte
	}{{"episode.yaml", serialized}, {"show-notes.md", notes}} {
		if err := writeFile(filepath.Join(destination, file.name), file.data); err != nil {
			return Prepared{}, err
		}
	}
	sourceEpisode, err := os.ReadFile(filepath.Join(options.SourceDir, "episode.yaml"))
	if err != nil {
		return Prepared{}, fmt.Errorf("read source episode metadata: %w", err)
	}
	if err := writeFile(filepath.Join(destination, "source-episode.yaml"), sourceEpisode); err != nil {
		return Prepared{}, err
	}
	sealBytes, err := os.ReadFile(filepath.Join(options.SourceDir, "source-release-seal.yaml"))
	if err != nil {
		return Prepared{}, fmt.Errorf("read source release seal: %w", err)
	}
	if err := writeFile(filepath.Join(destination, "source-release-seal.yaml"), sealBytes); err != nil {
		return Prepared{}, err
	}
	return Prepared{ID: input.ID, StagingKey: input.Audio.StagingKey, PublicKey: input.Audio.PublicKey, SHA256: input.Audio.SHA256, Bytes: input.Audio.Bytes}, nil
}

func verifyRefreshable(destination string, input podcast.Episode, notes []byte) error {
	existing, err := podcast.LoadEpisode(filepath.Join(destination, "episode.yaml"))
	if err != nil {
		return fmt.Errorf("read existing episode metadata: %w", err)
	}
	if existing.Audio != input.Audio {
		return errors.New("audio identity differs")
	}
	left, right := existing, input
	left.Audio, right.Audio = podcast.Audio{}, podcast.Audio{}
	left.SourceReleaseSealSHA256, right.SourceReleaseSealSHA256 = "", ""
	if len(left.Chapters) == 0 {
		left.Chapters = nil
	}
	if len(right.Chapters) == 0 {
		right.Chapters = nil
	}
	if !reflect.DeepEqual(left, right) {
		return errors.New("episode content differs")
	}
	// #nosec G304 -- destination is derived from a validated output root and validated episode ID.
	existingNotes, err := os.ReadFile(filepath.Join(destination, "show-notes.md"))
	if err != nil {
		return fmt.Errorf("read existing show notes: %w", err)
	}
	if !bytes.Equal(existingNotes, notes) {
		return errors.New("show notes differ")
	}
	return nil
}

func rejectUnsafeOutput(path string) error {
	clean := filepath.Clean(path)
	if clean == "." || clean == string(filepath.Separator) || clean == "" {
		return fmt.Errorf("refusing unsafe output directory %q", path)
	}
	return nil
}
