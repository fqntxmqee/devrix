package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// UserConfig represents user-specific configuration.
//
// Most fields are "user identity" / "UI preference" data that does not
// influence server-side routing. The LLMGateway field is the user-level
// override for the project-level llm_gateway section — see
// internal/layers/llmgateway/configure for the merge rules.
type UserConfig struct {
	User      UserInfoConfig         `yaml:"user"`
	UI        UIConfig               `yaml:"ui"`
	Model     ModelConfig            `yaml:"model"`
	Shortcuts ShortcutsConfig        `yaml:"shortcuts"`
	Plugins   PluginsConfig          `yaml:"plugins"`
	Privacy   PrivacyConfig          `yaml:"privacy"`
	YOLO      YOLOConfig             `yaml:"yolo"`   // YOLO mode - permission auto-approve
	IM        IMConfig               `yaml:"im"`     // IM 平台配置
	// LLMGateway 是用户态对项目级 llm_gateway 的覆盖层（深合并）。
	// nil 表示用户未声明任何 LLM gateway 字段，将完全沿用项目配置 / 编译期默认值。
	// 通过 DEVRIX_LLM_* 环境变量或 ~/.devrix/config.yaml 的 llm_gateway 段设置。
	LLMGateway *LLMGatewayFileConfig `yaml:"llm_gateway,omitempty"`
}

// IMConfig IM 平台配置
type IMConfig struct {
	Enabled  bool                    `yaml:"enabled"`  // 是否启用 IM
	Engine   string                  `yaml:"engine"`   // context | stub，默认 context（真实 LLM）
	Platform IMPlatformConfig        `yaml:"platform"` // 当前选中的平台
	Feishu  FeishuUserConfig        `yaml:"feishu"`  // 飞书配置
	DingTalk DingTalkUserConfig     `yaml:"dingtalk"` // 钉钉配置
}

// IMPlatformConfig 当前使用的 IM 平台
type IMPlatformConfig struct {
	Provider string `yaml:"provider"` // feishu | dingtalk | none
}

// FeishuUserConfig 飞书用户配置 (当 im.platform="feishu" 时生效)
type FeishuUserConfig struct {
	AppID         string `yaml:"app_id"`
	AppSecret     string `yaml:"app_secret"`
	BotName       string `yaml:"bot_name"`
	Domain        string `yaml:"domain"`         // 自定义域名
	EncryptKey    string `yaml:"encrypt_key"`    // 回调加密密钥
	CallbackURL   string `yaml:"callback_url"`   // 回调地址
	UseWebhook    bool   `yaml:"use_webhook"`    // 使用 Webhook 模式
	ReactionEmoji string `yaml:"reaction_emoji"` // 收到消息时的表情回复，默认 OnIt；设为 none 禁用
	DoneEmoji     string `yaml:"done_emoji"`     // agent 完成时的表情回复，如 Done；设为 none 禁用
	ReplyInThread *bool  `yaml:"reply_in_thread"` // 在用户消息下以话题回复，默认 true
	ProgressStyle string `yaml:"progress_style"` // legacy | compact | card | structured，默认 structured
	ShowToolResults *bool `yaml:"show_tool_results"` // 是否在 IM 展示工具执行结果，默认 false
	Streaming       FeishuStreamingUserConfig `yaml:"streaming"`
}

// FeishuStreamingUserConfig controls cardkit element-level reply streaming.
type FeishuStreamingUserConfig struct {
	Enabled       *bool `yaml:"enabled"`         // 默认 true
	IntervalMs    int   `yaml:"interval_ms"`     // 流式 PUT 最小间隔，默认 400
	MinDeltaChars int   `yaml:"min_delta_chars"` // 最小字符增量，默认 24
}

// IsStreamingEnabled reports whether cardkit streaming is enabled for reply cards.
func (f FeishuUserConfig) IsStreamingEnabled() bool {
	if f.Streaming.Enabled == nil {
		return true
	}
	return *f.Streaming.Enabled
}

// ResolveFeishuStreamingConfig maps user YAML to adapter config with defaults.
func (f FeishuUserConfig) ResolveFeishuStreamingConfig() (enabled bool, intervalMs, minDeltaChars int) {
	enabled = f.IsStreamingEnabled()
	intervalMs = f.Streaming.IntervalMs
	if intervalMs <= 0 {
		intervalMs = 400
	}
	minDeltaChars = f.Streaming.MinDeltaChars
	if minDeltaChars <= 0 {
		minDeltaChars = 24
	}
	return enabled, intervalMs, minDeltaChars
}

