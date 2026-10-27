<img src="assets/logo.svg" alt="agentworld" width="200" height="40">

[English](README.md) | **简体中文**

# Agent World

*一个事件驱动的文明模拟器：只在某个决策可能改变文明尺度结果的地方，才花掉稀缺的语言模型推理。*

**Agent World 把一个完整社会跑成一个确定的事件驱动世界——每个人都作为独立行动者长期存在，日常时数由廉价的策略处理，真实模型只被用在值得付费的决策上，而每一次调用、每一次策略补丁、每一个结果都被记录并可重放。**

它的目标是在有限的算力与语言模型预算下，最大化文明层面的因果保真度。在初始设计中，每个人都作为独立行动者持久存在，但大部分行为走廉价的可执行策略。昂贵推理被留给新颖、不确定或潜在影响重大的决策；语言模型调用通路已经实现，并被现行计划（[docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md)）中两个已交付切片同时使用——VS1（M1）的机制已经完整，十一条冻结退出判据在封闭（hermetic）证据下全部 PASS；VS2（M2，注意力）的按价值分配机制已经实现并通过封闭验证。两个切片的真实模型分支仍被花费授权阻塞，因此 VS2 的核心主张——在同等预算下，选择性花费比均匀或周期性花费更好地保持保真度——**尚未被接受**。

本项目并不打算精确重放已记录的历史。它要产出的是另一种历史：其中个体的行为、制度、技术与宏观结果，在显式的人力、物质与信息约束下仍然可以被因果解释。

## 状态

P0 基底是实验地基，而现行计划是纵向切片路线图（[docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md)；VS1 已交付，VS2 的注意力机制已交付，VS2 的核心主张未被接受）。Go 是暂定的实现语言。S0–S5 提供了模拟原语、不可变组件、内核、调度，以及一个行动者安全的意图边界。S6 增加了一个有界的自主生存演示。单机 P0 检查点现在能持久化其被接受的历史、调度器状态与带类型的尝试日志。完整的 P0 生存仍未完成。语言模型集成**不是**被推后的未来阶段工作：它是现行计划第一个切片（VS1）的主题（[docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md)）。

## 快速开始

需要 Go 1.24 或更新版本（已在 Go 1.24.13 上验证）；模块只使用 Go 标准库，因此除工具链外无需安装任何东西。

```sh
git clone https://github.com/ZisIsNotZis/agentworld.git
cd agentworld

# 一个有界、可复现的生存世界：48 个行动者，12 个模拟小时，4 个工作线程。
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1

# 一个更丰富的世界：土地、耕作、粮仓与出生，作为单个封闭格子运行。
mkdir -p .tmp
go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=both -out=.tmp/natural-r8-s0-w1.json
```

两条命令都会把 JSON 报告打印到标准输出；第二条还会写入 `.tmp/`。除非明确授权真实模型运行，否则不会访问网络。

## 现在能做什么

- **确定性事件驱动内核**——每一次状态变化都是追加式历史中一条被授权、带时间戳的事件，被接受的历史重放后得到同一状态（`internal/kernel`）。
- **持久行动者，有限算力**——显式的行动者纤程跑在工作线程池上；休眠的行动者占用存储但不占执行槽位，唤醒队列决定谁运行（`internal/scheduler`）。
- **一套组件协议，两种实现**——高频状态放在编译期类型中，演进中或稀疏的状态放在注册表定义的动态存储中，并有一套一致性测试证明二者可观测语义等价（`internal/component`）。
- **检查点、续跑与分叉**——带摘要与父系血缘的不可变包，可在新进程中续跑（即使工作线程数不同），也可用声明式干预分叉，于是两个未来共享一段经过验证的前缀（`internal/checkpoint`、`internal/world`）。
- **版本化策略数据**——策略是版本化、经过编解码校验的可执行数据（`survival`、`foodflow`、`socialfood`、`capacity`、`reproduction`，以及世界机制 v5–v14 上的 `natural/5` … `natural/9`），而不是靠改代码来改变行为（`internal/strategy`）。
- **五个版本化的合成世界**——食物流（v1）、社会食物转移（v2）、生产能力（v3）、繁殖与遗传（v4）、自然生产上限（v5，含机制轴 v5–v14），每个都有预先声明的矩阵、冻结的守恒恒等式与可证伪的判据带（`cmd/foodflow`、`cmd/socialfood`、`cmd/capacity`、`cmd/reproduction`、`cmd/natural`）。
- **语言模型认知层**——在观测边界之下由行动者发起的 Role A 决策、带生命周期与回滚的 Role B 版本化策略补丁、调用记录账本、精确重放、认知预算，以及按价值的优先级分配（`internal/cognition`）。
- **两个封闭实验运行器**——VS1（把心智放进回路）与 VS2（注意力）用确定性的假模型跑出各自声明的实验臂与格子，并输出冻结的退出判据，因此机制在零花费下即可被测试（`cmd/vs1`、`cmd/vs2`）。
- **测量本身就是输出的一部分**——每份报告都带墙钟/CPU 纳秒、峰值 RSS、序列化字节数，以及在未调用模型时的零 token 断言。

