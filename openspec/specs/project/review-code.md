# 代码 Review 规范

**版本:** 1.0.0
**状态:** Active
**所属阶段:** S4-Gate
**关联规范:** `coding.md`、`testing.md`

---

## 1. 触发条件

以下任一情况必须进行代码 Review：

- S4 实现阶段完成，tasks.md 所有任务标记 done
- PR 从 Draft 转为 Ready
- 安全相关变更（认证、授权、加密、输入校验、文件操作）

---

## 2. Review 维度

### 2.1 OpenSpec 文档完整性

| 检查项 | 说明 |
|--------|------|
| Change 文件齐全 | `.openspec.yaml`、`proposal.md`、`design.md`、`tasks.md`、`specs/` |
| T 层已登记 | T 层注册表（根索引 + 域 `openspec/specs/d{N}-*/t-registry.md`）中有对应条目 |
| 文档状态一致 | `.openspec.yaml` 和 `proposal.md` 中的 status 一致 |

### 2.2 代码质量

| 检查项 | 说明 |
|--------|------|
| 包位置正确 | 代码放在正确的 D-S 目录 |
| 函数规模 | < 50 行（超过需说明理由） |
| 文件规模 | < 800 行（超过需拆分） |
| 嵌套深度 | <= 4 层 |
| 命名清晰 | 无 `data`、`temp`、`result` 等无意义变量名 |
| 接口合理 | 1-3 个方法的小接口 |

### 2.3 错误与安全

| 检查项 | 说明 |
|--------|------|
| 错误不静默 | 无 `_ = err` 无注释的情况 |
| Sentinel Error 正确 | 使用 `internal/shared/errors/` 模式 |
| 输入校验 | 系统边界有验证（用户输入、外部 API 响应） |
| 无硬编码密钥 | 无 API key、密码、token |
| 并发安全 | 共享状态有 `sync.RWMutex` 保护 |
| 值对象不可变 | 值对象使用 `With*` 返回新副本，禁止直接改字段 |
| 实体受控可变 | 实体状态变更通过 method + 锁，禁止外部直接赋值 |
| 类型断言安全 | 无裸 `.(*Type)`，使用 type switch 或 `ok` 模式 |
| CQS | 读方法（Get/List）无副作用 |

### 2.4 测试完整性

| 检查项 | 说明 |
|--------|------|
| 单元测试存在 | 每个新函数/方法有对应 `_test.go` |
| Happy path + sad path | 正常和异常路径均有测试 |
| T 层测试覆盖 | 所有 P0 T 层有对应的验收测试 |
| Race 检测 | 并发代码通过 `-race` |

---

## 3. 严重级别

| 级别 | 含义 | 行动 |
|------|------|------|
| **CRITICAL** | 安全漏洞、数据丢失风险 | **必须修复**，否则不能合并 |
| **HIGH** | Bug、功能缺失、明显的设计问题 | **应该修复**，合并前处理 |
| **MEDIUM** | 可维护性问题 | **建议修复**，不阻塞合并 |
| **LOW** | 风格建议、非关键优化 | 可忽略或后续处理 |

---

## 4. Review 流程

```
1. 检查 OpenSpec 文档完整性
2. 检查代码质量（包位置、命名、规模）
3. 检查安全（密钥、输入校验、并发）
4. 检查测试（覆盖率、T 层映射、race）
5. 运行测试确认 CI 通过
6. 提交 Review 结论
```

---

## 5. Review 命令

```bash
# 查看 PR diff
gh pr diff

# 查看 CI 状态
gh pr checks

# 提交 Review
gh pr review --approve
gh pr review --request-changes --body "需要修复: ..."
gh pr review --comment --body "建议: ..."
```

---

## 6. 检查清单

Review 完成时确认：

- [ ] OpenSpec 文档齐全且状态一致
- [ ] T 层注册表已更新（根索引 + 域注册表）
- [ ] 代码在正确的 D-S 包中
- [ ] 无 CRITICAL 安全问题
- [ ] `go vet` 和 `./scripts/test-unit.sh` 通过
- [ ] 新功能有对应测试
- [ ] P0 T 层测试通过
- [ ] CI 全绿
- [ ] Review 结论明确（Approved / Changes Requested）
