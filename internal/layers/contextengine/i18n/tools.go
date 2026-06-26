package i18n

import "strings"

type toolLocaleEntry struct {
	description string
	parameters  string
}

// LocalizeTool returns LLM-facing description and parameters for the given locale.
// English locale passes through the source strings; Chinese applies catalog overrides.
func LocalizeTool(name, description, parameters string, loc Locale) (string, string) {
	if loc != LocaleZH {
		return description, parameters
	}
	if e, ok := toolCatalogZH[name]; ok {
		desc := e.description
		if desc == "" {
			desc = description
		}
		params := e.parameters
		if params == "" {
			params = parameters
		}
		return desc, params
	}
	if strings.HasPrefix(name, "call_") {
		return "调用已注册的外部 Agent 工具 " + name + "。", parameters
	}
	return description, parameters
}

// toolCatalogZH maps tool names to Chinese LLM-facing schemas.
var toolCatalogZH = map[string]toolLocaleEntry{
	"bash": {
		description: "在会话 WorkDir 下执行 shell 命令（沙箱环境）。使用相对路径；读文件优先用 read_file/glob/list_dir。",
		parameters:  `{"type":"object","required":["command"],"properties":{"command":{"type":"string","description":"要执行的 shell 命令。优先相对路径；读文件用 read_file/glob/grep。"}}}`,
	},
	"read_file": {
		description: "读取工作区中的文件。参数示例：{\"path\":\"相对或绝对路径\"}",
		parameters:  `{"type":"object","required":["path"],"properties":{"path":{"type":"string","description":"文件路径（相对或绝对）"},"file_path":{"type":"string","description":"path 的别名"}}}`,
	},
	"write_file": {
		description: "将内容写入工作区文件。参数示例：{\"path\":\"...\",\"content\":\"...\"}",
		parameters:  `{"type":"object","required":["path","content"],"properties":{"path":{"type":"string","description":"目标文件路径"},"file_path":{"type":"string","description":"path 的别名"},"content":{"type":"string","description":"要写入的完整文件内容"}}}`,
	},
	"glob": {
		description: "按 glob 模式快速匹配文件。支持 \"**/*.js\"、\"src/**/*.ts\" 等。返回按修改时间排序的匹配路径。",
		parameters:  `{"type":"object","required":["pattern"],"properties":{"pattern":{"type":"string","description":"glob 模式，如 **/*.go"},"path":{"type":"string","description":"搜索根目录（可选，默认工作区）"}}}`,
	},
	"grep": {
		description: "基于正则的强大搜索工具。搜索文件内容，支持 content、files_with_matches、count 三种输出模式。",
		parameters:  `{"type":"object","required":["pattern"],"properties":{"pattern":{"type":"string","description":"正则表达式"},"path":{"type":"string","description":"搜索目录或文件"},"output_mode":{"type":"string","enum":["content","files_with_matches","count"],"description":"输出模式"},"-i":{"type":"boolean","description":"忽略大小写"},"head_limit":{"type":"integer","description":"限制输出条数"},"offset":{"type":"integer","description":"跳过前 N 条"},"-C":{"type":"integer","description":"匹配行上下文行数"},"glob":{"type":"string","description":"文件 glob 过滤，如 *.ts"}}}`,
	},
	"edit_file": {
		description: "通过精确文本替换编辑文件。用 old_string 匹配要替换的文本，new_string 为替换内容。 targeted 修改优先于 write_file。",
		parameters:  `{"type":"object","required":["file_path","old_string","new_string"],"properties":{"file_path":{"type":"string","description":"目标文件路径"},"old_string":{"type":"string","description":"要匹配的原文（须精确匹配）"},"new_string":{"type":"string","description":"替换后的文本"},"replace_all":{"type":"boolean","description":"是否替换所有匹配项"}}}`,
	},
	"todo_write": {
		description: "管理会话任务清单（全量快照替换）。跟踪 pending、in_progress、completed 状态。",
		parameters:  `{"type":"object","required":["todos"],"properties":{"todos":{"type":"array","description":"完整待办列表","items":{"type":"object","required":["content","status","activeForm"],"properties":{"content":{"type":"string","description":"任务描述（祈使句）"},"status":{"type":"string","enum":["pending","in_progress","completed"],"description":"任务状态"},"activeForm":{"type":"string","description":"进行时描述"}}}}}}`,
	},
	"enter_plan_mode": {
		description: "进入只读 plan 模式，在实现前探索并起草计划。",
		parameters:  `{"type":"object","properties":{"plan_file_path":{"type":"string","description":"计划文件路径（可选）"}}}`,
	},
	"exit_plan_mode": {
		description: "退出 plan 模式并请求用户批准开始实现。",
		parameters:  `{"type":"object","properties":{}}`,
	},
	"task_stop": {
		description: "按 task_id 取消运行中的后台 SubQuery（幂等；返回先前状态）。",
		parameters:  `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string","description":"async=true 时 delegate_* 或 RunBackground 返回的后台任务 ID"}}}`,
	},
	"task_output": {
		description: "读取后台 SubQuery 的状态/输出。block=true 时等待任务到达终态（最长 600s）。",
		parameters: `{"type":"object","required":["task_id"],"properties":{
			"task_id":{"type":"string","description":"后台任务 ID"},
			"block":{"type":"boolean","default":false,"description":"是否阻塞等待完成"},
			"timeout_ms":{"type":"integer","default":30000,"minimum":1,"maximum":600000,"description":"阻塞超时（毫秒）"}
		}}`,
	},
	"task_list_background": {
		description: "列出当前会话所有后台 SubQuery 任务（运行中 + 已结束）。bg_ 前缀为后台 SubQuery；task_ 前缀为 Plan 任务。",
		parameters:  `{"type":"object","properties":{}}`,
	},
	"tool_search": {
		description: "搜索延迟加载的工具目录。传入 query（子串或 glob，如 delegate_*）和可选 category。返回最多 5 个匹配工具 schema，之后可直接调用。",
		parameters:  `{"type":"object","required":["query"],"properties":{"query":{"type":"string","description":"搜索关键词或 glob"},"category":{"type":"string","description":"可选分类前缀"}}}`,
	},
	"query_diagnostics": {
		description: "查询周期性 linter tick 维护的最近文件诊断缓冲。返回最多 limit（默认 50）条诊断，含 file/line/severity/source/message。可用 file、severity 过滤。",
		parameters:  `{"type":"object","properties":{"limit":{"type":"integer","description":"最大返回条数，默认 50"},"file":{"type":"string","description":"按文件路径过滤"},"severity":{"type":"string","description":"按严重级别过滤"}}}`,
	},
	"verify_plan_execution": {
		description: "验证变更 tasks.md 中所有 done 项的证据文件是否存在；对 _test.go 检查是否含 func TestXxx()。返回 verified/unverified/skipped 计数的 Report JSON。",
		parameters:  `{"type":"object","properties":{"change_id":{"type":"string","description":"变更 ID（可选）"}}}`,
	},
	"free_fork": {
		description: "批量 fork N 个子 agent（1..5）于父会话下。默认每个子 agent 在隔离 worker 目录沙箱中运行。",
		parameters:  `{"type":"object","required":["count","directive"],"properties":{"count":{"type":"integer","minimum":1,"maximum":5,"description":"子 agent 数量"},"directive":{"type":"string","description":"给每个子 agent 的指令"}}}`,
	},
	"ask_user_question": {
		description: "向用户提出 1–4 个多选题。问题以 IM 消息发送并带编号选项；用户可回复数字（如「1」）或选项文字，回复在下一 turn 到达。仅在 genuinely 不确定时使用（如歧义需求、工具选择、设计权衡）。 trivial 澄清请直接用文字询问。",
		parameters: `{
			"type": "object",
			"required": ["questions"],
			"properties": {
				"questions": {
					"type": "array",
					"minItems": 1,
					"maxItems": 4,
					"description": "问题列表",
					"items": {
						"type": "object",
						"required": ["question", "options"],
						"properties": {
							"question": {"type": "string", "description": "问题文本，以问号结尾"},
							"header": {"type": "string", "maxLength": 12, "description": "短标签（最多 12 字）"},
							"options": {
								"type": "array",
								"minItems": 2,
								"maxItems": 4,
								"items": {
									"type": "object",
									"required": ["label", "description"],
									"properties": {
										"label": {"type": "string", "description": "1–5 词选项标签"},
										"description": {"type": "string", "description": "选项说明"}
									}
								}
							},
							"multi_select": {"type": "boolean", "default": false, "description": "是否允许多选"}
						}
					}
				}
			}
		}`,
	},
	"lsp": {
		description: "LSP 代码智能。操作：definition | references | incoming_calls",
	},
	"lsp_go_to_definition": {
		description: "LSP 代码智能：跳转到定义（只读）。",
		parameters: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string", "description": "源文件路径"},
    "line": {"type": "integer", "minimum": 1, "description": "1-based 行号"},
    "character": {"type": "integer", "minimum": 1, "description": "1-based 字符偏移"}
  }
}`,
	},
	"lsp_find_references": {
		description: "LSP 代码智能：查找引用（只读）。",
		parameters: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string", "description": "源文件路径"},
    "line": {"type": "integer", "minimum": 1, "description": "1-based 行号"},
    "character": {"type": "integer", "minimum": 1, "description": "1-based 字符偏移"},
    "include_declaration": {"type": "boolean", "default": true, "description": "是否包含声明处"}
  }
}`,
	},
	"lsp_incoming_calls": {
		description: "LSP 代码智能：入站调用（只读）。",
		parameters: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string", "description": "源文件路径"},
    "line": {"type": "integer", "minimum": 1, "description": "1-based 行号"},
    "character": {"type": "integer", "minimum": 1, "description": "1-based 字符偏移"}
  }
}`,
	},
	"lsp_hover": {
		description: "LSP 代码智能：悬停信息（只读）。",
		parameters: `{
  "type": "object",
  "required": ["file_path", "line", "character"],
  "properties": {
    "file_path": {"type": "string", "description": "源文件路径"},
    "line": {"type": "integer", "minimum": 1, "description": "1-based 行号"},
    "character": {"type": "integer", "minimum": 1, "description": "1-based 字符偏移"}
  }
}`,
	},
	"lsp_workspace_symbol": {
		description: "LSP 代码智能：工作区符号搜索（只读）。",
		parameters: `{
  "type": "object",
  "required": ["query"],
  "properties": {
    "query": {"type": "string", "description": "要搜索的符号名或子串"}
  }
}`,
	},
	"delegate_explore": {
		description: "启动只读 Explore worker 调查代码库（grep、read、list — 不写文件）。不要用于 trivial 单文件编辑或能从已知上下文直接回答的 Q&A。返回简洁摘要 — 范围大时 prefer async=true。Explore 后用 todo_write 或 delegate_plan。",
		parameters: delegateToolParametersZH(),
	},
	"delegate_plan": {
		description: "启动只读 Plan worker，基于调研上下文产出结构化实现计划。不要用于明显单行编辑；用户要立即改代码时不要只用 plan。返回阶段、文件、依赖和测试说明 — 然后拆成 todo_write 并逐项 delegate_implement。",
		parameters: delegateToolParametersZH(),
	},
	"delegate_implement": {
		description: "启动 Implement worker 执行一个 scoped 任务（创建/编辑文件、跑测试）。用于具体编码；每次调用传一个任务，directive 含路径和验收标准。不要打包无关功能；陌生区域先 explore。有 todo_write 的 task_id 时带上。改多文件或长测试 prefer async=true。",
		parameters: delegateToolParametersZH(),
	},
	"delegate_status": {
		description: "读取本会话的 WorkPlan 快照（运行中/已完成 worker、摘要、错误）。async delegate_* 之后用此轮询，不要盲目重复 delegate；用户问进度或下一 implement 依赖 prior worker 时也用此工具。",
		parameters:  `{"type":"object","properties":{}}`,
	},
	"task_create": {
		description: "创建持久化任务",
		parameters:  `{"type":"object","required":["subject"],"properties":{"subject":{"type":"string","description":"任务标题"},"description":{"type":"string","description":"任务描述"}}}`,
	},
	"task_get": {
		description: "按 ID 获取任务",
		parameters:  `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string","description":"任务 ID"}}}`,
	},
	"task_list": {
		description: "列出会话所有任务",
		parameters:  `{"type":"object","properties":{"format":{"type":"string","enum":["flat","tree"],"description":"列表格式"}}}`,
	},
	"task_update": {
		description: "更新任务",
		parameters:  `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string","description":"任务 ID"},"status":{"type":"string","description":"新状态"},"owner":{"type":"string","description":"负责人"},"blocked_by":{"type":"string","description":"阻塞原因或依赖"}}}`,
	},
}

func delegateToolParametersZH() string {
	return `{"type":"object","required":["directive"],"properties":{"directive":{"type":"string","description":"给 worker 的清晰、自包含指令（目标、范围、文件/模块、期望输出）"},"task_id":{"type":"string","description":"可选 TaskManager ID；省略则根据 directive 自动创建"},"sandbox_slug":{"type":"string","description":"可选隔离 worker 目录 slug，用于并行 implement"},"worktree_slug":{"type":"string","description":"sandbox_slug 的废弃别名"},"async":{"type":"boolean","description":"为 true 时立即返回，用 delegate_status 轮询进度。Explore/plan 耗时长时 prefer async"},"mode":{"type":"string","enum":["brief","fork","full"],"default":"brief","description":"子 agent 上下文继承：brief=无父历史（默认）；fork=缓存友好前缀；full=完整父历史（legacy）"}}}`
}
