package site

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

const episodesPerPage = 10

func Build(outDir string, config podcast.ShowConfig, episodes []podcast.LoadedEpisode) error {
	if err := RejectUnsafeOutput(outDir); err != nil {
		return err
	}
	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("clear output directory: %w", err)
	}
	// #nosec G301 -- generated public-site directories must be traversable by the hosting server.
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := WriteFeed(filepath.Join(outDir, "feed.xml"), config, episodes); err != nil {
		return err
	}
	if err := WriteIndex(filepath.Join(outDir, "index.html"), config); err != nil {
		return err
	}
	if err := CopyCoverArt(outDir, config.CoverArtSource); err != nil {
		return err
	}
	if err := CopyStaticAssets(outDir); err != nil {
		return err
	}
	if err := WriteEpisodeArchive(outDir, config, episodes); err != nil {
		return err
	}
	if err := WriteSitemap(filepath.Join(outDir, "sitemap.xml"), config, episodes); err != nil {
		return err
	}
	if err := WriteRobots(filepath.Join(outDir, "robots.txt"), config); err != nil {
		return err
	}
	for _, episode := range episodes {
		if err := WriteEpisodePage(filepath.Join(outDir, "episodes", episode.ID, "index.html"), config, episode); err != nil {
			return err
		}
	}
	return WriteFile(filepath.Join(outDir, "_headers"), []byte("/feed.xml\n  Cache-Control: no-cache, max-age=0, must-revalidate\n\n/episodes/*\n  Cache-Control: public, max-age=3600\n"))
}

func WriteFeed(path string, config podcast.ShowConfig, episodes []podcast.LoadedEpisode) error {
	items := make([]rssItem, 0, len(episodes))
	for _, episode := range episodes {
		items = append(items, rssItem{
			Title: episode.Title, Description: episode.PlayerNotesText, ContentEncoded: string(episode.NotesHTML),
			GUID: guid{Value: episode.GUID, IsPermaLink: "false"}, PubDate: episode.PublishedAt.Format(time.RFC1123Z),
			Link:        fmt.Sprintf("%s/episodes/%s/", config.BaseURL, episode.ID),
			Enclosure:   enclosure{URL: config.MediaURL + "/" + episode.Audio.PublicKey, Length: fmt.Sprintf("%d", episode.Audio.Bytes), Type: episode.Audio.ContentType},
			ItunesTitle: episode.Title, ItunesSummary: episode.PlayerNotesText, ItunesDuration: episode.Duration,
			ItunesSeason: episode.Season, ItunesEpisode: episode.Number, ItunesExplicit: fmt.Sprintf("%t", episode.Explicit),
		})
	}
	channel := rssChannel{
		Title: config.Title, Link: config.BaseURL + "/", Description: config.Description,
		Language: config.Language, Copyright: config.Copyright, LastBuildDate: time.Now().UTC().Format(time.RFC1123Z),
		ItunesAuthor: config.Author, ItunesExplicit: fmt.Sprintf("%t", config.Explicit),
		ItunesCategory: itunesCategory{Text: config.Category, Child: &itunesCategory{Text: config.Subcategory}},
		AtomLink:       atomLink{Href: config.BaseURL + "/feed.xml", Rel: "self", Type: "application/rss+xml"}, Items: items,
	}
	if config.CoverArtURL != "" {
		channel.ItunesImage = &itunesImage{Href: config.CoverArtURL}
	}
	if config.OwnerName != "" || config.OwnerEmail != "" {
		channel.ItunesOwner = &itunesOwner{Name: config.OwnerName, Email: config.OwnerEmail}
	}
	data, err := xml.MarshalIndent(rss{Version: "2.0", ItunesNS: "http://www.itunes.com/dtds/podcast-1.0.dtd", AtomNS: "http://www.w3.org/2005/Atom", ContentNS: "http://purl.org/rss/1.0/modules/content/", Channel: channel}, "", "  ")
	if err != nil {
		return fmt.Errorf("render RSS feed: %w", err)
	}
	return WriteFile(path, append([]byte(xml.Header), data...))
}

func WriteIndex(path string, config podcast.ShowConfig) error {
	return executeTemplate(path, "index.html", struct {
		Config       podcast.ShowConfig
		CanonicalURL string
	}{config, config.BaseURL + "/"})
}