## 如何运作

世界等于内核加纤程。一个行动者纤程在有意义的时刻被唤醒，它的策略只读取自己的观测边界，然后提交一个*意图*——永远不是直接改写状态。内核拿当前规则与快照校验这个意图，解决并发效应，把由此产生的事件提交进不可变历史，组件转移再产出下一个状态。日志记录了什么被接受、什么被拒绝以及原因；检查点把状态、历史、调度器待办与日志一起捕获，这正是让重放、续跑与分叉成为同一套机制而不是三件功能的原因。

这个核心之外是各个版本化的世界。`internal/world` 中的每个世界（`foodflow`、`socialfood`、`capacity`、`reproduction`、`natural`）拥有自己声明的一套世界规则、守恒契约、运行器与检查点绑定，于是后来的版本可以添加机制而不悄悄改变更早一次运行的字节：策略引用、机制与夹具从头到尾都是运行身份的一部分，任何不匹配都会在任何东西被发布之前被拒绝。

认知层坐在同一个接缝之上。Role A 回答行动者声明的决策请求；Role B 提出策略补丁，补丁必须通过带类型的沙箱才能成为活动代。每个模型答案都被记录、计价并可重放，因此一次实验可以只凭其账本重新打分，而不必再次花钱。

```text
cmd/                  每个试点或实验一个二进制
  survival/           有界的 P0 生存演示（S6）
  foodflow/ socialfood/ capacity/ reproduction/ natural/
                      版本化合成世界 v1 ... v5（含机制/策略变体）
  vs1/ vs2/           语言模型实验运行器（默认封闭）
internal/
  sim/                原语：id、模拟时间、确定性随机、带类型的值
  component/          统一组件协议：模式、快照、转移、一致性
  kernel/             事件日志、不可变历史、重放、事务式转移
  scheduler/          行动者纤程、唤醒队列、活动、有界工作线程
  strategy/           版本化策略数据与编解码
  world/              各版本的规则、契约、日志、运行器、检查点绑定
  checkpoint/         不可变存储、摘要、血缘
  cognition/          模型客户端与网关、协议、账本、重放、预算、优先级、Role A/B
  experiment/         保真度度量与投影
docs/                 愿景、原则、架构、规格、验证方法、ADR
.scratch/             每个工单的规格与其引用的记录证据
```

## 证据试点（合成、依赖模型条件）

每个世界版本都随附它被声明要跑的矩阵，以及那个矩阵得出的裁决——从不中途调参。以下小节是试点记录；其中的数字可以用给出的命令复现。

### 合成可再生食物试点（与 S6 相互独立）

固定的 food-flow v1 夹具包含 16 个行动者、两块八槽地块与 168 个模拟小时。它是关于内部记账与所声明策略的**合成、依赖模型条件**的证据，不是历史标定，也不是 P0 在 1,000 行动者/数十年尺度上表现的证据。`-q` 是**每地块每小时**的产量（8 充裕、3 稀缺、0 为零）；`-hours` 是固定 168 小时夹具的绝对报告前缀，不是新的模拟规则。

```sh
go run ./cmd/foodflow -q=8 -seed=0 -workers=1 -hours=168
go run ./cmd/foodflow -q=3 -seed=7 -workers=4 -hours=168
go run ./cmd/foodflow -q=0 -seed=0 -workers=1 -hours=168
# 在完成的第 4 小时写不可变检查点，然后在另一个进程中续跑：
mkdir -p .tmp
go run ./cmd/foodflow -q=3 -seed=7 -workers=1 -hours=168 -checkpoint-hour=4 -checkpoint=.tmp/foodflow-h4.bundle
go run ./cmd/foodflow -q=3 -seed=7 -workers=4 -hours=168 -resume=.tmp/foodflow-h4.bundle
```

