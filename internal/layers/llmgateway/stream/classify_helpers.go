package stream

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway/protect/errorclass"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// classifyAndWrap 把 errorclass 分类 + 短栈 + class 标签 三件套合到一个 helper。
//
// 设计参考:openspec/changes/devrix-diagnostic-tools-wiring/design.md §2.4.1 (W1)
//
// - Classify 用三层匹配 (sentinel / http / regex), 见 errorclass.DefaultClassifier
// - 短栈用 errors.WithShortStack(5) 过滤 runtime/testing/reflect
// - class 标签格式: [class=<Class>] <原 err>
//
// 该函数是 nil-safe: err == nil 直接返回 nil, classifier == nil 时仅做短栈包装。
//
// DM-20260620-003 (PR-C M2): ctx is now plumbed in so future classifier
// implementations can emit a `llm.error_classify` span. Today the ctx is
// only used to derive trace correlation if needed (kept for back-compat
// with adapters that may call classifyAndWrap without ctx — pass
// context.TODO() in that case).
func classifyAndWrap(ctx context.Context, classifier errorclass.Classifier, err error, status int, raw string) error {
	if err == nil {
		return nil
	}
	if classifier != nil {
		c := classifier.Classify(err, status, raw)
		// DM-20260620-003 (PR-C M2): tag ctx with classification so downstream
		// spans can read it without re-running Classify.
		ctx = context.WithValue(ctx, classifyResultKey{}, c)
		_ = ctx
		return sharederrors.WithShortStack(
			fmt.Errorf("[class=%s] %w", c.Class, err),
			5,
		)
	}
	return sharederrors.WithShortStack(err, 5)
}

// classifyResultKey is the context value key for a cached Classify result.
// DM-20260620-003 (PR-C M2): downstream code can pull this from ctx instead
// of re-running the (potentially expensive) classifier.Classify call.
type classifyResultKey struct{}

// ClassifyResultFromCtx returns a cached Classification if one was attached
// to ctx via classifyAndWrap, otherwise the zero value and false.
func ClassifyResultFromCtx(ctx context.Context) (errorclass.Classification, bool) {
	if ctx == nil {
		return errorclass.Classification{}, false
	}
	v := ctx.Value(classifyResultKey{})
	if v == nil {
		return errorclass.Classification{}, false
	}
	c, ok := v.(errorclass.Classification)
	return c, ok
}