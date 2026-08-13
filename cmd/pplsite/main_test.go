package main

import (
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

func TestBuildWritesFeedAndShowNotes(t *testing.T) {
	root := t.TempDir()
	config := testConfig()
	episode := testEpisode("first", "pplstudyguide.com:first")
	episode.NotesHTML = "<h1>Episode notes</h1>"
	if err := buildInto(root, config, []loadedEpisode{episode}); err != nil {
		t.Fatal(err)
	}
	feed, err := os.ReadFile(filepath.Join(root, "dist", "feed.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(feed), "https://media.pplstudyguide.com/audio/first.mp3") || !strings.Contains(string(feed), "isPermaLink=\"false\"") {
		t.Fatalf("unexpected feed: %s", feed)
	}
	page, err := os.ReadFile(filepath.Join(root, "dist", "episodes", "first", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "Episode notes") || !strings.Contains(string(page), "<audio controls") || !strings.Contains(string(page), "https://media.pplstudyguide.com/audio/first.mp3") || !strings.Contains(string(page), "rel=\"canonical\" href=\"https://pplstudyguide.com/episodes/first/\"") || !strings.Contains(string(page), "property=\"og:type\" content=\"article\"") {
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

func TestWriteEpisodeArchivePaginatesEpisodes(t *testing.T) {
	root := t.TempDir()
	episodes := make([]loadedEpisode, 0, episodesPerPage+1)
	for number := episodesPerPage + 1; number >= 1; number-- {
		episode := testEpisode(fmt.Sprintf("episode-%02d", number), fmt.Sprintf("guid-%02d", number))
		episode.Title = fmt.Sprintf("Episode %02d", number)
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
	if !strings.Contains(string(first), "Episode 11") || strings.Contains(string(first), "Episode 01") || !strings.Contains(string(first), "href=\"/episodes/page/2/\"") {
		t.Fatalf("first archive page has incorrect pagination: %s", first)
	}
	second, err := os.ReadFile(filepath.Join(root, "episodes", "page", "2", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(second), "Episode 01") || strings.Contains(string(second), "Episode 02") || !strings.Contains(string(second), "href=\"/episodes/\"") {
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