每次发布都请使用**新的**检查点文件名。不给 `-checkpoint` 时，报告器会在被忽略的 `.tmp/` 下创建并删除一个测量包；给了 `-checkpoint` 而没有 `-checkpoint-hour` 时，就在 `-hours` 处发布。JSON 包含 h0、到达时的 h1/4/12/24/72/120/168 以及人口变化边界，地块存量/产量/采集/消耗，行动者 1/8/16 的能量与独立的饥饿度以及前一小时的尝试，被接受事件的链接，被拒绝的 Gather 总数与分开的拒绝原因计数，以及精确的食物/能量收支。已过小时的基础需求只作用于仍存活的行动者：一旦停下，他们的能量与饥饿度保持固定，即使世界产量脉冲仍在继续。报告给出墙钟/进程 CPU 纳秒、Linux 峰值 RSS、采样的活跃 Go 堆峰值、序列化的事件体/历史/日志/检查点字节数，以及零模型 token。计时包含构建或恢复、执行、验证与序列化/检查点发布，但不含 `go run` 的编译与 JSON 输出。完整夹具结果与局限：[试点证据](.scratch/10-food-flow-pilot/spec.md)。

### 社会食物 v2 试点（合成、依赖模型条件）

单独版本化的 social-food v2 夹具在同样的 16 行动者/两地块/168 小时形状上，增加了私有的定向食物请求与独立的捐赠者同意：一个从未采集过的接受者可以持有并吃掉一件带有来源标记的赠予单位，而每一次赠予都是原子的「捐赠者袋 −1 / 接受者袋 +1」转移，食物总量变化为零。`cmd/socialfood` 从不修改策略字节：它发布一个经过验证的中性 h0 检查点，在两个子分支的清单血缘中记录显式的启用/停用干预，把每个子分支续跑到 `-hours`，并按分支报告。

```sh
mkdir -p .tmp
go run ./cmd/socialfood -q=3 -seed=0 -workers=1 -hours=24 -out=.tmp/socialfood-q3-s0-w1-h24.json
go run ./cmd/socialfood -q=3 -seed=7 -workers=4 -hours=24 -out=.tmp/socialfood-q3-s7-w4-h24.json
go run ./cmd/socialfood -q=8 -seed=0 -workers=1 -hours=24 -out=.tmp/socialfood-q8-s0-w1-h24.json
go run ./cmd/socialfood -q=0 -seed=0 -workers=4 -hours=12 -out=.tmp/socialfood-q0-s0-w4-h12.json
# 仅在启用分支上测量一个完整时段的开销：
go run ./cmd/socialfood -q=3 -seed=0 -workers=1 -hours=168 -branch=enabled -out=.tmp/socialfood-q3-s0-w1-h168-enabled.json
```

`-branch` 选择 `enabled`、`disabled` 或 `both`（默认）；包（`social-neutral-h0.bundle`、`social-branch-enabled.bundle`、`social-branch-disabled.bundle`）发布到 `-checkpoint-dir`，该目录必须不存在（默认：一个被保留并报告的全新临时目录）。JSON 输出到标准输出，若给了 `-out` 则同时写到一个此前不存在的文件；既有输出不会被复用或覆盖，多余的位置参数会被拒绝。报告标注为 `synthetic/model-conditional`，包含中性根摘要与两个分支的摘要（分叉血缘）、每小时的存活检查点、每个行动者的消耗/能量/饥饿/存活与账本合计、行动者 1/8/16 的逐小时轨迹、被见证的请求/赠予/拒绝/过期计数以及一个有特权的「声明对真相」赠予审计、有向二元组援助比例 `(gifts+1)/(requests+2)`、每地块消耗基尼系数（总量为零定义为不平等为零）、赠予捐赠者的单位成本、事件/历史/日志/检查点字节数以及墙钟/CPU/峰值 RSS 开销与零模型 token。当两个分支都运行时，一个比较小节记录存活轨迹差异、幸存者集合与捐赠者成本。完整的预先声明矩阵结果与局限：[工单证据](.scratch/12-social-food-transfer/spec.md)。

### 生产能力 v3 试点（合成、依赖模型条件）

单独版本化的 capacity v3 夹具在同样的 16 行动者/两地块/168 小时形状上，增加了每个行动者私有的工地与粮仓：一次建造（20 分钟，占用一个持有的袋单位）投入在建工程，两次建造完成一点资本（k ∈ [0,8]），每个脉冲恰好向拥有者的粮仓 [0,8] 产出 1 单位食物，并在恰好六个逐小时折旧脉冲后失效；野生生产与 v1 的进食生理保持不变。`cmd/capacity` 从不修改策略字节：它发布一个经过验证的中性 h0 检查点，把启用/停用干预（以及创始人-B 的初始禀赋（endowment）——行动者 1 的粮仓 4，仅限启用分支，因为在冻结的 h0 守恒下资本禀赋不可能成立）记录为共享父摘要的分支子代，把每个子分支续跑到 `-hours`，并按分支报告。