// IsShowToolResults reports whether tool_result content is pushed to IM.
func (f FeishuUserConfig) IsShowToolResults() bool {
	if f.ShowToolResults == nil {
		return false
	}
	return *f.ShowToolResults
}

// IsReplyInThread returns whether bot replies should appear in a thread under the user's message.
func (f FeishuUserConfig) IsReplyInThread() bool {
	if f.ReplyInThread == nil {
		// Match cc-connect default: Reply API with quote header; Feishu still shows "N 条回复".
		return false
	}
	return *f.ReplyInThread
}

// DingTalkUserConfig 钉钉用户配置 (当 im.platform="dingtalk" 时生效)
type DingTalkUserConfig struct {
	AppKey     string `yaml:"app_key"`
	AppSecret  string `yaml:"app_secret"`
	BotCode    string `yaml:"bot_code"`    // 机器人 code
	CallbackURL string `yaml:"callback_url"` // 回调地址
	EncryptKey string `yaml:"encrypt_key"`  // 加密密钥
	UseWebhook bool   `yaml:"use_webhook"`  // 使用 Webhook 模式
}

// UserInfoConfig 用户信息
type UserInfoConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// UIConfig 界面偏好
type UIConfig struct {
	Theme       string `yaml:"theme"`        // auto | light | dark
	Language    string `yaml:"language"`     // zh-CN | en-US
	Emoji       bool   `yaml:"emoji"`        // 是否使用 emoji
	ColorOutput bool   `yaml:"color_output"` // 彩色输出
}

// ModelConfig AI 模型配置
type ModelConfig struct {
	Provider string `yaml:"provider"` // openai | anthropic | local
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"` // 自定义 API 地址
}

// ShortcutsConfig 快捷键配置
type ShortcutsConfig struct {
	NewSession string `yaml:"new_session"` // Ctrl+N
	Stop       string `yaml:"stop"`       // Ctrl+C
	Help       string `yaml:"help"`       // Ctrl+H
}

// PluginsConfig 插件配置
type PluginsConfig struct {
	Enabled    bool     `yaml:"enabled"`
	AutoUpdate bool     `yaml:"auto_update"`
	List       []string `yaml:"list"`
}

// PrivacyConfig 隐私配置
type PrivacyConfig struct {
	Telemetry   bool `yaml:"telemetry"`    // 是否发送使用统计
	SaveHistory bool `yaml:"save_history"` // 保存聊天历史
}

// YOLOConfig YOLO 模式配置 - 默认全部授权
type YOLOConfig struct {
	Enabled           bool   `yaml:"enabled"`            // 启用 YOLO 模式
	AutoApproveTools  bool   `yaml:"auto_approve_tools"` // 自动批准工具执行
	AutoApproveFiles  bool   `yaml:"auto_approve_files"` // 自动批准文件操作
	AutoApproveNetwork bool  `yaml:"auto_approve_network"` // 自动批准网络请求
	ConfirmBeforeExec  bool   `yaml:"confirm_before_exec"` // 执行前确认（非 YOLO 时生效）
	TrustPlugins      bool   `yaml:"trust_plugins"`      // 信任插件
}

// DefaultUserConfig 返回默认用户配置
func DefaultUserConfig() *UserConfig {
	return &UserConfig{
		User: UserInfoConfig{
			Name:  "",
			Email: "",
		},
		UI: UIConfig{
			Theme:       "auto",
			Language:    "zh-CN",
			Emoji:       true,
			ColorOutput: true,
		},
		Model: ModelConfig{
			Provider: "openai",
			Model:    "gpt-4",
			APIKey:   "",
			BaseURL:  "",
		},
		Shortcuts: ShortcutsConfig{
			NewSession: "Ctrl+N",
			Stop:       "Ctrl+C",
			Help:       "Ctrl+H",
		},
		Plugins: PluginsConfig{
			Enabled:    true,
			AutoUpdate: true,
			List:       []string{},
		},
		Privacy: PrivacyConfig{
			Telemetry:   false,
			SaveHistory: true,
		},
		YOLO: YOLOConfig{
			Enabled:           false,
			AutoApproveTools:  false,
			AutoApproveFiles:  false,
			AutoApproveNetwork: false,
			ConfirmBeforeExec: true,
			TrustPlugins:      false,
		},
		IM: IMConfig{
			Enabled: false,
			Platform: IMPlatformConfig{
				Provider: "none",
			},
			Feishu: FeishuUserConfig{
				AppID:     "",
				AppSecret: "",
				BotName:   "Devrix",
			},
			DingTalk: DingTalkUserConfig{
				AppKey:   "",
				AppSecret: "",
				BotCode:  "",
			},
		},
	}
}

