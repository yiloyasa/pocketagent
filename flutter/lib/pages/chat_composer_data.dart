// Chat 输入区纯函数层（B3 第 5 波③）：斜杠词元 / 候选过滤合并 / 点选回填 +
// 输入校验 + 附件类型判定 + 能力位门控。
//
// 【对位】Android `ui/SlashCommandsLogic.kt`（词元 `slashTokenOf` / 过滤
// `filterSlashCandidates` / 合并 `slashCandidatesFor` / 可见性
// `slashPanelVisible` / 空态 `slashUnknownAgentHintVisible` / 回填
// `applySlashSelection`·`applySlashPick` / 清理 `stripSlashToken` / 内置伪命令
// `builtinSlashCommandsFor` / 门控 `agentCapableOf`·`slashPanelAgentOf`）与
// `ui/ChatScreen.kt`（`attachmentEntryHiddenFor`）与 `ui/AttachmentPreview.kt`
// （`attachmentKind`）。本文件无 Flutter 依赖，widget 只做渲染与接线。
//
// 【动作执行留 C4】内置伪命令（/目标 /压缩 /计划）点选后执行动作而非回填
// （Android `SlashAction` GOAL/COMPACT/REVIEW 由 ChatScreen 接线）：本层只做
// FILL 回填 + 把点选事件原样交出（见 [SlashPick]），动作执行与 RPC 由 C4 在
// ChatPage 组装时接，注释交接，不摆假执行。

import '../gen/pocketagent/v1/thread.pb.dart';

// ── 斜杠命令展平视图 ────────────────────────────────────────────────────
//
// 【为什么摆脱 proto 类型】与 Android `SlashCommandInfo` 同款：纯逻辑可用，
// daemon 命令恒为 FILL（见 [slashInfoFromProto]），内置伪命令带动作。

/// 点选后的动作分流（Android `SlashAction` 的 Dart 形态：FILL 回填输入框，
/// 其余三项执行动作——本波只转发，执行留 C4）。
enum SlashPickAction {
  /// 普通命令：回填（daemon 命令恒为此；codex 技能填自然语言，见
  /// [applySlashPick]）。
  fill,

  /// 内置伪命令「/目标」：打开设置会话目标弹窗（C4 接线）。
  goal,

  /// 内置伪命令「/压缩」：触发 CompactThread（C4 接线）。
  compact,

  /// 内置伪命令「/计划」：触发 StartReview（C4 接线）。
  review,
}

/// 一条候选命令（daemon 命令经 [slashInfoFromProto] 展平，内置伪命令见
/// [builtinSlashCommandsFor]）。
class SlashCommandInfo {
  const SlashCommandInfo({
    required this.name,
    this.description = '',
    this.inputHint = '',
    this.action = SlashPickAction.fill,
  });

  /// 命令名（不带斜杠，如 "goal"；内置伪命令用中文真名，如 "目标"）。
  final String name;

  /// 一句话说明（可空）。
  final String description;

  /// 参数提示（可空；codex 技能恒为空，见 [applySlashPick]）。
  final String inputHint;

  /// 内置伪命令带动作；daemon 命令恒为 [SlashPickAction.fill]。
  final SlashPickAction action;

  @override
  bool operator ==(Object other) =>
      other is SlashCommandInfo &&
      other.name == name &&
      other.description == description &&
      other.inputHint == inputHint &&
      other.action == action;

  @override
  int get hashCode => Object.hash(name, description, inputHint, action);

  @override
  String toString() => 'SlashCommandInfo($name, $action)';
}

/// proto `SlashCommand` → 展平视图（daemon 命令恒为 FILL，不带动作）。
SlashCommandInfo slashInfoFromProto(SlashCommand command) => SlashCommandInfo(
      name: command.name,
      description: command.description,
      inputHint: command.inputHint,
    );

/// 未知助手的空态文案（Android `SLASH_UNKNOWN_AGENT_HINT` 原样：遇未知值给
/// 人话，禁止静默、禁止「未知/错误」字样）。
const String kSlashUnknownAgentHint = '该助手暂无可用命令';

