package cli

import (
	"errors"
	"flag"

	"github.com/benvon/ppl-podcast-hosting/internal/release"
)

func verifyHandoffCommand(args []string) error {
	fs := flag.NewFlagSet("verify-handoff", flag.ContinueOnError)
	sourceDir := fs.String("source", "", "sealed local release directory")
	audioPath := fs.String("audio", "", "local MP3 path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sourceDir == "" || *audioPath == "" {
		return errors.New("--source and --audio are required")
	}
	return release.VerifyHandoff(*sourceDir, *audioPath)
}
