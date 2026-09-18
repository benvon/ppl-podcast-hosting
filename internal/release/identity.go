package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

var (
	sha256Pattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
	releaseKeyPattern = regexp.MustCompile(`^(?:(episode|supplement)-([0-9]{2})|(rough-spot)-([0-9]{3}))$`)
	semverPattern     = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(?:0|[1-9]\d*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

func ReadHandoffSeal(directory string) (HandoffSeal, error) {
	var seal HandoffSeal
	if err := podcast.DecodeYAMLFile(filepath.Join(directory, "source-release-seal.yaml"), &seal); err != nil {
		return HandoffSeal{}, fmt.Errorf("read source release seal: %w", err)
	}
	if seal.SchemaVersion != 1 || seal.Payload.SchemaVersion != 1 {
		return HandoffSeal{}, errors.New("unsupported source release seal schema")
	}
	encoded, err := json.Marshal(seal.Payload)
	if err != nil {
		return HandoffSeal{}, fmt.Errorf("serialize source release seal payload: %w", err)
	}
	payloadHash := sha256.Sum256(encoded)
	if seal.PayloadSHA256 != hex.EncodeToString(payloadHash[:]) {
		return HandoffSeal{}, errors.New("source release seal digest does not match its payload")
	}
	return seal, nil
}

// TagFor validates the sealed public release identity; the internal episode ID
// is intentionally not part of a GitHub release tag.
func TagFor(episode podcast.Episode, sealedVersion string) (string, error) {
	matches := releaseKeyPattern.FindStringSubmatch(episode.ReleaseKey)
	if matches == nil {
		return "", fmt.Errorf("release_key %q must match %s", episode.ReleaseKey, releaseKeyPattern.String())
	}
	keyNumberText := matches[2]
	if keyNumberText == "" {
		keyNumberText = matches[4]
	}
	keyNumber, err := strconv.Atoi(keyNumberText)
	if err != nil || keyNumber < 1 || keyNumber != episode.Number {
		return "", fmt.Errorf("release_key %q must use episode number %d", episode.ReleaseKey, episode.Number)
	}
	if !semverPattern.MatchString(episode.ContentVersion) {
		return "", fmt.Errorf("content_version %q must be semantic versioning", episode.ContentVersion)
	}
	if sealedVersion != "" && episode.ContentVersion != sealedVersion {
		return "", fmt.Errorf("content_version %q does not match sealed source version %q", episode.ContentVersion, sealedVersion)
	}
	return fmt.Sprintf("%s/v%s", episode.ReleaseKey, episode.ContentVersion), nil
}

func VerifyHandoff(sourceDir, audioPath string) error {
	seal, err := ReadHandoffSeal(sourceDir)
	if err != nil {
		return err
	}
	for _, name := range []string{"episode.yaml", "show-notes.md", "audio.mp3"} {
		expected := strings.ToLower(seal.Payload.HandoffFiles[name])
		if !sha256Pattern.MatchString(expected) {
			return fmt.Errorf("source release seal has no valid checksum for %s", name)
		}
		actual, _, err := podcast.FileSHA256(filepath.Join(sourceDir, name))
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("sealed %s does not match its current bytes", name)
		}
	}
	actualHash, actualBytes, err := podcast.FileSHA256(audioPath)
	if err != nil {
		return err
	}
	if actualHash != strings.ToLower(seal.Payload.Audio.SHA256) || actualBytes != seal.Payload.Audio.Bytes || actualHash != strings.ToLower(seal.Payload.HandoffFiles["audio.mp3"]) {
		return errors.New("sealed audio identity does not match the supplied MP3")
	}
	input, err := podcast.LoadEpisode(filepath.Join(sourceDir, "episode.yaml"))
	if err != nil {
		return err
	}
	if input.ID != seal.Payload.Episode.ID || input.Title != seal.Payload.Episode.Title || input.PublishedAt.Format(time.RFC3339) != seal.Payload.Episode.PublishedAt {
		return errors.New("sealed episode identity does not match the handoff metadata")
	}
	if _, err := TagFor(input, seal.Payload.Episode.Version); err != nil {
		return fmt.Errorf("sealed public release identity does not match the handoff metadata: %w", err)
	}
	return nil
}
