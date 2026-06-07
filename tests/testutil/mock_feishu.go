package testutil

import (
	"context"

	"github.com/devrix/devrix/internal/layers/communication/adapters"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MockFeishuAPI is a test double for adapters.FeishuAPI.
type MockFeishuAPI struct {
	GetFunc func(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error)
	ImAPI   adapters.ImAPI
}

var _ adapters.FeishuAPI = (*MockFeishuAPI)(nil)

func (m *MockFeishuAPI) Get(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, path, params, tokenType)
	}
	return &larkcore.ApiResp{}, nil
}

func (m *MockFeishuAPI) Im() adapters.ImAPI {
	return m.ImAPI
}

// MockImAPI is a test double for adapters.ImAPI.
type MockImAPI struct {
	MessageAPI adapters.MessageAPI
}

var _ adapters.ImAPI = (*MockImAPI)(nil)

func (m *MockImAPI) Message() adapters.MessageAPI {
	return m.MessageAPI
}

// MockMessageAPI is a test double for adapters.MessageAPI.
type MockMessageAPI struct {
	CreateFunc func(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error)
	ReplyFunc  func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error)
}

var _ adapters.MessageAPI = (*MockMessageAPI)(nil)

func (m *MockMessageAPI) Create(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	return &larkim.CreateMessageResp{}, nil
}

func (m *MockMessageAPI) Reply(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
	if m.ReplyFunc != nil {
		return m.ReplyFunc(ctx, req)
	}
	return &larkim.ReplyMessageResp{}, nil
}

// NewMockFeishuAPI returns a MockFeishuAPI with default nested mocks.
func NewMockFeishuAPI() *MockFeishuAPI {
	mockMsgAPI := &MockMessageAPI{}
	mockImAPI := &MockImAPI{MessageAPI: mockMsgAPI}
	return &MockFeishuAPI{ImAPI: mockImAPI}
}
