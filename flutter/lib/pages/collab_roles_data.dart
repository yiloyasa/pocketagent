// 协同角色（Collab Role）的纯函数层：id 校验 / 显示名回落 / 摘要行拼接 +
// agent 下拉的能力位提取 + 页面模型映射。
//
// 【为什么独立成层】这些规则最容易被"顺手改个条件"悄悄反转，而反转后没有任何
// 报错，只能等用户派发失败、看到「未定义的协同角色 xx」才暴露（Android
// CollabRolesLogic.kt 的注释原话）。集中到这里用单测钉死，改坏会在 test
// 阶段直接红。
//
// 【为什么直接用 gen stub 的 `CollabRole` 而不是再造一个页面模型类】
// proto `CollabRole` 只有八个 string 字段，且页面（列表行 / 编辑表单）每个
// 字段都要原样读写——中间加一层投影只会多一次「字段漏抄」的机会，还会让
// save() 的往返多一次转换（daemon 回显的落库真值必须与表单逐字段一致）。
// threads_data.dart 的 ThreadSummary 是另一回事：proto `Thread` 有 21 个
// 字段而行渲染只要六个，投影是**收窄**；这里没有可收窄的。store 与页面
// 因此共用 gen 的 `CollabRole`（双端同源，Android 侧同样直接用
// pocketagent.v1.CollabRole）。
//
// 【校验口径与 daemon 同源但职责不同】这里做的是**客户端前置校验**：让保存
// 按钮在 id 非法时直接禁用并给一句人话，用户不必点了保存、跨进程 round-trip
// 一圈才被告知。daemon 端若仍返回 InvalidArgument（例如并发边界下的 trim
// 差异），其原文会经 store 原样透传，前端禁用只是「第一道闸门」，不是「吞掉
// 后端错误」的借口（Android CollabRolesLogic.kt:10-16 的设计照搬）。

import '../gen/pocketagent/v1/thread.pb.dart';
import 'collab_session_data.dart' as session_data;

/// role.id 长度上限，与 daemon 契约一致（`maxCollabRoleIDLen`，
/// daemon collab_roles.go:32；超过返回人话错误，不截断、不静默）。
const int kCollabRoleIdMax = 64;

/// 校验 role.id（纯函数，单测钉死）。
///
/// @return null = 合法；否则返回**人话**原因（UI 直接展示这串文字）。
///
/// 【空格判定用 RegExp 全空白集而非只判 ' '】slug 型 id 里无论空格还是
/// tab/换行都该拒，统一拦下；文案沿用「不能含空格」（用户视角的空格即任意
/// 空白）——与 Kotlin `validateCollabRoleId` 逐条对位（它用
/// `it.isWhitespace()`，Dart 侧等价于 `contains(RegExp(r'\s'))`）。
///
/// 【口径对账 daemon collab_roles.go:111-124】三条判定同序同义：空 →
/// 过长（**rune 数**，与 Go `len([]rune(id))` 同口径，中文/emoji 不按字节截）→
/// 含空白。文案取 Kotlin 侧的短句（UI 内嵌显示，不带 daemon 的长解释段）。
String? validateCollabRoleId(String id) {
  final String t = id.trim();
  if (t.isEmpty) return '协同角色 id 不能为空';
  if (RegExp(r'\s').hasMatch(t)) return '协同角色 id 不能含空格';
  if (t.runes.length > kCollabRoleIdMax) {
    return '协同角色 id 过长(上限 $kCollabRoleIdMax)';
  }
  return null;
}

/// 角色显示名回落（纯函数）：name 非空用 name，否则回落 id。
///
/// 列表/卡片靠它「一眼看到对应关系」；两者都空（理论不发生，daemon 要求 id
/// 必填）回落 id 原文（Android `collabRoleDisplayName` 同款）。
///
/// 【实现唯一出处在 collab_session_data.dart】字符串版
/// `collabRoleDisplayName(name, id)` 与本函数逻辑逐字同义（非空判 trim、
/// 返回用原文），本函数直接委托，调用方与单测零改动。
String collabRoleDisplayName(CollabRole role) =>
    session_data.collabRoleDisplayName(role.name, role.id);

/// 角色摘要行（纯函数）：列表项第二行要的「agent + model/effort」对应关系。
/// - agent 名优先走既有展示名映射（与全 App 命名一致），未知原样显示；
/// - agent 空回落 id；两者都空返回空串，调用方据此不占位、不伪造；
/// - model/effort 任一非空就补进摘要，中间用「·」分隔；都为空则只留 agent。
///
/// 对位 Android `collabRoleSummaryLine`（CollabRolesLogic.kt:51）。
String collabRoleSummaryLine(CollabRole role) {
  final String agent = role.agentId.trim().isNotEmpty
      ? role.agentId
      : (role.id.trim().isNotEmpty ? role.id : '');
  if (agent.isEmpty) return '';
  final List<String> parts = <String>[selectedAgentLabel(agent)];
  if (role.model.trim().isNotEmpty) parts.add(role.model);
  if (role.effort.trim().isNotEmpty) parts.add(role.effort);
  return parts.join(' · ');
}

/// agent id → 展示名：与 Android `selectedAgentLabel`（HomeScreen.kt:2198）
/// 同款映射——与主页「新建任务」区块头、待审批卡、最近会话行的命名一致。
///
/// 【为什么允许这份名单】它**不是**可用性名单（可用性一律由 daemon 能力位
/// 决定）：这是「同一个 id 在 App 各处显示同一个名字」的展示层收敛，未知 id
/// 原样透传（新增 agent 零改动自动正确显示）。
String selectedAgentLabel(String agentId) => switch (agentId) {
      '' => 'Codex',
      'codex' => 'Codex',
      'opencode' => 'OpenCode',
      'claude' => 'Claude',
      'pi' => 'Pi',
      'hermes' => 'Hermes',
      'zcode' => 'ZCode',
      'dsh' => 'DSH',
      _ => agentId,
    };

