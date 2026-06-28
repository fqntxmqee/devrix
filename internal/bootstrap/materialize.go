package bootstrap

import (
	"os"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/shared/textutil"
)

func newDefaultMaterializer() materialize.Materializer {
	store, err := materialize.NewPartitionStore(defaultTranscriptBaseDir())
	if err != nil {
		return nil
	}
	return materialize.NewDefaultMaterializer(store)
}

func defaultTranscriptBaseDir() string {
	if v := os.Getenv("DEVRIX_TRANSCRIPT_DIR"); v != "" {
		return textutil.ExpandPath(v)
	}
	return textutil.ExpandPath("~/.devrix/sessions")
}