// UserConfigDir 返回用户配置目录路径
func UserConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".devrix"
	}
	return filepath.Join(home, ".devrix")
}

// EnsureUserConfigDir 确保用户配置目录存在
func EnsureUserConfigDir() error {
	dir := UserConfigDir()
	return os.MkdirAll(dir, 0755)
}

// UserConfigPath 返回用户配置文件的路径
func UserConfigPath() string {
	return filepath.Join(UserConfigDir(), "config.yaml")
}

// LoadUserConfig 加载用户配置
func LoadUserConfig() (*UserConfig, error) {
	cfg := DefaultUserConfig()

	// 应用环境变量覆盖（先应用，确保默认值可被环境变量覆盖）
	applyUserEnvOverrides(cfg)

	path := UserConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，使用环境变量覆盖后的默认配置
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read user config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	// 文件配置存在时，再次应用环境变量覆盖（环境变量优先）
	applyUserEnvOverrides(cfg)

	return cfg, nil
}

// applyUserEnvOverrides 应用环境变量覆盖
func applyUserEnvOverrides(cfg *UserConfig) {
	if val := os.Getenv("DEVRIX_USER_NAME"); val != "" {
		cfg.User.Name = val
	}
	if val := os.Getenv("DEVRIX_USER_EMAIL"); val != "" {
		cfg.User.Email = val
	}
	if val := os.Getenv("DEVRIX_MODEL_PROVIDER"); val != "" {
		cfg.Model.Provider = val
	}
	if val := os.Getenv("DEVRIX_MODEL_NAME"); val != "" {
		cfg.Model.Model = val
	}
	if val := os.Getenv("DEVRIX_MODEL_API_KEY"); val != "" {
		cfg.Model.APIKey = val
	}
	if val := os.Getenv("DEVRIX_MODEL_BASE_URL"); val != "" {
		cfg.Model.BaseURL = val
	}
	if val := os.Getenv("DEVRIX_YOLO_MODE"); val == "true" || val == "1" {
		cfg.YOLO.Enabled = true
		cfg.YOLO.AutoApproveTools = true
		cfg.YOLO.AutoApproveFiles = true
		cfg.YOLO.AutoApproveNetwork = true
	}
	if val := os.Getenv("DEVRIX_UI_THEME"); val != "" {
		cfg.UI.Theme = val
	}
	if val := os.Getenv("DEVRIX_UI_LANGUAGE"); val != "" {
		cfg.UI.Language = val
	}

	// IM 配置环境变量覆盖
	if val := os.Getenv("DEVRIX_IM_ENABLED"); val != "" {
		cfg.IM.Enabled = val == "true" || val == "1"
	}
	if val := os.Getenv("DEVRIX_IM_PROVIDER"); val != "" {
		cfg.IM.Platform.Provider = val
	}
	if val := os.Getenv("DEVRIX_ENGINE"); val != "" {
		cfg.IM.Engine = val
	}

	// 飞书配置
	if val := os.Getenv("FEISHU_APP_ID"); val != "" {
		cfg.IM.Feishu.AppID = val
		cfg.IM.Platform.Provider = "feishu"
	}
	if val := os.Getenv("FEISHU_APP_SECRET"); val != "" {
		cfg.IM.Feishu.AppSecret = val
	}
	if val := os.Getenv("FEISHU_BOT_NAME"); val != "" {
		cfg.IM.Feishu.BotName = val
	}

	// 钉钉配置
	if val := os.Getenv("DINGTALK_APP_KEY"); val != "" {
		cfg.IM.DingTalk.AppKey = val
		cfg.IM.Platform.Provider = "dingtalk"
	}
	if val := os.Getenv("DINGTALK_APP_SECRET"); val != "" {
		cfg.IM.DingTalk.AppSecret = val
	}
	if val := os.Getenv("DINGTALK_BOT_CODE"); val != "" {
		cfg.IM.DingTalk.BotCode = val
	}

	// LLM gateway 覆盖层。任意一个 DEVRIX_LLM_* 被设置都会让 cfg.LLMGateway
	// 从 nil 变为非 nil，使得下游 deep-merge 知道"用户态有声明"并启动合并。
	// 单变量映射见 applyLLMEnvOverrides 函数注释。
	applyLLMEnvOverrides(cfg)
}

