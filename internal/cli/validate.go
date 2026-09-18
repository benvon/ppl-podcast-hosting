package cli

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
	"github.com/benvon/ppl-podcast-hosting/internal/release"
)

func validateCommand(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	configPath := fs.String("config", "config/show.yaml", "show configuration path")
	episodesDir := fs.String("episodes", "episodes", "episodes directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	config, err := podcast.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	episodes, err := podcast.LoadEpisodes(*episodesDir)
	if err != nil {
		return err
	}
	plain := make([]podcast.Episode, len(episodes))
	for i := range episodes {
		plain[i] = episodes[i].Episode
	}
	if err := podcast.ValidateAll(config, plain); err != nil {
		return err
	}
	if err := validateProvenance(*episodesDir, episodes); err != nil {
		return err
	}
	fmt.Printf("validated %d episode(s)\n", len(episodes))
	return nil
}

func validateProvenance(episodesDir string, episodes []podcast.LoadedEpisode) error {
	for _, episode := range episodes {
		if episode.SourceReleaseSealSHA256 == "" {
			continue
		}
		if err := release.VerifyHostedProvenance(filepath.Join(episodesDir, episode.ID, "episode.yaml"), episode.Episode); err != nil {
			return fmt.Errorf("verify sealed episode %q: %w", episode.ID, err)
		}
	}
	return nil
}
