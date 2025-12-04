# **EulerGuard 3.0: 极简内核智能体 (Minimal Kernel Intelligence)**
### **核心设计哲学**
1. **后端：Stateful & Queryable (有状态与可查询)**
   - 不再是“阅后即焚”的流式处理，而是构建一个内存中的**时序数据库**。
   - 不再是简单的“匹配即拦截”，而是引入**“影子执行”**与**“虚拟回放”**的双模引擎。
2. **前端：Professional Interface (专业界面)**
   - 像 IDE 一样专业、精密，承载强大的后端数据。
3. **AI：Embedded & Active (嵌入式与主动式)**
   - AI 是系统的"大脑"，时刻在后台分析数据，并主动向"手"（规则引擎）和"脸"（前端）推送建议。
------
### **第零阶段：后端架构重构与代码整理 (Backend Refactoring & Code Organization)**
*目标：重构现有代码结构，使其直接服务于后续阶段的功能开发，避免过度设计。*
#### **1. 重新组织 Package 结构**
- **当前问题**：package 职责不清晰，`ui` 包混合了 Web 服务器、事件桥接、统计等多种职责
- **重构方案**：按功能模块重新组织
  - **`pkg/ebpf/`**：eBPF 程序加载与链接（保持不变）
  - **`pkg/events/`**：事件定义、解码、基础类型（保持不变）
  - **`pkg/storage/`**：**新增**，为第一阶段做准备
    - 将 `ui.Stats` 中的事件存储逻辑抽离
    - 预留 `ringbuffer.go` 和 `indexer.go` 的位置（第一阶段实现）
    - `stats.go`：统计聚合（从 `ui.Stats` 简化而来，只保留计数和告警）
  - **`pkg/rules/`**：规则引擎（保持不变，第二阶段扩展）
  - **`pkg/proc/`**：进程树管理（保持不变，第一阶段添加 `profile.go`）
  - **`pkg/workload/`**：工作负载注册（保持不变）
  - **`pkg/ai/`**：AI 服务（保持不变，第三阶段扩展）
  - **`pkg/simulation/`**：**新增空目录**，第二阶段实现
  - **`pkg/api/`**：**新增空目录**，第三阶段实现语义查询
  - **`pkg/server/`**：Web 服务器（从 `ui` 重命名，只保留 HTTP 路由和 WebSocket）
  - **`pkg/cli/`**：CLI 模式（保持不变）
  - **`pkg/config/`**：配置管理（保持不变，仅优化常量管理）
  - **`pkg/types/`**：共享类型定义（保持不变）
  - **`pkg/metrics/`**：速率统计（保持不变，CLI 专用）
  - **`pkg/output/`**：输出和日志（保持不变，CLI 专用）
  - **`pkg/profiler/`**：学习模式分析（保持不变，CLI 和 Web 都使用）
  - **`pkg/utils/`**：工具函数（保持不变，被多个包使用）
  - **`pkg/core/`**：**新增**，统一初始化流程
- **价值**：清晰的模块划分，每个 package 职责单一，便于后续扩展

#### **2. 彻底重构事件存储，删除旧实现**
- **当前痛点**：
  - `ui.Stats` 混合了统计计数、事件存储、前端事件转换等多种职责
  - `RecentExecs`、`RecentFiles`、`RecentConnects` 使用简单切片，容量只有 50 条
  - 这些旧实现无法支撑第一阶段的需求，必须完全删除
- **重构方案**：
  - **删除 `ui.Stats` 中的事件存储相关代码**：
    - 删除 `RecentExecs()`, `RecentFiles()`, `RecentConnects()` 方法
    - 删除 `recentExecs`, `recentFiles`, `recentConnects` 字段
    - 删除 `RecordExec()`, `RecordFileEvent()`, `RecordConnectEvent()` 中的切片追加逻辑
  - **创建 `pkg/storage/` 包**：
    - `storage/stats.go`：只保留统计计数（execCount, fileCount, connectCount, alerts）
      - **职责**：无状态的统计聚合，不涉及事件存储
      - **接口**：`RecordExec()`, `RecordFile()`, `RecordConnect()`, `AddAlert()`, `Counts()`, `Rates()`
    - `storage/store.go`：定义存储接口
      - **职责**：定义 `EventStore` 接口，不包含实现
      - **接口**：`Append()`, `Query()`, `Latest()` 等
    - `storage/ringbuffer.go`：**第一阶段实现**，实现 `EventStore` 接口
    - `storage/indexer.go`：**第一阶段实现**，倒排索引
    - **职责分离**：`stats.go` 和 `store.go` 完全独立，`stats.go` 不依赖存储实现
  - **事件转换逻辑**：
    - 从 `ui` 包移到 `events/transform.go`
    - 前端事件类型定义移到 `types/events.go`（与后端事件类型分离）
  - **简化 `server.Bridge`**（原 `ui.Bridge`）：
    - 只负责事件转发和告警生成
    - 事件存储直接调用 `storage` 包接口
- **删除的文件/代码**：
  - `ui/stats.go` 中的 `RecentExecs()`, `RecentFiles()`, `RecentConnects()` 方法
  - `ui/stats.go` 中的 `recentExecs`, `recentFiles`, `recentConnects` 字段及相关逻辑
  - `ui/stats.go` 中的 `maxRecent` 常量
