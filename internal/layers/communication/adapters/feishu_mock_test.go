package adapters

import (
	"context"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type mockFeishuAPI struct {
	getFunc func(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error)
	imAPI   ImAPI
}

var _ FeishuAPI = (*mockFeishuAPI)(nil)

func (m *mockFeishuAPI) Get(ctx context.Context, path string, params interface{}, tokenType larkcore.AccessTokenType) (*larkcore.ApiResp, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, path, params, tokenType)
	}
	return &larkcore.ApiResp{}, nil
}

func (m *mockFeishuAPI) Im() ImAPI {
	return m.imAPI
}

type mockImAPI struct {
	messageAPI MessageAPI
}

var _ ImAPI = (*mockImAPI)(nil)

func (m *mockImAPI) Message() MessageAPI {
	return m.messageAPI
}

type mockMessageAPI struct {
	createFunc func(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error)
	replyFunc  func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error)
}

var _ MessageAPI = (*mockMessageAPI)(nil)

func (m *mockMessageAPI) Create(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return &larkim.CreateMessageResp{}, nil
}

func (m *mockMessageAPI) Reply(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error) {
	if m.replyFunc != nil {
		return m.replyFunc(ctx, req)
	}
	return &larkim.ReplyMessageResp{}, nil
}

func newMockFeishuAPI() *mockFeishuAPI {
	mockMsgAPI := &mockMessageAPI{}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI}
	return &mockFeishuAPI{imAPI: mockImAPI}
}
