package cli

import (
	"flag"

	"github.com/benvon/ppl-podcast-hosting/internal/release"
)

func releaseRecordCommand(args []string) error {
	fs := flag.NewFlagSet("release-record", flag.ContinueOnError)
	episodePath := fs.String("episode", "", "committed hosted episode.yaml")
	outputPath := fs.String("out", "", "output JSON path")
	commit := fs.String("commit", "", "source Git commit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return release.GenerateRecord(*episodePath, *outputPath, *commit)
}
