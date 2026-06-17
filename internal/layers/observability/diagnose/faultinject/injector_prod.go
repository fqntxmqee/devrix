// Package faultinject — A4 故障注入生产 no-op stub(无 testbuild tag)。
//
// 生产 binary 永远启用 no-op 实现,避免注入逻辑泄漏到 release。
// 测试 binary 通过 `go test -tags testbuild` 启用真实实现(injector.go)。
//
//go:build !testbuild
// +build !testbuild

package faultinject

// Rule 一条注入规则(占位)。
type Rule struct {
	Target string
	Mode   string
	Param  string
	Once   bool
}

// Injector 故障注入器。生产 no-op。
type Injector struct{}

// New 总是返回禁用 injector。
func New() *Injector { return &Injector{} }

// Enabled 永远 false。
func (i *Injector) Enabled() bool { return false }

// AddRule no-op。
func (i *Injector) AddRule(_ Rule) {}

// Reset no-op。
func (i *Injector) Reset() {}

// Hook 永远 nil(无注入)。
func (i *Injector) Hook(_ string) error { return nil }
