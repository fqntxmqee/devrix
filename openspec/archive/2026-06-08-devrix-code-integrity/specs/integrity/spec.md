# Code Integrity Specification

**Module:** Integrity

> 本规格定义 Devrix 代码健康规范的验收场景。<!-- L5: L5-0-1-01, L5-0-1-02, L5-0-1-03, L5-0-1-04, L5-0-1-05, L5-0-1-06, L5-0-1-07, L5-0-1-08, L5-0-1-09 -->

## ADDED

### Requirement: 分层不可变性规范发布

#### Scenario: coding.md §9 包含代码完整性规范
- GIVEN `openspec/specs/project/coding.md` 文件存在
- WHEN 读取 §9 代码完整性
- THEN 明确区分值对象、实体、Service 三类不可变策略
- AND 值对象部分要求 `With*` 方法返回新副本
- AND 实体部分要求状态变更通过 method + 加锁

#### Scenario: CLAUDE.md 引用新规范
- GIVEN `CLAUDE.md` 存在
- WHEN 读取不可变性条款
- THEN 不再包含"创建新对象，禁止原地修改"的绝对表述
- AND 引用 `openspec/specs/project/coding-integrity.md` 作为权威来源

<!-- L5: L5-0-1-01, L5-0-1-02 -->

### Requirement: Type Assertion 类型安全

#### Scenario: emitEvent 处理 EventConnectionLostData
- GIVEN `connection/manager.go` 的 `emitEvent` 方法
- WHEN 传入 `*DomainEvent{Data: &EventConnectionLostData{}}`
- THEN 不 panic
- AND 日志输出包含 connection_id

#### Scenario: emitEvent 处理 EventConnectionRestoredData
- GIVEN `connection/manager.go` 的 `emitEvent` 方法
- WHEN 传入 `*DomainEvent{Data: &EventConnectionRestoredData{}}`
- THEN 不 panic
- AND 日志输出包含 connection_id

#### Scenario: emitEvent 处理未知类型
- GIVEN `connection/manager.go` 的 `emitEvent` 方法
- WHEN 传入 `*DomainEvent{Data: &SomeUnknownType{}}`
- THEN 不 panic
- AND 日志输出 warn 级日志

<!-- L5: L5-0-1-03, L5-0-1-04, L5-0-1-05 -->

### Requirement: D1 Communication L5 测试补全

#### Scenario: 新会话创建被拒绝
- GIVEN Gateway 的 CreateSession 返回错误
- WHEN 适配器请求创建新会话
- THEN 错误传播到调用方
- AND 无 session 文件写入磁盘

#### Scenario: /new 命令解析
- GIVEN 用户输入 `/new /tmp/workdir`
- WHEN `ParseCommand` 被调用
- THEN `Command.Type == CommandNew`
- AND `Command.Args == ["/tmp/workdir"]`

#### Scenario: /help 命令解析
- GIVEN 用户输入 `/help`
- WHEN `ParseCommand` 被调用
- THEN `Command.Type == CommandHelp`

#### Scenario: /stop 命令解析
- GIVEN 用户输入 `/stop`
- WHEN `ParseCommand` 被调用
- THEN `Command.Type == CommandStop`

#### Scenario: 飞书消息解析
- GIVEN 飞书 webhook JSON 消息体
- WHEN 解析函数被调用
- THEN `InboundMessage` 的字段映射正确
- AND ChatID/UserID/Content 非空

<!-- L5: L5-0-1-06, L5-0-1-07, L5-0-1-08 -->

### Requirement: D6 Evolution L5 排期标注

#### Scenario: L5 注册表 D6 条目包含目标版本
- GIVEN `openspec/l5-registry.md`
- WHEN 读取 D6 条目
- THEN L5-6-1-01 和 L5-6-2-01 均标注 `PlannedVersion`
- AND PlannedVersion 为具体版本号（如 v2.1.0）

<!-- L5: L5-0-1-09 -->

### Requirement: 命名与异味清理

#### Scenario: CLIRenderer 命名正确
- GIVEN `internal/layers/communication/renderers/message.go`
- WHEN 搜索 `CLRenderer` 类型定义
- THEN 无匹配结果
- WHEN 搜索 `CLIRenderer`
- THEN 存在类型定义和导出工厂函数

#### Scenario: 无重复 min 函数
- GIVEN 项目中所有 Go 文件
- WHEN 搜索函数定义 `func min(`
- THEN 仅在 Go built-in 行为中出现，无自定义 `min` 函数
- AND `status.go` 不包含自定义 `min`

#### Scenario: GetInstances 无副作用
- GIVEN `instance/registry.go`
- WHEN `GetInstances` 被调用
- THEN 不修改任何 InstanceInfo 的 Status 字段
- AND 存在独立的 `RefreshHealthStatus` 方法用于状态刷新

### Requirement: S4-Gate code review 包含不可变性检查

#### Scenario: code review checklist 更新
- GIVEN `openspec/specs/project/review-code.md`
- WHEN 检查清单加载
- THEN 包含"值对象使用 With* 而非直接赋值"检查项
- AND 包含"实体状态变更通过 method + 加锁"检查项

## MODIFIED

(None)

## REMOVED

(None)
