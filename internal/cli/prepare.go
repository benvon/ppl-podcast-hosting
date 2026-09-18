package cli

import (
	"flag"
	"fmt"

	"github.com/benvon/ppl-podcast-hosting/internal/release"
)

func prepareCommand(args []string) error {
	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	sourceDir := fs.String("source", "", "local directory containing episode.yaml and show-notes.md")
	audioPath := fs.String("audio", "", "local MP3 path")
	outDir := fs.String("out", "episodes", "episodes output directory")
	stagingPrefix := fs.String("staging-prefix", "staging", "private R2 staging key prefix")
	refreshExisting := fs.Bool("refresh-existing", false, "refresh an unmodified existing release package with a new source release seal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	prepared, err := release.Prepare(release.PrepareOptions{SourceDir: *sourceDir, AudioPath: *audioPath, OutputDir: *outDir, StagingPrefix: *stagingPrefix, RefreshExisting: *refreshExisting})
	if err != nil {
		return err
	}
	fmt.Printf("episode_id=%s\nstaging_key=%s\npublic_key=%s\nsha256=%s\nbytes=%d\n", prepared.ID, prepared.StagingKey, prepared.PublicKey, prepared.SHA256, prepared.Bytes)
	return nil
}
