# Tasks: devrix-event-channel (S4)

## S4-1 配置骨架
- [x] 创建 `internal/shared/config/eventbus.go`
- [x] 默认值 + env 覆盖
- [x] TestEventBusConfig_Defaults / TestEventBusConfig_EnvOverride

## S4-2 eventbus 包骨架
- [x] `types.go`：Event wrapper + Priority + With* 不可变
- [x] `bus.go`：BackpressureEventBus struct + 状态机字段
- [x] 状态机 Running 状态
- [x] D2-S3-T01 TestNormalEventFlow_NoLoss（RED → GREEN）
- [x] D2-S3-T05 TestCompleteEventNeverDropped
- [x] D2-S3-T06 TestErrorEventNeverDropped
- [x] D2-S3-T07 TestPublishBlocksAtHighWatermark

## S4-3 Drain 协议
- [x] `drain.go`：Drain report + drainLoop
- [x] 状态机 Draining 转换
- [x] D2-S3-T02 TestBackpressureTriggersDrain
- [x] Critical 事件不被 Drain（断言：drain report 不含 complete/error）

## S4-4 Compact 协议
- [x] `compact.go`：相邻同 type 事件合并
- [x] 状态机 Compacting 转换
- [x] D2-S3-T03 TestCompactConsecutiveEvents
- [x] Critical 不参与 Compact

## S4-5 Reconnect 协议
- [x] `reconnect.go`：Drain→Compact→重建 channel→刷 pending
- [x] 状态机 Reconnecting → Running
- [x] D2-S3-T04 TestReconnectRecovery

## S4-6 Gateway 集成
- [x] `gateway.go` 注入可选 bus
- [x] `handleEngineEvents` 改为 `bus.Publish` + 独立 consume goroutine
- [x] 保持 wire 兼容（OutboundMessage / OnMessage / metric 不变）
- [x] nil bus 时降级为原 fanout
- [x] 现有 gateway 测试保持绿

## S4-7 验证
- [x] `go build ./...` 通过
- [x] `go test -race -count=1 ./internal/layers/communication/eventbus/... ./internal/layers/communication/gateway/...` 全绿
- [x] `go vet ./...` 干净
- [x] `gofmt -l` 干净

## S4-8 提交
- [x] `feat(event-channel): 实现 S4（BackpressureEventBus + Drain/Compact/Reconnect）`
