package release

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

// VerifyHostedProvenance is deliberately fail-closed: any missing retained
// source artifact, altered seal, or divergent public metadata rejects a
// candidate instead of allowing an ambiguous historical release.
func VerifyHostedProvenance(episodePath string, hosted podcast.Episode) error {
	directory := filepath.Dir(episodePath)
	seal, err := ReadHandoffSeal(directory)
	if err != nil {
		return err
	}
	sealHash, _, err := podcast.FileSHA256(filepath.Join(directory, "source-release-seal.yaml"))
	if err != nil {
		return err
	}
	if sealHash != hosted.SourceReleaseSealSHA256 {
		return errors.New("hosted source release seal does not match episode metadata")
	}
	sourcePath := filepath.Join(directory, "source-episode.yaml")
	sourceHash, _, err := podcast.FileSHA256(sourcePath)
	if err != nil {
		return err
	}
	if sourceHash != strings.ToLower(seal.Payload.HandoffFiles["episode.yaml"]) {
		return errors.New("retained source episode metadata does not match the source release seal")
	}
	source, err := podcast.LoadEpisode(sourcePath)
	if err != nil {
		return err
	}
	if source.ID != seal.Payload.Episode.ID || source.Title != seal.Payload.Episode.Title || source.PublishedAt.Format(time.RFC3339) != seal.Payload.Episode.PublishedAt {
		return errors.New("retained source episode metadata does not match the sealed episode identity")
	}
	if _, err := TagFor(source, seal.Payload.Episode.Version); err != nil {
		return fmt.Errorf("retained source episode metadata has an invalid public release identity: %w", err)
	}
	if _, err := TagFor(hosted, seal.Payload.Episode.Version); err != nil {
		return fmt.Errorf("hosted episode metadata has an invalid public release identity: %w", err)
	}
	comparableSource, comparableHosted := source, hosted
	comparableSource.Audio, comparableHosted.Audio = podcast.Audio{}, podcast.Audio{}
	comparableSource.SourceReleaseSealSHA256, comparableHosted.SourceReleaseSealSHA256 = "", ""
	if len(comparableSource.Chapters) == 0 {
		comparableSource.Chapters = nil
	}
	if len(comparableHosted.Chapters) == 0 {
		comparableHosted.Chapters = nil
	}
	if !reflect.DeepEqual(comparableSource, comparableHosted) {
		return errors.New("hosted episode metadata differs from the retained sealed source metadata")
	}
	notesHash, _, err := podcast.FileSHA256(filepath.Join(directory, "show-notes.md"))
	if err != nil {
		return err
	}
	if notesHash != strings.ToLower(seal.Payload.HandoffFiles["show-notes.md"]) {
		return errors.New("hosted show notes do not match the source release seal")
	}
	if hosted.Audio.SHA256 != strings.ToLower(seal.Payload.Audio.SHA256) || hosted.Audio.SHA256 != strings.ToLower(seal.Payload.HandoffFiles["audio.mp3"]) || hosted.Audio.Bytes != seal.Payload.Audio.Bytes {
		return errors.New("hosted audio identity does not match the source release seal")
	}
	return nil
}

// Candidates discovers only sealed, verifiable releases. Unsealed historical
// packages are preservation-only and deliberately omitted.
func Candidates(episodesDir string) ([]Candidate, error) {
	paths, err := filepath.Glob(filepath.Join(episodesDir, "*", "episode.yaml"))
	if err != nil {
		return nil, fmt.Errorf("find episode manifests: %w", err)
	}
	candidates := make([]Candidate, 0, len(paths))
	for _, path := range paths {
		episode, err := podcast.LoadEpisode(path)
		if err != nil {
			return nil, err
		}
		if episode.SourceReleaseSealSHA256 == "" {
			continue
		}
		if err := podcast.ValidateEpisode(episode); err != nil {
			return nil, err
		}
		if err := VerifyHostedProvenance(path, episode); err != nil {
			return nil, fmt.Errorf("verify release candidate %q: %w", episode.ID, err)
		}
		candidates = append(candidates, Candidate{ID: episode.ID, ReleaseKey: episode.ReleaseKey, ContentVersion: episode.ContentVersion})
	}
	if err := ValidateCandidateIdentities(candidates); err != nil {
		return nil, err
	}
	return candidates, nil
}

func ValidateCandidateIdentities(candidates []Candidate) error {
	seen := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		identity := fmt.Sprintf("%s/v%s", candidate.ReleaseKey, candidate.ContentVersion)
		if previous, exists := seen[identity]; exists && previous != candidate.ID {
			return fmt.Errorf("sealed episodes %q and %q share public release identity %q", previous, candidate.ID, identity)
		}
		seen[identity] = candidate.ID
	}
	return nil
}
