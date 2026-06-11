# 飞书任务规划功能验收指南

**目标:** 在飞书上验证任务规划功能

---

## 验收前准备

### 1. 确认飞书机器人已启动

```bash
# 检查飞书配置
cat devrix.yaml | grep -A 10 "feishu:"
```

### 2. 查看 /help 命令

在飞书发送：
```
/help
```

**预期输出：**
```
🤖 **Devrix - 开发大脑**

**基础命令：**
/new - 开始新会话
/task - 任务管理
/plan - 规划模式
/help - 显示帮助信息
/stop - 停止当前生成

**任务命令 (/task)：**
/task create <任务> - 创建任务
/task list - 列出所有任务
/task ready - 显示就绪任务

**规划命令 (/plan)：**
/plan <目标> - 进入规划模式
/plan approve - 审批计划
/plan show - 显示计划
/plan reject - 拒绝计划
```

---

## 验收测试用例

### 测试 1: 任务命令 `/task`

| 步骤 | 输入 | 预期输出 |
|------|------|----------|
| 1 | `/task` | 显示任务命令帮助 |
| 2 | `/task create 添加单元测试` | `✓ Task created: task_xxx` |
| 3 | `/task list` | `Tasks (1):` + 任务列表 |
| 4 | `/task ready` | `Ready tasks (1):` 或 `No ready tasks` |

### 测试 2: 规划命令 `/plan`

| 步骤 | 输入 | 预期输出 |
|------|------|----------|
| 1 | `/plan` | 显示规划命令帮助 |
| 2 | `/plan Add user authentication` | `📝 **规划模式**` + 已收到目标 |
| 3 | `/plan show` | 显示计划内容（开发中） |
| 4 | `/plan approve` | 审批或提示功能开发中 |
| 5 | `/plan reject` | `Plan rejected` |

### 测试 3: 基础命令

| 步骤 | 输入 | 预期输出 |
|------|------|----------|
| 1 | `/help` | 显示完整帮助 |
| 2 | `/new` | `✅ 新会话已创建` |
| 3 | `/stop` | `⏸️ 停止功能开发中` |

---

## 完整验收流程

```
1. 打开飞书，与 Devrix 机器人对话
         ↓
2. 发送 /help
         ↓
3. 确认帮助信息包含 /task 和 /plan
         ↓
4. 发送 /task create 测试任务
         ↓
5. 发送 /task list 确认任务已创建
         ↓
6. 发送 /plan 添加用户认证
         ↓
7. 发送 /plan show 查看计划
         ↓
8. 发送 /plan approve 或 /plan reject
         ↓
9. 验收完成 ✓
```

---

## 故障排查

| 问题 | 可能原因 | 解决方案 |
|------|----------|----------|
| 命令无响应 | 飞书连接断开 | 重启机器人 |
| 帮助信息不更新 | 缓存问题 | 发送 `/new` 后重试 |
| 任务命令无法识别 | CommandTask 未定义 | 检查 command.go |

---

## 验收确认清单

- [ ] `/help` 显示 `/task` 和 `/plan` 命令
- [ ] `/task` 显示任务命令帮助
- [ ] `/task create <任务>` 创建成功
- [ ] `/task list` 显示任务列表
- [ ] `/plan <目标>` 进入规划模式
- [ ] `/plan show` 显示计划（开发中）
- [ ] `/new` 创建新会话

---

**验收人:** ___________  
**验收日期:** ___________