/// 会话归属 agent 是否为 zcode（空/未知一律不算，大小写不敏感）。
bool isZcodeAgent(String? agentId) =>
    agentId != null && agentId.trim().toLowerCase() == 'zcode';

/// 会话归属 agent 是否为 codex（同上）。
bool isCodexAgent(String? agentId) =>
    agentId != null && agentId.trim().toLowerCase() == 'codex';

/// 会话归属 agent 是否为 pi（同上）。
bool isPiAgent(String? agentId) =>
    agentId != null && agentId.trim().toLowerCase() == 'pi';

/// 旧三源名单（zcode/codex/pi）：旧 daemon 未宣告 `slash:<agentId>` 位时的
/// 兼容垫（Android `isSlashPanelAgent` 原样；位优先化后只作回落，见
/// [slashPanelAgentOf]）。
bool isSlashPanelAgentFallback(String? agentId) =>
    isZcodeAgent(agentId) || isCodexAgent(agentId) || isPiAgent(agentId);

/// 按 agent 绑定的能力位判定（Android `agentCapableOf` 原样：大小写/空白不
/// 敏感；agentId 空/空白一律 false——硬红线「无能力不显示假入口」；只认绑定
/// 同一 agent 的位，别家的位不开我家的门）。
bool agentCapableOf(
  String bit,
  String? agentId,
  List<String> capabilities,
) {
  final String b = bit.trim();
  final String id = agentId?.trim() ?? '';
  if (b.isEmpty || id.isEmpty) return false;
  final String want = '$b:$id'.toLowerCase();
  return capabilities.any((String c) => c.trim().toLowerCase() == want);
}

/// 面板 agent 判定（门控统一入口，Android `slashPanelAgentOf` 原样）：
/// `slash:<agentId>` 位在 → 真（新家靠位点亮，不必改名单）；位不在 → 回落
/// 旧三源名单（旧 daemon 兼容垫）；都不在 → 假（落空态分支）。
bool slashPanelAgentOf(String? agentId, List<String> capabilities) =>
    agentCapableOf('slash', agentId, capabilities) ||
    isSlashPanelAgentFallback(agentId);

/// 内置伪命令全集（Android `builtinSlashCommands` 原样：/目标 /压缩 /计划，
/// 置顶展示）。
List<SlashCommandInfo> builtinSlashCommands() => const <SlashCommandInfo>[
      SlashCommandInfo(
        name: '目标',
        description: '设置会话目标（常驻上下文，agent 每轮都会看到）',
        action: SlashPickAction.goal,
      ),
      SlashCommandInfo(
        name: '压缩',
        description: '压缩会话历史上下文，省 token',
        action: SlashPickAction.compact,
      ),
      SlashCommandInfo(
        name: '计划',
        description: '发起计划审查（审查结果以事件流回本会话）',
        action: SlashPickAction.review,
      ),
    ];

/// 内置伪命令按会话归属 agent 与能力位过滤（Android
/// `builtinSlashCommandsFor` 原样）：pi 没有 goal 概念（落库跟随也不展示）；
/// /压缩认 `compact:<agentId>`、/计划认 `review:<agentId>`——位不在绝不进
/// 列表，不摆点了必败的假入口。
List<SlashCommandInfo> builtinSlashCommandsFor(
  String? agentId,
  List<String> capabilities,
) {
  final List<SlashCommandInfo> base = isPiAgent(agentId)
      ? builtinSlashCommands()
          .where((SlashCommandInfo c) => c.action != SlashPickAction.goal)
          .toList()
      : builtinSlashCommands();
  return base.where((SlashCommandInfo cmd) {
    switch (cmd.action) {
      case SlashPickAction.compact:
        return agentCapableOf('compact', agentId, capabilities);
      case SlashPickAction.review:
        return agentCapableOf('review', agentId, capabilities);
      case SlashPickAction.fill:
      case SlashPickAction.goal:
        return true;
    }
  }).toList();
}