- **价值**：彻底清理旧代码，为第一阶段的 `TimeRingBuffer` 和 `Indexer` 实现扫清障碍

#### **3. 彻底简化 `tracer` 包，删除无用代码**
- **当前痛点**：
  - `tracer.Core` 承担过多职责：eBPF 加载、规则加载、BPF Map 填充、事件分发
  - `PopulateMonitoredFiles` 和 `PopulateBlockedPorts` 逻辑复杂，与核心职责无关
  - `ReloadRules()` 方法混合了规则加载和 BPF Map 更新
- **重构方案**：
  - **删除 `tracer.Core` 结构体**，拆分为更小的组件：
    - `ebpf/loader.go`：eBPF 程序加载与链接（已存在，优化）
    - `ebpf/maps.go`：**新增**，BPF Map 填充逻辑（从 `tracer` 移入）
    - `tracer/reader.go`：只负责 RingBuffer Reader 的封装
  - **删除 `tracer.Core.Init()`**：
    - 改为 `ebpf.Load()` 和 `ebpf.Attach()` 等独立函数
    - 规则加载由 `rules` 包负责
    - BPF Map 填充由 `ebpf/maps.go` 负责
    - 初始化逻辑统一到 `core.Bootstrap()`
  - **删除 `tracer.Core.ReloadRules()`**：
    - 规则重载逻辑移到调用方（`cli` 和 `server` 包）
    - 使用 `ebpf/maps.go` 中的函数更新 BPF Map
  - **简化 `tracer.DispatchEvent()`**：
    - 只负责事件解码和分发
    - 事件存储由 `storage` 包处理
  - **`tracer.EventLoop()` 保持不变**：
    - 只负责从 RingBuffer 读取并调用 `DispatchEvent()`
    - 不依赖 `tracer.Core` 结构体
    - 改为接收独立的组件参数（`*ringbuf.Reader`, `*events.HandlerChain` 等）
  - **`tracer` 包中的其他函数**：
    - `AttachLSMHooks()`：移到 `ebpf/attach.go` 或保留在 `tracer/`（如果与事件处理相关）
    - `CloseLinks()`：移到 `ebpf/attach.go` 或保留在 `tracer/`
    - `LoadRules()`：删除，由 `rules` 包或调用方负责
    - `PopulateMonitoredFiles()`, `PopulateBlockedPorts()`：移到 `ebpf/maps.go`
    - `RepopulateMonitoredFiles()`, `RepopulateBlockedPorts()`：移到 `ebpf/maps.go`
    - `extractParentFilename()`, `newPIDResolver()`：移到 `ebpf/maps.go` 或 `ebpf/utils.go`
- **删除的代码**：
  - `tracer/core.go` 中的 `Core` 结构体（完全删除）
  - `tracer/core.go` 中的 `Init()`, `ReloadRules()`, `Close()` 方法
  - `tracer/core.go` 中的 `PopulateMonitoredFiles`, `PopulateBlockedPorts` 等函数
  - `tracer/core.go` 中的 `LoadRules()` 函数（移到 `rules` 包或调用方）
  - `ui.App.core` 字段（改为使用 `core.Bootstrap()` 返回的组件）
  - `cli.CLI.Core` 字段（改为使用 `core.Bootstrap()` 返回的组件）
- **更新的代码**：
  - `ui.App.Run()`：改为调用 `core.Bootstrap()`，不再调用 `tracer.Init()`
  - `cli.RunCLI()`：改为调用 `core.Bootstrap()`，不再调用 `tracer.Init()`
  - `tracer.EventLoop()`：改为接收独立的组件参数，不依赖 `Core` 结构体
- **价值**：`tracer` 包职责单一，代码更清晰，便于测试和维护

#### **4. 统一初始化流程，消除重复代码**
- **当前痛点**：
  - `ui.App.Run()` 和 `cli.RunCLI()` 有大量重复的初始化代码
  - 组件初始化顺序不清晰
- **重构方案**：
  - **创建 `pkg/core/bootstrap.go`**（简单实用，不过度设计）：
    - `Bootstrap()` 函数：统一的初始化流程
    - 返回 `*CoreComponents` 结构体，包含所有核心组件
    - 分阶段初始化，清晰的错误处理
  - **`CoreComponents` 结构**：
    ```go
    type CoreComponents struct {
        EBpfObjs    *ebpf.LSMObjects
        EBpfLinks   []link.Link
        Reader      *ringbuf.Reader
        ProcessTree *proc.ProcessTree
        WorkloadReg *workload.Registry
        RuleEngine  *rules.Engine
        Rules       []types.Rule
    }
    ```
  - **`cli` 和 `server` 包**：
    - 都调用 `core.Bootstrap()` 初始化核心组件
    - 然后各自添加特定逻辑（CLI 的 printer，Web 的 stats/bridge）
  - **`cmd/cli.go` 和 `cmd/web.go`**：
    - 保持不变，只调用 `cli.RunCLI()` 和 `server.RunWebServer()`
    - 初始化逻辑已移到 `core.Bootstrap()`
