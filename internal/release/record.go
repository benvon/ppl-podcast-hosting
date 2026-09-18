package release

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

func GenerateRecord(episodePath, outputPath, commit string) error {
	if episodePath == "" || outputPath == "" || commit == "" {
		return errors.New("--episode, --out, and --commit are required")
	}
	episode, err := podcast.LoadEpisode(episodePath)
	if err != nil {
		return err
	}
	if err := podcast.ValidateEpisode(episode); err != nil {
		return err
	}
	if err := VerifyHostedProvenance(episodePath, episode); err != nil {
		return err
	}
	tag, err := TagFor(episode, "")
	if err != nil {
		return err
	}
	episodeHash, _, err := podcast.FileSHA256(episodePath)
	if err != nil {
		return err
	}
	notesHash, _, err := podcast.FileSHA256(filepath.Join(filepath.Dir(episodePath), "show-notes.md"))
	if err != nil {
		return err
	}
	record := Record{SchemaVersion: 1, SourceCommit: commit, ReleaseTag: tag, EpisodeManifestSHA: episodeHash, ShowNotesSHA: notesHash, Episode: RecordEpisode{ID: episode.ID, ReleaseKey: episode.ReleaseKey, ContentVersion: episode.ContentVersion, GUID: episode.GUID, Title: episode.Title, PublishedAt: episode.PublishedAt, Duration: episode.Duration, Season: episode.Season, Number: episode.Number, Explicit: episode.Explicit, Audio: episode.Audio, Chapters: episode.Chapters, ChaptersAudioSHA256: episode.ChaptersAudioSHA256, SourceReleaseSealSHA256: episode.SourceReleaseSealSHA256}}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize release record: %w", err)
	}
	return writeFile(outputPath, append(data, '\n'))
}

func writeFile(path string, data []byte) error {
	// #nosec G301 -- release-record directories are reviewable publication artifacts, not private data.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	// #nosec G306,G703 -- path is the operator-selected CLI output; publication records are intentionally public-readable.
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
