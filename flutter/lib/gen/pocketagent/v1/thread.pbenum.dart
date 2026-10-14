// This is a generated file - do not edit.
//
// Generated from v1/thread.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class ThreadStatus extends $pb.ProtobufEnum {
  static const ThreadStatus THREAD_STATUS_UNSPECIFIED =
      ThreadStatus._(0, _omitEnumNames ? '' : 'THREAD_STATUS_UNSPECIFIED');
  static const ThreadStatus THREAD_STATUS_IDLE =
      ThreadStatus._(1, _omitEnumNames ? '' : 'THREAD_STATUS_IDLE');
  static const ThreadStatus THREAD_STATUS_RUNNING =
      ThreadStatus._(2, _omitEnumNames ? '' : 'THREAD_STATUS_RUNNING');
  static const ThreadStatus THREAD_STATUS_WAITING_APPROVAL =
      ThreadStatus._(3, _omitEnumNames ? '' : 'THREAD_STATUS_WAITING_APPROVAL');
  static const ThreadStatus THREAD_STATUS_ARCHIVED =
      ThreadStatus._(4, _omitEnumNames ? '' : 'THREAD_STATUS_ARCHIVED');

  static const $core.List<ThreadStatus> values = <ThreadStatus>[
    THREAD_STATUS_UNSPECIFIED,
    THREAD_STATUS_IDLE,
    THREAD_STATUS_RUNNING,
    THREAD_STATUS_WAITING_APPROVAL,
    THREAD_STATUS_ARCHIVED,
  ];

  static final $core.List<ThreadStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static ThreadStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ThreadStatus._(super.value, super.name);
}

/// OutputState 说明「这一轮的最终正文是怎么来的」。
///
/// 【为什么需要它】此前适配器把空正文伪装成 "(完成,无文本输出)" 这样一条**普通
/// assistant 文本**发出来,手机端无法区分三种完全不同的情况:
///   - 真的没话说(EMPTY);
///   - 只产出了非文本产物,比如图片/文件(ARTIFACT_ONLY);
///   - 适配器把正文弄丢了,原生历史里其实有(ADAPTER_LOST_OUTPUT / RECOVERED_*)。
/// 用户看到的都是"任务完成 + 一句占位文案",于是无从判断该不该重试。
class OutputState extends $pb.ProtobufEnum {
  /// 旧 daemon 未上报:手机端按 final_blocks 自行判断(向后兼容)。
  static const OutputState OUTPUT_STATE_UNSPECIFIED =
      OutputState._(0, _omitEnumNames ? '' : 'OUTPUT_STATE_UNSPECIFIED');
  static const OutputState OUTPUT_STATE_TEXT =
      OutputState._(1, _omitEnumNames ? '' : 'OUTPUT_STATE_TEXT');

  /// 确实没有正文(也没有产物)—— 这是事实陈述,不是错误。
  static const OutputState OUTPUT_STATE_EMPTY =
      OutputState._(2, _omitEnumNames ? '' : 'OUTPUT_STATE_EMPTY');

  /// 只有非文本块(图片/附件等),没有正文。
  static const OutputState OUTPUT_STATE_ARTIFACT_ONLY =
      OutputState._(3, _omitEnumNames ? '' : 'OUTPUT_STATE_ARTIFACT_ONLY');

  /// 轮次失败(正文见 TurnFailed.partial_text)。
  static const OutputState OUTPUT_STATE_ERROR =
      OutputState._(4, _omitEnumNames ? '' : 'OUTPUT_STATE_ERROR');

  /// 增量流丢了正文,已从原生历史(thread/read)补偿回真实正文。
  static const OutputState OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY =
      OutputState._(5,
          _omitEnumNames ? '' : 'OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY');

  /// 适配器确认丢了正文,且原生历史也补不回来 —— 必须让用户知道"该重试",
  /// 而不是假装任务正常完成。
  static const OutputState OUTPUT_STATE_ADAPTER_LOST_OUTPUT = OutputState._(
      6, _omitEnumNames ? '' : 'OUTPUT_STATE_ADAPTER_LOST_OUTPUT');

  static const $core.List<OutputState> values = <OutputState>[
    OUTPUT_STATE_UNSPECIFIED,
    OUTPUT_STATE_TEXT,
    OUTPUT_STATE_EMPTY,
    OUTPUT_STATE_ARTIFACT_ONLY,
    OUTPUT_STATE_ERROR,
    OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY,
    OUTPUT_STATE_ADAPTER_LOST_OUTPUT,
  ];

  static final $core.List<OutputState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static OutputState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const OutputState._(super.value, super.name);
}