- **价值**：消除重复代码，初始化流程清晰，便于维护

#### **5. 优化配置管理，集中常量定义**
- **当前痛点**：
  - 硬编码常量散布在代码中（如 `maxRecent: 50`, `maxAlerts: 100`）
  - 配置项与常量混用
- **重构方案**：
  - **`pkg/config/constants.go`**：
    - 定义所有默认值和常量
    - 如：`DefaultRecentEventsCapacity = 50`（第一阶段将改为 10000+）
    - 如：`DefaultMaxAlerts = 100`
  - **配置结构优化**（可选，不强制）：
    - 在 `config.Options` 中添加 `Storage` 子结构
    - 为第一阶段的存储配置预留位置
- **价值**：常量集中管理，便于后续调整和扩展

#### **6. 重构事件处理流程，删除冗余逻辑**
- **当前痛点**：
  - `HandlerChain` 设计简单，但事件处理逻辑分散
  - `ui.Bridge` 和 `cli.alertHandler` 有重复的事件处理逻辑
  - 事件转换逻辑混在 UI 层
- **重构方案**：
  - **统一事件处理流程**：
    - `tracer.DispatchEvent()` 中：事件解码 → 存储到 `storage` → 分发到 HandlerChain
    - 所有事件必须经过存储层，不再有"可选"的存储逻辑
  - **简化 HandlerChain**：
    - `server.Bridge`：只负责告警生成和前端通知
    - `cli.alertHandler`：只负责 CLI 输出
    - 删除重复的事件处理逻辑
  - **删除无用代码**：
    - `ui.Bridge` 中的事件转换逻辑（移到 `events/transform.go`）
    - `ui.Bridge` 中的 `SetRuleEngine()`, `SetWorkloadRegistry()` 等 Setter（改为构造函数注入）
    - `ui.Stats` 中的 `PublishEvent()`, `SubscribeEvents()` 等发布订阅逻辑（如果不需要）
- **价值**：事件处理流程清晰，删除冗余代码，为存储层集成做好准备

#### **7. 彻底清理无用代码**
- **删除的文件**：
  - `pkg/ui/` 整个目录（重命名为 `pkg/server/`，删除旧实现）
  - `pkg/tracer/core.go`（功能拆分到其他包，但保留 `EventLoop` 和 `DispatchEvent` 函数）
- **删除的函数/方法**：
  - `ui.Stats.RecentExecs()`, `RecentFiles()`, `RecentConnects()`
  - `ui.Stats.RecordExec()`, `RecordFileEvent()`, `RecordConnectEvent()` 中的切片逻辑
  - `tracer.Core.Init()`, `ReloadRules()`, `Close()`
  - `tracer.PopulateMonitoredFiles()`, `PopulateBlockedPorts()`, `Repopulate*()`
  - `tracer.LoadRules()`（移到调用方或 `rules` 包）
  - `ui.Bridge.SetRuleEngine()`, `SetWorkloadRegistry()`, `SetProfiler()`（改为构造函数注入）
  - `ui.App.core` 字段（改为使用 `core.Bootstrap()` 返回的组件）
  - `cli.CLI.Core` 字段（改为使用 `core.Bootstrap()` 返回的组件）
- **删除的字段/变量**：
  - `ui.Stats` 中的 `recentExecs`, `recentFiles`, `recentConnects`, `recentMu`, `maxRecent`
  - `tracer.Core` 结构体中的所有字段（结构体已删除）
  - 未使用的导入和变量
- **保留的代码**：
  - `tracer.EventLoop()` 和 `tracer.DispatchEvent()`：保留，但改为接收独立参数
  - `tracer.AttachLSMHooks()` 和 `tracer.CloseLinks()`：保留，移到 `ebpf/` 包或保留在 `tracer/`
  - `profiler` 包：保持不变，CLI 和 Web 都使用
  - `metrics` 和 `output` 包：保持不变，CLI 专用
- **代码优化**：
  - **函数拆分**：将长函数拆分为小函数
  - **错误处理**：统一使用 `fmt.Errorf` 包装错误，添加上下文
  - **GoDoc 注释**：为所有公开接口添加注释
- **价值**：代码库更精简，只保留必要的代码，降低维护成本

#### **8. 测试说明**
- **单元测试**：
  - `pkg/core/bootstrap_test.go`：测试 `Bootstrap()` 函数的初始化流程
  - `pkg/ebpf/maps_test.go`：测试 BPF Map 填充逻辑（使用 Mock eBPF Map）
  - `pkg/storage/stats_test.go`：测试统计计数功能
  - `pkg/events/transform_test.go`：测试事件转换逻辑
- **集成测试**：
  - `pkg/core/bootstrap_integration_test.go`：测试完整的初始化流程（需要 root 权限）
  - `pkg/server/bridge_test.go`：测试事件桥接功能（Mock 依赖）
- **测试覆盖率目标**：
  - 新增代码覆盖率 > 80%
  - 关键路径（初始化、事件处理）覆盖率 > 90%
- **测试工具**：
  - 使用 `testify` 进行断言和 Mock
  - 使用 `testcontainers` 或 Mock 对象模拟 eBPF 环境

