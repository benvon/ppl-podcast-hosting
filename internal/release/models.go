// Package release contains release-specific, immutable identity contracts.
// It depends on podcast data but never on site generation.
package release

import (
	"time"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

type HandoffSeal struct {
	SchemaVersion int                `yaml:"schema_version"`
	SealedAtUTC   string             `yaml:"sealed_at_utc"`
	Payload       HandoffSealPayload `yaml:"payload"`
	PayloadSHA256 string             `yaml:"payload_sha256"`
}

// HandoffSealPayload fields are deliberately declared in alphabetic order.
// encoding/json preserves declaration order, which is part of the seal digest.
type HandoffSealPayload struct {
	Audio              HandoffSealAudio   `yaml:"audio" json:"audio"`
	Episode            HandoffSealEpisode `yaml:"episode" json:"episode"`
	HandoffFiles       map[string]string  `yaml:"handoff_files" json:"handoff_files"`
	SchemaVersion      int                `yaml:"schema_version" json:"schema_version"`
	SourcePackageFiles map[string]string  `yaml:"source_package_files" json:"source_package_files"`
}

type HandoffSealAudio struct {
	Bytes  int64  `yaml:"bytes" json:"bytes"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}

type HandoffSealEpisode struct {
	ID          string `yaml:"id" json:"id"`
	PublishedAt string `yaml:"published_at" json:"published_at"`
	Title       string `yaml:"title" json:"title"`
	Version     string `yaml:"version" json:"version"`
}

type Candidate struct {
	ID             string `json:"id"`
	ReleaseKey     string `json:"release_key"`
	ContentVersion string `json:"content_version"`
}

type Record struct {
	SchemaVersion      int           `json:"schema_version"`
	SourceCommit       string        `json:"source_commit"`
	ReleaseTag         string        `json:"release_tag"`
	EpisodeManifestSHA string        `json:"episode_manifest_sha256"`
	ShowNotesSHA       string        `json:"show_notes_sha256"`
	Episode            RecordEpisode `json:"episode"`
}

type RecordEpisode struct {
	ID                      string            `json:"id"`
	ReleaseKey              string            `json:"release_key"`
	ContentVersion          string            `json:"content_version"`
	GUID                    string            `json:"guid"`
	Title                   string            `json:"title"`
	PublishedAt             time.Time         `json:"published_at"`
	Duration                string            `json:"duration"`
	Season                  int               `json:"season"`
	Number                  int               `json:"number"`
	Explicit                bool              `json:"explicit"`
	Audio                   podcast.Audio     `json:"audio"`
	Chapters                []podcast.Chapter `json:"chapters,omitempty"`
	ChaptersAudioSHA256     string            `json:"chapters_audio_sha256,omitempty"`
	SourceReleaseSealSHA256 string            `json:"source_release_seal_sha256"`
}
