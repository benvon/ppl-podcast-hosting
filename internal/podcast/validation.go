package podcast

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	episodeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
	sha256Pattern    = regexp.MustCompile(`^[a-f0-9]{64}$`)
	durationPattern  = regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)
)

func ValidEpisodeID(id string) bool { return episodeIDPattern.MatchString(id) }

func EpisodeIDPattern() string { return episodeIDPattern.String() }

func ValidateAll(config ShowConfig, episodes []Episode) error {
	if err := ValidateConfig(config, len(episodes)); err != nil {
		return err
	}
	ids, guids, keys := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, episode := range episodes {
		if err := ValidateEpisode(episode); err != nil {
			return err
		}
		if ids[episode.ID] || guids[episode.GUID] || keys[episode.Audio.PublicKey] {
			return fmt.Errorf("episode %q duplicates an episode id, GUID, or public audio key", episode.ID)
		}
		ids[episode.ID], guids[episode.GUID], keys[episode.Audio.PublicKey] = true, true, true
	}
	return nil
}

func ValidateConfig(config ShowConfig, episodeCount int) error {
	for field, value := range map[string]string{"title": config.Title, "description": config.Description, "language": config.Language, "author": config.Author, "category": config.Category, "base_url": config.BaseURL, "media_url": config.MediaURL, "ai_disclosure": config.AIDisclosure} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("show configuration %s is required", field)
		}
	}
	if !strings.HasPrefix(config.BaseURL, "https://") || strings.HasSuffix(config.BaseURL, "/") {
		return errors.New("base_url must be an https URL without a trailing slash")
	}
	if !strings.HasPrefix(config.MediaURL, "https://") || strings.HasSuffix(config.MediaURL, "/") {
		return errors.New("media_url must be an https URL without a trailing slash")
	}
	if config.CoverArtURL != "" && !strings.HasPrefix(config.CoverArtURL, "https://") {
		return errors.New("cover_art_url must be an https URL")
	}
	if config.HomepageCoverURL != "" && !strings.HasPrefix(config.HomepageCoverURL, "https://") {
		return errors.New("homepage_cover_url must be an https URL")
	}
	if episodeCount > 0 && (strings.TrimSpace(config.CoverArtURL) == "" || strings.TrimSpace(config.OwnerName) == "" || strings.TrimSpace(config.OwnerEmail) == "") {
		return errors.New("cover_art_url, owner_name, and owner_email are required before publishing an episode")
	}
	if config.CoverArtSource != "" {
		if config.CoverArtURL == "" {
			return errors.New("cover_art_source requires cover_art_url")
		}
		if err := ValidateCoverArtSource(config.CoverArtSource); err != nil {
			return err
		}
	}
	return nil
}