#### **9. 模块化清晰度确认**
- **分层架构**（清晰的依赖方向，避免循环依赖）：
  - **基础设施层**（最底层，不依赖业务逻辑）：
    - `pkg/ebpf/`：eBPF 程序加载、链接、Map 管理
    - `pkg/events/`：事件定义、解码（纯数据结构）
    - `pkg/types/`：共享类型定义（纯数据结构）
    - `pkg/utils/`：工具函数（无状态函数）
    - `pkg/config/`：配置管理（纯配置）
  - **核心业务层**（依赖基础设施层）：
    - `pkg/storage/`：事件存储和统计（依赖 `events`, `types`）
    - `pkg/rules/`：规则引擎（依赖 `events`, `types`）
    - `pkg/proc/`：进程树管理（依赖 `events`, `types`）
    - `pkg/workload/`：工作负载注册（依赖 `proc`, `types`）
    - `pkg/tracer/`：事件分发（依赖 `events`, `storage`, `proc`, `workload`）
  - **服务层**（依赖核心业务层）：
    - `pkg/ai/`：AI 服务（依赖 `storage`, `proc`, `rules`, `types`）
    - `pkg/simulation/`：模拟引擎（依赖 `storage`, `rules`）
    - `pkg/api/`：语义查询（依赖 `storage`, `ai`）
  - **应用层**（依赖所有下层）：
    - `pkg/core/`：统一初始化（依赖所有核心业务层）
    - `pkg/server/`：Web 服务器（依赖 `core`, `storage`, `rules`, `ai`, `api`）
    - `pkg/cli/`：CLI 模式（依赖 `core`, `output`, `metrics`, `profiler`）
  - **辅助包**（被应用层使用）：
    - `pkg/metrics/`：速率统计（CLI 专用）
    - `pkg/output/`：输出和日志（CLI 专用）
    - `pkg/profiler/`：学习模式分析（CLI 和 Web 都使用）
- **职责边界清晰**：
  - ✅ **`storage`**：只负责事件存储和统计，不涉及业务逻辑
  - ✅ **`rules`**：只负责规则匹配，不涉及存储和事件分发
  - ✅ **`tracer`**：只负责事件解码和分发，不涉及存储实现
  - ✅ **`proc`**：只负责进程树管理，不涉及事件处理
  - ✅ **`server`**：只负责 HTTP/WebSocket，不涉及核心业务逻辑
  - ✅ **`cli`**：只负责 CLI 输出，不涉及核心业务逻辑
- **依赖关系验证**：
  - ✅ **无循环依赖**：依赖方向单向（基础设施 → 核心业务 → 服务 → 应用）
  - ✅ **接口隔离**：`storage` 通过接口暴露，`rules` 通过接口暴露
  - ✅ **最小依赖**：每个包只依赖必要的包，不引入不必要的依赖
- **模块边界明确**：
  - **`storage` 包内部职责分离**：
    - `stats.go`：统计计数（无状态聚合，独立于存储实现）
    - `store.go`：存储接口定义（纯接口，无实现）
    - `ringbuffer.go`：存储实现（第一阶段，实现 `EventStore` 接口）
    - `indexer.go`：索引实现（第一阶段，与 `ringbuffer` 协同工作）
    - **边界**：`stats.go` 不依赖 `store.go`，两者完全独立
  - **`events` 包职责明确**：
    - `types.go`：事件类型定义（纯数据结构）
    - `decoder.go`：事件解码（从字节流解码）
    - `handler.go`：事件处理器接口（`EventHandler`）
    - `transform.go`：事件格式转换（后端事件 → 前端事件，不涉及业务逻辑）
  - **`tracer` 包简化后职责单一**：
    - `EventLoop()`：事件循环（从 RingBuffer 读取）
    - `DispatchEvent()`：事件分发（解码 → 存储 → HandlerChain）
    - **不包含**：存储实现、规则加载、BPF Map 管理等职责
  - **`ebpf` 包职责明确**：
    - `loader.go`：eBPF 程序加载
    - `attach.go`：LSM Hook 链接（从 `tracer` 移入）
    - `maps.go`：BPF Map 填充和管理（从 `tracer` 移入）
    - **不包含**：事件处理、规则匹配等业务逻辑
  - **`core` 包职责明确**：
    - `bootstrap.go`：统一初始化流程
    - **只负责**：组件初始化和组装，不包含业务逻辑
    - **依赖方向**：依赖所有核心业务层，但不被业务层依赖
- **依赖关系图**（确保无循环依赖）：
  ```
  应用层: server, cli
    ↓
  服务层: api, simulation, ai
    ↓
  核心业务层: tracer, storage, rules, proc, workload
    ↓
  基础设施层: ebpf, events, types, utils, config
  ```
  - **依赖规则**：只能依赖下层，不能依赖上层或同层
  - **例外**：`tracer` 可以依赖 `storage`（同层，但 `storage` 不依赖 `tracer`）
  - **验证**：通过 `go mod graph` 或静态分析工具验证无循环依赖

