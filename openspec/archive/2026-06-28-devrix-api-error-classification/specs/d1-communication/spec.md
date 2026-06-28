# D1 Communication Spec Delta — IM error_code 差异化文案 (DM-20260628-001)

**Change ID:** devrix-api-error-classification
**Demand ID:** DM-20260628-001
**Delta Type:** ADDED (v3.0.0 → v3.1.0 → v3.1.1)
**SOT:** `internal/layers/communication/channel/adapters/error_format.go`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. `errorCopyByCode(code, fallback)` 5 类 + 兜底 | `internal/layers/communication/channel/adapters/error_format.go` | NEW | RateLimit/AuthFailed/PromptTooLong/MediaSize/ServerError + Unknown |
| 2. `errorCodeFromMetadata(metadata)` 提取 | `internal/layers/communication/channel/adapters/error_format.go` | NEW | 从 `Event.Metadata["error_code"]` 转 APIErrorCode |
| 3. feishu adapter "error" case 文案 | `internal/layers/communication/channel/adapters/feishu.go` | MODIFIED | 走差异化文案，fallback 兼容老路径 |

---

## 2. 差异化文案映射

| APIErrorCode | 文案 |
|--------------|------|
| APICodeRateLimit | ⚠️ 模型繁忙，请稍候重试 |
| APICodeAuthenticationFailed | 🔑 API key 失效，请检查 ~/.devrix/config.yaml |
| APICodePromptTooLong | 📦 会话过长，已尝试压缩 |
| APICodeMediaSize / APICodeImageSize | 📎 文件/图片过大，请缩小后重试 |
| APICodeServerError | 🔧 服务暂时不可用，请稍候重试 |
| APICodeUnknown | 现有通用文案（fallback 透传） |

---

## 3. T 点增量（v3.0.0 → v3.1.0 → v3.1.1）

| T ID | 描述 | Status |
|------|------|--------|
| D1-S3-A08-T01 | feishu / cli IM 适配器基于 `Event.Metadata["error_code"]` 走差异化文案（5 类 + Unknown 兜底） | IMPLEMENTED (DM-20260628-001 S5 PR #265, 7 sub-test 全 PASS) |

---

## 4. 行为不变保证

- 老路径无 `error_code` metadata 时，errorCodeFromMetadata 返回 APICodeUnknown，errorCopyByCode 透传原 content
- cli 适配器（如果存在）同样享受差异化文案
- 现有 feishu 卡片格式 / 飞书 API 调用契约不变