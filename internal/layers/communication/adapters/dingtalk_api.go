package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const dingTalkTokenURL = "https://oapi.dingtalk.com/gettoken"

// DingTalkAPI abstracts DingTalk HTTP operations for testing.
type DingTalkAPI interface {
	GetAccessToken(ctx context.Context, appKey, appSecret string) (string, error)
	SendSessionMessage(ctx context.Context, sessionWebhook, content string) error
}

type dingTalkHTTPAPI struct {
	client *http.Client
}

// NewDingTalkHTTPAPI creates a production DingTalk API client.
func NewDingTalkHTTPAPI() DingTalkAPI {
	return &dingTalkHTTPAPI{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

type dingTalkTokenResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Token   string `json:"access_token"`
}

func (c *dingTalkHTTPAPI) GetAccessToken(ctx context.Context, appKey, appSecret string) (string, error) {
	u, err := url.Parse(dingTalkTokenURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("appkey", appKey)
	q.Set("appsecret", appSecret)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed dingTalkTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.ErrCode != 0 {
		return "", fmt.Errorf("dingtalk token error: %s", parsed.ErrMsg)
	}
	if parsed.Token == "" {
		return "", fmt.Errorf("dingtalk token empty")
	}
	return parsed.Token, nil
}

type dingTalkSendPayload struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

type dingTalkSendResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (c *dingTalkHTTPAPI) SendSessionMessage(ctx context.Context, sessionWebhook, content string) error {
	if sessionWebhook == "" {
		return fmt.Errorf("session webhook is required")
	}

	payload := dingTalkSendPayload{MsgType: "text"}
	payload.Text.Content = content
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sessionWebhook, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var parsed dingTalkSendResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return err
	}
	if parsed.ErrCode != 0 {
		return fmt.Errorf("dingtalk send error: %s", parsed.ErrMsg)
	}
	return nil
}

type mockDingTalkAPI struct {
	mu           sync.Mutex
	tokenCalls   int
	sendCalls    []string
	sendContents []string
	token        string
	sendErr      error
}

func (m *mockDingTalkAPI) GetAccessToken(_ context.Context, _, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenCalls++
	if m.token == "" {
		return "mock-token", nil
	}
	return m.token, nil
}

func (m *mockDingTalkAPI) SendSessionMessage(_ context.Context, sessionWebhook, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCalls = append(m.sendCalls, sessionWebhook)
	m.sendContents = append(m.sendContents, content)
	return m.sendErr
}
