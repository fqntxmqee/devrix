package token

import (
	"embed"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

//go:embed testdata/cl100k_base.tiktoken
var embeddedBPE embed.FS

var setupBPEOnce sync.Once

func ensureEmbeddedBPELoader() {
	setupBPEOnce.Do(func() {
		tiktoken.SetBpeLoader(embeddedBpeLoader{})
	})
}

type embeddedBpeLoader struct{}

func (embeddedBpeLoader) LoadTiktokenBpe(tiktokenBpeFile string) (map[string]int, error) {
	if strings.Contains(tiktokenBpeFile, "cl100k_base.tiktoken") {
		data, err := embeddedBPE.ReadFile("testdata/cl100k_base.tiktoken")
		if err != nil {
			return nil, fmt.Errorf("read embedded cl100k_base: %w", err)
		}
		return parseTiktokenBPE(data)
	}
	return nil, fmt.Errorf("unsupported bpe file: %s", tiktokenBpeFile)
}

func parseTiktokenBPE(contents []byte) (map[string]int, error) {
	bpeRanks := make(map[string]int)
	for _, line := range strings.Split(string(contents), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) < 2 {
			continue
		}
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, err
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
		bpeRanks[string(token)] = rank
	}
	return bpeRanks, nil
}
