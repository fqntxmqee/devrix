### Requirement: W3C Baggage Propagation

系统 MUST 通过 W3C `baggage` 头在 context 与 HTTP/子进程边界传播业务键值；Gateway 入站 MUST 写入 `session.id`，并在 `user.id` 可用时写入 baggage。

**Priority**: P2
**L4**: L4-OBS-BAGGAGE
**L5**: L5-OBS-TRACE-03

#### Scenario: Propagator 往返 baggage

- GIVEN context 含有效 span 与 baggage `session.id=sess_1`
- WHEN `Propagator.Inject` 后 `ExtractContext`
- THEN `baggage` 头 MUST 非空
- AND 提取后 context MUST 含 `session.id=sess_1`

#### Scenario: CLI 子进程继承传播环境

- GIVEN 父 context 含 trace 与 baggage
- WHEN `CLIAgentTool` 创建新子进程
- THEN 子进程环境 MUST 含 `TRACEPARENT` 与 `BAGGAGE`