```sh
mkdir -p .tmp
go run ./cmd/capacity -q=8 -seed=0 -workers=1 -hours=168 -branch=enabled -out=.tmp/capacity-q8-s0-w1-enabled.json
go run ./cmd/capacity -q=5 -seed=7 -workers=4 -hours=168 -branch=both -out=.tmp/capacity-q5-s7-w4.json
go run ./cmd/capacity -q=0 -seed=0 -workers=1 -hours=12 -branch=both -out=.tmp/capacity-q0-s0-w1.json
# 记录在案的创始人-B 变体（仅限启用分支）：
go run ./cmd/capacity -q=3 -seed=0 -workers=1 -hours=168 -branch=enabled -founder=B -out=.tmp/capacity-q3-s0-w1-enabled-founderB.json
```

`-branch` 选择 `enabled`、`disabled` 或 `both`（默认）；包（`capacity-neutral-h0.bundle`、`capacity-branch-enabled.bundle`、`capacity-branch-disabled.bundle`）发布到 `-checkpoint-dir`，该目录必须不存在（默认：一个被保留并报告的全新临时目录）。JSON 输出到标准输出，若给了 `-out` 则同时写到一个此前不存在的文件；既有输出不会被复用或覆盖，多余的位置参数会被拒绝。报告标注为 `synthetic/model-conditional`，包含中性根摘要与各分支摘要（分叉血缘）、每小时存活检查点与逐小时守恒再验证、每个行动者的能量/饥饿/资本/在建工程/粮仓存量/投入单位/已产生与已折旧的点数/产出总量与未实现量/储存餐数与野生餐数/存活小时数、聚合账本、预先声明的 `surplus-flow-48h` 窗口（第 120–168 小时的资本产出减去储存餐数，在 h168 以下按可用小时截断）、每地块野生消耗基尼与资本持有基尼（总量为零定义为不平等为零）、带类型的建造/配对进餐/回退与采集拒绝计数、恢复后的事件链接检查、事件/历史/日志/检查点字节数以及墙钟/CPU/峰值 RSS 开销与零模型 token。停用分支或 q0 格子若记录了任何资本事件，运行即失败。完整的预先声明矩阵结果与证伪裁决：[工单证据](.scratch/13-productive-capacity/spec.md)。

### 繁殖与遗传 v4 试点（合成、依赖模型条件）

单独版本化的 reproduction v4 夹具增加了运行时实体分配器、可遗传的三位点整数基因组（位点 m/y/w，等位基因 [5,7]，中性取 6，在中性取值下与 v3 字节完全一致）、由冻结的剩余指标把关的双亲同意出生、带有不可变寿命（96+抽取 mod 48）的衰老，以及只由基因组父行推导的亲属关系。`cmd/reproduction` 从不修改策略字节：它发布一个在**出生停用**下捕获的、经过验证的中性 h0 检查点（v4 的 h0 脉冲按冻结设计在出生上是不对称的，因此停用的根是唯一合法的分支父本），把「出生停用」（同标志血缘）与「出生启用」干预记录为共享父摘要的分支子代（停用方向被持久化层拒绝），把每个子分支续跑到 `-hours`，并按分支报告。

```sh
mkdir -p .tmp
go run ./cmd/reproduction -q=8 -seed=0 -workers=1 -hours=168 -births=both -out=.tmp/reproduction-q8-s0-w1.json
go run ./cmd/reproduction -q=5 -seed=7 -workers=4 -hours=168 -births=both -out=.tmp/reproduction-q5-s7-w4.json
go run ./cmd/reproduction -q=0 -seed=0 -workers=1 -hours=12 -births=both -out=.tmp/reproduction-q0-s0-w1.json
# 记录在案的基因组变体夹具（行动者 1-8 位点 m=7，9-16 位点 m=5；这是运行器选项而非策略变更；仅限出生启用分支）：
go run ./cmd/reproduction -q=5 -seed=0 -workers=1 -hours=168 -births=enabled -genome-variant -out=.tmp/reproduction-gv-q5-s0-w1.json
# 与记录在案的工单 13 capacity 报告做相等性夹具（仅限停用的中性格子）：
go run ./cmd/reproduction -q=3 -seed=0 -workers=1 -hours=95 -births=disabled -v3-reference=.tmp/capacity-q3-s0-w1-enabled.json -out=.tmp/reproduction-equality-q3-s0-w1-h95.json
```

