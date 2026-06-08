package llmresponses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Fixture captures a recorded HTTP response for replay.
type Fixture struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// ReplayTransport serves fixtures as HTTP responses.
type ReplayTransport struct {
	FixturePath   string
	RealTransport http.RoundTripper
	Record        bool
}

// RoundTrip replays a fixture or records a live response.
func (v *ReplayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if v.Record {
		if v.RealTransport == nil {
			v.RealTransport = http.DefaultTransport
		}
		resp, err := v.RealTransport.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		if err := saveFixture(v.FixturePath, resp); err != nil {
			return nil, err
		}
		return replayFixture(v.FixturePath)
	}
	return replayFixture(v.FixturePath)
}

func replayFixture(path string) (*http.Response, error) {
	fixture, err := loadFixture(path)
	if err != nil {
		return nil, err
	}
	header := make(http.Header)
	for k, v := range fixture.Headers {
		header.Set(k, v)
	}
	return &http.Response{
		StatusCode: fixture.Status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(fixture.Body)),
	}, nil
}

func loadFixture(path string) (*Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", path, err)
	}
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", path, err)
	}
	return &fixture, nil
}

func saveFixture(path string, resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	headers := make(map[string]string)
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}
	fixture := Fixture{
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    string(body),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// FixtureDir returns the absolute fixtures directory for tests.
func FixtureDir() string {
	return filepath.Join("tests", "fixtures", "llm_responses")
}
