//go:build testbuild
// +build testbuild

package faultinject

import "time"

// sleepMillis 抽象 sleep 便于测试桩。
func sleepMillis(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