func ValidateEpisode(episode Episode) error {
	if !ValidEpisodeID(episode.ID) {
		return fmt.Errorf("episode id %q must match %s", episode.ID, episodeIDPattern.String())
	}
	for field, value := range map[string]string{"guid": episode.GUID, "title": episode.Title, "description": episode.Description, "duration": episode.Duration, "staging_key": episode.Audio.StagingKey, "public_key": episode.Audio.PublicKey, "sha256": episode.Audio.SHA256, "content_type": episode.Audio.ContentType} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("episode %q %s is required", episode.ID, field)
		}
	}
	if episode.PublishedAt.IsZero() || episode.PublishedAt.Location() != time.UTC {
		return fmt.Errorf("episode %q published_at must be an RFC3339 UTC timestamp", episode.ID)
	}
	if !durationPattern.MatchString(episode.Duration) || episode.Season < 1 || episode.Number < 1 {
		return fmt.Errorf("episode %q has an invalid duration, season, or number", episode.ID)
	}
	parsed, err := time.Parse("15:04:05", episode.Duration)
	if err != nil {
		return fmt.Errorf("episode %q has an invalid duration: %w", episode.ID, err)
	}
	durationMS := int64(parsed.Hour()*3600+parsed.Minute()*60+parsed.Second()) * 1000
	if !sha256Pattern.MatchString(episode.Audio.SHA256) || episode.Audio.Bytes < 1 {
		return fmt.Errorf("episode %q has an invalid audio checksum or byte count", episode.ID)
	}
	if episode.Audio.ContentType != "audio/mpeg" || !strings.HasSuffix(episode.Audio.StagingKey, ".mp3") || !strings.HasSuffix(episode.Audio.PublicKey, ".mp3") {
		return fmt.Errorf("episode %q audio must be an MP3 with content type audio/mpeg", episode.ID)
	}
	for _, key := range []string{episode.Audio.StagingKey, episode.Audio.PublicKey} {
		if strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
			return fmt.Errorf("episode %q has an unsafe object key", episode.ID)
		}
	}
	if len(episode.Chapters) > 0 && episode.ChaptersAudioSHA256 != episode.Audio.SHA256 {
		return fmt.Errorf("episode %q chapter markers must be bound to the staged audio checksum", episode.ID)
	}
	if len(episode.Chapters) == 0 && episode.ChaptersAudioSHA256 != "" {
		return fmt.Errorf("episode %q has a chapter-marker checksum without chapter markers", episode.ID)
	}
	for index, chapter := range episode.Chapters {
		if strings.TrimSpace(chapter.Title) == "" {
			return fmt.Errorf("episode %q chapter %d title is required", episode.ID, index+1)
		}
		if chapter.StartMS < 0 {
			return fmt.Errorf("episode %q chapter %d has a negative start time", episode.ID, index+1)
		}
		if chapter.StartMS >= durationMS {
			return fmt.Errorf("episode %q chapter %d must start before the episode duration", episode.ID, index+1)
		}
		if index == 0 && chapter.StartMS != 0 {
			return fmt.Errorf("episode %q first chapter must start at zero", episode.ID)
		}
		if index > 0 && chapter.StartMS <= episode.Chapters[index-1].StartMS {
			return fmt.Errorf("episode %q chapter start times must be strictly increasing", episode.ID)
		}
	}
	return nil
}

func ValidateCoverArtSource(source string) error {
	clean := filepath.Clean(source)
	if filepath.IsAbs(source) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("cover_art_source must be a repository-relative file path")
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return fmt.Errorf("open cover art: %w", err)
	}
	width, height, _, err := CoverArtDimensions(data)
	if err != nil {
		return fmt.Errorf("decode cover art: %w", err)
	}
	if width != height || width < 1400 || width > 3000 {
		return errors.New("cover art must be square and between 1400 and 3000 pixels")
	}
	return nil
}

func CoverArtDimensions(data []byte) (int, int, string, error) {
	if len(data) >= 26 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) && bytes.Equal(data[12:16], []byte("IHDR")) {
		width, height := int(data[16])<<24|int(data[17])<<16|int(data[18])<<8|int(data[19]), int(data[20])<<24|int(data[21])<<16|int(data[22])<<8|int(data[23])
		if width < 1 || height < 1 {
			return 0, 0, "", errors.New("PNG has invalid dimensions")
		}
		if data[25] == 4 || data[25] == 6 {
			return 0, 0, "", errors.New("PNG cover art must not have an alpha channel")
		}
		return width, height, "png", nil
	}
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 0, 0, "", errors.New("cover art must be a PNG or JPEG")
	}
	for offset := 2; offset < len(data); {
		if data[offset] != 0xff {
			return 0, 0, "", errors.New("JPEG has an invalid marker")
		}
		for offset < len(data) && data[offset] == 0xff {
			offset++
		}
		if offset >= len(data) {
			break
		}
		marker := data[offset]
		offset++
		if marker == 0xd8 || marker == 0xd9 || marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if offset+2 > len(data) {
			break
		}
		length := int(data[offset])<<8 | int(data[offset+1])
		if length < 2 || offset+length > len(data) {
			return 0, 0, "", errors.New("JPEG has an invalid segment")
		}
		if jpegSOF(marker) {
			if length < 8 {
				return 0, 0, "", errors.New("JPEG frame header is too short")
			}
			height, width := int(data[offset+3])<<8|int(data[offset+4]), int(data[offset+5])<<8|int(data[offset+6])
			if width < 1 || height < 1 {
				return 0, 0, "", errors.New("JPEG has invalid dimensions")
			}
			return width, height, "jpeg", nil
		}
		offset += length
	}
	return 0, 0, "", errors.New("JPEG frame header was not found")
}

func jpegSOF(marker byte) bool {
	return marker >= 0xc0 && marker <= 0xc3 || marker >= 0xc5 && marker <= 0xc7 || marker >= 0xc9 && marker <= 0xcb || marker >= 0xcd && marker <= 0xcf
}
