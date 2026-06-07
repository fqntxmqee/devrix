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
	messageAPI         MessageAPI
	messageReactionAPI MessageReactionAPI
}

var _ ImAPI = (*mockImAPI)(nil)

func (m *mockImAPI) Message() MessageAPI {
	return m.messageAPI
}

func (m *mockImAPI) MessageReaction() MessageReactionAPI {
	return m.messageReactionAPI
}

type mockMessageAPI struct {
	createFunc func(ctx context.Context, req *larkim.CreateMessageReq) (*larkim.CreateMessageResp, error)
	replyFunc  func(ctx context.Context, req *larkim.ReplyMessageReq) (*larkim.ReplyMessageResp, error)
	patchFunc  func(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error)
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

func (m *mockMessageAPI) Patch(ctx context.Context, req *larkim.PatchMessageReq) (*larkim.PatchMessageResp, error) {
	if m.patchFunc != nil {
		return m.patchFunc(ctx, req)
	}
	return &larkim.PatchMessageResp{}, nil
}

type mockMessageReactionAPI struct {
	createFunc func(ctx context.Context, req *larkim.CreateMessageReactionReq) (*larkim.CreateMessageReactionResp, error)
	deleteFunc func(ctx context.Context, req *larkim.DeleteMessageReactionReq) (*larkim.DeleteMessageReactionResp, error)
}

var _ MessageReactionAPI = (*mockMessageReactionAPI)(nil)

func (m *mockMessageReactionAPI) Create(ctx context.Context, req *larkim.CreateMessageReactionReq) (*larkim.CreateMessageReactionResp, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return &larkim.CreateMessageReactionResp{}, nil
}

func (m *mockMessageReactionAPI) Delete(ctx context.Context, req *larkim.DeleteMessageReactionReq) (*larkim.DeleteMessageReactionResp, error) {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, req)
	}
	return &larkim.DeleteMessageReactionResp{}, nil
}

func newMockFeishuAPI() *mockFeishuAPI {
	mockMsgAPI := &mockMessageAPI{}
	mockReactionAPI := &mockMessageReactionAPI{}
	mockImAPI := &mockImAPI{messageAPI: mockMsgAPI, messageReactionAPI: mockReactionAPI}
	return &mockFeishuAPI{imAPI: mockImAPI}
}
