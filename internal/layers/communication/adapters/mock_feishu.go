package adapters

import (
	"context"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MockFeishuAPI is a mock implementation of FeishuAPI for testing
type MockFeishuAPI struct {
	// GetFunc mocks the Get method
	GetFunc func(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error)

	// ImAPI mock
	ImAPI ImAPI
}

// Ensure MockFeishuAPI implements FeishuAPI
var _ FeishuAPI = (*MockFeishuAPI)(nil)

// Get implements FeishuAPI
func (m *MockFeishuAPI) Get(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, path, params, tokenType)
	}
	// Default implementation returns empty response
	return &larkcore.ApiResp{}, nil
}

// Im implements FeishuAPI
func (m *MockFeishuAPI) Im() ImAPI {
	return m.ImAPI
}

// MockImAPI is a mock implementation of ImAPI
type MockImAPI struct {
	MessageAPI MessageAPI
}

// Ensure MockImAPI implements ImAPI
var _ ImAPI = (*MockImAPI)(nil)

// Message implements ImAPI
func (m *MockImAPI) Message() MessageAPI {
	return m.MessageAPI
}

// MockMessageAPI is a mock implementation of MessageAPI
type MockMessageAPI struct {
	// CreateFunc mocks the Create method
	CreateFunc func(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error)
	// ReplyFunc mocks the Reply method
	ReplyFunc func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error)
}

// Ensure MockMessageAPI implements MessageAPI
var _ MessageAPI = (*MockMessageAPI)(nil)

// Create implements MessageAPI
func (m *MockMessageAPI) Create(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	return &larkim.CreateMessageResp{}, nil
}

// Reply implements MessageAPI
func (m *MockMessageAPI) Reply(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
	if m.ReplyFunc != nil {
		return m.ReplyFunc(ctx, req)
	}
	return &larkim.ReplyMessageResp{}, nil
}

// NewMockFeishuAPI creates a MockFeishuAPI with default mock implementations
func NewMockFeishuAPI() *MockFeishuAPI {
	mockMsgAPI := &MockMessageAPI{}
	mockImAPI := &MockImAPI{MessageAPI: mockMsgAPI}
	return &MockFeishuAPI{ImAPI: mockImAPI}
}
