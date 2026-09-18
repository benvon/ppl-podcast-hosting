package podcast

import (
	"bytes"
	"strings"
	"testing"
)

func TestShowNotesRenderTablesAndRemoveDuplicateNotice(t *testing.T) {
	notes := []byte("# Notes\n\n## Production notice\n\nDuplicate.\n\n## Sources\n\n| Topic | Source |\n| --- | --- |\n| ADM | FAA |\n")
	formatted := HostingShowNotes(notes)
	if strings.Contains(string(formatted), "Duplicate") {
		t.Fatalf("HostingShowNotes() retained duplicate production notice: %s", formatted)
	}
	var rendered bytes.Buffer
	if err := showNotesMarkdown.Convert(formatted, &rendered); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "<table>") || !strings.Contains(rendered.String(), "<th>Topic</th>") {
		t.Fatalf("show notes table was not rendered: %s", rendered.String())
	}
}

func TestValidEpisodeIDUsesHostedContract(t *testing.T) {
	for _, id := range []string{"", "A-bad", "x", "../escape"} {
		if ValidEpisodeID(id) {
			t.Fatalf("ValidEpisodeID(%q) = true, want false", id)
		}
	}
	if !ValidEpisodeID("core-17") {
		t.Fatal("ValidEpisodeID(core-17) = false, want true")
	}
}