// applyLLMEnvOverrides 将 6 个 DEVRIX_LLM_* 环境变量写入 cfg.LLMGateway。
//
// 语义：
//   - 任一 env 被设置 → 整个 LLMGateway 覆盖层被视为"存在"（即使只设了一个字段）
//   - 字段语义与项目级 llm_gateway.yaml 完全一致（default_* / model_tiers.*）
//   - 优先级：env > ~/.devrix/config.yaml.llm_gateway > devrix.yaml.llm_gateway > 编译期默认
//
// 详细 env 列表：
//
//	DEVRIX_LLM_DEFAULT_MODEL    → llm_gateway.default_model
//	DEVRIX_LLM_DEFAULT_PROVIDER → llm_gateway.default_provider
//	DEVRIX_LLM_DEFAULT_TIER     → llm_gateway.default_tier
//	DEVRIX_LLM_MODEL_FAST       → llm_gateway.model_tiers.fast
//	DEVRIX_LLM_MODEL_DEFAULT    → llm_gateway.model_tiers.default
//	DEVRIX_LLM_MODEL_POWERFUL   → llm_gateway.model_tiers.powerful
func applyLLMEnvOverrides(cfg *UserConfig) {
	getOrInit := func() *LLMGatewayFileConfig {
		if cfg.LLMGateway == nil {
			cfg.LLMGateway = &LLMGatewayFileConfig{}
		}
		return cfg.LLMGateway
	}
	if val := os.Getenv("DEVRIX_LLM_DEFAULT_MODEL"); val != "" {
		getOrInit().DefaultModel = val
	}
	if val := os.Getenv("DEVRIX_LLM_DEFAULT_PROVIDER"); val != "" {
		getOrInit().DefaultProvider = val
	}
	if val := os.Getenv("DEVRIX_LLM_DEFAULT_TIER"); val != "" {
		getOrInit().DefaultTier = val
	}
	if val := os.Getenv("DEVRIX_LLM_MODEL_FAST"); val != "" {
		g := getOrInit()
		if g.ModelTiers == nil {
			g.ModelTiers = map[string]string{}
		}
		g.ModelTiers["fast"] = val
	}
	if val := os.Getenv("DEVRIX_LLM_MODEL_DEFAULT"); val != "" {
		g := getOrInit()
		if g.ModelTiers == nil {
			g.ModelTiers = map[string]string{}
		}
		g.ModelTiers["default"] = val
	}
	if val := os.Getenv("DEVRIX_LLM_MODEL_POWERFUL"); val != "" {
		g := getOrInit()
		if g.ModelTiers == nil {
			g.ModelTiers = map[string]string{}
		}
		g.ModelTiers["powerful"] = val
	}
}

// SaveUserConfig 保存用户配置
func SaveUserConfig(cfg *UserConfig) error {
	if err := EnsureUserConfigDir(); err != nil {
		return fmt.Errorf("failed to create user config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal user config: %w", err)
	}

	if err := os.WriteFile(UserConfigPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write user config: %w", err)
	}

	return nil
}

// IsYOLOMode 检查是否启用 YOLO 模式
func (c *UserConfig) IsYOLOMode() bool {
	return c.YOLO.Enabled
}

// ShouldAutoApproveTool 检查是否应自动批准工具执行
func (c *UserConfig) ShouldAutoApproveTool() bool {
	return c.YOLO.Enabled && c.YOLO.AutoApproveTools
}

// ShouldAutoApproveFile 检查是否应自动批准文件操作
func (c *UserConfig) ShouldAutoApproveFile() bool {
	return c.YOLO.Enabled && c.YOLO.AutoApproveFiles
}

// ShouldAutoApproveNetwork 检查是否应自动批准网络请求
func (c *UserConfig) ShouldAutoApproveNetwork() bool {
	return c.YOLO.Enabled && c.YOLO.AutoApproveNetwork
}

// ResolveContextEngine returns the context engine name when IM is enabled.
// Priority: DEVRIX_ENGINE env > im.engine config > "context" (real LLM).
func ResolveContextEngine(im IMConfig) string {
	if val := strings.ToLower(strings.TrimSpace(os.Getenv("DEVRIX_ENGINE"))); val != "" {
		return val
	}
	if val := strings.ToLower(strings.TrimSpace(im.Engine)); val != "" {
		return val
	}
	return "context"
}

