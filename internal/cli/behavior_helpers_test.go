package cli

// Test helpers keep the pre-refactor behavioral tests focused on
// public behavior while production ownership lives in the internal packages.

import (
	"regexp"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
	"github.com/benvon/ppl-podcast-hosting/internal/release"
	"github.com/benvon/ppl-podcast-hosting/internal/site"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

const episodesPerPage = 10

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var showNotesMarkdown = goldmark.New(goldmark.WithExtensions(extension.Table))

type showConfig = podcast.ShowConfig
type episode = podcast.Episode
type chapter = podcast.Chapter
type audio = podcast.Audio
type loadedEpisode = podcast.LoadedEpisode
type handoffSeal = release.HandoffSeal
type handoffSealPayload = release.HandoffSealPayload
type handoffSealAudio = release.HandoffSealAudio
type handoffSealEpisode = release.HandoffSealEpisode
type releaseRecord = release.Record
type releaseCandidate = release.Candidate

func loadEpisode(path string) (episode, error)          { return podcast.LoadEpisode(path) }
func loadEpisodes(path string) ([]loadedEpisode, error) { return podcast.LoadEpisodes(path) }
func fileSHA256(path string) (string, int64, error)     { return podcast.FileSHA256(path) }
func hostingShowNotes(notes []byte) []byte              { return podcast.HostingShowNotes(notes) }
func playerNotesText(synopsis, notesHTML string) string {
	return podcast.PlayerNotesText(synopsis, notesHTML)
}
func validateEpisode(value episode) error              { return podcast.ValidateEpisode(value) }
func validateConfig(value showConfig, count int) error { return podcast.ValidateConfig(value, count) }
func coverArtDimensions(data []byte) (int, int, string, error) {
	return podcast.CoverArtDimensions(data)
}
func validateAll(config showConfig, loaded []loadedEpisode) error {
	episodes := make([]podcast.Episode, len(loaded))
	for i := range loaded {
		episodes[i] = loaded[i].Episode
	}
	return podcast.ValidateAll(config, episodes)
}
func releaseTagFor(value episode, version string) (string, error) {
	return release.TagFor(value, version)
}
func releaseCandidates(path string) ([]releaseCandidate, error) { return release.Candidates(path) }
func releaseCandidateIDs(path string) ([]string, error) {
	candidates, err := release.Candidates(path)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(candidates))
	for i := range candidates {
		ids[i] = candidates[i].ID
	}
	return ids, nil
}
func validateReleaseCandidateIdentities(candidates []releaseCandidate) error {
	return release.ValidateCandidateIdentities(candidates)
}
func validateSealedEpisodeProvenance(dir string, episodes []loadedEpisode) error {
	return validateProvenance(dir, episodes)
}
func writeFeed(path string, config showConfig, episodes []loadedEpisode) error {
	return site.WriteFeed(path, config, episodes)
}
func writeIndex(path string, config showConfig) error { return site.WriteIndex(path, config) }
func writeEpisodePage(path string, config showConfig, episode loadedEpisode) error {
	return site.WriteEpisodePage(path, config, episode)
}
func writeEpisodeArchive(path string, config showConfig, episodes []loadedEpisode) error {
	return site.WriteEpisodeArchive(path, config, episodes)
}
func writeSitemap(path string, config showConfig, episodes []loadedEpisode) error {
	return site.WriteSitemap(path, config, episodes)
}
func writeRobots(path string, config showConfig) error { return site.WriteRobots(path, config) }
func copyStaticAssets(path string) error               { return site.CopyStaticAssets(path) }