#### **重构与后续阶段的对接关系**
- **第零阶段 → 第一阶段**：
  - `pkg/storage/` 包已创建，`store.go` 定义接口
  - 第一阶段直接实现 `ringbuffer.go` 和 `indexer.go`，实现 `store.go` 中的接口
  - `proc/profile.go` 在第一阶段添加，无需重构准备
  - **旧代码已完全删除**，无向后兼容负担
- **第零阶段 → 第二阶段**：
  - `pkg/rules/` 包结构清晰，直接扩展 `engine.go` 支持 Shadow Mode
  - `pkg/simulation/` 目录已创建，第二阶段直接实现 `runner.go`
  - 模拟引擎依赖 `storage` 包的 `TimeRingBuffer`（第一阶段已实现）
- **第零阶段 → 第三阶段**：
  - `pkg/api/` 目录已创建，第三阶段直接实现 `query.go`
  - `pkg/ai/sentinel.go` 直接添加，依赖第一阶段和第二阶段的功能
  - 语义查询依赖 `storage` 包的 `Indexer`（第一阶段已实现）
- **重构原则**：
  - ✅ **彻底删除**：旧实现完全删除，不保留临时代码
  - ✅ **直接对接**：重构后的结构直接服务于后续阶段
  - ✅ **清晰模块化**：每个包职责单一，依赖关系清晰，无循环依赖
  - ❌ **不过度抽象**：不创建复杂的 DI 容器，不定义过多接口
  - ❌ **不提前实现**：不实现第一阶段的功能，只做结构准备
  - ❌ **不保留无用代码**：所有无用代码必须删除

------
### **第一阶段：后端重构——构建"全息遥测仓库" (Holographic Telemetry Warehouse)**
*目标：打造一个高性能的内存时序数据底座，支持毫秒级的复杂查询与回溯。这是 AI "拥有记忆"的前提。*
#### **1. 实现 `TimeRingBuffer` (高性能时序环形缓冲)**
- **对接第零阶段**：`pkg/storage/store.go` 已定义接口，现在实现具体存储
- **实现 (`pkg/storage/ringbuffer.go`)**：
  - **数据结构**：
    - 基于定长数组 + 原子游标的环形缓冲区
    - 容量：**10,000+** 条（配置项 `Storage.RingBufferCapacity`，默认 10000）
    - 存储统一的事件类型 `storage.Event`（包含 `Type`, `Timestamp`, `Data` 等）
  - **零拷贝优化**：
    - 使用指针存储事件对象（`*storage.Event`）
    - 事件对象在堆上分配，避免栈拷贝
  - **并发安全**：
    - 写入使用原子操作更新游标
    - 读取使用 `RWMutex` 保护范围查询
  - **接口实现**：
    - 实现 `storage/store.go` 中定义的 `EventStore` 接口
    - `Append(event *Event)`：追加事件
    - `Query(start, end time.Time) []*Event`：时间范围查询
    - `Latest(n int) []*Event`：获取最近 N 条
- **集成点**：
  - 在 `tracer.DispatchEvent()` 中，事件解码后立即调用 `storage.Append()`
  - 删除所有旧的事件存储逻辑（第零阶段已清理）
- **价值**：为 AI 分析提供足够的历史上下文，支持时间窗口查询

#### **2. 构建"倒排索引" (Inverted Indexing)**
- **实现 (`pkg/storage/indexer.go`)**：
  - **索引结构**：
    - `pidIndex map[uint32][]*Event`：PID 到事件列表
    - `cgroupIndex map[uint64][]*Event`：CgroupID 到事件列表
    - `typeIndex map[EventType][]*Event`：事件类型到事件列表
    - `processIndex map[string][]*Event`：进程名到事件列表（用于 "redis" 等查询）
  - **实时维护**：
    - 在 `TimeRingBuffer.Append()` 时，同步更新所有索引
    - 使用 `sync.Map` 或 `RWMutex` 保护并发访问
    - 索引只存储指针，不复制事件数据
  - **查询接口**：
    - `QueryByPID(pid uint32) []*Event`
    - `QueryByCgroup(cgroupID uint64) []*Event`
    - `QueryByType(eventType EventType) []*Event`
    - `QueryByProcess(processName string) []*Event`
    - `QueryByFilter(filter Filter) []*Event`：组合查询（用于语义查询）
  - **索引清理**：
    - 当 `TimeRingBuffer` 覆盖旧数据时，同步清理索引中的过期指针
    - 使用时间戳判断是否过期
- **集成点**：
  - `Indexer` 与 `TimeRingBuffer` 紧密耦合，在 `storage/store.go` 中统一管理
  - 第三阶段的语义查询直接调用 `Indexer` 的查询接口
- **价值**：查询复杂度从 **O(N)** 降为 **O(1)**，支持 AI 快速检索

