package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	stdhtml "html"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"gopkg.in/yaml.v3"
)

var episodeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var durationPattern = regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)
var releaseKeyPattern = regexp.MustCompile(`^(?:(episode|supplement)-([0-9]{2})|(rough-spot)-([0-9]{3}))$`)
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(?:0|[1-9]\d*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
var showNotesLinkPattern = regexp.MustCompile(`(?s)<a\b[^>]*\bhref="(https://[^"]+)"[^>]*>(.+?)</a>`)
var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
var showNotesMarkdown = goldmark.New(goldmark.WithExtensions(extension.Table))

const episodesPerPage = 10

type showConfig struct {
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

type episode struct {
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
	Audio                   audio     `yaml:"audio"`
	Chapters                []chapter `yaml:"chapters"`
	ChaptersAudioSHA256     string    `yaml:"chapters_audio_sha256"`
	SourceReleaseSealSHA256 string    `yaml:"source_release_seal_sha256"`
}

type chapter struct {
	Title   string `yaml:"title"`
	StartMS int64  `yaml:"start_ms"`
}

type audio struct {
	StagingKey  string `yaml:"staging_key"`
	PublicKey   string `yaml:"public_key"`
	SHA256      string `yaml:"sha256"`
	Bytes       int64  `yaml:"bytes"`
	ContentType string `yaml:"content_type"`
}

type handoffSeal struct {
	SchemaVersion int                `yaml:"schema_version"`
	SealedAtUTC   string             `yaml:"sealed_at_utc"`
	Payload       handoffSealPayload `yaml:"payload"`
	PayloadSHA256 string             `yaml:"payload_sha256"`
}

// Field order is alphabetical so encoding/json produces the same canonical
// payload digest as the source repository's stable JSON serializer.
type handoffSealPayload struct {
	Audio              handoffSealAudio   `yaml:"audio" json:"audio"`
	Episode            handoffSealEpisode `yaml:"episode" json:"episode"`
	HandoffFiles       map[string]string  `yaml:"handoff_files" json:"handoff_files"`
	SchemaVersion      int                `yaml:"schema_version" json:"schema_version"`
	SourcePackageFiles map[string]string  `yaml:"source_package_files" json:"source_package_files"`
}

type handoffSealAudio struct {
	Bytes  int64  `yaml:"bytes" json:"bytes"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}

type handoffSealEpisode struct {
	ID          string `yaml:"id" json:"id"`
	PublishedAt string `yaml:"published_at" json:"published_at"`
	Title       string `yaml:"title" json:"title"`
	Version     string `yaml:"version" json:"version"`
}

type releaseRecord struct {
	SchemaVersion      int                  `json:"schema_version"`
	SourceCommit       string               `json:"source_commit"`
	ReleaseTag         string               `json:"release_tag"`
	EpisodeManifestSHA string               `json:"episode_manifest_sha256"`
	ShowNotesSHA       string               `json:"show_notes_sha256"`
	Episode            releaseRecordEpisode `json:"episode"`
}

type releaseRecordEpisode struct {
	ID                      string    `json:"id"`
	ReleaseKey              string    `json:"release_key"`
	ContentVersion          string    `json:"content_version"`
	GUID                    string    `json:"guid"`
	Title                   string    `json:"title"`
	PublishedAt             time.Time `json:"published_at"`
	Duration                string    `json:"duration"`
	Season                  int       `json:"season"`
	Number                  int       `json:"number"`
	Explicit                bool      `json:"explicit"`
	Audio                   audio     `json:"audio"`
	Chapters                []chapter `json:"chapters,omitempty"`
	ChaptersAudioSHA256     string    `json:"chapters_audio_sha256,omitempty"`
	SourceReleaseSealSHA256 string    `json:"source_release_seal_sha256"`
}

type loadedEpisode struct {
	episode
	NotesHTML       template.HTML
	PlayerNotesText string
}

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: pplsite <validate|build|prepare|verify-handoff|release-candidates|release-record|publish-plan>")
	}

	var err error
	switch os.Args[1] {
	case "validate":
		err = validateCommand(os.Args[2:])
	case "build":
		err = buildCommand(os.Args[2:])
	case "prepare":
		err = prepareCommand(os.Args[2:])
	case "verify-handoff":
		err = verifyHandoffCommand(os.Args[2:])
	case "release-candidates":
		err = releaseCandidatesCommand(os.Args[2:])
	case "release-record":
		err = releaseRecordCommand(os.Args[2:])
	case "publish-plan":
		err = publishPlanCommand(os.Args[2:])
	default:
		fatalf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fatalf("%v", err)
	}
}

func validateCommand(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	configPath := fs.String("config", "config/show.yaml", "show configuration path")
	episodesDir := fs.String("episodes", "episodes", "episodes directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	config, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	episodes, err := loadEpisodes(*episodesDir)
	if err != nil {
		return err
	}
	if err := validateAll(config, episodes); err != nil {
		return err
	}
	if err := validateSealedEpisodeProvenance(*episodesDir, episodes); err != nil {
		return err
	}
	fmt.Printf("validated %d episode(s)\n", len(episodes))
	return nil
}

func buildCommand(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	configPath := fs.String("config", "config/show.yaml", "show configuration path")
	episodesDir := fs.String("episodes", "episodes", "episodes directory")
	outDir := fs.String("out", "dist", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := rejectUnsafeOutput(*outDir); err != nil {
		return err
	}
	config, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	episodes, err := loadEpisodes(*episodesDir)
	if err != nil {
		return err
	}
	if err := validateAll(config, episodes); err != nil {
		return err
	}
	if err := validateSealedEpisodeProvenance(*episodesDir, episodes); err != nil {
		return err
	}
	if err := os.RemoveAll(*outDir); err != nil {
		return fmt.Errorf("clear output directory: %w", err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := writeFeed(filepath.Join(*outDir, "feed.xml"), config, episodes); err != nil {
		return err
	}
	if err := writeIndex(filepath.Join(*outDir, "index.html"), config); err != nil {
		return err
	}
	if err := copyCoverArt(*outDir, config.CoverArtSource); err != nil {
		return err
	}
	if err := copyStaticAssets(*outDir); err != nil {
		return err
	}
	if err := writeEpisodeArchive(*outDir, config, episodes); err != nil {
		return err
	}
	if err := writeSitemap(filepath.Join(*outDir, "sitemap.xml"), config, episodes); err != nil {
		return err
	}
	if err := writeRobots(filepath.Join(*outDir, "robots.txt"), config); err != nil {
		return err
	}
	for _, episode := range episodes {
		path := filepath.Join(*outDir, "episodes", episode.ID, "index.html")
		if err := writeEpisodePage(path, config, episode); err != nil {
			return err
		}
	}
	return writeFile(filepath.Join(*outDir, "_headers"), []byte("/feed.xml\n  Cache-Control: no-cache, max-age=0, must-revalidate\n\n/episodes/*\n  Cache-Control: public, max-age=3600\n"))
}

func prepareCommand(args []string) error {
	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	sourceDir := fs.String("source", "", "local directory containing episode.yaml and show-notes.md")
	audioPath := fs.String("audio", "", "local MP3 path")
	outDir := fs.String("out", "episodes", "episodes output directory")
	stagingPrefix := fs.String("staging-prefix", "staging", "private R2 staging key prefix")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sourceDir == "" || *audioPath == "" {
		return errors.New("--source and --audio are required")
	}
	if err := verifyHandoffCommand([]string{"--source", *sourceDir, "--audio", *audioPath}); err != nil {
		return err
	}
	if err := rejectUnsafeOutput(*outDir); err != nil {
		return err
	}
	input, err := loadEpisode(filepath.Join(*sourceDir, "episode.yaml"))
	if err != nil {
		return err
	}
	seal, err := readHandoffSeal(*sourceDir)
	if err != nil {
		return err
	}
	if _, err := releaseTagFor(input, seal.Payload.Episode.Version); err != nil {
		return fmt.Errorf("validate sealed public release identity: %w", err)
	}
	if input.ID == "" || !episodeIDPattern.MatchString(input.ID) {
		return fmt.Errorf("episode id must match %s", episodeIDPattern.String())
	}
	notes, err := os.ReadFile(filepath.Join(*sourceDir, "show-notes.md"))
	if err != nil {
		return fmt.Errorf("read show notes: %w", err)
	}
	sum, size, err := fileSHA256(*audioPath)
	if err != nil {
		return err
	}
	if strings.ToLower(filepath.Ext(*audioPath)) != ".mp3" {
		return errors.New("audio file must have a .mp3 extension")
	}
	prefix := strings.Trim(strings.TrimSpace(*stagingPrefix), "/")
	if prefix == "" || strings.Contains(prefix, "..") {
		return errors.New("staging prefix must be a simple object-key prefix")
	}
	input.Audio = audio{
		StagingKey:  fmt.Sprintf("%s/%s/%s.mp3", prefix, input.ID, sum),
		PublicKey:   fmt.Sprintf("audio/%s-%s.mp3", input.ID, sum[:16]),
		SHA256:      sum,
		Bytes:       size,
		ContentType: "audio/mpeg",
	}
	sealHash, _, err := fileSHA256(filepath.Join(*sourceDir, "source-release-seal.yaml"))
	if err != nil {
		return err
	}
	input.SourceReleaseSealSHA256 = sealHash
	if len(input.Chapters) > 0 && input.ChaptersAudioSHA256 != sum {
		return fmt.Errorf("episode %q chapter markers are not bound to the supplied MP3", input.ID)
	}
	outputDir := filepath.Join(*outDir, input.ID)
	if _, err := os.Stat(outputDir); err == nil {
		return fmt.Errorf("refusing to replace existing release directory %q", outputDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output directory: %w", err)
	}
	serialized, err := yaml.Marshal(input)
	if err != nil {
		return fmt.Errorf("serialize episode manifest: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create release directory: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "episode.yaml"), serialized); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(outputDir, "show-notes.md"), notes); err != nil {
		return err
	}
	sourceEpisode, err := os.ReadFile(filepath.Join(*sourceDir, "episode.yaml"))
	if err != nil {
		return fmt.Errorf("read source episode metadata: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "source-episode.yaml"), sourceEpisode); err != nil {
		return err
	}
	sealBytes, err := os.ReadFile(filepath.Join(*sourceDir, "source-release-seal.yaml"))
	if err != nil {
		return fmt.Errorf("read source release seal: %w", err)
	}
	if err := writeFile(filepath.Join(outputDir, "source-release-seal.yaml"), sealBytes); err != nil {
		return err
	}
	fmt.Printf("episode_id=%s\nstaging_key=%s\npublic_key=%s\nsha256=%s\nbytes=%d\n", input.ID, input.Audio.StagingKey, input.Audio.PublicKey, input.Audio.SHA256, input.Audio.Bytes)
	return nil
}

func verifyHandoffCommand(args []string) error {
	flags := flag.NewFlagSet("verify-handoff", flag.ContinueOnError)
	sourceDir := flags.String("source", "", "sealed local release directory")
	audioPath := flags.String("audio", "", "local MP3 path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *sourceDir == "" || *audioPath == "" {
		return errors.New("--source and --audio are required")
	}
	seal, err := readHandoffSeal(*sourceDir)
	if err != nil {
		return err
	}
	for _, name := range []string{"episode.yaml", "show-notes.md", "audio.mp3"} {
		expected := strings.ToLower(seal.Payload.HandoffFiles[name])
		if !sha256Pattern.MatchString(expected) {
			return fmt.Errorf("source release seal has no valid checksum for %s", name)
		}
		actual, _, err := fileSHA256(filepath.Join(*sourceDir, name))
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("sealed %s does not match its current bytes", name)
		}
	}
	actualAudioHash, actualAudioBytes, err := fileSHA256(*audioPath)
	if err != nil {
		return err
	}
	if actualAudioHash != strings.ToLower(seal.Payload.Audio.SHA256) || actualAudioBytes != seal.Payload.Audio.Bytes || actualAudioHash != strings.ToLower(seal.Payload.HandoffFiles["audio.mp3"]) {
		return errors.New("sealed audio identity does not match the supplied MP3")
	}
	input, err := loadEpisode(filepath.Join(*sourceDir, "episode.yaml"))
	if err != nil {
		return err
	}
	if input.ID != seal.Payload.Episode.ID || input.Title != seal.Payload.Episode.Title || input.PublishedAt.Format(time.RFC3339) != seal.Payload.Episode.PublishedAt {
		return errors.New("sealed episode identity does not match the handoff metadata")
	}
	if _, err := releaseTagFor(input, seal.Payload.Episode.Version); err != nil {
		return fmt.Errorf("sealed public release identity does not match the handoff metadata: %w", err)
	}
	return nil
}

func readHandoffSeal(directory string) (handoffSeal, error) {
	var seal handoffSeal
	if err := decodeYAMLFile(filepath.Join(directory, "source-release-seal.yaml"), &seal); err != nil {
		return handoffSeal{}, fmt.Errorf("read source release seal: %w", err)
	}
	if seal.SchemaVersion != 1 || seal.Payload.SchemaVersion != 1 {
		return handoffSeal{}, errors.New("unsupported source release seal schema")
	}
	encoded, err := json.Marshal(seal.Payload)
	if err != nil {
		return handoffSeal{}, fmt.Errorf("serialize source release seal payload: %w", err)
	}
	payloadHash := sha256.Sum256(encoded)
	if seal.PayloadSHA256 != hex.EncodeToString(payloadHash[:]) {
		return handoffSeal{}, errors.New("source release seal digest does not match its payload")
	}
	return seal, nil
}

// releaseTagFor validates the source-sealed public release identity. The
// internal episode ID remains an implementation detail; only this namespaced
// key and content version are used to create GitHub tags and release records.
func releaseTagFor(episode episode, sealedVersion string) (string, error) {
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

func verifyHostedReleaseProvenance(episodePath string, hosted episode) error {
	directory := filepath.Dir(episodePath)
	seal, err := readHandoffSeal(directory)
	if err != nil {
		return err
	}
	sealHash, _, err := fileSHA256(filepath.Join(directory, "source-release-seal.yaml"))
	if err != nil {
		return err
	}
	if sealHash != hosted.SourceReleaseSealSHA256 {
		return errors.New("hosted source release seal does not match episode metadata")
	}
	sourceEpisodePath := filepath.Join(directory, "source-episode.yaml")
	sourceEpisodeHash, _, err := fileSHA256(sourceEpisodePath)
	if err != nil {
		return err
	}
	if sourceEpisodeHash != strings.ToLower(seal.Payload.HandoffFiles["episode.yaml"]) {
		return errors.New("retained source episode metadata does not match the source release seal")
	}
	sourceEpisode, err := loadEpisode(sourceEpisodePath)
	if err != nil {
		return err
	}
	if sourceEpisode.ID != seal.Payload.Episode.ID || sourceEpisode.Title != seal.Payload.Episode.Title || sourceEpisode.PublishedAt.Format(time.RFC3339) != seal.Payload.Episode.PublishedAt {
		return errors.New("retained source episode metadata does not match the sealed episode identity")
	}
	if _, err := releaseTagFor(sourceEpisode, seal.Payload.Episode.Version); err != nil {
		return fmt.Errorf("retained source episode metadata has an invalid public release identity: %w", err)
	}
	if _, err := releaseTagFor(hosted, seal.Payload.Episode.Version); err != nil {
		return fmt.Errorf("hosted episode metadata has an invalid public release identity: %w", err)
	}
	comparableSource := sourceEpisode
	comparableSource.Audio = audio{}
	comparableSource.SourceReleaseSealSHA256 = ""
	comparableHosted := hosted
	comparableHosted.Audio = audio{}
	comparableHosted.SourceReleaseSealSHA256 = ""
	if len(comparableSource.Chapters) == 0 {
		comparableSource.Chapters = nil
	}
	if len(comparableHosted.Chapters) == 0 {
		comparableHosted.Chapters = nil
	}
	if !reflect.DeepEqual(comparableSource, comparableHosted) {
		return errors.New("hosted episode metadata differs from the retained sealed source metadata")
	}
	showNotesHash, _, err := fileSHA256(filepath.Join(directory, "show-notes.md"))
	if err != nil {
		return err
	}
	if showNotesHash != strings.ToLower(seal.Payload.HandoffFiles["show-notes.md"]) {
		return errors.New("hosted show notes do not match the source release seal")
	}
	if hosted.Audio.SHA256 != strings.ToLower(seal.Payload.Audio.SHA256) || hosted.Audio.SHA256 != strings.ToLower(seal.Payload.HandoffFiles["audio.mp3"]) || hosted.Audio.Bytes != seal.Payload.Audio.Bytes {
		return errors.New("hosted audio identity does not match the source release seal")
	}
	return nil
}

func releaseCandidatesCommand(args []string) error {
	flags := flag.NewFlagSet("release-candidates", flag.ContinueOnError)
	episodesDir := flags.String("episodes", "episodes", "episodes directory")
	jsonOutput := flags.Bool("json", false, "write release candidates as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	candidates, err := releaseCandidates(*episodesDir)
	if err != nil {
		return err
	}
	if *jsonOutput {
		data, err := json.Marshal(candidates)
		if err != nil {
			return fmt.Errorf("serialize release candidates: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	for _, candidate := range candidates {
		fmt.Println(candidate.ID)
	}
	return nil
}

type releaseCandidate struct {
	ID             string `json:"id"`
	ReleaseKey     string `json:"release_key"`
	ContentVersion string `json:"content_version"`
}

func releaseCandidateIDs(episodesDir string) ([]string, error) {
	candidates, err := releaseCandidates(episodesDir)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(candidates))
	for _, id := range candidates {
		ids = append(ids, id.ID)
	}
	return ids, nil
}

func releaseCandidates(episodesDir string) ([]releaseCandidate, error) {
	paths, err := filepath.Glob(filepath.Join(episodesDir, "*", "episode.yaml"))
	if err != nil {
		return nil, fmt.Errorf("find episode manifests: %w", err)
	}
	candidates := make([]releaseCandidate, 0, len(paths))
	for _, episodePath := range paths {
		loaded, err := loadEpisode(episodePath)
		if err != nil {
			return nil, err
		}
		if loaded.SourceReleaseSealSHA256 == "" {
			continue
		}
		if err := validateEpisode(loaded); err != nil {
			return nil, err
		}
		if err := verifyHostedReleaseProvenance(episodePath, loaded); err != nil {
			return nil, fmt.Errorf("verify release candidate %q: %w", loaded.ID, err)
		}
		candidates = append(candidates, releaseCandidate{ID: loaded.ID, ReleaseKey: loaded.ReleaseKey, ContentVersion: loaded.ContentVersion})
	}
	if err := validateReleaseCandidateIdentities(candidates); err != nil {
		return nil, err
	}
	return candidates, nil
}

func validateReleaseCandidateIdentities(candidates []releaseCandidate) error {
	seen := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		identity := fmt.Sprintf("%s/v%s", candidate.ReleaseKey, candidate.ContentVersion)
		if priorID, exists := seen[identity]; exists && priorID != candidate.ID {
			return fmt.Errorf("sealed episodes %q and %q share public release identity %q", priorID, candidate.ID, identity)
		}
		seen[identity] = candidate.ID
	}
	return nil
}

func validateSealedEpisodeProvenance(episodesDir string, episodes []loadedEpisode) error {
	for _, loaded := range episodes {
		if loaded.SourceReleaseSealSHA256 == "" {
			continue
		}
		if err := verifyHostedReleaseProvenance(filepath.Join(episodesDir, loaded.ID, "episode.yaml"), loaded.episode); err != nil {
			return fmt.Errorf("verify sealed episode %q: %w", loaded.ID, err)
		}
	}
	return nil
}

func releaseRecordCommand(args []string) error {
	flags := flag.NewFlagSet("release-record", flag.ContinueOnError)
	episodePath := flags.String("episode", "", "committed hosted episode.yaml")
	outputPath := flags.String("out", "", "output JSON path")
	commit := flags.String("commit", "", "source Git commit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *episodePath == "" || *outputPath == "" || *commit == "" {
		return errors.New("--episode, --out, and --commit are required")
	}
	loaded, err := loadEpisode(*episodePath)
	if err != nil {
		return err
	}
	if err := validateEpisode(loaded); err != nil {
		return err
	}
	if err := verifyHostedReleaseProvenance(*episodePath, loaded); err != nil {
		return err
	}
	releaseTag, err := releaseTagFor(loaded, "")
	if err != nil {
		return err
	}
	episodeHash, _, err := fileSHA256(*episodePath)
	if err != nil {
		return err
	}
	notesHash, _, err := fileSHA256(filepath.Join(filepath.Dir(*episodePath), "show-notes.md"))
	if err != nil {
		return err
	}
	record := releaseRecord{SchemaVersion: 1, SourceCommit: *commit, ReleaseTag: releaseTag, EpisodeManifestSHA: episodeHash, ShowNotesSHA: notesHash, Episode: releaseRecordEpisode{ID: loaded.ID, ReleaseKey: loaded.ReleaseKey, ContentVersion: loaded.ContentVersion, GUID: loaded.GUID, Title: loaded.Title, PublishedAt: loaded.PublishedAt, Duration: loaded.Duration, Season: loaded.Season, Number: loaded.Number, Explicit: loaded.Explicit, Audio: loaded.Audio, Chapters: loaded.Chapters, ChaptersAudioSHA256: loaded.ChaptersAudioSHA256, SourceReleaseSealSHA256: loaded.SourceReleaseSealSHA256}}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize release record: %w", err)
	}
	return writeFile(*outputPath, append(data, '\n'))
}

func publishPlanCommand(args []string) error {
	fs := flag.NewFlagSet("publish-plan", flag.ContinueOnError)
	episodesDir := fs.String("episodes", "episodes", "episodes directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	episodes, err := loadEpisodes(*episodesDir)
	if err != nil {
		return err
	}
	for _, episode := range episodes {
		fmt.Printf("%s\t%s\t%s\t%d\n", episode.Audio.StagingKey, episode.Audio.PublicKey, episode.Audio.SHA256, episode.Audio.Bytes)
	}
	return nil
}

func loadConfig(path string) (showConfig, error) {
	var config showConfig
	if err := decodeYAMLFile(path, &config); err != nil {
		return showConfig{}, fmt.Errorf("load show configuration: %w", err)
	}
	return config, nil
}

func loadEpisodes(dir string) ([]loadedEpisode, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*", "episode.yaml"))
	if err != nil {
		return nil, fmt.Errorf("find episode manifests: %w", err)
	}
	episodes := make([]loadedEpisode, 0, len(paths))
	for _, path := range paths {
		episode, err := loadEpisode(path)
		if err != nil {
			return nil, err
		}
		notesPath := filepath.Join(filepath.Dir(path), "show-notes.md")
		notes, err := os.ReadFile(notesPath)
		if err != nil {
			return nil, fmt.Errorf("read show notes for %q: %w", episode.ID, err)
		}
		hostedNotes := hostingShowNotes(notes)
		var html bytes.Buffer
		if err := showNotesMarkdown.Convert(hostedNotes, &html); err != nil {
			return nil, fmt.Errorf("render show notes for %q: %w", episode.ID, err)
		}
		notesHTML := html.String()
		episodes = append(episodes, loadedEpisode{episode: episode, NotesHTML: template.HTML(notesHTML), PlayerNotesText: playerNotesText(episode.Description, notesHTML)})
	}
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].PublishedAt.After(episodes[j].PublishedAt) })
	return episodes, nil
}

func hostingShowNotes(notes []byte) []byte {
	normalizedNotes := strings.ReplaceAll(string(notes), "\r\n", "\n")
	lines := strings.Split(normalizedNotes, "\n")
	withoutDuplicateNotice := make([]string, 0, len(lines))
	skippingNotice := false
	for _, line := range lines {
		if line == "## Production notice" {
			skippingNotice = true
			continue
		}
		if skippingNotice && strings.HasPrefix(line, "## ") {
			skippingNotice = false
		}
		if !skippingNotice {
			withoutDuplicateNotice = append(withoutDuplicateNotice, line)
		}
	}
	lines = withoutDuplicateNotice
	start := 0
	for start < len(lines) && !strings.HasPrefix(lines[start], "**") {
		start++
	}
	end := start
	for end < len(lines) && strings.HasPrefix(lines[end], "**") && strings.Contains(lines[end], ":**") {
		end++
	}
	if end-start < 2 {
		return []byte(strings.Join(lines, "\n"))
	}
	metadata := make([]string, 0, end-start)
	for _, line := range lines[start:end] {
		metadata = append(metadata, "- "+line)
	}
	formatted := append([]string{}, lines[:start]...)
	formatted = append(formatted, metadata...)
	formatted = append(formatted, lines[end:]...)
	return []byte(strings.Join(formatted, "\n"))
}

func playerNotesText(synopsis string, notesHTML string) string {
	links := showNotesLinkPattern.FindAllStringSubmatch(notesHTML, -1)
	if len(links) == 0 {
		return synopsis
	}
	seen := map[string]bool{}
	lines := []string{synopsis, "", "Study materials and visual aids:"}
	for _, link := range links {
		url := stdhtml.UnescapeString(link[1])
		if seen[url] {
			continue
		}
		seen[url] = true
		label := strings.Join(strings.Fields(stdhtml.UnescapeString(htmlTagPattern.ReplaceAllString(link[2], ""))), " ")
		if label == "" {
			label = url
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", label, url))
	}
	return strings.Join(lines, "\n")
}

func loadEpisode(path string) (episode, error) {
	var episode episode
	if err := decodeYAMLFile(path, &episode); err != nil {
		return episode, fmt.Errorf("load episode manifest %q: %w", path, err)
	}
	return episode, nil
}

func decodeYAMLFile(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("expected one YAML document")
		}
		return err
	}
	return nil
}

func validateAll(config showConfig, episodes []loadedEpisode) error {
	if err := validateConfig(config, len(episodes)); err != nil {
		return err
	}
	ids := map[string]bool{}
	guids := map[string]bool{}
	keys := map[string]bool{}
	for _, loaded := range episodes {
		if err := validateEpisode(loaded.episode); err != nil {
			return err
		}
		if ids[loaded.ID] || guids[loaded.GUID] || keys[loaded.Audio.PublicKey] {
			return fmt.Errorf("episode %q duplicates an episode id, GUID, or public audio key", loaded.ID)
		}
		ids[loaded.ID] = true
		guids[loaded.GUID] = true
		keys[loaded.Audio.PublicKey] = true
	}
	return nil
}

func validateConfig(config showConfig, episodeCount int) error {
	for field, value := range map[string]string{
		"title": config.Title, "description": config.Description, "language": config.Language,
		"author": config.Author, "category": config.Category, "base_url": config.BaseURL,
		"media_url": config.MediaURL, "ai_disclosure": config.AIDisclosure,
	} {
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
	if episodeCount > 0 {
		if strings.TrimSpace(config.CoverArtURL) == "" || strings.TrimSpace(config.OwnerName) == "" || strings.TrimSpace(config.OwnerEmail) == "" {
			return errors.New("cover_art_url, owner_name, and owner_email are required before publishing an episode")
		}
	}
	if config.CoverArtSource != "" {
		if config.CoverArtURL == "" {
			return errors.New("cover_art_source requires cover_art_url")
		}
		if err := validateCoverArtSource(config.CoverArtSource); err != nil {
			return err
		}
	}
	return nil
}

func validateCoverArtSource(source string) error {
	clean := filepath.Clean(source)
	if filepath.IsAbs(source) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("cover_art_source must be a repository-relative file path")
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return fmt.Errorf("open cover art: %w", err)
	}
	width, height, _, err := coverArtDimensions(data)
	if err != nil {
		return fmt.Errorf("decode cover art: %w", err)
	}
	if width != height || width < 1400 || width > 3000 {
		return errors.New("cover art must be square and between 1400 and 3000 pixels")
	}
	return nil
}

func coverArtDimensions(data []byte) (int, int, string, error) {
	if len(data) >= 24 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) && bytes.Equal(data[12:16], []byte("IHDR")) {
		width := int(data[16])<<24 | int(data[17])<<16 | int(data[18])<<8 | int(data[19])
		height := int(data[20])<<24 | int(data[21])<<16 | int(data[22])<<8 | int(data[23])
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
		if marker == 0xd8 || marker == 0xd9 || marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if offset+2 > len(data) {
			break
		}
		length := int(data[offset])<<8 | int(data[offset+1])
		if length < 2 || offset+length > len(data) {
			return 0, 0, "", errors.New("JPEG has an invalid segment")
		}
		if isJPEGStartOfFrame(marker) {
			if length < 8 {
				return 0, 0, "", errors.New("JPEG frame header is too short")
			}
			height := int(data[offset+3])<<8 | int(data[offset+4])
			width := int(data[offset+5])<<8 | int(data[offset+6])
			if width < 1 || height < 1 {
				return 0, 0, "", errors.New("JPEG has invalid dimensions")
			}
			return width, height, "jpeg", nil
		}
		offset += length
	}
	return 0, 0, "", errors.New("JPEG frame header was not found")
}

func isJPEGStartOfFrame(marker byte) bool {
	return (marker >= 0xc0 && marker <= 0xc3) || (marker >= 0xc5 && marker <= 0xc7) || (marker >= 0xc9 && marker <= 0xcb) || (marker >= 0xcd && marker <= 0xcf)
}

func validateEpisode(episode episode) error {
	if !episodeIDPattern.MatchString(episode.ID) {
		return fmt.Errorf("episode id %q must match %s", episode.ID, episodeIDPattern.String())
	}
	for field, value := range map[string]string{
		"guid": episode.GUID, "title": episode.Title, "description": episode.Description,
		"duration": episode.Duration, "staging_key": episode.Audio.StagingKey,
		"public_key": episode.Audio.PublicKey, "sha256": episode.Audio.SHA256,
		"content_type": episode.Audio.ContentType,
	} {
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
	parsedDuration, err := time.Parse("15:04:05", episode.Duration)
	if err != nil {
		return fmt.Errorf("episode %q has an invalid duration: %w", episode.ID, err)
	}
	durationMS := int64(parsedDuration.Hour()*3600+parsedDuration.Minute()*60+parsedDuration.Second()) * 1000
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

func writeFeed(path string, config showConfig, episodes []loadedEpisode) error {
	items := make([]rssItem, 0, len(episodes))
	for _, episode := range episodes {
		items = append(items, rssItem{
			Title:          episode.Title,
			Description:    episode.PlayerNotesText,
			ContentEncoded: string(episode.NotesHTML),
			GUID:           guid{Value: episode.GUID, IsPermaLink: "false"},
			PubDate:        episode.PublishedAt.Format(time.RFC1123Z),
			Link:           fmt.Sprintf("%s/episodes/%s/", config.BaseURL, episode.ID),
			Enclosure:      enclosure{URL: config.MediaURL + "/" + episode.Audio.PublicKey, Length: fmt.Sprintf("%d", episode.Audio.Bytes), Type: episode.Audio.ContentType},
			ItunesTitle:    episode.Title, ItunesSummary: episode.PlayerNotesText, ItunesDuration: episode.Duration,
			ItunesSeason: episode.Season, ItunesEpisode: episode.Number, ItunesExplicit: fmt.Sprintf("%t", episode.Explicit),
		})
	}
	channel := rssChannel{
		Title: config.Title, Link: config.BaseURL + "/", Description: config.Description,
		Language: config.Language, Copyright: config.Copyright, LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
		ItunesAuthor: config.Author, ItunesExplicit: fmt.Sprintf("%t", config.Explicit),
		ItunesCategory: itunesCategory{Text: config.Category, Child: &itunesCategory{Text: config.Subcategory}},
		AtomLink:       atomLink{Href: config.BaseURL + "/feed.xml", Rel: "self", Type: "application/rss+xml"},
		Items:          items,
	}
	if config.CoverArtURL != "" {
		channel.ItunesImage = &itunesImage{Href: config.CoverArtURL}
	}
	if config.OwnerName != "" || config.OwnerEmail != "" {
		channel.ItunesOwner = &itunesOwner{Name: config.OwnerName, Email: config.OwnerEmail}
	}
	feed := rss{Version: "2.0", ItunesNS: "http://www.itunes.com/dtds/podcast-1.0.dtd", AtomNS: "http://www.w3.org/2005/Atom", ContentNS: "http://purl.org/rss/1.0/modules/content/", Channel: channel}
	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return fmt.Errorf("render RSS feed: %w", err)
	}
	return writeFile(path, append([]byte(xml.Header), data...))
}

func writeIndex(path string, config showConfig) error {
	return executeTemplate(path, indexTemplate, struct {
		Config       showConfig
		CanonicalURL string
	}{config, config.BaseURL + "/"})
}

func writeEpisodePage(path string, config showConfig, episode loadedEpisode) error {
	return executeTemplate(path, episodeTemplate, struct {
		Config       showConfig
		Episode      loadedEpisode
		CanonicalURL string
	}{config, episode, fmt.Sprintf("%s/episodes/%s/", config.BaseURL, episode.ID)})
}

func writeEpisodeArchive(outDir string, config showConfig, episodes []loadedEpisode) error {
	pageCount := episodeArchivePageCount(len(episodes))
	for page := 1; page <= pageCount; page++ {
		start := (page - 1) * episodesPerPage
		end := min(start+episodesPerPage, len(episodes))
		pageEpisodes := episodes[start:end]
		data := archivePage{Config: config, Episodes: pageEpisodes, Page: page, PageCount: pageCount, CanonicalURL: config.BaseURL + archivePageURL(page)}
		if page > 1 {
			data.PreviousURL = archivePageURL(page - 1)
		}
		if page < pageCount {
			data.NextURL = archivePageURL(page + 1)
		}
		if err := executeTemplate(filepath.Join(outDir, archivePagePath(page)), archiveTemplate, data); err != nil {
			return err
		}
	}
	return nil
}

type archivePage struct {
	Config       showConfig
	Episodes     []loadedEpisode
	Page         int
	PageCount    int
	PreviousURL  string
	NextURL      string
	CanonicalURL string
}

func episodeArchivePageCount(episodeCount int) int {
	pageCount := (episodeCount + episodesPerPage - 1) / episodesPerPage
	if pageCount == 0 {
		return 1
	}
	return pageCount
}

func archivePagePath(page int) string {
	if page == 1 {
		return filepath.Join("episodes", "index.html")
	}
	return filepath.Join("episodes", "page", fmt.Sprint(page), "index.html")
}

func archivePageURL(page int) string {
	if page == 1 {
		return "/episodes/"
	}
	return fmt.Sprintf("/episodes/page/%d/", page)
}

func writeSitemap(path string, config showConfig, episodes []loadedEpisode) error {
	urls := []sitemapURL{{Location: config.BaseURL + "/"}}
	for page := 1; page <= episodeArchivePageCount(len(episodes)); page++ {
		urls = append(urls, sitemapURL{Location: config.BaseURL + archivePageURL(page)})
	}
	for _, episode := range episodes {
		urls = append(urls, sitemapURL{
			Location:       fmt.Sprintf("%s/episodes/%s/", config.BaseURL, episode.ID),
			LastModifiedAt: episode.PublishedAt.Format("2006-01-02"),
		})
	}
	data, err := xml.MarshalIndent(sitemap{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls}, "", "  ")
	if err != nil {
		return fmt.Errorf("render sitemap: %w", err)
	}
	return writeFile(path, append([]byte(xml.Header), data...))
}

func writeRobots(path string, config showConfig) error {
	return writeFile(path, []byte(fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", config.BaseURL)))
}

func executeTemplate(path, source string, data any) error {
	tmpl, err := template.New("page").Funcs(template.FuncMap{
		"chapterStartSeconds": func(startMS int64) string { return fmt.Sprintf("%.3f", float64(startMS)/1000) },
		"chapterTimestamp":    formatChapterTimestamp,
	}).Parse(source)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return err
	}
	return writeFile(path, output.Bytes())
}

func formatChapterTimestamp(startMS int64) string {
	seconds := startMS / 1000
	minutes := seconds / 60
	if minutes >= 60 {
		return fmt.Sprintf("%d:%02d:%02d", minutes/60, minutes%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds%60)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func copyCoverArt(outDir, source string) error {
	if source == "" {
		return nil
	}
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open cover art for build: %w", err)
	}
	defer input.Close()
	destination := filepath.Join(outDir, filepath.Clean(source))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create cover art output directory: %w", err)
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create cover art output: %w", err)
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("copy cover art: %w", err)
	}
	return nil
}

func copyStaticAssets(outDir string) error {
	for _, name := range []string{"favicon.png", "pplsg-cover-840.webp"} {
		data, err := os.ReadFile(filepath.Join("static", name))
		if err != nil {
			return fmt.Errorf("read static asset %q: %w", name, err)
		}
		if err := writeFile(filepath.Join(outDir, name), data); err != nil {
			return err
		}
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

func fileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open audio: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash audio: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "pplsite: "+format+"\n", args...)
	os.Exit(1)
}

type rss struct {
	XMLName   xml.Name   `xml:"rss"`
	Version   string     `xml:"version,attr"`
	ItunesNS  string     `xml:"xmlns:itunes,attr"`
	AtomNS    string     `xml:"xmlns:atom,attr"`
	ContentNS string     `xml:"xmlns:content,attr"`
	Channel   rssChannel `xml:"channel"`
}
type rssChannel struct {
	Title          string         `xml:"title"`
	Link           string         `xml:"link"`
	Description    string         `xml:"description"`
	Language       string         `xml:"language"`
	Copyright      string         `xml:"copyright,omitempty"`
	LastBuildDate  string         `xml:"lastBuildDate"`
	AtomLink       atomLink       `xml:"atom:link"`
	ItunesAuthor   string         `xml:"itunes:author"`
	ItunesExplicit string         `xml:"itunes:explicit"`
	ItunesImage    *itunesImage   `xml:"itunes:image,omitempty"`
	ItunesOwner    *itunesOwner   `xml:"itunes:owner,omitempty"`
	ItunesCategory itunesCategory `xml:"itunes:category"`
	Items          []rssItem      `xml:"item"`
}
type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}
type itunesImage struct {
	Href string `xml:"href,attr"`
}
type itunesOwner struct {
	Name  string `xml:"itunes:name"`
	Email string `xml:"itunes:email"`
}
type itunesCategory struct {
	Text  string          `xml:"text,attr"`
	Child *itunesCategory `xml:"itunes:category,omitempty"`
}
type rssItem struct {
	Title          string    `xml:"title"`
	Description    string    `xml:"description"`
	ContentEncoded string    `xml:"content:encoded"`
	GUID           guid      `xml:"guid"`
	PubDate        string    `xml:"pubDate"`
	Link           string    `xml:"link"`
	Enclosure      enclosure `xml:"enclosure"`
	ItunesTitle    string    `xml:"itunes:title"`
	ItunesSummary  string    `xml:"itunes:summary"`
	ItunesDuration string    `xml:"itunes:duration"`
	ItunesSeason   int       `xml:"itunes:season"`
	ItunesEpisode  int       `xml:"itunes:episode"`
	ItunesExplicit string    `xml:"itunes:explicit"`
}
type guid struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}
type enclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}
type sitemap struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}
type sitemapURL struct {
	Location       string `xml:"loc"`
	LastModifiedAt string `xml:"lastmod,omitempty"`
}

const indexTemplate = `<!doctype html><html lang="{{.Config.Language}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Config.Title}}</title><meta name="description" content="{{.Config.Description}}"><meta name="robots" content="index,follow"><link rel="canonical" href="{{.CanonicalURL}}"><meta property="og:type" content="website"><meta property="og:site_name" content="{{.Config.Title}}"><meta property="og:url" content="{{.CanonicalURL}}"><meta property="og:title" content="{{.Config.Title}}"><meta property="og:description" content="{{.Config.Description}}">{{if .Config.CoverArtURL}}<meta property="og:image" content="{{.Config.CoverArtURL}}">{{end}}<meta name="twitter:card" content="summary_large_image"><meta name="twitter:title" content="{{.Config.Title}}"><meta name="twitter:description" content="{{.Config.Description}}"><link rel="icon" type="image/png" href="/favicon.png"><link rel="alternate" type="application/rss+xml" title="{{.Config.Title}}" href="/feed.xml"><style>body{font:18px/1.55 system-ui,sans-serif;max-width:760px;margin:3rem auto;padding:0 1rem;color:#18232d}a{color:#075985}.cover{display:block;width:min(100%,420px);height:auto;margin:0 auto 2rem}.notice,.source{padding:1rem;border-left:4px solid #0284c7}.notice{background:#eff6ff}.source{background:#f8fafc;margin:1.5rem 0}</style></head><body><header>{{if .Config.CoverArtURL}}{{if .Config.HomepageCoverURL}}<picture><source srcset="{{.Config.HomepageCoverURL}}" type="image/webp"><img class="cover" src="{{.Config.CoverArtURL}}" alt="{{.Config.Title}} cover art" width="3000" height="3000" fetchpriority="high"></picture>{{else}}<img class="cover" src="{{.Config.CoverArtURL}}" alt="{{.Config.Title}} cover art" width="3000" height="3000" fetchpriority="high">{{end}}{{end}}<h1>{{.Config.Title}}</h1><p>{{.Config.Description}}</p><p><a href="/episodes/">Browse episodes</a> · <a href="/feed.xml">Subscribe with RSS</a></p></header><main><aside class="notice">{{.Config.AIDisclosure}}</aside><section class="source" aria-labelledby="source-materials"><h2 id="source-materials">Open source production materials</h2><p>The podcast’s production material is open source. You are welcome to review it, offer feedback, or contribute improvements on <a href="https://github.com/benvon/ppl-podcast">GitHub</a>.</p><p>The website and publishing workflow are maintained in the <a href="https://github.com/benvon/ppl-podcast-hosting">hosting repository on GitHub</a>.</p></section></main></body></html>`
const archiveTemplate = `<!doctype html><html lang="{{.Config.Language}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Episodes — {{.Config.Title}}</title><meta name="description" content="{{.Config.Description}}"><meta name="robots" content="index,follow"><link rel="canonical" href="{{.CanonicalURL}}"><meta property="og:type" content="website"><meta property="og:site_name" content="{{.Config.Title}}"><meta property="og:url" content="{{.CanonicalURL}}"><meta property="og:title" content="Episodes — {{.Config.Title}}"><meta property="og:description" content="{{.Config.Description}}">{{if .Config.CoverArtURL}}<meta property="og:image" content="{{.Config.CoverArtURL}}">{{end}}<meta name="twitter:card" content="summary_large_image"><meta name="twitter:title" content="Episodes — {{.Config.Title}}"><meta name="twitter:description" content="{{.Config.Description}}"><link rel="icon" type="image/png" href="/favicon.png"><link rel="alternate" type="application/rss+xml" title="{{.Config.Title}}" href="/feed.xml"><style>body{font:18px/1.55 system-ui,sans-serif;max-width:760px;margin:3rem auto;padding:0 1rem;color:#18232d}a{color:#075985}.meta{color:#4b5563;font-size:.9em}.episode-list{list-style:none;margin:0;padding:0}.episode-list li{margin:1rem 0}.pagination{display:flex;justify-content:space-between;gap:1rem;margin-top:2rem}.pagination span{flex:1}</style></head><body><header><p><a href="/">{{.Config.Title}}</a></p><h1>Episodes</h1><p><a href="/feed.xml">Subscribe with RSS</a></p></header><main>{{if .Episodes}}<ul class="episode-list" role="list">{{range .Episodes}}<li><a href="/episodes/{{.ID}}/"><strong>Episode {{.Number}}: {{.Title}}</strong></a><div class="meta">{{.PublishedAt.Format "January 2, 2006"}} · {{.Duration}}</div><p>{{.Description}}</p></li>{{end}}</ul>{{else}}<p>The first episode is in production. Please check back soon.</p>{{end}}{{if gt .PageCount 1}}<nav class="pagination" aria-label="Episode pages">{{if .PreviousURL}}<a href="{{.PreviousURL}}">Newer episodes</a>{{else}}<span></span>{{end}}<span>Page {{.Page}} of {{.PageCount}}</span>{{if .NextURL}}<a href="{{.NextURL}}">Older episodes</a>{{else}}<span></span>{{end}}</nav>{{end}}</main></body></html>`
const episodeTemplate = `<!doctype html><html lang="{{.Config.Language}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Episode.Title}} — {{.Config.Title}}</title><meta name="description" content="{{.Episode.Description}}"><meta name="robots" content="index,follow"><link rel="canonical" href="{{.CanonicalURL}}"><meta property="og:type" content="article"><meta property="og:site_name" content="{{.Config.Title}}"><meta property="og:url" content="{{.CanonicalURL}}"><meta property="og:title" content="{{.Episode.Title}}"><meta property="og:description" content="{{.Episode.Description}}"><meta property="article:published_time" content="{{.Episode.PublishedAt.Format "2006-01-02T15:04:05Z07:00"}}">{{if .Config.CoverArtURL}}<meta property="og:image" content="{{.Config.CoverArtURL}}">{{end}}<meta property="og:audio" content="{{.Config.MediaURL}}/{{.Episode.Audio.PublicKey}}"><meta property="og:audio:type" content="{{.Episode.Audio.ContentType}}"><meta name="twitter:card" content="summary_large_image"><meta name="twitter:title" content="{{.Episode.Title}}"><meta name="twitter:description" content="{{.Episode.Description}}"><link rel="icon" type="image/png" href="/favicon.png"><link rel="alternate" type="application/rss+xml" title="{{.Config.Title}}" href="/feed.xml"><style>body{font:18px/1.55 system-ui,sans-serif;max-width:760px;margin:3rem auto;padding:0 1rem;color:#18232d}a{color:#075985}.notice{background:#eff6ff;padding:1rem;border-left:4px solid #0284c7}.meta{color:#4b5563;font-size:.9em}audio{display:block;width:100%;margin:1.5rem 0}.chapters{margin:1.5rem 0}.chapters summary{cursor:pointer;font-weight:700}.chapters ol{list-style:none;margin:1rem 0 0;padding:0;border-top:1px solid #cbd5e1}.chapters button{appearance:none;background:none;border:0;border-bottom:1px solid #cbd5e1;color:inherit;cursor:pointer;display:grid;font:inherit;gap:1rem;grid-template-columns:4rem 1fr;padding:.7rem .35rem;text-align:left;width:100%}.chapters button:hover,.chapters button:focus-visible{background:#e0f2fe;outline:2px solid #0284c7;outline-offset:-2px}.chapters time{color:#4b5563;font-variant-numeric:tabular-nums}table{border-collapse:collapse}td,th{padding:.4rem;border:1px solid #cbd5e1}</style></head><body><header><p><a href="/">{{.Config.Title}}</a> · <a href="/episodes/">Episodes</a></p><h1>{{.Episode.Title}}</h1><p class="meta">Published {{.Episode.PublishedAt.Format "January 2, 2006"}} · {{.Episode.Duration}}</p><p>{{.Episode.Description}}</p><audio id="episode-audio" controls preload="metadata" data-audio-sha256="{{.Episode.Audio.SHA256}}"><source src="{{.Config.MediaURL}}/{{.Episode.Audio.PublicKey}}" type="{{.Episode.Audio.ContentType}}">Your browser does not support the audio player. <a href="{{.Config.MediaURL}}/{{.Episode.Audio.PublicKey}}">Download MP3</a>.</audio><p><a href="{{.Config.MediaURL}}/{{.Episode.Audio.PublicKey}}">Download MP3</a></p>{{if .Episode.Chapters}}<details class="chapters" data-chapters-audio-sha256="{{.Episode.ChaptersAudioSHA256}}"><summary>Chapters</summary><ol>{{range .Episode.Chapters}}<li><button type="button" data-chapter-start="{{chapterStartSeconds .StartMS}}"><time>{{chapterTimestamp .StartMS}}</time><span>{{.Title}}</span></button></li>{{end}}</ol></details>{{end}}</header><aside class="notice">{{.Config.AIDisclosure}}</aside><main>{{.Episode.NotesHTML}}</main>{{if .Episode.Chapters}}<script>const audio=document.getElementById("episode-audio");for(const button of document.querySelectorAll("[data-chapter-start]")){button.addEventListener("click",()=>{audio.currentTime=Number(button.dataset.chapterStart);audio.play();});}</script>{{end}}</body></html>`
