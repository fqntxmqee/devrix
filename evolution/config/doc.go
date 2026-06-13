// Package config 提供配置热重载功能
package config

import (
	"context"
	"time"

	"github.com/fukaiyi/devrix/internal/shared/errors"
)

// ErrAlreadyStarted 热重载服务已启动
var ErrAlreadyStarted = errors.NewSentinel("config: hotreload service already started")

// ErrAlreadyStopped 热重载服务已停止
var ErrAlreadyStopped = errors.NewSentinel("config: hotreload service already stopped")

// ErrMaxSubscribers 订阅者数量已达上限
var ErrMaxSubscribers = errors.NewSentinel("config: max subscribers reached")

// ErrNilSubscriber 订阅者为 nil
var ErrNilSubscriber = errors.NewSentinel("config: nil subscriber")

// DefaultDebounce 默认防抖延迟
const DefaultDebounce = 500 * time.Millisecond

// DefaultMaxSubscribers 默认最大订阅者数量
const DefaultMaxSubscribers = 10