/// 输入框当前敲出的「命令词元」（Android `slashTokenOf` 原样）：第一个空白
/// （空格/换行/Tab）之前的前缀；不以 "/" 开头（或空输入）返回 null——面板的
/// 开关信号；裸 "/" 也算词元（刚敲出斜杠就要看到全量列表）。
String? slashTokenOf(String input) {
  final StringBuffer token = StringBuffer();
  for (int i = 0; i < input.length; i++) {
    final String ch = input[i];
    if (RegExp(r'\s').hasMatch(ch)) break;
    token.write(ch);
  }
  final String t = token.toString();
  return t.startsWith('/') ? t : null;
}

/// 候选过滤（Android `filterSlashCandidates` 原样，输入是「已合并」的候选
/// 全集）：命令名对 "/后" 的词元做大小写不敏感前缀匹配；词元后已敲空白
/// （"/goal "）→ 空（命令已选定在敲参数，面板收起）；裸 "/" → 全量；无前缀
/// 命中 → 退回全量（宽进）；候选全集为空 → 空（面板不显示）。
List<SlashCommandInfo> filterSlashCandidates(
  List<SlashCommandInfo> commands,
  String input,
) {
  if (commands.isEmpty) return const <SlashCommandInfo>[];
  final String? token = slashTokenOf(input);
  if (token == null) return const <SlashCommandInfo>[];
  if (input.length > token.length) return const <SlashCommandInfo>[];
  final String query = token.substring(1).toLowerCase();
  if (query.isEmpty) return commands;
  final List<SlashCommandInfo> hit = commands
      .where(
        (SlashCommandInfo c) => c.name.toLowerCase().startsWith(query),
      )
      .toList();
  return hit.isEmpty ? commands : hit;
}

/// 面板候选全集（Android `slashCandidatesFor` 原样）：内置伪命令（随能力位
/// 点亮）置顶 + daemon 命令（重名让位），一次合并去重；非面板会话直接空
/// （保证「看到什么」与「能不能开面板」一致）。
List<SlashCommandInfo> slashCandidatesFor(
  String? agentId,
  List<SlashCommandInfo> commands,
  List<String> capabilities,
) {
  if (!slashPanelAgentOf(agentId, capabilities)) {
    return const <SlashCommandInfo>[];
  }
  final Set<String> names =
      commands.map((SlashCommandInfo c) => c.name.toLowerCase()).toSet();
  final List<SlashCommandInfo> merged = <SlashCommandInfo>[];
  for (final SlashCommandInfo b in builtinSlashCommandsFor(
    agentId,
    capabilities,
  )) {
    final String nativeAlias;
    switch (b.action) {
      case SlashPickAction.goal:
        nativeAlias = 'goal';
      case SlashPickAction.compact:
        nativeAlias = 'compact';
      case SlashPickAction.fill:
      case SlashPickAction.review:
        nativeAlias = b.name.toLowerCase();
    }
    if (!names.contains(b.name.toLowerCase()) &&
        !names.contains(nativeAlias)) {
      merged.add(b);
    }
  }
  merged.addAll(commands);
  return merged;
}

/// 面板可见性（Android `slashPanelVisible` 原样）：面板会话 + 非离线 + 输入
/// 敲出词元 + 过滤结果非空（内置伪命令兜底，在线面板会话哪怕 daemon 命令还
/// 没到位也有卡面可弹）。
bool slashPanelVisible(
  String? agentId,
  String input,
  List<SlashCommandInfo> commands,
  bool offline,
  List<String> capabilities,
) {
  if (!slashPanelAgentOf(agentId, capabilities) || offline) return false;
  return filterSlashCandidates(
    slashCandidatesFor(agentId, commands, capabilities),
    input,
  ).isNotEmpty;
}

/// 未知助手的空态提示是否显示（Android `slashUnknownAgentHintVisible`
/// 原样）：agent 已知非空但面板判定为否（如 hermes 连旧 daemon）且敲出 "/"
/// 词元 → 人话空态；加载中（agent 空）与离线态不提示。
bool slashUnknownAgentHintVisible(
  String? agentId,
  String input,
  bool offline,
  List<String> capabilities,
) {
  if (offline) return false;
  if (slashTokenOf(input) == null) return false;
  final String id = agentId?.trim() ?? '';
  if (id.isEmpty) return false;
  return !slashPanelAgentOf(id, capabilities);
}