`-births` 选择 `enabled`、`disabled` 或 `both`（默认）；包（`reproduction-neutral-h0.bundle`、`reproduction-branch-enabled.bundle`、`reproduction-branch-disabled.bundle`）发布到 `-checkpoint-dir`，该目录必须不存在（默认：一个被保留并报告的全新临时目录）。JSON 输出到标准输出，若给了 `-out` 则同时写到一个此前不存在的文件；既有输出不会被复用或覆盖，多余的位置参数会被拒绝。报告标注为 `synthetic/model-conditional`，包含中性根摘要与各分支摘要（分叉血缘）、人口轨迹（逐小时存活数、每小时出生与归因死亡、灭绝小时）、基因组多样性（存活行动者两两 L1 的整数精确均值、不同基因组数、冻结小时 h1/4/12/24/72/96/120/144/168 上每个位点的等位基因频率）、亲属密度（相关的存活配对比例、有限亲缘距离均值、每个行动者的存活子女数、近亲繁殖拒绝计数）、出生经济学（出生数、申报数、带类型的拒绝直方图、有特权的「声明对真相」分歧）、带基因组与血缘的每个行动者账本、带类型的带恢复事件链接检查的事件计数、每小时再验证的扩展冻结守恒、事件/历史/日志/检查点字节数以及墙钟/CPU/峰值 RSS 开销与零模型 token。停用出生的分支或记录了任何出生证据的 q0 格子会使运行失败；q0 在两个分支上都必须在 h11 前灭绝。声明的创始人夹具是运行身份的一部分：检查点清单与日志验证把它钉住，因此变体选项下发布的中性根、以及默认选项下发布的变体根，都会被拒绝且不发布任何东西。完整的预先声明矩阵结果、两项证伪裁决与缺陷/修复记录：[工单证据](.scratch/14-reproduction-genetics/spec.md)。

### 自然上限 v5 试点（合成、依赖模型条件）

单独版本化的 natural v5 夹具把生产重新落到土地上：P 块地块各有 8 个槽位（野生或耕作），唯一的自然生产输入是每地块每小时的地块再生预算 R，按「先耕作后野生」分配（余量标记为未实现），粮仓由声明的腐烂率（S=24）而不是容量上限约束，出生没有上限并使用分配器签发的实体 id，算力天花板使用冻结的内核/调度器/运行器常量，逐小时以余量表（headroom ledger）插桩，并带有一个会 fail-loud 停机的带类型 `NaturalCeilingHit`。`cmd/natural` 从不修改策略字节：它发布一个在**出生停用**下捕获的、经过验证的中性 h0 检查点（v5 的 h0 脉冲按冻结设计在出生上是不对称的），把「出生停用」（同标志血缘）与「出生启用」干预记录为共享父摘要的分支子代（停用方向被持久化层拒绝），并把每个子分支续跑到 `-hours`。

```sh
go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=both -out=.tmp/natural-A-r8-s0-w1.json
go run ./cmd/natural -r=0 -seed=0 -workers=1 -hours=12 -births=both -out=.tmp/natural-B-r0-s0-w1.json
go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=disabled -cultivation-disabled -out=.tmp/natural-D-ablation-r8-s0-w1.json
go run ./cmd/natural -r=8 -seed=0 -workers=1 -hours=168 -births=enabled -founder-tenure -out=.tmp/natural-E-fixture-r8-s0-w1.json
go run ./cmd/natural -patches=32 -r=8 -seed=0 -workers=1 -hours=168 -births=enabled -out=.tmp/natural-F-ceiling-p32-r8-s0-w1.json
```

