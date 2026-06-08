# Tasks: devrix-feishu-cards

**Change ID:** devrix-feishu-cards
**Status:** Completed

---

## Milestone 1: Card 模型

- [x] **T1**: 新增 `internal/layers/communication/core/card.go`
- [x] **T2**: 实现 RenderText 回退
- [x] **T3**: 支持 CardMarkdown/Divider/Actions/ListItem/Select/Note

## Milestone 2: Feishu 渲染器

- [x] **T4**: 重写 `adapters/feishu_card.go`
- [x] **T5**: 7 种 Header 颜色
- [x] **T6**: 更新 `adapters/feishu.go` 适配新接口

## Milestone 3: 用户体验

- [x] **T7**: 即时 OK reaction（typing emoji）
- [x] **T8**: done_emoji 完成通知
- [x] **T9**: 新增 `core/progress.go` 进度类型

## Milestone 4: 测试

- [x] **T10**: `feishu_card_test.go` 单元测试
- [x] **T11**: `feishu_integration_test.go` RenderText 回退测试

## Backlog（未纳入本变更）

- [ ] 12 种颜色扩展
- [ ] RichCardTextStreamer 流式预览
