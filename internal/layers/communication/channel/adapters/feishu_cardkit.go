package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

const (
	replyTextElementID      = "reply_text"
	feishuCardRateLimitCode = 230020
)

// ErrFeishuCardRateLimited indicates cardkit rate limit; caller may skip the frame.
var ErrFeishuCardRateLimited = errors.New("feishu cardkit rate limited")

// ErrFeishuCardStreamClosed indicates the card's streaming mode was closed by Feishu
// (e.g. idle timeout or previous finalization); caller should reset and create a new card.
var ErrFeishuCardStreamClosed = errors.New("feishu cardkit stream closed")

const feishuCardStreamClosedCode = 300309

type cardkitClient struct {
	api FeishuAPI
}

func newCardkitClient(api FeishuAPI) *cardkitClient {
	return &cardkitClient{api: api}
}

// CardkitClient is the public surface for cardkit operations. The
// WaveScheduler worker card renderer uses this to stream element content
// without depending on the full FeishuAdapter. It wraps a FeishuAPI and is
// safe for concurrent use.
type CardkitClient struct {
	api FeishuAPI
}

// NewCardkitClient returns a public CardkitClient.
func NewCardkitClient(api FeishuAPI) *CardkitClient {
	return &CardkitClient{api: api}
}

// CreateCard creates a cardkit card and returns its ID.
func (c *CardkitClient) CreateCard(ctx context.Context, cardJSON string) (string, error) {
	body := map[string]any{
		"type": "card_json",
		"data": cardJSON,
	}
	apiResp, err := c.api.Post(ctx, "/open-apis/cardkit/v1/cards", body, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return "", fmt.Errorf("cardkit create card: %w", err)
	}
	return parseCardkitCreateResponse(apiResp)
}

// StreamElementContent streams a markdown/text element by element id.
func (c *CardkitClient) StreamElementContent(ctx context.Context, cardID, elementID, content string, sequence int) error {
	apiPath := fmt.Sprintf("/open-apis/cardkit/v1/cards/%s/elements/%s/content", cardID, elementID)
	body := map[string]any{
		"content":  content,
		"sequence": sequence,
	}
	apiResp, err := c.api.Put(ctx, apiPath, body, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return fmt.Errorf("cardkit stream element: %w", err)
	}
	return parseCardkitMutationResponse(apiResp, "stream element")
}

// UpdateCard replaces the entire card JSON.
func (c *CardkitClient) UpdateCard(ctx context.Context, cardID, cardJSON string, sequence int) error {
	apiPath := fmt.Sprintf("/open-apis/cardkit/v1/cards/%s", cardID)
	body := map[string]any{
		"card": map[string]any{
			"type": "card_json",
			"data": cardJSON,
		},
		"sequence": sequence,
	}
	apiResp, err := c.api.Put(ctx, apiPath, body, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return fmt.Errorf("cardkit update card: %w", err)
	}
	return parseCardkitMutationResponse(apiResp, "update card")
}

// BuildCardIDContent serializes a card reference message body. Exposed for
// callers that need to deliver a cardkit card via the IM Create API.
func (c *CardkitClient) BuildCardIDContent(cardID string) (string, error) {
	return buildCardIDMessageContent(cardID)
}

func (c *cardkitClient) CreateCard(ctx context.Context, cardJSON string) (string, error) {
	body := map[string]any{
		"type": "card_json",
		"data": cardJSON,
	}
	apiResp, err := c.api.Post(ctx, "/open-apis/cardkit/v1/cards", body, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return "", fmt.Errorf("cardkit create card: %w", err)
	}
	return parseCardkitCreateResponse(apiResp)
}

func (c *cardkitClient) StreamElementContent(ctx context.Context, cardID, elementID, content string, sequence int) error {
	apiPath := fmt.Sprintf("/open-apis/cardkit/v1/cards/%s/elements/%s/content", cardID, elementID)
	body := map[string]any{
		"content":  content,
		"sequence": sequence,
	}
	apiResp, err := c.api.Put(ctx, apiPath, body, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return fmt.Errorf("cardkit stream element: %w", err)
	}
	return parseCardkitMutationResponse(apiResp, "stream element")
}

func (c *cardkitClient) UpdateCard(ctx context.Context, cardID, cardJSON string, sequence int) error {
	apiPath := fmt.Sprintf("/open-apis/cardkit/v1/cards/%s", cardID)
	body := map[string]any{
		"card": map[string]any{
			"type": "card_json",
			"data": cardJSON,
		},
		"sequence": sequence,
	}
	apiResp, err := c.api.Put(ctx, apiPath, body, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return fmt.Errorf("cardkit update card: %w", err)
	}
	return parseCardkitMutationResponse(apiResp, "update card")
}

type cardkitCreateResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		CardID string `json:"card_id"`
	} `json:"data"`
}

type cardkitMutationResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func parseCardkitCreateResponse(apiResp *larkcore.ApiResp) (string, error) {
	if apiResp == nil {
		return "", fmt.Errorf("cardkit create card: empty response")
	}
	if apiResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cardkit create card: HTTP %d", apiResp.StatusCode)
	}
	var resp cardkitCreateResponse
	if err := json.Unmarshal(apiResp.RawBody, &resp); err != nil {
		return "", fmt.Errorf("cardkit create card: parse response: %w", err)
	}
	if resp.Code == feishuCardRateLimitCode {
		return "", ErrFeishuCardRateLimited
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("cardkit create card: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data.CardID == "" {
		return "", fmt.Errorf("cardkit create card: empty card_id")
	}
	return resp.Data.CardID, nil
}

func parseCardkitMutationResponse(apiResp *larkcore.ApiResp, action string) error {
	if apiResp == nil {
		return fmt.Errorf("cardkit %s: empty response", action)
	}
	if apiResp.StatusCode != http.StatusOK {
		return fmt.Errorf("cardkit %s: HTTP %d", action, apiResp.StatusCode)
	}
	var resp cardkitMutationResponse
	if err := json.Unmarshal(apiResp.RawBody, &resp); err != nil {
		return fmt.Errorf("cardkit %s: parse response: %w", action, err)
	}
	if resp.Code == feishuCardRateLimitCode {
		return ErrFeishuCardRateLimited
	}
	if resp.Code == feishuCardStreamClosedCode {
		return ErrFeishuCardStreamClosed
	}
	if resp.Code != 0 {
		return fmt.Errorf("cardkit %s: code=%d msg=%s", action, resp.Code, resp.Msg)
	}
	return nil
}

func buildCardIDMessageContent(cardID string) (string, error) {
	b, err := json.Marshal(map[string]any{
		"type": "card",
		"data": map[string]string{"card_id": cardID},
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
