package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateAllRejectsDuplicateGUID(t *testing.T) {
	config := testConfig()
	first := testEpisode("first", "same-guid")
	second := testEpisode("second", "same-guid")
	if err := validateAll(config, []loadedEpisode{first, second}); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("validateAll() error = %v, want duplicate rejection", err)
	}
}

func TestPrepareBuildsImmutableAudioKeys(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(source, "audio.mp3"), []byte("not a real MP3, but deterministic release bytes"))
	writeTestFile(t, filepath.Join(source, "show-notes.md"), []byte("# Notes\n\nReviewed notes."))
	writeTestFile(t, filepath.Join(source, "episode.yaml"), []byte(`id: first
guid: pplstudyguide.com:first
title: First episode
description: A tested episode.
published_at: 2026-08-15T14:00:00Z
duration: "00:01:00"
season: 1
number: 1
explicit: false
audio: {}
`))

	if err := prepareCommand([]string{"--source", source, "--audio", filepath.Join(source, "audio.mp3"), "--out", filepath.Join(root, "episodes")}); err != nil {
		t.Fatalf("prepareCommand() error = %v", err)
	}
	prepared, err := loadEpisode(filepath.Join(root, "episodes", "first", "episode.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(prepared.Audio.StagingKey, "staging/first/") || !strings.HasPrefix(prepared.Audio.PublicKey, "audio/first-") {
		t.Fatalf("prepared audio keys = %#v", prepared.Audio)
	}
	if err := validateEpisode(prepared); err != nil {
		t.Fatalf("validateEpisode(prepared) error = %v", err)
	}
}

func TestPrepareRejectsChapterMarkersForDifferentAudio(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(source, "audio.mp3"), []byte("new audio"))
	writeTestFile(t, filepath.Join(source, "show-notes.md"), []byte("# Notes\n"))
	writeTestFile(t, filepath.Join(source, "episode.yaml"), []byte(`id: chapter-audio
guid: pplstudyguide.com:chapter-audio
title: Chapter audio binding
description: Reject stale chapter metadata.
published_at: 2026-08-15T14:00:00Z
duration: "00:01:00"
season: 1
number: 1
explicit: false
chapters:
  - title: Opening
    start_ms: 0
chapters_audio_sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
audio: {}
`))
	if err := prepareCommand([]string{"--source", source, "--audio", filepath.Join(source, "audio.mp3"), "--out", filepath.Join(root, "episodes")}); err == nil || !strings.Contains(err.Error(), "chapter markers are not bound to the supplied MP3") {
		t.Fatalf("prepareCommand() error = %v, want stale chapter metadata rejection", err)
	}
}

