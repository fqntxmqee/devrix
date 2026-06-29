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

// regexRule 一个正则匹配规则。详见 regex_rules.go。
//
// DM-20260629-003 PR-3 (#1 god-fn-split pt2): the struct moved to
// regex_rules.go alongside the rule specs + compile helper.

// DefaultClassifier 默认分类器。
type DefaultClassifier struct {
	sentinelSentinels []sentinelMatch
	rules             []regexRule
}

// NewDefaultClassifier 构造默认分类器。
func NewDefaultClassifier() *DefaultClassifier {
	c := &DefaultClassifier{}
	c.registerSentinels()
	c.rules = compileRegexRules(allRegexRuleSpecs())
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