func WriteEpisodePage(path string, config podcast.ShowConfig, episode podcast.LoadedEpisode) error {
	return executeTemplate(path, "episode.html", struct {
		Config       podcast.ShowConfig
		Episode      podcast.LoadedEpisode
		CanonicalURL string
	}{config, episode, fmt.Sprintf("%s/episodes/%s/", config.BaseURL, episode.ID)})
}

func WriteEpisodeArchive(outDir string, config podcast.ShowConfig, episodes []podcast.LoadedEpisode) error {
	pageCount := EpisodeArchivePageCount(len(episodes))
	for page := 1; page <= pageCount; page++ {
		start := (page - 1) * episodesPerPage
		end := min(start+episodesPerPage, len(episodes))
		data := archivePage{Config: config, Episodes: episodes[start:end], Page: page, PageCount: pageCount, CanonicalURL: config.BaseURL + ArchivePageURL(page)}
		if page > 1 {
			data.PreviousURL = ArchivePageURL(page - 1)
		}
		if page < pageCount {
			data.NextURL = ArchivePageURL(page + 1)
		}
		if err := executeTemplate(filepath.Join(outDir, ArchivePagePath(page)), "archive.html", data); err != nil {
			return err
		}
	}
	return nil
}

type archivePage struct {
	Config                             podcast.ShowConfig
	Episodes                           []podcast.LoadedEpisode
	Page, PageCount                    int
	PreviousURL, NextURL, CanonicalURL string
}

func EpisodeArchivePageCount(count int) int {
	if count == 0 {
		return 1
	}
	return (count + episodesPerPage - 1) / episodesPerPage
}
func ArchivePagePath(page int) string {
	if page == 1 {
		return filepath.Join("episodes", "index.html")
	}
	return filepath.Join("episodes", "page", fmt.Sprint(page), "index.html")
}
func ArchivePageURL(page int) string {
	if page == 1 {
		return "/episodes/"
	}
	return fmt.Sprintf("/episodes/page/%d/", page)
}

func WriteSitemap(path string, config podcast.ShowConfig, episodes []podcast.LoadedEpisode) error {
	urls := []sitemapURL{{Location: config.BaseURL + "/"}}
	for page := 1; page <= EpisodeArchivePageCount(len(episodes)); page++ {
		urls = append(urls, sitemapURL{Location: config.BaseURL + ArchivePageURL(page)})
	}
	for _, episode := range episodes {
		urls = append(urls, sitemapURL{Location: fmt.Sprintf("%s/episodes/%s/", config.BaseURL, episode.ID), LastModifiedAt: episode.PublishedAt.Format("2006-01-02")})
	}
	data, err := xml.MarshalIndent(sitemap{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls}, "", "  ")
	if err != nil {
		return fmt.Errorf("render sitemap: %w", err)
	}
	return WriteFile(path, append([]byte(xml.Header), data...))
}

func WriteRobots(path string, config podcast.ShowConfig) error {
	return WriteFile(path, []byte(fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", config.BaseURL)))
}

func executeTemplate(path, name string, data any) error {
	source, err := pageTemplates.ReadFile("templates/" + name)
	if err != nil {
		return fmt.Errorf("read page template %q: %w", name, err)
	}
	footer, err := pageTemplates.ReadFile("templates/footer.html")
	if err != nil {
		return fmt.Errorf("read footer template: %w", err)
	}
	text := strings.TrimSuffix(string(source), "\n")
	if !strings.Contains(text, "</body>") {
		return errors.New("page template must contain a closing body tag")
	}
	text = strings.Replace(text, "</body>", `{{template "site-footer" .}}</body>`, 1)
	tmpl, err := template.New("page").Funcs(template.FuncMap{
		"chapterStartSeconds": func(startMS int64) string { return fmt.Sprintf("%.3f", float64(startMS)/1000) },
		"chapterTimestamp":    FormatChapterTimestamp,
	}).Parse(strings.TrimSuffix(string(footer), "\n"))
	if err != nil {
		return err
	}
	if tmpl, err = tmpl.Parse(text); err != nil {
		return err
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return err
	}
	return WriteFile(path, output.Bytes())
}

func FormatChapterTimestamp(startMS int64) string {
	seconds := startMS / 1000
	minutes := seconds / 60
	if minutes >= 60 {
		return fmt.Sprintf("%d:%02d:%02d", minutes/60, minutes%60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds%60)
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
