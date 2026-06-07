package adapters

import (
	"context"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuAPI defines the interface for Feishu API operations
// This interface allows for mocking in tests
type FeishuAPI interface {
	// Get performs a GET request to the Feishu API
	Get(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error)
	// Im returns the IM API interface
	Im() ImAPI
}

// ImAPI defines the interface for Feishu IM (Instant Messaging) operations
type ImAPI interface {
	// Message returns the Message API interface
	Message() MessageAPI
}

// MessageAPI defines the interface for Feishu message operations
type MessageAPI interface {
	// Create sends a new message
	Create(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error)
	// Reply replies to an existing message
	Reply(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error)
}

// larkFeishuAPI is the real Feishu API implementation using the lark SDK
type larkFeishuAPI struct {
	client *lark.Client
}

// Ensure larkFeishuAPI implements FeishuAPI
var _ FeishuAPI = (*larkFeishuAPI)(nil)

// NewLarkFeishuAPI creates a new Feishu API implementation using the lark client
func NewLarkFeishuAPI(client *lark.Client) FeishuAPI {
	return &larkFeishuAPI{client: client}
}

func (f *larkFeishuAPI) Get(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
	return f.client.Get(ctx, path, params, tokenType)
}

func (f *larkFeishuAPI) Im() ImAPI {
	return &larkImAPI{client: f.client}
}

// larkImAPI implements ImAPI using the lark client
type larkImAPI struct {
	client *lark.Client
}

var _ ImAPI = (*larkImAPI)(nil)

func (f *larkImAPI) Message() MessageAPI {
	return &larkMessageAPI{client: f.client}
}

// larkMessageAPI implements MessageAPI using the lark client
type larkMessageAPI struct {
	client *lark.Client
}

var _ MessageAPI = (*larkMessageAPI)(nil)

func (f *larkMessageAPI) Create(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error) {
	return f.client.Im.Message.Create(ctx, req)
}

func (f *larkMessageAPI) Reply(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
	return f.client.Im.Message.Reply(ctx, req)
}