#### **3. 进程画像快照 (Live Process Profile)**
- **实现 (`pkg/proc/profile.go`)**：
  - **数据结构**：
    ```go
    type ProcessProfile struct {
        PID      uint32
        Static   StaticProfile    // 静态信息
        Dynamic  DynamicProfile   // 动态统计
        Baseline *BaselineProfile // 基线（可选）
    }
    type StaticProfile struct {
        StartTime    time.Time
        CommandLine  string
        Genealogy    []uint32  // 父进程链
    }
    type DynamicProfile struct {
        FileOpenCount    int64
        NetConnectCount  int64
        LastFileOpen     time.Time
        LastConnect      time.Time
        // 过去 5 分钟的统计
    }
    ```
  - **维护机制**：
    - 在 `ProcessTree.AddProcess()` 时创建 `ProcessProfile`
    - 在事件存储时，同步更新对应 PID 的 `DynamicProfile`
    - 使用 `sync.Map` 存储 `map[uint32]*ProcessProfile`
  - **查询接口**：
    - `GetProfile(pid uint32) (*ProcessProfile, bool)`
    - `GetAnomalousProcesses() []*ProcessProfile`：检测行为突变
  - **集成点**：
    - 与 `ProcessTree` 集成，在进程创建时初始化
    - 与 `storage` 集成，在事件存储时更新统计
    - 第三阶段的 Sentinel 调用 `GetAnomalousProcesses()` 检测异常
- **价值**：为 AI 提供进程行为上下文，支持异常检测

#### **4. 测试说明**
- **单元测试**：
  - `pkg/storage/ringbuffer_test.go`：
    - 测试 `Append()` 的并发安全性（使用 `go test -race`）
    - 测试 `Query()` 的时间范围查询准确性
    - 测试环形缓冲的覆盖行为（容量满时覆盖旧数据）
    - 测试 `Latest()` 返回最近 N 条的正确性
  - `pkg/storage/indexer_test.go`：
    - 测试索引的实时维护（添加事件后索引立即更新）
    - 测试各种查询接口（`QueryByPID`, `QueryByCgroup` 等）
    - 测试组合查询 `QueryByFilter` 的准确性
    - 测试索引清理（过期数据从索引中移除）
  - `pkg/proc/profile_test.go`：
    - 测试 `ProcessProfile` 的创建和更新
    - 测试 `GetAnomalousProcesses()` 的异常检测逻辑
    - 测试 `DynamicProfile` 的统计准确性
- **集成测试**：
  - `pkg/storage/integration_test.go`：
    - 测试 `TimeRingBuffer` 与 `Indexer` 的协同工作
    - 测试高并发场景下的性能和正确性
    - 测试事件存储与查询的端到端流程
  - `pkg/tracer/dispatch_integration_test.go`：
    - 测试事件解码 → 存储 → 分发的完整流程
    - 验证事件确实被存储到 `TimeRingBuffer`
- **性能测试**：
  - `pkg/storage/ringbuffer_bench_test.go`：
    - 基准测试：`Append()` 的吞吐量（目标：> 100k events/sec）
    - 基准测试：`Query()` 的延迟（目标：< 1ms for 10k events）
  - `pkg/storage/indexer_bench_test.go`：
    - 基准测试：索引查询性能（目标：O(1) 复杂度）
- **测试覆盖率目标**：
  - `storage` 包覆盖率 > 85%
  - `proc/profile.go` 覆盖率 > 80%
------
### **第二阶段：后端重构——双模执行引擎 (Dual-Mode Engine)**
*目标：让规则引擎支持"测试服"逻辑，这是系统工具安全落地的核心机制。*
#### **1. 影子模式 (Shadow Mode)**
- **规则属性扩展 (`pkg/types/rules.go`)**：
  - `Rule` 结构体增加 `Mode` 字段：
    ```go
    type RuleMode string
    const (
        ModeEnforce RuleMode = "enforce"  // 强制拦截
        ModeShadow  RuleMode = "shadow"   // 影子观察
    )
    type Rule struct {
        // ... 现有字段
        Mode RuleMode `yaml:"mode,omitempty"` // 默认为 "enforce"
    }
    ```
- **规则引擎扩展 (`pkg/rules/engine.go`)**：
  - **执行逻辑修改**：
    - `MatchExec()`, `MatchFile()`, `MatchConnect()` 返回结果增加 `Mode` 信息
    - 当 `Mode == Shadow` 且规则命中时：
      - **不返回 `-EPERM`**（在 eBPF 层面放行）
      - 生成 `ShadowHit` 事件，推送到 `ShadowBuffer`
  - **ShadowBuffer 实现 (`pkg/rules/shadow.go`)**：
    - 使用 `TimeRingBuffer` 的简化版本存储影子命中事件
    - 记录：规则名、命中时间、事件详情、是否误报（由 AI 判断）
    - 查询接口：`GetHits(ruleName string, timeWindow time.Duration) []ShadowHit`
- **集成点**：
  - 在 `server.Bridge` 和 `cli.alertHandler` 中，根据 `Mode` 决定是否真正拦截
  - Shadow 模式的命中事件不触发告警，只记录到 `ShadowBuffer`
- **价值**：AI 生成的规则默认进入 Shadow 模式，通过实战数据验证准确性