/// 从能力位提取「已接入协同」的 agent id（纯函数）：扫 `collab:<id>` 族。
///
/// daemon 报 `collab:<agentId>`——语义是「该 agent 已把本地 collab MCP 挂进
/// 自己的会话」。判定完全由 daemon 的接线事实驱动（新增一家自动出现），手机端
/// **不写名单**；能力缺失 = 该家尚未接线，如实不显示，不做假入口。
/// 口径对位 Android `resolveCollabAgentIds`（HomeScreen.kt:2216）：去空、去重、
/// 排序（同数据两次渲染同序，不跳动）。
///
/// 【实现唯一出处在 collab_session_data.dart】两份循环体曾逐行相同（收敛前已
/// Compare-Object 证实 11 行逐字节一致），现本函数直接委托正典实现；同名保留
/// 只为调用方零改动（编辑器候选/默认值 + 两份单测各钉各的 import）。
List<String> resolveCollabAgentIds(List<String> capabilities) =>
    session_data.resolveCollabAgentIds(capabilities);

/// 编辑页的目标 agent 下拉候选（纯函数）：已接入协同的优先，其后是能力位里
/// 报过的其余 agent（`agent:<id>` 族），去空去重排序。
///
/// 【为什么不是硬编码五家】Android 的 buildAgents 是展示目录 + capabilities
/// 动态点亮 + daemon 新增 agent 动态追加的三段合成；Flutter 侧没有 proto 的
/// listAgents RPC，拿不到那份静态目录，于是退到「能力位报过什么就列什么」——
/// 这是**诚实降级**：目录缺席时不编五家名单充数（编了就成了第二份写死名单，
/// daemon 加新 agent 时旧 APK 反而漏掉它），但 daemon 报的每家都会出现。
/// collab 位缺失时候选为空，页面按「没有位数据」如实说明（见
/// CollabRoleEditorPage 的降级文案），保存仍可用（agent_id 是透传真值，
/// daemon 在委派那一刻才做存在性裁决，见 collab_roles.go:106-110）。
List<String> collabEditorAgentCandidates(List<String> capabilities) {
  final Set<String> out = <String>{};
  out.addAll(resolveCollabAgentIds(capabilities));
  const String family = 'agent:';
  for (final String bit in capabilities) {
    if (!bit.toLowerCase().startsWith(family)) continue;
    final String id = bit.substring(family.length).trim();
    if (id.isNotEmpty) out.add(id);
  }
  final List<String> list = out.toList()..sort();
  return list;
}

/// 编辑页 agent 下拉的默认值（纯函数）：已接入协同的第一家 → 候选第一家 →
/// 空串（Kotlin `collabFirst ?: agents.firstOrNull()?.id ?: ""` 同款优先级）。
String collabEditorDefaultAgentId(List<String> capabilities) {
  final List<String> collabFirst = resolveCollabAgentIds(capabilities);
  if (collabFirst.isNotEmpty) return collabFirst.first;
  final List<String> candidates = collabEditorAgentCandidates(capabilities);
  return candidates.isNotEmpty ? candidates.first : '';
}

/// 表单 → proto `CollabRole`（纯函数）：八个字段的口径唯一出处。
///
/// 【trim 口径逐字段对位 daemon `protoToCollabRole`（collab_roles.go:49-62）】
/// id/name/agentId/description/model/effort/workspace 都 trim（它们是「值」，
/// 空格无语义）；**instructions 刻意不 trim**——它是自由文本（long-term
/// prompt），行首空格可能是用户有意的排版（列表缩进、代码块缩进）。
/// 客户端与 daemon 同口径，用户看到的回显才是落库后的样子。
CollabRole collabRoleFromForm({
  required String id,
  required String name,
  required String agentId,
  required String description,
  required String instructions,
  required String model,
  required String effort,
  required String workspace,
}) =>
    CollabRole(
      id: id.trim(),
      name: name.trim(),
      agentId: agentId.trim(),
      description: description.trim(),
      // 不 trim：见函数注释（daemon 同款刻意例外）。
      instructions: instructions,
      model: model.trim(),
      effort: effort.trim(),
      workspace: workspace.trim(),
    );

/// 思考强度档位（纯常量表，对应 Android CollabRoleEditorScreen 的
/// `effortOptions`）。空串 = 「跟随默认」（不覆盖 agent 当前默认）。
///
/// 【为什么只有五档】Android 同款五档；daemon 侧各家 agent 的 effort 值域
/// 不完全相同（codex: low/medium/high/xhigh，hermes 还认 max/ultra），但
/// **保存时不校验**（effort 是透传真值，未知值由 agent 自己拒绝或忽略）——
/// 下拉只给常用档，用户仍可在别处配非常规值。
const List<String> kCollabEffortOptions = <String>['', 'low', 'medium', 'high', 'xhigh'];

/// effort 值的展示名（纯函数）：空 = 跟随默认；表外值原样透传
/// （不显「未知」，未知输入透传红线）。
String collabEffortLabel(String effort) => switch (effort) {
      '' => '跟随默认',
      'low' => '低',
      'medium' => '中',
      'high' => '高',
      'xhigh' => '极高',
      _ => effort,
    };
