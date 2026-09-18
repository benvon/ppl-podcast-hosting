package cli

import (
	"flag"
	"fmt"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

func publishPlanCommand(args []string) error {
	fs := flag.NewFlagSet("publish-plan", flag.ContinueOnError)
	episodesDir := fs.String("episodes", "episodes", "episodes directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	episodes, err := podcast.LoadEpisodes(*episodesDir)
	if err != nil {
		return err
	}
	for _, episode := range episodes {
		fmt.Printf("%s\t%s\t%s\t%d\n", episode.Audio.StagingKey, episode.Audio.PublicKey, episode.Audio.SHA256, episode.Audio.Bytes)
	}
	return nil
}