`-births` 选择 `enabled`、`disabled` 或 `both`（默认）；`-founder-tenure` 选择持久化通道声明的创始人任期夹具创世（仅限出生启用分支；这是唯一能让有机出生可达的创世——它发布的根是该夹具第一个脉冲边界的续跑点，因此轨迹从 h2 开始）；`-cultivation-disabled` 选择声明的消融干预（仅限出生停用分支）：冻结策略不变，运行器把每一次 Build 选择都送进文档化的拒绝回路，任何建造事件都是硬缺陷；`-policy` 选择冻结的行为引用，`natural/5`（默认，与所有已记录的运行字节兼容）、`natural/6`（在不变的 v5 世界之上的剩余行为策略）、供给臂 `natural/7` 与 `natural/7b`（两者都要求机制 v6 或 v7），或工资契约臂 `natural/9`（要求机制 v7）；`-mechanism` 选择声明的世界轴，`v5`（默认，与所有已记录的运行字节兼容）、`v6`（规则 515 的供给世界）、`v7`（工资契约世界，规则 516–519）、`v8`（全带配偶池与家庭同意门槛）、`v9`（v8 加声明的粮仓产出）、`v10`（轮转创始带）、`v11`（按产出缩放的野生填充）、`v12`（双能量进餐）、`v13`（v12 加声明的出生价格）或 `v14`（v13 加公地底线：一块地块至少保留一半槽位为野生），其中 `-yield` 设定机制 v9 的粮仓产出（0 = 冻结的固定授予 1；v9 及以后可取 1..8），`-birth-cost` 设定机制 v13 的出生价格（0 = 冻结价格 2；仅 v13 下可取 1..8）；非法的策略/机制组合会在发布前被拒绝；`-founders` 选择工单 18 声明的创始带，即从冻结的创始人流前缀中实体化的创始人行数——`0`（默认）是完整冻结块 8·patches 的默认写法，并让每一次已记录的运行保持字节一致，其他任何值必须落在 1..8·patches 内（在任何发布之前通过 `world.NaturalFoundersValid` 做范围检查，创始人任期夹具还额外拒绝 f=1，因为它的连贯史前段落坐实了创始人 1 与 2）。策略引用、机制与创始带从头到尾都是运行身份（日志配置、检查点清单、双向不匹配拒绝）。在 `natural/6` 下，报告器纯粹以追加方式增加工单 16 的报告小节（每个行动者过去 24 小时的剩余流量及其持续小时计数器与首次持续剩余耗时、资本持续性、首次建造/槽位/出生的耗时、出生付款后 24 小时内死亡的父母，以及预先声明的 v6 判据带 A2/A3/A4 与 B–F）；在 `natural/7` 或 `natural/7b` 下则改为增加工单 17 的 v7 小节——SF′（把供给披露为提取）、每个行动者带真实运行最大值的**门控**持续 12 小时计数器与不门控的小时级并集计数器、每个分支与每个行动者的 `provisioned` 合计、每格配对进餐、首次供给/存粮小时/k≥2 持有/持续剩余/出生申报/出生的耗时、资本持续性与父母死亡小节、V7-A5 的非普遍性证据，以及预先声明的 v7 判据带 V7-A2/A3/A4/A5 与 V7-B–F；在声明的非默认创始带下，它还增加工单 18 的 v8 密度小节——该轴与 `density_world = f/(8·P)`、每地块座位与每小时口数及每地块 Σk、带归属地块的每行动者剩余计数器、世代索引与出生队列表（出生小时、父母、申报亲缘距离）、在 G3 内部交叉校验的 8·births 禀赋账本、父母死亡与配对进餐金丝雀、每地块的首次耗时字段，以及预先声明的 v8 判据带（有机格子的 V8-A2/A2b/A3/A4/A5/K，格子形状的 V8-B/V8-C2/V8-D/V8-E12/V8-F，其中出生停用分支以 V8-C2 作为其主判据带）。默认带下的 natural/5 报告不包含任何追加键。包（`natural-neutral-h0.bundle`、`natural-branch-enabled.bundle`、`natural-branch-disabled.bundle`、`natural-fixture-root.bundle`）发布到 `-checkpoint-dir`，该目录必须不存在（默认：一个被保留并报告的全新临时目录）；JSON 输出到标准输出，若给了 `-out` 则同时写到一个此前不存在的文件；既有输出不会被复用或覆盖，多余的位置参数会被拒绝。报告标注为 `synthetic/model-conditional`，包含根摘要与各分支摘要（分叉血缘）、人口轨迹（逐小时存活数、每小时出生与归因死亡、灭绝小时）、逐小时再生分配（耕作/野生/未实现）与每地块耕作槽位、粮仓流入/腐烂/储存的合计、马尔萨斯指数（过去 24 小时 Σ进餐 − Σ基础需求）、每个行动者账本（能量、资本、在建工程、粮仓、任期计数）、带恢复事件链接检查的带类型事件计数、每小时独立再验证的冻结守恒恒等式（`-mechanism=v6` 下的机制感知 G2a′ 形式）、完整 14 行的算力天花板余量表、每一个带类型 `NaturalCeilingHit` 及其 P15 裁决分类（超出测试框架 vs 设计缺陷 vs 实现缺陷 vs 运行时缺陷，并带上冻结表格要求的证据字段），以及事件/历史/日志/检查点字节数与墙钟/CPU/峰值 RSS 开销，它们对照预先声明的 v4 相对预算衡量（每模拟小时 CPU ≤ 3× v4，每模拟小时字节数 ≤ 2× v4；未达标者原样报告）。停机式天花板命中是一等结果：运行记录该命中，然后跑一个 workers-4 的双生子（当该格在 4 上运行时则用 workers-1），截断在命中小时处做重放相等性检查。冻结判据带在每份报告中逐字求值；被证伪的判据带照实报告，从不调参。完整的预先声明矩阵结果、A 带证伪、天花板裁决与记录在案的贫困陷阱：[工单证据](.scratch/15-natural-limits/spec.md)；工单 16 的 v6 矩阵、其绿色对照门与证伪裁决：[工单 16 证据](.scratch/16-surplus-behaviour/spec.md)；工单 17 的机制矩阵、两个绿色对照门、被如实记录的空结果与自我证伪结论：[工单 17 证据](.scratch/17-surplus-mechanism/spec.md)；工单 18 的创始带密度曲线及其预先声明的 V8 判据带：[工单 18 证据](.scratch/18-surplus-via-reproduction/spec.md)。同一世界的后续机制重标定——密度网格（[48](.scratch/48-natural12-grid/spec.md)）、v8 配偶池（[49](.scratch/49-mechanism-v8/spec.md)）、v9 产出（[50](.scratch/50-mechanism-v9-yield/spec.md)）、v11 野生产出（[51](.scratch/51-mechanism-v11-wild-yield/spec.md)）、v12 进餐（[52](.scratch/52-mechanism-v12-meal/spec.md)）、v13 出生价格（[53](.scratch/53-mechanism-v13-birthcost/spec.md)）与 v14 公地（[54](.scratch/54-mechanism-v14-commons/spec.md)）——都在 `.scratch/` 中按工单逐项记录。