class CodexEvent_Kind extends $pb.ProtobufEnum {
  static const CodexEvent_Kind CODEX_EVENT_UNSPECIFIED =
      CodexEvent_Kind._(0, _omitEnumNames ? '' : 'CODEX_EVENT_UNSPECIFIED');
  static const CodexEvent_Kind ERROR =
      CodexEvent_Kind._(1, _omitEnumNames ? '' : 'ERROR');
  static const CodexEvent_Kind RETRY_STARTED =
      CodexEvent_Kind._(2, _omitEnumNames ? '' : 'RETRY_STARTED');
  static const CodexEvent_Kind RETRY_STOPPED =
      CodexEvent_Kind._(3, _omitEnumNames ? '' : 'RETRY_STOPPED');
  static const CodexEvent_Kind STATUS =
      CodexEvent_Kind._(4, _omitEnumNames ? '' : 'STATUS');
  static const CodexEvent_Kind WARNING =
      CodexEvent_Kind._(5, _omitEnumNames ? '' : 'WARNING');
  static const CodexEvent_Kind MODEL_REROUTED =
      CodexEvent_Kind._(6, _omitEnumNames ? '' : 'MODEL_REROUTED');
  static const CodexEvent_Kind RATE_LIMIT =
      CodexEvent_Kind._(7, _omitEnumNames ? '' : 'RATE_LIMIT');
  static const CodexEvent_Kind CONTEXT_COMPACTED =
      CodexEvent_Kind._(8, _omitEnumNames ? '' : 'CONTEXT_COMPACTED');

  /// ---- 以下为"彻底接入"补齐的事件类型 ----
  /// 审查者(guardian/auto_review)对某个动作的判定过程与结论。
  /// 【为什么单独一类】它不是错误也不是警告:动作可能被**自动拒绝**,
  /// 而用户的 approvals_reviewer 一旦不是 user,审批卡根本不会弹。
  /// 不展示的话,用户看到的就是"任务莫名其妙失败"。
  static const CodexEvent_Kind GUARDIAN_REVIEW =
      CodexEvent_Kind._(9, _omitEnumNames ? '' : 'GUARDIAN_REVIEW');

  /// 本回合的文件改动 diff(turn/diff/updated)。手机端渲染成可折叠的代码块。
  static const CodexEvent_Kind TURN_DIFF =
      CodexEvent_Kind._(10, _omitEnumNames ? '' : 'TURN_DIFF');

  /// codex 自己的弃用/配置/沙箱警告:以前这些通知我们直接丢掉了。
  static const CodexEvent_Kind DEPRECATION =
      CodexEvent_Kind._(11, _omitEnumNames ? '' : 'DEPRECATION');
  static const CodexEvent_Kind CONFIG_WARNING =
      CodexEvent_Kind._(12, _omitEnumNames ? '' : 'CONFIG_WARNING');
  static const CodexEvent_Kind SANDBOX_WARNING =
      CodexEvent_Kind._(13, _omitEnumNames ? '' : 'SANDBOX_WARNING');

  /// hooks 执行与技能变化:回答"它到底跑了什么、有哪些能力"。
  static const CodexEvent_Kind HOOK =
      CodexEvent_Kind._(14, _omitEnumNames ? '' : 'HOOK');
  static const CodexEvent_Kind SKILLS_CHANGED =
      CodexEvent_Kind._(15, _omitEnumNames ? '' : 'SKILLS_CHANGED');

  /// 会话排队状态变化(用户连续发消息时,后面的在排队)。
  static const CodexEvent_Kind QUEUE =
      CodexEvent_Kind._(16, _omitEnumNames ? '' : 'QUEUE');

  /// 模型安全缓冲/校验:属于"为什么这次输出很慢/被拦了"的解释面。
  static const CodexEvent_Kind MODEL_SAFETY =
      CodexEvent_Kind._(17, _omitEnumNames ? '' : 'MODEL_SAFETY');

  static const $core.List<CodexEvent_Kind> values = <CodexEvent_Kind>[
    CODEX_EVENT_UNSPECIFIED,
    ERROR,
    RETRY_STARTED,
    RETRY_STOPPED,
    STATUS,
    WARNING,
    MODEL_REROUTED,
    RATE_LIMIT,
    CONTEXT_COMPACTED,
    GUARDIAN_REVIEW,
    TURN_DIFF,
    DEPRECATION,
    CONFIG_WARNING,
    SANDBOX_WARNING,
    HOOK,
    SKILLS_CHANGED,
    QUEUE,
    MODEL_SAFETY,
  ];

  static final $core.List<CodexEvent_Kind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 17);
  static CodexEvent_Kind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CodexEvent_Kind._(super.value, super.name);
}

class ToolCallUpdate_Phase extends $pb.ProtobufEnum {
  static const ToolCallUpdate_Phase UNSPECIFIED =
      ToolCallUpdate_Phase._(0, _omitEnumNames ? '' : 'UNSPECIFIED');
  static const ToolCallUpdate_Phase STARTED =
      ToolCallUpdate_Phase._(1, _omitEnumNames ? '' : 'STARTED');
  static const ToolCallUpdate_Phase FINISHED =
      ToolCallUpdate_Phase._(2, _omitEnumNames ? '' : 'FINISHED');

  /// 进行中进度(多 agent 接入新增,只增不改):Pi 的 tool_execution_update
  /// 持续吐进度,partialResult 是**累积值**,App 端按 callId 原地替换(不追加)。
  static const ToolCallUpdate_Phase UPDATE =
      ToolCallUpdate_Phase._(3, _omitEnumNames ? '' : 'UPDATE');

  static const $core.List<ToolCallUpdate_Phase> values = <ToolCallUpdate_Phase>[
    UNSPECIFIED,
    STARTED,
    FINISHED,
    UPDATE,
  ];

  static final $core.List<ToolCallUpdate_Phase?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ToolCallUpdate_Phase? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ToolCallUpdate_Phase._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
