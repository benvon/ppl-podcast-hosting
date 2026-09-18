package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

func TestEpisodePageUsesEmbeddedTemplateAndFooter(t *testing.T) {
	config := podcast.ShowConfig{Title: "PPL Study Guide", Language: "en-US", BaseURL: "https://pplstudyguide.com", MediaURL: "https://media.pplstudyguide.com", OwnerName: "Owner", AIDisclosure: "Disclosure"}
	episode := podcast.LoadedEpisode{Episode: podcast.Episode{ID: "core-17", Title: "Episode", Description: "Description", PublishedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Duration: "00:01:00", Audio: podcast.Audio{PublicKey: "audio/core-17.mp3", SHA256: strings.Repeat("a", 64), ContentType: "audio/mpeg"}}}
	path := filepath.Join(t.TempDir(), "index.html")
	if err := WriteEpisodePage(path, config, episode); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<audio id=\"episode-audio\" controls", "By Owner.", "CC BY 4.0", "BSD 3-Clause"} {
		if !strings.Contains(string(page), expected) {
			t.Fatalf("rendered page missing %q: %s", expected, page)
		}
	}
}

func TestArchivePaginationPaths(t *testing.T) {
	if EpisodeArchivePageCount(0) != 1 || EpisodeArchivePageCount(11) != 2 {
		t.Fatal("unexpected archive page count")
	}
	if ArchivePagePath(2) != filepath.Join("episodes", "page", "2", "index.html") || ArchivePageURL(2) != "/episodes/page/2/" {
		t.Fatal("unexpected archive page path or URL")
	}
}
