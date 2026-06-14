package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

func TestCardkitClient_CreateCard_Success(t *testing.T) {
	client := newCardkitClient(&mockFeishuAPI{
		postFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			if path != "/open-apis/cardkit/v1/cards" {
				t.Fatalf("path = %q", path)
			}
			return &larkcore.ApiResp{
				StatusCode: http.StatusOK,
				RawBody:    []byte(`{"code":0,"data":{"card_id":"card_abc"}}`),
			}, nil
		},
	})
	cardID, err := client.CreateCard(context.Background(), BuildStreamingReplyCardJSON("", true))
	if err != nil {
		t.Fatalf("CreateCard() error = %v", err)
	}
	if cardID != "card_abc" {
		t.Fatalf("cardID = %q, want card_abc", cardID)
	}
}

func TestCardkitClient_StreamElementContent_SequenceMonotonic(t *testing.T) {
	var sequences []int
	client := newCardkitClient(&mockFeishuAPI{
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			m, ok := body.(map[string]any)
			if !ok {
				t.Fatalf("body type %T", body)
			}
			seq, _ := m["sequence"].(int)
			sequences = append(sequences, seq)
			return &larkcore.ApiResp{StatusCode: http.StatusOK, RawBody: []byte(`{"code":0}`)}, nil
		},
	})
	for seq := 1; seq <= 3; seq++ {
		if err := client.StreamElementContent(context.Background(), "card_1", replyTextElementID, "text", seq); err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
	}
	if len(sequences) != 3 || sequences[0] != 1 || sequences[2] != 3 {
		t.Fatalf("sequences = %v", sequences)
	}
}

func TestCardkitClient_RateLimitSkipped(t *testing.T) {
	client := newCardkitClient(&mockFeishuAPI{
		putFunc: func(ctx context.Context, path string, body interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
			return &larkcore.ApiResp{
				StatusCode: http.StatusOK,
				RawBody:    []byte(`{"code":230020,"msg":"rate limited"}`),
			}, nil
		},
	})
	err := client.StreamElementContent(context.Background(), "card_1", replyTextElementID, "x", 1)
	if err != ErrFeishuCardRateLimited {
		t.Fatalf("err = %v, want ErrFeishuCardRateLimited", err)
	}
}

func TestBuildStreamingReplyCardJSON_HasElementID(t *testing.T) {
	raw := BuildStreamingReplyCardJSON("hello", true)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	config := parsed["config"].(map[string]any)
	if config["streaming_mode"] != true {
		t.Fatalf("streaming_mode = %v", config["streaming_mode"])
	}
	body := parsed["body"].(map[string]any)
	elements := body["elements"].([]any)
	elem := elements[0].(map[string]any)
	if elem["element_id"] != replyTextElementID {
		t.Fatalf("element_id = %v", elem["element_id"])
	}
}

func TestBuildCardIDMessageContent(t *testing.T) {
	got, err := buildCardIDMessageContent("card_xyz")
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"data":{"card_id":"card_xyz"},"type":"card"}` && got != `{"type":"card","data":{"card_id":"card_xyz"}}` {
		// json.Marshal key order may vary — parse instead
		var parsed map[string]any
		if err := json.Unmarshal([]byte(got), &parsed); err != nil {
			t.Fatalf("parse: %v raw=%q", err, got)
		}
		data, _ := parsed["data"].(map[string]any)
		if data["card_id"] != "card_xyz" {
			t.Fatalf("card_id = %v", data["card_id"])
		}
	}
}
