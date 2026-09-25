# ZCode 0.16.9 设置面只读探针记录(zcprobe -settings)

> 生成:2026-09-25 工具:daemon/cmd/zcprobe -settings(node 直拉 zcode.cjs app-server --stdio,零写操作)
> 工作区:临时目录(探针结束即毁);全程未调用 session/setMode / setModel / setThoughtLevel。

## 应答键形状(0.16.9 运行时实测)

session/create、session/subscribe(快照)、session/resume 三条路径的 `settings`
对象键清单**一致**,共 4 键:

| 键 | 实测形状 |
|---|---|
| `mode` | `{"current":"build"}`(会话模式,create 参数 mode 的回显) |
| `model` | `{available:[{ref,label,providerLabel,contextWindow,maxOutputTokens,reasoning:{levels,defaultLevel},properties}],current:{ref,options:{reasoningLevel}},lastUsed}` |
| `permission` | `{"mode":"build"}` |
| `thoughtLevel` | `{available:[{label,value}…],current:"max",enabled:true}` |

其它实测:
- `runtime/capabilities` → `{"independentPlanState":true}`(与 daemon 独立计划态判定同源);
- 订阅窗口 3s 内 session/event×0、state.updated×0(纯只读路径无设置类事件)。

## 结论(对设置透出的约束)

1. **permission.mode 运行时在场**:`settings.permission.mode="build"`(0.16.9 实测),
   与 settings.mode.current 同值。daemon 既有链路(session/resume →
   parseZcodeModeSnapshot 读 settings.mode.current + session/setMode 写)已经在
   设置页「ZCode 权限默认值 / 当前会话权限」呈现并可写(NativePermissionsSection;
   写通道 session/setMode 为 daemon 既有已用通道)。**本轮无新增面可做** —— 不重复
   呈现同一开关,避免双入口。
2. **settings.model 的 available/levels 是动态模型目录** —— 但该目录早已由
   parseZcodeModels 消费进 ListModels(手机端模型/思考档选择已在用),不另开通道。
3. **无其它未知设置键**:0.16.9 的 settings 面就是 mode/model/permission/thoughtLevel
   四键,没有 bundle 静态提取之外的隐藏键;没有"可选项清单"形状的 permission 档位表
   (permission.mode 只有当前值,档位表来自包内 schema,zcodeapp.ModeSchema 已消费)。