/// 点选回填（Android `applySlashSelection` 原样）：把输入开头的 "/词元" 替换
/// 成 "/命令 "（补一个空格方便接着敲参数），词元后文字原样保留；词元后本来
/// 就是空白/换行不再重复补空格；无 "/" 词元原样返回。
String applySlashSelection(String input, String commandName) {
  final String? token = slashTokenOf(input);
  if (token == null) return input;
  final String rest = input.substring(token.length);
  final String sep =
      rest.isNotEmpty && RegExp(r'\s').hasMatch(rest[0]) ? '' : ' ';
  return '/$commandName$sep$rest';
}

/// 点选回填入口（Android `applySlashPick` 的 FILL 部分原样；内置伪命令
/// action != fill 在此 no-op——点选语义是「执行动作」，由调用方在调用本函数
/// 之前分流，走到这里是防御）：codex 技能（无 "/" 语法、inputHint 恒为空）
/// 填自然语言「请使用 <名称> 技能：」；其余走 [applySlashSelection]。
String applySlashPick(
  String input,
  SlashCommandInfo command,
  String? agentId,
) {
  if (command.action != SlashPickAction.fill) return input;
  if (isCodexAgent(agentId) && command.inputHint.trim().isEmpty) {
    return '请使用 ${command.name} 技能：';
  }
  return applySlashSelection(input, command.name);
}

/// 内置伪命令点选后的输入清理（Android `stripSlashToken` 原样）：敲出的
/// "/词元" 是唤起面板的临时手段，动作执行后不能残留在输入框里（否则发送会
/// 把 "/目标" 当正文发给 agent）；只摘词元、保留词元后已敲文字，结果掐头去
/// 尾空白。
String stripSlashToken(String input) {
  final String? token = slashTokenOf(input);
  if (token == null) return input;
  return input.substring(token.length).trim();
}

// ── 输入校验 ────────────────────────────────────────────────────────────

/// 单轮输入文本的本地上限（字符数）。
///
/// 【为什么是本地值】proto `TurnInput` 未声明文本上限（daemon 无显式上限，
/// 超大文本随 gRPC 帧走）；本值只是客户端前置提示——防手滑粘贴超大文本，
/// 超限时发送键禁用并给人话提示，不静默截断（截断等于替用户丢内容）。
const int kComposerMaxLength = 20000;

/// 输入校验（空/超长）：返回 null = 可发送；否则为发送键旁展示的人话原因。
/// 空口径与 Android `TurnSession.send` 一致（`isBlank`：空串与纯空白都拒）。
String? validateComposerInput(String text) {
  if (text.trim().isEmpty) return '输入不能为空';
  if (text.length > kComposerMaxLength) {
    return '输入过长（${text.length}/$kComposerMaxLength），请分段发送';
  }
  return null;
}

// ── 附件类型与门控 ──────────────────────────────────────────────────────

/// 附件类型三态（Android `AttachmentKind` 是 IMAGE/FILE 二态；此处 FILE 拆成
/// 文本 vs 不支持：图片走 `ImageBlock` 内联，文本走文本块说明随 TurnInput 发
/// 送，其余类型本波无发送链路——判出来是为了诚实提示，不摆能发的假象）。
enum ComposerAttachmentKind {
  /// 图片（mime `image/*` 或常见图片扩展名）：`ImageBlock` 内联发送。
  image,

  /// 文本（含常见代码/配置扩展名与文本 mime）：随正文说明发送。
  text,

  /// 不支持（压缩包/二进制/未知）：本波无发送链路，UI 如实提示。
  unsupported,
}

/// 文本附件扩展名集合（小写、无点；命中即按文本发送——本地纯函数分类，不涉
/// 及能力位判定，不与「写死名单」红线冲突）。
const Set<String> kComposerTextExtensions = <String>{
  'txt', 'md', 'markdown', 'json', 'jsonl', 'log', 'csv', 'tsv', 'xml',
  'yaml', 'yml', 'toml', 'ini', 'properties', 'env',
  'java', 'kt', 'kts', 'py', 'js', 'jsx', 'ts', 'tsx', 'vue', 'dart',
  'go', 'rs', 'c', 'h', 'cpp', 'hpp', 'cs', 'swift', 'rb', 'php',
  'sh', 'bash', 'sql', 'html', 'css', 'scss', 'gradle',
};

