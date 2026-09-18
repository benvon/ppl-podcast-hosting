package cli

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/benvon/ppl-podcast-hosting/internal/release"
)

func releaseCandidatesCommand(args []string) error {
	fs := flag.NewFlagSet("release-candidates", flag.ContinueOnError)
	episodesDir := fs.String("episodes", "episodes", "episodes directory")
	jsonOutput := fs.Bool("json", false, "write release candidates as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	candidates, err := release.Candidates(*episodesDir)
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