func TestBuildWritesFeedAndShowNotes(t *testing.T) {
	root := t.TempDir()
	config := testConfig()
	episode := testEpisode("first", "pplstudyguide.com:first")
	episode.NotesHTML = "<h1>Episode notes</h1><p><a href=\"https://example.com/diagram\">Diagram</a></p>"
	episode.PlayerNotesText = playerNotesText(episode.Description, string(episode.NotesHTML))
	if err := buildInto(root, config, []loadedEpisode{episode}); err != nil {
		t.Fatal(err)
	}
	feed, err := os.ReadFile(filepath.Join(root, "dist", "feed.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(feed), "https://media.pplstudyguide.com/audio/first.mp3") || !strings.Contains(string(feed), "isPermaLink=\"false\"") || !strings.Contains(string(feed), "Study materials and visual aids:") || !strings.Contains(string(feed), "https://example.com/diagram") || !strings.Contains(string(feed), "<content:encoded>&lt;h1&gt;Episode notes&lt;/h1&gt;") {
		t.Fatalf("unexpected feed: %s", feed)
	}
	page, err := os.ReadFile(filepath.Join(root, "dist", "episodes", "first", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "Episode notes") || !strings.Contains(string(page), "<audio id=\"episode-audio\" controls") || !strings.Contains(string(page), "https://media.pplstudyguide.com/audio/first.mp3") || !strings.Contains(string(page), "rel=\"canonical\" href=\"https://pplstudyguide.com/episodes/first/\"") || !strings.Contains(string(page), "property=\"og:type\" content=\"article\"") {
		t.Fatalf("show notes were not rendered")
	}
	homepage, err := os.ReadFile(filepath.Join(root, "dist", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(homepage), "https://github.com/benvon/ppl-podcast") || !strings.Contains(string(homepage), "href=\"/episodes/\"") || !strings.Contains(string(homepage), "rel=\"canonical\" href=\"https://pplstudyguide.com/\"") || !strings.Contains(string(homepage), "name=\"twitter:card\" content=\"summary_large_image\"") {
		t.Fatalf("homepage does not link to the open source production materials")
	}
	archive, err := os.ReadFile(filepath.Join(root, "dist", "episodes", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(archive), "Episode first") || !strings.Contains(string(archive), "rel=\"canonical\" href=\"https://pplstudyguide.com/episodes/\"") {
		t.Fatalf("episode archive does not contain the episode: %s", archive)
	}
	sitemap, err := os.ReadFile(filepath.Join(root, "dist", "sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sitemap), "https://pplstudyguide.com/episodes/first/") || !strings.Contains(string(sitemap), "<lastmod>2026-08-15</lastmod>") {
		t.Fatalf("sitemap does not contain the published episode: %s", sitemap)
	}
	robots, err := os.ReadFile(filepath.Join(root, "dist", "robots.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(robots) != "User-agent: *\nAllow: /\n\nSitemap: https://pplstudyguide.com/sitemap.xml\n" {
		t.Fatalf("unexpected robots.txt: %s", robots)
	}
}

func TestPlayerNotesTextIncludesOnlyUniqueHTTPSStudyLinks(t *testing.T) {
	notes := `<p><a href="https://example.com/diagram?a=1&amp;b=2" title="FAA source">Diagram <strong>one</strong></a></p><p><a href="mailto:feedback@example.com">Feedback</a></p><p><a href="https://example.com/diagram?a=1&amp;b=2">Duplicate</a></p>`
	got := playerNotesText("A concise synopsis.", notes)
	want := "A concise synopsis.\n\nStudy materials and visual aids:\n- Diagram one: https://example.com/diagram?a=1&b=2"
	if got != want {
		t.Fatalf("playerNotesText() = %q, want %q", got, want)
	}
}

func TestHostingShowNotesKeepsOneDisclosureAndFormatsMetadata(t *testing.T) {
	notes := []byte("# Title\r\n\r\n**Episode:** 4\r\n**Version:** 1.0.0\r\n**Source verification:** Reviewed today.\r\n\r\n## Production notice\r\n\r\nThis duplicate notice should not appear on the episode page.\r\n\r\n## In this episode\r\n\r\n- A useful lesson.\r\n")
	formatted := hostingShowNotes(notes)
	if strings.Contains(string(formatted), "Production notice") || strings.Contains(string(formatted), "duplicate notice") {
		t.Fatalf("hostingShowNotes() retained the duplicate disclosure: %s", formatted)
	}
	if !strings.Contains(string(formatted), "- **Episode:** 4\n- **Version:** 1.0.0\n- **Source verification:** Reviewed today.") {
		t.Fatalf("hostingShowNotes() did not format metadata as a list: %s", formatted)
	}
	var rendered bytes.Buffer
	if err := showNotesMarkdown.Convert(formatted, &rendered); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "<ul>") || !strings.Contains(rendered.String(), "<strong>Episode:</strong> 4") {
		t.Fatalf("formatted metadata did not render as a list: %s", rendered.String())
	}
}

func TestEpisodePageRendersCollapsibleChaptersOnlyWhenPresent(t *testing.T) {
	root := t.TempDir()
	config := testConfig()
	withChapters := testEpisode("with-chapters", "pplstudyguide.com:with-chapters")
	withChapters.Chapters = []chapter{{Title: "Opening", StartMS: 0}, {Title: "Lesson", StartMS: 74_000}}
	withChapters.ChaptersAudioSHA256 = withChapters.Audio.SHA256
	withChaptersPath := filepath.Join(root, "with-chapters.html")
	if err := writeEpisodePage(withChaptersPath, config, withChapters); err != nil {
		t.Fatal(err)
	}
	withChaptersPage, err := os.ReadFile(withChaptersPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<details class=\"chapters\" data-chapters-audio-sha256=\"" + withChapters.Audio.SHA256 + "\">", "data-audio-sha256=\"" + withChapters.Audio.SHA256 + "\"", "<summary>Chapters</summary>", "data-chapter-start=\"74.000\"", "<time>1:14</time>", "<span>Lesson</span>"} {
		if !strings.Contains(string(withChaptersPage), expected) {
			t.Fatalf("chapter page is missing %q: %s", expected, withChaptersPage)
		}
	}

	withoutChapters := testEpisode("without-chapters", "pplstudyguide.com:without-chapters")
	withoutChaptersPath := filepath.Join(root, "without-chapters.html")
	if err := writeEpisodePage(withoutChaptersPath, config, withoutChapters); err != nil {
		t.Fatal(err)
	}
	withoutChaptersPage, err := os.ReadFile(withoutChaptersPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(withoutChaptersPage), "<details class=\"chapters\">") {
		t.Fatalf("episode without chapter markers rendered a Chapters control: %s", withoutChaptersPage)
	}
}

func TestValidateEpisodeRejectsChapterAtOrBeyondDuration(t *testing.T) {
	episode := testEpisode("chapter-boundary", "pplstudyguide.com:chapter-boundary")
	episode.Duration = "00:01:00"
	episode.Chapters = []chapter{{Title: "Opening", StartMS: 0}, {Title: "Too late", StartMS: 60_000}}
	episode.ChaptersAudioSHA256 = episode.Audio.SHA256
	if err := validateEpisode(episode.episode); err == nil || !strings.Contains(err.Error(), "must start before the episode duration") {
		t.Fatalf("validateEpisode() error = %v, want chapter-duration bound failure", err)
	}
}

func TestValidateEpisodeRejectsChapterMarkersForDifferentAudio(t *testing.T) {
	episode := testEpisode("chapter-audio-binding", "pplstudyguide.com:chapter-audio-binding")
	episode.Chapters = []chapter{{Title: "Opening", StartMS: 0}}
	episode.ChaptersAudioSHA256 = strings.Repeat("b", 64)
	if err := validateEpisode(episode.episode); err == nil || !strings.Contains(err.Error(), "chapter markers must be bound to the staged audio checksum") {
		t.Fatalf("validateEpisode() error = %v, want chapter/audio checksum binding failure", err)
	}
}

func TestWriteEpisodeArchivePaginatesEpisodes(t *testing.T) {
	root := t.TempDir()
	episodes := make([]loadedEpisode, 0, episodesPerPage+1)
	for number := episodesPerPage + 1; number >= 1; number-- {
		episode := testEpisode(fmt.Sprintf("episode-%02d", number), fmt.Sprintf("guid-%02d", number))
		episode.Title = fmt.Sprintf("Test title %02d", number)
		episode.Number = number
		episode.PublishedAt = time.Date(2026, 8, number, 14, 0, 0, 0, time.UTC)
		episodes = append(episodes, episode)
	}
	if err := writeEpisodeArchive(root, testConfig(), episodes); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(root, "episodes", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "<ul class=\"episode-list\" role=\"list\">") || strings.Contains(string(first), "<ol>") || !strings.Contains(string(first), "Episode 11: Test title 11") || strings.Contains(string(first), "Episode 01: Test title 01") || !strings.Contains(string(first), "href=\"/episodes/page/2/\"") {
		t.Fatalf("first archive page has incorrect pagination: %s", first)
	}
	second, err := os.ReadFile(filepath.Join(root, "episodes", "page", "2", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(second), "Episode 1: Test title 01") || strings.Contains(string(second), "Episode 2: Test title 02") || !strings.Contains(string(second), "href=\"/episodes/\"") {
		t.Fatalf("second archive page has incorrect pagination: %s", second)
	}
}

func TestCopyStaticAssetsWritesFavicon(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot := filepath.Clean(filepath.Join(workingDir, "..", ".."))
	if err := os.Chdir(repositoryRoot); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(workingDir); err != nil {
			t.Fatal(err)
		}
	}()

	root := t.TempDir()
	if err := copyStaticAssets(root); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(root, "favicon.png"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("static", "favicon.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(source) {
		t.Fatal("built favicon does not match the static source")
	}
}

func TestLoadEpisodesRendersPipeTables(t *testing.T) {
	root := t.TempDir()
	episodeDir := filepath.Join(root, "first")
	if err := os.MkdirAll(episodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(episodeDir, "episode.yaml"), []byte("id: first\n"))
	writeTestFile(t, filepath.Join(episodeDir, "show-notes.md"), []byte(`| Topic | Source |
| --- | --- |
| ADM | FAA |
`))

	episodes, err := loadEpisodes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 {
		t.Fatalf("loadEpisodes() loaded %d episodes, want 1", len(episodes))
	}
	if !strings.Contains(string(episodes[0].NotesHTML), "<table>") || !strings.Contains(string(episodes[0].NotesHTML), "<th>Topic</th>") {
		t.Fatalf("pipe table was not rendered as HTML table: %s", episodes[0].NotesHTML)
	}
}

func TestCoverArtDimensionsRejectsAlphaPNG(t *testing.T) {
	png := make([]byte, 26)
	copy(png, []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 'I', 'H', 'D', 'R'})
	png[16], png[17] = 0x0b, 0xb8 // 3000 pixels wide
	png[20], png[21] = 0x0b, 0xb8 // 3000 pixels tall
	png[25] = 6                   // truecolor with alpha
	if _, _, _, err := coverArtDimensions(png); err == nil || !strings.Contains(err.Error(), "alpha") {
		t.Fatalf("coverArtDimensions() error = %v, want alpha rejection", err)
	}
}

func buildInto(root string, config showConfig, episodes []loadedEpisode) error {
	out := filepath.Join(root, "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := writeFeed(filepath.Join(out, "feed.xml"), config, episodes); err != nil {
		return err
	}
	if err := writeIndex(filepath.Join(out, "index.html"), config); err != nil {
		return err
	}
	if err := writeEpisodeArchive(out, config, episodes); err != nil {
		return err
	}
	if err := writeSitemap(filepath.Join(out, "sitemap.xml"), config, episodes); err != nil {
		return err
	}
	if err := writeRobots(filepath.Join(out, "robots.txt"), config); err != nil {
		return err
	}
	return writeEpisodePage(filepath.Join(out, "episodes", episodes[0].ID, "index.html"), config, episodes[0])
}

func testConfig() showConfig {
	return showConfig{Title: "PPL Study Guide", Description: "Description", Language: "en-US", Author: "Author", Category: "Education", Subcategory: "Courses", BaseURL: "https://pplstudyguide.com", MediaURL: "https://media.pplstudyguide.com", CoverArtURL: "https://pplstudyguide.com/cover.png", OwnerName: "Owner", OwnerEmail: "owner@example.com", AIDisclosure: "Disclosure"}
}

func testEpisode(id, guidValue string) loadedEpisode {
	return loadedEpisode{episode: episode{ID: id, GUID: guidValue, Title: "Episode " + id, Description: "Description", PublishedAt: time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC), Duration: "00:01:00", Season: 1, Number: 1, Audio: audio{StagingKey: "staging/" + id + "/audio.mp3", PublicKey: "audio/" + id + ".mp3", SHA256: strings.Repeat("a", 64), Bytes: 10, ContentType: "audio/mpeg"}}}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