## 开发

需要 Go 1.24 或更新版本。当前的基底已在 Go 1.24.13 上验证。模块只使用 Go 标准库，因此没有需要安装的开发依赖。

```sh
make fmt-check   # 对每个 .go 文件跑 gofmt，并带一个证明该检查会失败的探针
make test        # 完整测试套件（30 分钟计算上限）
make race        # 在同一套件上跑竞态检测（90 分钟计算上限）
make vet
make build
make bench       # 组件点读 / 扫描 / 投影基准
```

`make check` 运行除基准之外的所有门。`test` 与 `race` 的超时上限来自在门禁机器上实测的墙钟时间，而不是任意的上限（[docs/VALIDATION.md](docs/VALIDATION.md)，P15）。

快速开始里那个生存演示的 JSON 报告记录初始/最终能量、食物与饥饿度分布、被接受的动作与被拒绝的原因、每动作一单位的耗散、模拟小时数、进程 CPU 与墙钟时长（`cpu_time_ns` 与 `wall_time_ns`）、零 token 成本，以及被接受状态的重放。用 `-eat-cost=0` 或另一个 `-seed` 对比，可以检验敏感性。重放重建被接受的组件状态；检查点还会额外验证带类型的历史尝试、策略绑定与待办的调度器状态。

在第 5 个小时后创建检查点，在新进程中续跑（包括用不同的工作线程数），或带着改变后的 Eat 权重分叉：

```sh
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=1 -eat-cost=1 -checkpoint-hour=5 -checkpoint=run.checkpoint
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1 -resume=run.checkpoint
go run ./cmd/survival -actors=48 -seed=7 -hours=12 -workers=4 -eat-cost=1 -resume=run.checkpoint -branch-eat-cost=0 -checkpoint=fork.checkpoint
```

检查点路径必须是已存在目录下的新文件。恢复时的 `-eat-cost` 命名原始运行的权重；`-branch-eat-cost` 只在经过验证的前缀之后生效。分叉命令在继续跑到最终报告之前先发布分叉边界。恢复时 `-checkpoint-hour` 计的是额外步数；省略它就一直跑到没有唤醒为止。来自检查点命令的报告不测量 CPU/墙钟时间（其计时字段为零）。包摘要与父摘要能发现损坏并识别血缘，但不认证对抗性改写；若这一点重要，请独立保存期望的摘要。

生成的输出属于被忽略的 `.tmp/` 目录；其下的任何内容都不被跟踪。

## 参与贡献

