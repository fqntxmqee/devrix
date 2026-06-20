// Package errorclass — D3 llmgateway 错误分类引擎。
//
// 对标 clawcode 错误分类（散落于 utils/errors）+ devrix 既有 sentinel。
// 三层匹配：(a) errors.Is 检测既有 sentinel → (b) HTTP 状态码 → (c) 正则匹配 provider 错误 body。
//
// 用于 retry / circuit_breaker / observability 三个位置对 LLM 调用失败进行归因，
// 输出 Class（20+ 类别）+ Retry 标记 + UserHint + Detail。
package errorclass

import (
	"context"
	stderrors "errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Class 错误分类标签。
type Class string

const (
	ClassRateLimit       Class = "rate_limit"
	ClassQuota           Class = "quota_exceeded"
	ClassAuth            Class = "auth_failed"
	ClassPermission      Class = "permission_denied"
	ClassNotFound        Class = "model_not_found"
	ClassInvalidRequest  Class = "invalid_request"
	ClassContextOverflow Class = "context_overflow"
	ClassTimeout         Class = "timeout"
	ClassNetwork         Class = "network"
	ClassUpstreamDown    Class = "upstream_unavailable"
	ClassOverloaded      Class = "overloaded"
	ClassContentFilter   Class = "content_filter"
	ClassToolUseError    Class = "tool_use_error"
	ClassPromptTooLong   Class = "prompt_too_long"
	ClassResponseTooLong Class = "response_too_long"
	ClassStreamError     Class = "stream_error"
	ClassParseError      Class = "parse_error"
	ClassCircuitOpen     Class = "circuit_open"
	ClassBudgetExceeded  Class = "budget_exceeded"
	ClassCancelled       Class = "cancelled"
	ClassUnknown         Class = "unknown"
)

// AllClasses 列出全部 20+ 分类常量，便于测试 + 文档。
func AllClasses() []Class {
	return []Class{
		ClassRateLimit, ClassQuota, ClassAuth, ClassPermission, ClassNotFound,
		ClassInvalidRequest, ClassContextOverflow, ClassTimeout, ClassNetwork,
		ClassUpstreamDown, ClassOverloaded, ClassContentFilter, ClassToolUseError,
		ClassPromptTooLong, ClassResponseTooLong, ClassStreamError, ClassParseError,
		ClassCircuitOpen, ClassBudgetExceeded, ClassCancelled, ClassUnknown,
	}
}

// Classification 错误分类结果。
type Classification struct {
	Class    Class
	Retry    bool
	UserHint string
	Detail   string
}

// Classifier 错误分类器接口。
type Classifier interface {
	Classify(err error, httpStatus int, raw string) Classification
}

// regexRule 一个正则匹配规则。
type regexRule struct {
	pattern  *regexp.Regexp
	class    Class
	retry    bool
	hint     string
}

// DefaultClassifier 默认分类器。
type DefaultClassifier struct {
	sentinelSentinels []sentinelMatch
	rules             []regexRule
}

// NewDefaultClassifier 构造默认分类器。
func NewDefaultClassifier() *DefaultClassifier {
	c := &DefaultClassifier{}
	c.registerSentinels()
	c.registerRegexRules()
	return c
}

func (c *DefaultClassifier) registerSentinels() {
	// 通过 wrap 后的 *SentinelError 标识 + SentinelError.Code 双通道映射
	c.sentinelSentinels = append(c.sentinelSentinels, sentinelMatch{
		target: sharederrors.ErrLLMTimeout,
		llmCode: sharederrors.CodeLLMTimeout,
		class:  ClassTimeout,
		retry:  true,
		hint:   "LLM 调用超时，可重试",
	})
	c.sentinelSentinels = append(c.sentinelSentinels, sentinelMatch{
		target: sharederrors.ErrProviderUnavailable,
		llmCode: sharederrors.CodeLLMProviderUnavailable,
		class:  ClassUpstreamDown,
		retry:  true,
		hint:   "上游 provider 暂不可用，可重试",
	})
	c.sentinelSentinels = append(c.sentinelSentinels, sentinelMatch{
		target: sharederrors.ErrCircuitOpen,
		llmCode: sharederrors.CodeLLMCircuitOpen,
		class:  ClassCircuitOpen,
		retry:  false,
		hint:   "熔断器开启，等待冷却后重试",
	})
	c.sentinelSentinels = append(c.sentinelSentinels, sentinelMatch{
		target: sharederrors.ErrLLMAuthFailed,
		llmCode: sharederrors.CodeLLMAuthFailed,
		class:  ClassAuth,
		retry:  false,
		hint:   "API key 鉴权失败，请检查配置",
	})
	c.sentinelSentinels = append(c.sentinelSentinels, sentinelMatch{
		target: sharederrors.ErrTokenBudgetExceeded,
		llmCode: sharederrors.CodeLLMTokenBudgetExceeded,
		class:  ClassBudgetExceeded,
		retry:  false,
		hint:   "已超出 token 预算上限",
	})
	c.sentinelSentinels = append(c.sentinelSentinels, sentinelMatch{
		target: sharederrors.ErrLLMParseError,
		llmCode: sharederrors.CodeLLMParseError,
		class:  ClassParseError,
		retry:  true,
		hint:   "上游响应解析失败，可重试",
	})
}

func (c *DefaultClassifier) registerRegexRules() {
	type ruleSpec struct {
		pattern string
		class   Class
		retry   bool
		hint    string
	}
	specs := []ruleSpec{
		// rate limit
		{`(?i)rate[_ ]?limit`, ClassRateLimit, true, "触发 provider 限流，指数退避后重试"},
		{`(?i)too many requests`, ClassRateLimit, true, "触发 provider 限流，指数退避后重试"},
		{`(?i)\b429\b`, ClassRateLimit, true, "HTTP 429 限流"},
		// quota / billing
		{`(?i)quota`, ClassQuota, false, "账户配额用完，请充值或更换 key"},
		{`(?i)billing`, ClassQuota, false, "账户账单问题，请检查 billing"},
		{`(?i)insufficient[_ ]?(?:quota|credit|balance)`, ClassQuota, false, "账户余额不足"},
		// auth
		{`(?i)invalid[_ ]?(?:api[_ ]?)?key`, ClassAuth, false, "API key 无效"},
		{`(?i)unauthorized`, ClassAuth, false, "未授权（401）"},
		{`(?i)\b401\b`, ClassAuth, false, "HTTP 401 未授权"},
		// model
		{`(?i)model[_ ]?not[_ ]?found`, ClassNotFound, false, "模型不存在"},
		{`(?i)the model .{0,64} does not exist`, ClassNotFound, false, "模型不存在"},
		// invalid request
		{`(?i)invalid[_ ]?request`, ClassInvalidRequest, false, "请求参数无效"},
		{`(?i)bad[_ ]?request`, ClassInvalidRequest, false, "HTTP 400 请求格式错误"},
		{`(?i)\b400\b`, ClassInvalidRequest, false, "HTTP 400 请求格式错误"},
		// context overflow
		{`(?i)context[_ ]?(?:length|window|overflow)`, ClassContextOverflow, false, "上下文长度超限"},
		{`(?i)maximum[_ ]?context`, ClassContextOverflow, false, "上下文长度超限"},
		{`(?i)prompt[_ ]?is[_ ]?too[_ ]?long`, ClassPromptTooLong, false, "prompt 过长"},
		{`(?i)reduce the length`, ClassContextOverflow, false, "上下文过长，需裁剪"},
		// overloaded
		{`(?i)overloaded`, ClassOverloaded, true, "上游过载，可重试"},
		{`(?i)\b503\b`, ClassUpstreamDown, true, "上游 503 不可用"},
		{`(?i)service[_ ]?unavailable`, ClassUpstreamDown, true, "上游服务不可用"},
		{`(?i)upstream[_ ]?unavailable`, ClassUpstreamDown, true, "上游不可用"},
		// content filter
		{`(?i)content[_ ]?(?:filter|moderation|policy)`, ClassContentFilter, false, "内容被策略拦截"},
		{`(?i)safety`, ClassContentFilter, false, "内容安全策略拦截"},
		// tool use
		{`(?i)tool[_ ]?use[_ ]?error`, ClassToolUseError, false, "工具调用错误"},
		{`(?i)invalid[_ ]?tool`, ClassToolUseError, false, "工具调用非法"},
		// network (must come before stream_error so EOF → Network wins)
		{`(?i)network[_ ]?(?:error|unreachable)`, ClassNetwork, true, "网络错误，可重试"},
		{`(?i)connection[_ ]?(?:reset|refused|timeout)`, ClassNetwork, true, "网络连接异常，可重试"},
		{`(?i)no[_ ]?such[_ ]?host`, ClassNetwork, true, "DNS 解析失败，可重试"},
		{`(?i)eof`, ClassNetwork, true, "连接意外关闭，可重试"},
		// stream
		{`(?i)stream[_ ]?(?:error|interrupted|closed)`, ClassStreamError, true, "流中断，可重试"},
		{`(?i)unexpected[_ ]?eof`, ClassStreamError, true, "流意外结束"},
		// response too long
		{`(?i)response[_ ]?(?:too[_ ]?long|max[_ ]?tokens)`, ClassResponseTooLong, false, "响应超长"},
		{`(?i)max[_ ]?output[_ ]?tokens`, ClassResponseTooLong, false, "响应超长"},
		// cancelled
		{`(?i)canceled|cancelled`, ClassCancelled, false, "已取消"},
		{`(?i)context[_ ]?canceled`, ClassCancelled, false, "context 已取消"},
	}
	for _, s := range specs {
		c.rules = append(c.rules, regexRule{
			pattern: regexp.MustCompile(s.pattern),
			class:   s.class,
			retry:   s.retry,
			hint:    s.hint,
		})
	}
}

// Classify 三层匹配：错误 sentinel → HTTP 状态码 → 正则 body。
func (c *DefaultClassifier) Classify(err error, httpStatus int, raw string) Classification {
	// Layer 1: 既有 sentinel (errors.Is + SentinelError.Code 双通道)
	if err != nil {
		for _, s := range c.sentinelSentinels {
			if stderrors.Is(err, s.target) {
				return Classification{
					Class:    s.class,
					Retry:    s.retry,
					UserHint: s.hint,
					Detail:   trimDetail(raw, err.Error()),
				}
			}
		}
		// SentinelError.Code 直接映射（devrix llmgateway 包装的错误）
		var llmErr *sharederrors.SentinelError
		if stderrors.As(err, &llmErr) {
			for _, s := range c.sentinelSentinels {
				if s.llmCode == llmErr.Code {
					return Classification{
						Class:    s.class,
						Retry:    s.retry,
						UserHint: s.hint,
						Detail:   trimDetail(raw, err.Error()),
					}
				}
			}
		}
	}

	// Layer 2: HTTP 状态码
	if cls, ok := classByHTTPStatus(httpStatus); ok {
		return Classification{
			Class:    cls.class,
			Retry:    cls.retry,
			UserHint: cls.hint,
			Detail:   trimDetail(raw, httpStatusText(httpStatus)),
		}
	}

	// Layer 3: 正则匹配
	haystack := raw
	if haystack == "" && err != nil {
		haystack = err.Error()
	}
	for _, r := range c.rules {
		if r.pattern.MatchString(haystack) {
			return Classification{
				Class:    r.class,
				Retry:    r.retry,
				UserHint: r.hint,
				Detail:   trimDetail(raw, haystack),
			}
		}
	}

	// 兜底
	detail := raw
	if detail == "" && err != nil {
		detail = err.Error()
	}
	return Classification{
		Class:    ClassUnknown,
		Retry:    false,
		UserHint: "未分类错误",
		Detail:   trim(detail, 256),
	}
}

// sentinelMatch 单个 sentinel → class 映射。
type sentinelMatch struct {
	target error
	llmCode string // 可选：匹配 *LLMError.Code
	class  Class
	retry  bool
	hint   string
}

func (c *DefaultClassifier) sentinelMatches() []sentinelMatch {
	return c.sentinelSentinels
}

type httpRule struct {
	class Class
	retry bool
	hint  string
}

func classByHTTPStatus(status int) (httpRule, bool) {
	switch status {
	case 0:
		return httpRule{}, false
	case 400:
		return httpRule{ClassInvalidRequest, false, "HTTP 400 请求格式错误"}, true
	case 401:
		return httpRule{ClassAuth, false, "HTTP 401 未授权"}, true
	case 403:
		return httpRule{ClassPermission, false, "HTTP 403 权限被拒"}, true
	case 404:
		return httpRule{ClassNotFound, false, "HTTP 404 资源不存在"}, true
	case 408:
		return httpRule{ClassTimeout, true, "HTTP 408 请求超时"}, true
	case 413:
		return httpRule{ClassPromptTooLong, false, "HTTP 413 请求体过大"}, true
	case 429:
		return httpRule{ClassRateLimit, true, "HTTP 429 限流"}, true
	case 500:
		return httpRule{ClassUpstreamDown, true, "HTTP 500 上游错误"}, true
	case 502, 504:
		return httpRule{ClassUpstreamDown, true, "HTTP 网关错误"}, true
	case 503:
		return httpRule{ClassOverloaded, true, "HTTP 503 上游过载"}, true
	case 509:
		return httpRule{ClassQuota, false, "HTTP 509 配额超限"}, true
	default:
		if status >= 500 {
			return httpRule{ClassUpstreamDown, true, fmt.Sprintf("HTTP %d 上游错误", status)}, true
		}
		return httpRule{}, false
	}
}

func httpStatusText(status int) string {
	if status == 0 {
		return ""
	}
	return strconv.Itoa(status)
}

func trim(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func trimDetail(raw, fallback string) string {
	if raw != "" {
		return trim(raw, 256)
	}
	return trim(fallback, 256)
}

// WithClassification 将分类结果挂到 ctx（key: classificationKey）。
// 其他模块可通过 FromContext 读取。
type ctxKey struct{ name string }

var classificationKey = ctxKey{"errorclass"}

// InjectClassification 在 ctx 上附加 classification。
func InjectClassification(ctx context.Context, c Classification) context.Context {
	return context.WithValue(ctx, classificationKey, c)
}

// FromContext 读取 classification，未注入时返回 zero value。
func FromContext(ctx context.Context) (Classification, bool) {
	v, ok := ctx.Value(classificationKey).(Classification)
	return v, ok
}
