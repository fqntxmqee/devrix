# 配置与环境规范

**版本:** 1.0.0
**状态:** Active
**所属阶段:** 通用（所有阶段适用）

---

## 1. 配置层级

Devrix 有 4 层配置，优先级从高到低：

```
CLI Flags  >  环境变量  >  config.yaml  >  devrix.yaml（默认）
```

| 层级 | 文件/来源 | 用途 |
|------|----------|------|
| 1 (最高) | CLI flags | 运行时覆盖（`--port 8080`） |
| 2 | 环境变量 | 密钥、部署特定配置 |
| 3 | `config.yaml` | 本地覆盖（gitignore，不提交） |
| 4 (最低) | `devrix.yaml` | 默认配置（提交到仓库） |

### 1.1 加载优先级示例

如果同一字段在多个层级定义，高优先级覆盖低优先级：

```
devrix.yaml:     port: 8080
config.yaml:     port: 9090     ← 覆盖 8080
环境变量:         PORT=7070     ← 覆盖 9090
CLI flag:        --port 6060    ← 最终生效: 6060
```

---

## 2. 配置文件规范

### 2.1 devrix.yaml — 默认配置（仓库内）

- 包含所有模块的默认值
- 每个 section 对应一个内部配置结构体
- 添加新配置项时必须在对应 `internal/shared/config/*.go` 中定义

### 2.2 config.yaml — 本地覆盖（gitignore）

- 仅覆盖需要个性化的字段
- 不包含完整的配置
- **不提交到仓库**

### 2.3 ~/.devrix/config.yaml — 用户级配置

- 用户本地偏好（主题、语言、IM 平台选择）
- 示例文件：`.devrix/config.example.yaml`
- **LLM 网关覆盖层（`llm_gateway` 段）**：v2.x 新增。
  - 路径：内部实现见 `internal/shared/config/user.go` 的 `UserConfig.LLMGateway` 字段
  - 合并语义：与项目级 `devrix.yaml.llm_gateway` 段做深合并（见 §8）
  - 完整可用模型 ID 见 `internal/layers/llmgateway/configure/data/models.yaml`（嵌入到二进制中）
  - 模型版本不在 Go 代码中硬编码：任何具体模型选择都由用户态或项目态配置承担

---

## 3. 环境变量

### 3.1 命名规范

```
DEVRX_{SECTION}_{KEY}
```

示例：
```bash
DEVRX_LLM_API_KEY=sk-xxx
DEVRX_FEISHU_APP_ID=xxx
DEVRX_SESSION_SECRET=xxx
```

### 3.2 模板文件

`.env.example` 包含所有可配置的环境变量模板（不含真实值）：

```bash
# LLM 网关（v2.x：模型选择通过 env 或用户配置覆盖）
DEVRX_LLM_DEFAULT_MODEL=
DEVRX_LLM_DEFAULT_PROVIDER=
DEVRX_LLM_DEFAULT_TIER=
DEVRX_LLM_MODEL_FAST=
DEVRX_LLM_MODEL_DEFAULT=
DEVRX_LLM_MODEL_POWERFUL=

# 飞书
DEVRX_FEISHU_APP_ID=
DEVRX_FEISHU_APP_SECRET=
```

---

## 4. Secret 管理

### 4.1 禁止

- **禁止**将密钥/凭证硬编码在源码中
- **禁止**将 `.env` 或含真实密钥的 `config.yaml` 提交到仓库
- **禁止**在日志中打印密钥

### 4.2 必须

- 所有密钥通过环境变量或 `.env` 加载
- 启动时校验必需的环境变量是否存在，缺失则 Fatal
- 使用 `.env.example` 作为文档，列出所有需要的变量

### 4.3 密钥读取模式

```go
apiKey := os.Getenv("DEVRX_LLM_API_KEY")
if apiKey == "" {
    log.Fatal("DEVRX_LLM_API_KEY not configured")
}
```

---

## 5. 多入口配置

Devrix 有多个入口点，各有不同配置需求：

| 入口 | 文件 | 主要配置来源 |
|------|------|-------------|
| 统一主程序 | `cmd/devrix/main.go` | `devrix.yaml` + `~/.devrix/config.yaml` + env |
| 冒烟测试 | `cmd/feishu-smoke/main.go` | 硬编码测试值（仅测试用） |
| 覆盖率报告 | `cmd/obs-coverage-report/main.go` | CLI flags |

IM（飞书 / 钉钉）由 `~/.devrix/config.yaml` 中 `im.enabled` 与 `im.platform.provider` 决定；仅运行一个 `devrix` 进程持有 WebSocket。

---

## 6. 配置代码规范

### 6.1 新增配置项

新增一个配置项需要修改：

1. `internal/shared/config/<module>.go` — 添加字段到 FileConfig 和 Config
2. `devrix.yaml` — 添加默认值
3. `.env.example` — 如果可用环境变量覆盖则添加

### 6.2 Config 结构模式

参见 `coding.md` §4.2。

