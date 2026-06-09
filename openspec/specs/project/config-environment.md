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

### 2.3 .devrix/config.yaml — 用户级配置

- 用户本地偏好（主题、语言、IM 平台选择）
- 示例文件：`.devrix/config.example.yaml`

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
# LLM 网关
DEVRX_LLM_API_KEY=
DEVRX_LLM_BASE_URL=

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
