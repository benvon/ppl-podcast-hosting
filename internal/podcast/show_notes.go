package podcast

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	showNotesMarkdown    = goldmark.New(goldmark.WithExtensions(extension.Table))
	showNotesLinkPattern = regexp.MustCompile(`(?s)<a\b[^>]*\bhref="(https://[^"]+)"[^>]*>(.+?)</a>`)
	htmlTagPattern       = regexp.MustCompile(`<[^>]+>`)
)

type LoadedEpisode struct {
	Episode
	NotesHTML       template.HTML
	PlayerNotesText string
}

func LoadEpisodes(dir string) ([]LoadedEpisode, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*", "episode.yaml"))
	if err != nil {
		return nil, fmt.Errorf("find episode manifests: %w", err)
	}
	episodes := make([]LoadedEpisode, 0, len(paths))
	for _, path := range paths {
		episode, err := LoadEpisode(path)
		if err != nil {
			return nil, err
		}
		notes, err := os.ReadFile(filepath.Join(filepath.Dir(path), "show-notes.md"))
		if err != nil {
			return nil, fmt.Errorf("read show notes for %q: %w", episode.ID, err)
		}
		var rendered bytes.Buffer
		if err := showNotesMarkdown.Convert(HostingShowNotes(notes), &rendered); err != nil {
			return nil, fmt.Errorf("render show notes for %q: %w", episode.ID, err)
		}
		html := rendered.String()
		// #nosec G203 -- Goldmark raw HTML is disabled; this marks its generated HTML as trusted template content.
		episodes = append(episodes, LoadedEpisode{Episode: episode, NotesHTML: template.HTML(html), PlayerNotesText: PlayerNotesText(episode.Description, html)})
	}
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].PublishedAt.After(episodes[j].PublishedAt) })
	return episodes, nil
}

func HostingShowNotes(notes []byte) []byte {
	lines := strings.Split(strings.ReplaceAll(string(notes), "\r\n", "\n"), "\n")
	withoutNotice := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		if line == "## Production notice" {
			skipping = true
			continue
		}
		if skipping && strings.HasPrefix(line, "## ") {
			skipping = false
		}
		if !skipping {
			withoutNotice = append(withoutNotice, line)
		}
	}
	lines = withoutNotice
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
	result := append([]string{}, lines[:start]...)
	result = append(result, metadata...)
	result = append(result, lines[end:]...)
	return []byte(strings.Join(result, "\n"))
}

func PlayerNotesText(synopsis, notesHTML string) string {
	links := showNotesLinkPattern.FindAllStringSubmatch(notesHTML, -1)
	if len(links) == 0 {
		return synopsis
	}
	seen, lines := map[string]bool{}, []string{synopsis, "", "Study materials and visual aids:"}
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