欢迎贡献——简短版见 [CONTRIBUTING.md](CONTRIBUTING.md)。实际要求是：推送前跑 `make check`，把变更的证据与变更放在一起，并且不要为了让某次运行通过而调整冻结的判据带、夹具或度量。被证伪的判据带是一个结果，要如实报告，绝不修改。

## 路线图

**已实现**

- P0 基底：模拟原语、统一组件协议、带重放的事件内核、行动者纤程与有界调度、行动者安全的意图边界，以及带分支血缘的持久检查点。
- 五个版本化的合成世界（v1 食物流 → v5 自然生产上限）及其声明的矩阵、守恒契约与记录在案的裁决，另有直到 v14 的机制与策略变体（产出轴、进餐、声明的出生价格、公地规则）。
- 认知栈：模型客户端与网关、Role A 决策接缝、Role B 版本化补丁生命周期、调用记录账本、精确重放、认知预算与按价值的优先级分配。
- VS1（M1）：机制完整，十一条冻结退出判据在封闭证据下全部 PASS。
- VS2（M2）：分配机制及其实验框架，已通过封闭验证；一个预先注册的曲线判据，把推理努力深度与 3 次重复作为一等轴；测量工具（15 格预算矩阵约 17 分钟、约 0.57 美元）。
- `W1_world_runs`：声明的默认世界（机制 v14、`natural/14`、产出 1、出生价格 2、`f10-s0`、`H = 144`）在其声明格子上无认知通过——10 个创始人中存活 11 个、出生 12、饿死 1、老死 10——128 格回归已记录，每个失败格子都注明其失败子句（[docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md)，ticket 55）。

**未被接受 / 未决**

- VS2 的核心主张——在同等预算下，选择性花费比均匀或周期性花费更好地保持保真度。真实矩阵产出的是空结果与一次不可复制，且每个被打分的臂都与无 AI 对照 `A0` 持平或更差，而 `A0` 自身只剩一个幸存者：参照物本身就是一场死绝，因此那些空结果两边都不能算作证据（[docs/VERTICAL-SLICES.md](docs/VERTICAL-SLICES.md)，M2 行）。
- 工作顺序的下一项 `W2_reference_luna_max`：在 W1 夹具上产出一组 `luna:max` 常开参考曲线，并报告其价格。运行器仍带着旧夹具，因此须先把声明的世界接入其中。
- 两个切片的真实模型分支，被花费授权阻塞。

**计划**（声明的顺序；后继项不在前驱项的退出被测量之前开始）

1. `W2_reference_luna_max`——每个决策都由声明的参考模型在最高推理努力下作答，作为基准，而非作为真值。
2. `W3_axes`——模型、推理努力、介入时机与稳定性的效应。
3. `W4_curve_match`——以逐小时 KPI 曲线对照参考曲线（模糊匹配）来评判，而不是只看终局结果。
4. M2 之后：VS3 进化（策略在受审计的变异下可测地变好，并带出处与回滚）、VS4 赌注与规模、VS5 社会、VS6 历史。

长期目标是随着可用算力能撑到多高就做多高的自演化宇宙：行动者在运行中改变自己的行为逻辑，其测得的效应被报告而不是被断言；每一处修改都持久、可归属，且外部第三方能从账本与重放中验证，而无需相信任何行动者的自述；每一个决策都在行动者自己的观测边界之内作出。里程碑与切片是同一个对象——M*n* = VS*n*——整条链只在结束时报告一次。

## 文档

- [现行计划（纵向切片）](docs/VERTICAL-SLICES.md)
- [愿景](docs/VISION.md)
- [设计原则](docs/PRINCIPLES.md)
- [概念架构](docs/ARCHITECTURE.md)
- [P0 可执行规格](docs/P0-SPEC.md)
- [P0 实现计划](docs/P0-IMPLEMENTATION-PLAN.md)
- [验证](docs/VALIDATION.md)
- [路线图（历史阶段计划）](docs/ROADMAP.md)
- [开放问题](docs/OPEN-QUESTIONS.md)
- [术语表](docs/GLOSSARY.md)
- [变更日志（事件记录）](CHANGELOG.md)

## 材料性局限

- 一个看起来合理的模拟社会，并不能作为同一策略在现实中同样有效的证据。
- 历史标定检验的是机制与分布，而不是精确的事件重演。
- 未知的自然规律无法仅凭语言模型的判断来验证。
- 任何自我修改都必须保持版本化、可测试、可审计、可回滚。

## 许可

Agent World 以 [GNU Affero 通用公共许可证 v3.0](LICENSE)（AGPL-3.0）发布。
