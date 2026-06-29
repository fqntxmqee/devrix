package errorclass

import "regexp"

// regexRule 一个正则匹配规则。
//
// DM-20260629-003 PR-3 (#1 god-fn-split pt2): moved from classifier.go
// (was registerRegexRules inline). The 30+ regex specs are now in their
// own file so the classifier core stays focused on the Classify()
// 3-layer match logic (sentinel → HTTP → regex).
type regexRule struct {
	pattern *regexp.Regexp
	class   Class
	retry   bool
	hint    string
}

type regexRuleSpec struct {
	pattern string
	class   Class
	retry   bool
	hint    string
}

// allRegexRuleSpecs returns the canonical regex rule set used by
// DefaultClassifier. Order matters: e.g. network_* rules must come
// before stream_error so EOF → Network wins over stream_error.
func allRegexRuleSpecs() []regexRuleSpec {
	return []regexRuleSpec{
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
}

// compileRegexRules builds the regexRule slice from the spec list.
// Centralizes the compile step so callers don't reach into regexp
// internals; tests can use allRegexRuleSpecs directly for dry-run
// pattern validation.
func compileRegexRules(specs []regexRuleSpec) []regexRule {
	rules := make([]regexRule, 0, len(specs))
	for _, s := range specs {
		rules = append(rules, regexRule{
			pattern: regexp.MustCompile(s.pattern),
			class:   s.class,
			retry:   s.retry,
			hint:    s.hint,
		})
	}
	return rules
}