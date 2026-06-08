# Design: Domain Build Tags

**Parent:** `demand.md`
**Canonical Spec:** `openspec/specs/testing-framework/domain-segmentation.md`

## Tag Matrix

| 组合 | 用途 |
|------|------|
| `integration && d2` | D2 集成 |
| `integration && cross` | D2+D3、D1+D2、D4+D2 跨域 |
| `integration && d3 && live` | 真实 LLM API |
| `acceptance && d2` | D2 P0 验收 |
| `smoke && d4` | D4 E2E fork |

## Script Contract

见 `domain-segmentation.md` §5。