---

## 7. 检查清单

- [ ] 密钥不在源码中硬编码
- [ ] `.env` 或含密钥的 `config.yaml` 在 `.gitignore` 中
- [ ] `.env.example` 列出所有需要的环境变量
- [ ] 新增配置项在 `devrix.yaml` 和对应的 `config/*.go` 中都已添加
- [ ] 启动时校验了必需的环境变量

---

## 8. LLM 网关分层配置（v2.x）

DSAFT: D3-S6-A03 (model catalog, v2.x)。

**核心约束：Go 代码中不允许硬编码任何具体模型版本。** 模型选择完全由配置承担。

### 8.1 加载顺序（高 → 低）

```
环境变量  (DEVRIX_LLM_*)
  ↓
~/.devrix/config.yaml  (UserConfig.LLMGateway，yaml tag `llm_gateway`)
  ↓
devrix.yaml  (ConfigFile.LLMGateway，yaml tag `llm_gateway`)
  ↓
编译期默认值  (DefaultLLMGatewayConfig：仅 provider 基础设施，**无模型名**)
```

### 8.2 默认值边界

`DefaultLLMGatewayConfig()` 只返回 **provider 基础设施**：

- ✅ `BaseURL`、`APIKeyEnv`、`Timeout`、`Retry`、`Headers`、`Type` — 描述 provider 的传输契约
- ❌ `DefaultProvider`、`DefaultModel`、`DefaultTier`、`ModelTiers.*`、`providers.*.DefaultModel` — 全部空字符串 / 空 map

这意味着：项目仓库内 `devrix.yaml` 的 `llm_gateway` 段是 **必需** 的；如果用户没有改写且项目配置缺失，启动会因 `ErrLLMConfigMissing` 失败并提示用户在哪里补齐。

### 8.3 合并语义

`configure.MergeLLMGatewayFileConfig(base, override)` 规则：

| 字段类型 | 合并方式 |
|---------|---------|
| 标量（`DefaultModel` 等） | override 非零值覆盖 base |
| `ModelTiers`、`ModelRouting` | key 级合并，override 中存在的 key 覆盖 base |
| `Providers` | 按 provider 名合并；每个 provider 内部走标量规则 |
| `CircuitBreaker` | 仅非零字段被覆盖 |
| `Providers.X.Headers` | key 级深合并 |

**禁止 mutate 输入**：所有 map / 嵌套结构在合并前深拷贝。

### 8.4 校验

`configure.ValidateLLMGatewayConfig(cfg)` 强制以下字段在合并后非空：

- `cfg.DefaultProvider`
- `cfg.DefaultModel`
- `cfg.Providers[cfg.DefaultProvider].DefaultModel`

失败时返回 `ErrLLMConfigMissing`，并附用户配置路径 + env 提示：

```
llm_gateway config missing required fields: missing fields: [default_model].
set them in /Users/you/.devrix/config.yaml under llm_gateway:
(e.g. `default_model: <your-model>`), or via env vars
DEVRIX_LLM_DEFAULT_MODEL / DEVRIX_LLM_DEFAULT_PROVIDER
```

### 8.5 模型目录

公开模型 ID 与能力列在 `internal/layers/llmgateway/configure/data/models.yaml`，通过 `//go:embed` 嵌入到二进制中：

```go
catalog, _ := configure.DefaultCatalog()  // 加载嵌入目录
caps := catalog.Lookup("MiniMax-M3")
if caps != nil && caps.NativeThinking {
    // 该模型使用 provider-native 推理字段（delta.ReasoningContent 等），
    // SSE parser 跳过 inline <think> splitter。
}
```

完整可用模型：

- `minimax`: M3 / M3-highspeed / M2.7-highspeed / M2.7 / M2.5-highspeed
- `deepseek`: v4-latest / v4-flash / v4-pro
- `anthropic`: claude-sonnet-4 / claude-opus-4
- `openai`: gpt-5

私有 / 预览模型：复制 `data/models.yaml` 到 `~/.devrix/models.yaml` 并通过 `configure.LoadCatalogFromFile` 加载。

### 8.6 6 个 DEVRIX_LLM_* 环境变量

| 变量 | 写入位置 |
|-----|---------|
| `DEVRIX_LLM_DEFAULT_MODEL` | `llm_gateway.default_model` |
| `DEVRIX_LLM_DEFAULT_PROVIDER` | `llm_gateway.default_provider` |
| `DEVRIX_LLM_DEFAULT_TIER` | `llm_gateway.default_tier` |
| `DEVRIX_LLM_MODEL_FAST` | `llm_gateway.model_tiers.fast` |
| `DEVRIX_LLM_MODEL_DEFAULT` | `llm_gateway.model_tiers.default` |
| `DEVRIX_LLM_MODEL_POWERFUL` | `llm_gateway.model_tiers.powerful` |

任一被设置即把 `UserConfig.LLMGateway` 从 nil 切到非 nil，下游 deep-merge 即知"用户态有声明"。
