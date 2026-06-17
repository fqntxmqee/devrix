package stream

import (
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
func classifyAndWrap(classifier errorclass.Classifier, err error, status int, raw string) error {
	if err == nil {
		return nil
	}
	if classifier != nil {
		c := classifier.Classify(err, status, raw)
		return sharederrors.WithShortStack(
			fmt.Errorf("[class=%s] %w", c.Class, err),
			5,
		)
	}
	return sharederrors.WithShortStack(err, 5)
}