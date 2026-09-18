package cli

import (
	"flag"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
	"github.com/benvon/ppl-podcast-hosting/internal/site"
)

func buildCommand(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	configPath := fs.String("config", "config/show.yaml", "show configuration path")
	episodesDir := fs.String("episodes", "episodes", "episodes directory")
	outDir := fs.String("out", "dist", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := site.RejectUnsafeOutput(*outDir); err != nil {
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
	return site.Build(*outDir, config, episodes)
}