#### **2. 虚拟回放引擎 (Simulation Engine)**
- **实现 (`pkg/simulation/runner.go`)**：
  - **接口设计**：
    ```go
    type SimulationRequest struct {
        Rules      []types.Rule
        TimeWindow TimeWindow  // 时间窗口
    }
    type SimulationReport struct {
        TotalEvents     int
        Blocked         int
        ShadowHits      int
        FalsePositives  int  // 需要 AI 判断
        AffectedPIDs    []uint32
    }
    func RunSimulation(req SimulationRequest, store storage.EventStore) (*SimulationReport, error)
    ```
  - **执行逻辑**：
    1. 从 `TimeRingBuffer` 中拉取 `TimeWindow` 范围内的所有事件
    2. 对每个事件，使用临时规则引擎进行匹配
    3. 统计命中次数、拦截次数、影响的进程
    4. 返回 `SimulationReport`
  - **优化**：
    - 使用 `Indexer` 快速过滤相关事件（如只查询特定 PID 的事件）
    - 并行处理多个事件（如果数据量大）
- **集成点**：
  - 第三阶段的语义查询可以调用 `RunSimulation()` 预览规则效果
  - 第四阶段的前端 Policy Studio 调用此接口显示模拟结果
- **价值**：在部署规则前预览效果，降低误拦截风险

#### **3. 测试说明**
- **单元测试**：
  - `pkg/types/rules_test.go`：
    - 测试 `Rule.Mode` 字段的序列化/反序列化（YAML）
    - 测试默认值（未指定时默认为 `ModeEnforce`）
  - `pkg/rules/engine_test.go`：
    - 测试 Shadow 模式的匹配逻辑（命中但不拦截）
    - 测试 Enforce 模式的正常拦截逻辑
    - 测试规则引擎对两种模式的处理差异
  - `pkg/rules/shadow_test.go`：
    - 测试 `ShadowBuffer` 的存储和查询
    - 测试 `GetHits()` 的时间窗口过滤
    - 测试 Shadow 命中事件的记录格式
  - `pkg/simulation/runner_test.go`：
    - 测试 `RunSimulation()` 的基本功能
    - 测试模拟报告的准确性（统计数字正确）
    - 测试时间窗口过滤的正确性
    - 测试空规则列表的处理
- **集成测试**：
  - `pkg/rules/shadow_integration_test.go`：
    - 测试 Shadow 规则从匹配到记录的完整流程
    - 测试 Shadow 模式不影响实际系统行为（不拦截）
  - `pkg/simulation/integration_test.go`：
    - 测试模拟引擎与 `TimeRingBuffer` 的集成
    - 测试使用真实历史数据运行模拟
    - 验证模拟结果与实际规则执行的一致性
- **测试覆盖率目标**：
  - `rules` 包覆盖率 > 85%
  - `simulation` 包覆盖率 > 80%
------
### **第三阶段：AI 接口层——语义化与主动分析**
*目标：封装 AI 能力，使其能像 API 一样被调用。*
#### **1. 语义查询接口 (Semantic Query Layer)**
- **实现 (`pkg/api/query.go`)**：
  - **自然语言解析**：
    - 使用 AI 服务（`pkg/ai/`）将自然语言转换为结构化查询
    - Prompt 模板：将用户输入转换为 JSON 格式的 `QueryFilter`
    ```go
    type QueryFilter struct {
        Type        string   // "exec", "file", "connect"
        Process     string   // 进程名
        Action      string   // "block", "monitor"
        PID         uint32   // 可选
        CgroupID    uint64   // 可选
        TimeWindow  TimeWindow
    }
    ```
  - **查询执行**：
    - 调用 `storage.Indexer.QueryByFilter(filter)` 获取事件
    - 支持组合查询（多个条件 AND/OR）
    - 返回格式化的结果（JSON 或结构化数据）
  - **API 端点 (`pkg/server/api.go`)**：
    - `POST /api/query`：接收自然语言查询，返回结果
    - `POST /api/query/semantic`：语义查询（调用 AI）
    - `GET /api/query/history`：查询历史（从 `TimeRingBuffer` 获取）
- **集成点**：
  - 依赖第一阶段的 `storage.Indexer` 进行快速查询
  - 依赖 `pkg/ai/` 服务进行自然语言解析
  - 第四阶段的前端 Omnibox 调用此接口
- **价值**：用户可以用自然语言查询事件，降低使用门槛

#### **2. 后台主动巡检 (Background Sentinel)**
- **实现 (`pkg/ai/sentinel.go`)**：
  - **启动机制**：
    - 在 `core.Bootstrap()` 中初始化 Sentinel
    - 启动独立的 goroutine，使用 `Ticker`（默认每分钟）执行巡检
  - **任务 1：Shadow 规则转正建议**：
    - 遍历 `rules.ShadowBuffer` 中的所有规则
    - 对每条规则，统计命中率和误报率（需要 AI 判断是否为误报）
    - 如果命中率 > 阈值（如 10 次/小时）且误报率 < 阈值（如 5%），生成转正建议
    - 建议格式：`{ Rule: rule, Confidence: 0.95, Reason: "..." }`
  - **任务 2：进程行为异常检测**：
    - 调用 `proc.GetAnomalousProcesses()` 获取异常进程
    - 对每个异常进程，使用 AI 分析是否真的异常
    - 生成异常报告：`{ PID: pid, Anomaly: "sudden_file_reads", Severity: "medium" }`
  - **通知推送**：
    - 将建议和异常报告推送到 `server.NotificationStream`
    - 前端通过 WebSocket 接收实时通知
    - 通知格式：`{ Type: "shadow_promotion" | "anomaly", Data: {...} }`
