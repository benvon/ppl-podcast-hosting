package release

import (
	"strings"
	"testing"

	"github.com/benvon/ppl-podcast-hosting/internal/podcast"
)

func TestTagForRejectsMalformedOrMismatchedIdentity(t *testing.T) {
	for _, episode := range []podcast.Episode{
		{ReleaseKey: "episode-007", ContentVersion: "0.1.0", Number: 7},
		{ReleaseKey: "episode-07", ContentVersion: "1.0.0-01", Number: 7},
		{ReleaseKey: "episode-08", ContentVersion: "0.1.0", Number: 7},
	} {
		if _, err := TagFor(episode, ""); err == nil {
			t.Fatalf("TagFor(%#v) succeeded, want rejection", episode)
		}
	}
	if _, err := TagFor(podcast.Episode{ReleaseKey: "episode-07", ContentVersion: "0.1.0", Number: 7}, "0.1.1"); err == nil || !strings.Contains(err.Error(), "sealed source version") {
		t.Fatalf("TagFor() error = %v, want sealed version mismatch", err)
	}
}

func TestValidateCandidateIdentitiesRejectsDuplicates(t *testing.T) {
	err := ValidateCandidateIdentities([]Candidate{{ID: "core-07", ReleaseKey: "episode-07", ContentVersion: "0.1.0"}, {ID: "other", ReleaseKey: "episode-07", ContentVersion: "0.1.0"}})
	if err == nil || !strings.Contains(err.Error(), "share public release identity") {
		t.Fatalf("ValidateCandidateIdentities() error = %v, want duplicate rejection", err)
	}
}