/// 文本 mime 前缀之外的文本型 mime（扩展名未命中时的第二判定）。
const Set<String> kComposerTextMimeTypes = <String>{
  'application/json',
  'application/xml',
  'application/yaml',
  'application/toml',
  'application/javascript',
  'application/x-sh',
};

/// 附件类型判定（Android `attachmentKind` 的 IMAGE 半原样 + FILE 拆分）：
/// mime `image/*` 优先（不看扩展名）；否则按扩展名认图片；再按文本 mime/
/// 文本扩展名认文本；其余一律不支持（含无扩展名 + 未知 mime）。
ComposerAttachmentKind attachmentKindOf({
  required String name,
  required String mediaType,
}) {
  final String mime = mediaType.trim().toLowerCase();
  if (mime.startsWith('image/')) return ComposerAttachmentKind.image;
  final String ext = name.contains('.')
      ? name.substring(name.lastIndexOf('.') + 1).toLowerCase()
      : '';
  switch (ext) {
    case 'png':
    case 'jpg':
    case 'jpeg':
    case 'gif':
    case 'webp':
    case 'bmp':
    case 'heic':
      return ComposerAttachmentKind.image;
  }
  if (mime.startsWith('text/') ||
      kComposerTextMimeTypes.contains(mime) ||
      kComposerTextExtensions.contains(ext)) {
    return ComposerAttachmentKind.text;
  }
  return ComposerAttachmentKind.unsupported;
}

/// 图片入口是否可见（Android `attachmentEntryHiddenFor` 取反后的肯定形态）：
/// daemon 逐 agent 宣告 `image_input:<id>`（codex/pi/zcode 带位、dsh 无位），
/// 缺位 = 发图必败——无位不显示图片入口（文本附件入口不受影响，见
/// `ChatComposer` 工具行装配）。
bool composerImageEntryVisible(
  String? agentId,
  List<String> capabilities,
) =>
    agentCapableOf('image_input', agentId, capabilities);

// ── 受控附件与点选事件载荷 ──────────────────────────────────────────────

/// 待发送附件（受控列表的元素；实际拾取与上传由 C4/B5 接线，本层只展示与
/// 移除——见 `ChatComposer.attachments`）。
class ChatPendingAttachment {
  const ChatPendingAttachment({required this.name, this.mediaType = ''});

  /// 文件名（含扩展名）。
  final String name;

  /// mime（如 "image/png"；空 = 未知，走扩展名判定）。
  final String mediaType;

  /// 类型（见 [attachmentKindOf]）。
  ComposerAttachmentKind get kind =>
      attachmentKindOf(name: name, mediaType: mediaType);

  @override
  bool operator ==(Object other) =>
      other is ChatPendingAttachment &&
      other.name == name &&
      other.mediaType == mediaType;

  @override
  int get hashCode => Object.hash(name, mediaType);

  @override
  String toString() => 'ChatPendingAttachment($name, $mediaType)';
}

/// 斜杠点选事件（C4 消费：FILL 已由 composer 回填输入框，C4 无需再动作；
/// GOAL/COMPACT/REVIEW 由 C4 执行对应动作——本波 composer 只转发，注释交接）。
class SlashPick {
  const SlashPick({
    required this.name,
    required this.action,
    this.agentId,
  });

  /// 命令名（daemon 命令如 "init"；内置伪命令中文真名如 "目标"）。
  final String name;

  /// 动作分流（见 [SlashPickAction]）。
  final SlashPickAction action;

  /// 点选时的会话归属 agent（C4 按 agent 路由动作执行）。
  final String? agentId;

  @override
  bool operator ==(Object other) =>
      other is SlashPick &&
      other.name == name &&
      other.action == action &&
      other.agentId == agentId;

  @override
  int get hashCode => Object.hash(name, action, agentId);

  @override
  String toString() => 'SlashPick($name, $action)';
}
