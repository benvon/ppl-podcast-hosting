// Package podcast owns the data contract shared by release verification and
// static-site generation.  It deliberately has no release or site imports.
package podcast

import "time"

type ShowConfig struct {
	Title            string `yaml:"title"`
	Description      string `yaml:"description"`
	Language         string `yaml:"language"`
	Author           string `yaml:"author"`
	Copyright        string `yaml:"copyright"`
	Explicit         bool   `yaml:"explicit"`
	Category         string `yaml:"category"`
	Subcategory      string `yaml:"subcategory"`
	BaseURL          string `yaml:"base_url"`
	MediaURL         string `yaml:"media_url"`
	CoverArtURL      string `yaml:"cover_art_url"`
	CoverArtSource   string `yaml:"cover_art_source"`
	HomepageCoverURL string `yaml:"homepage_cover_url"`
	OwnerName        string `yaml:"owner_name"`
	OwnerEmail       string `yaml:"owner_email"`
	AIDisclosure     string `yaml:"ai_disclosure"`
}

type Episode struct {
	ID                      string    `yaml:"id"`
	ReleaseKey              string    `yaml:"release_key"`
	ContentVersion          string    `yaml:"content_version"`
	GUID                    string    `yaml:"guid"`
	Title                   string    `yaml:"title"`
	Description             string    `yaml:"description"`
	PublishedAt             time.Time `yaml:"published_at"`
	Duration                string    `yaml:"duration"`
	Season                  int       `yaml:"season"`
	Number                  int       `yaml:"number"`
	Explicit                bool      `yaml:"explicit"`
	Audio                   Audio     `yaml:"audio"`
	Chapters                []Chapter `yaml:"chapters"`
	ChaptersAudioSHA256     string    `yaml:"chapters_audio_sha256"`
	SourceReleaseSealSHA256 string    `yaml:"source_release_seal_sha256"`
}

type Chapter struct {
	Title   string `yaml:"title"`
	StartMS int64  `yaml:"start_ms"`
}

type Audio struct {
	StagingKey  string `yaml:"staging_key"`
	PublicKey   string `yaml:"public_key"`
	SHA256      string `yaml:"sha256"`
	Bytes       int64  `yaml:"bytes"`
	ContentType string `yaml:"content_type"`
}