- **集成点**：
  - 依赖第二阶段的 `rules.ShadowBuffer`
  - 依赖第一阶段的 `proc.ProcessProfile`
  - 依赖 `pkg/ai/` 服务进行智能分析
  - 通过 `server` 包的 WebSocket 推送通知
- **价值**：主动发现问题和机会，减少人工巡检

#### **3. 测试说明**
- **单元测试**：
  - `pkg/api/query_test.go`：
    - 测试自然语言到 `QueryFilter` 的转换（Mock AI 服务）
    - 测试 `QueryByFilter()` 的查询逻辑
    - 测试组合查询（AND/OR）的正确性
    - 测试无效查询的处理（错误输入）
  - `pkg/ai/sentinel_test.go`：
    - 测试 Shadow 规则转正建议的生成逻辑
    - 测试异常进程检测的准确性
    - 测试通知格式的正确性
    - 测试 Ticker 的定时执行
- **集成测试**：
  - `pkg/api/query_integration_test.go`：
    - 测试语义查询的端到端流程（AI 解析 → 查询执行 → 返回结果）
    - 使用真实的 `storage.Indexer` 进行查询
    - 测试查询性能（响应时间 < 100ms）
  - `pkg/ai/sentinel_integration_test.go`：
    - 测试 Sentinel 与 `ShadowBuffer` 和 `ProcessProfile` 的集成
    - 测试通知推送到 WebSocket 的流程
    - 测试 Sentinel 的完整巡检周期
- **API 测试**：
  - `pkg/server/api_test.go`：
    - 测试 `POST /api/query/semantic` 端点
    - 测试 `GET /api/query/history` 端点
    - 测试 WebSocket 通知推送
    - 使用 `httptest` 进行 HTTP 测试
- **测试覆盖率目标**：
  - `api` 包覆盖率 > 80%
  - `ai/sentinel.go` 覆盖率 > 75%
------
### **第四阶段：前端 UI 重构——专业工作台**
*目标：用专业的 UI 承载强大的后端数据。*
#### **1. 前端功能需求**
- **资源管理**：
  - 展示工作负载和进程列表
  - 支持实时更新（WebSocket）
  - 支持进程详情查看
- **事件展示**：
  - 实时事件流显示
  - 支持事件过滤和搜索
  - 集成语义查询（Omnibox）
- **数据分析**：
  - 威胁热力图展示
  - 健康分计算和展示
  - 进程行为分析
- **AI 集成**：
  - 显示 Sentinel 的实时分析结果
  - 显示异常检测结果
  - 显示 Shadow 规则转正建议

#### **2. 策略编排中心 (Policy Studio)**
- **功能需求**：
  - **规则生成**：
    - 支持自然语言输入生成规则
    - 调用 `POST /api/ai/generate-rule` 获取规则
    - 显示 AI 的推理过程
  - **规则预览**：
    - 显示生成的规则 YAML
    - 自动运行模拟预览规则效果
    - 显示模拟报告（拦截数量、影响进程等）
  - **规则部署**：
    - 支持部署为 Shadow 模式
    - 支持部署为 Enforce 模式（需确认）
    - 支持保存草稿
- **集成点**：
  - 依赖第三阶段的语义查询和 AI 服务
  - 依赖第二阶段的 Simulation Engine
  - 依赖 `pkg/server/api.go` 提供的 REST API
- **价值**：降低规则编写门槛，通过模拟预览降低误拦截风险

#### **3. API 端点汇总**
- **事件查询**：
  - `GET /api/events`：查询事件（支持过滤参数）
  - `GET /api/events/{id}`：获取单个事件详情
  - `POST /api/query/semantic`：语义查询
- **进程与工作负载**：
  - `GET /api/processes`：获取所有进程
  - `GET /api/process/{pid}/profile`：获取进程画像
  - `GET /api/workloads`：获取工作负载列表
- **规则管理**：
  - `GET /api/rules`：获取所有规则
  - `POST /api/rules`：创建规则（支持 `mode: shadow`）
  - `DELETE /api/rules/{id}`：删除规则
  - `POST /api/rules/{id}/promote`：Shadow 规则转正
- **模拟与 AI**：
  - `POST /api/simulation/run`：运行模拟
  - `POST /api/ai/generate-rule`：AI 生成规则
  - `GET /api/ai/sentinel/notifications`：获取 Sentinel 通知（WebSocket）

#### **4. 测试说明**
- **API 测试**：
  - `pkg/server/api_test.go`：
    - 测试所有 REST API 端点的正确性
    - 测试请求参数验证和错误处理
    - 测试响应格式和状态码
    - 使用 `httptest` 进行 HTTP 测试
  - `pkg/server/websocket_test.go`：
    - 测试 WebSocket 连接的建立和关闭
    - 测试事件推送功能
    - 测试通知推送功能
- **集成测试**：
  - `pkg/server/integration_test.go`：
    - 测试前端与后端的完整交互流程
    - 测试语义查询的端到端流程
    - 测试规则生成和部署的完整流程
    - 测试模拟引擎的前端调用