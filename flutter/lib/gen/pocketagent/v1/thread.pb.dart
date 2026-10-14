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

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'common.pb.dart' as $1;
import 'model.pb.dart' as $2;
import 'thread.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'thread.pbenum.dart';

class StartReviewRequest extends $pb.GeneratedMessage {
  factory StartReviewRequest({
    $core.String? threadId,
    $core.String? focus,
  }) {
    final result = StartReviewRequest._();
    if (threadId != null) result.threadId = threadId;
    if (focus != null) result.focus = focus;
    return result;
  }

  StartReviewRequest._();

  factory StartReviewRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartReviewRequest()..mergeFromBuffer(data, registry);
  factory StartReviewRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartReviewRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartReviewRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: StartReviewRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'focus')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartReviewRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartReviewRequest copyWith(void Function(StartReviewRequest) updates) =>
      super.copyWith((message) => updates(message as StartReviewRequest))
          as StartReviewRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use StartReviewRequest() / StartReviewRequest.new instead')
  static StartReviewRequest create() => StartReviewRequest._();
  static $pb.GeneratedMessage $_createMessage() => StartReviewRequest._();
  @$core.override
  StartReviewRequest createEmptyInstance() => StartReviewRequest._();
  @$core.pragma('dart2js:noInline')
  static StartReviewRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartReviewRequest>(
          StartReviewRequest.$_createMessage);
  static StartReviewRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  /// 审查重点(可空):空 = daemon 侧用默认口径(审查会话中拟定的计划);
  /// 非空 = 原样作为 codex 自由指令(custom target 的 instructions)下发。
  @$pb.TagNumber(2)
  $core.String get focus => $_getSZ(1);
  @$pb.TagNumber(2)
  set focus($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFocus() => $_has(1);
  @$pb.TagNumber(2)
  void clearFocus() => $_clearField(2);
}

class StartReviewResponse extends $pb.GeneratedMessage {
  factory StartReviewResponse({
    $core.String? reviewId,
    $core.bool? started,
  }) {
    final result = StartReviewResponse._();
    if (reviewId != null) result.reviewId = reviewId;
    if (started != null) result.started = started;
    return result;
  }

  StartReviewResponse._();

  factory StartReviewResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartReviewResponse()..mergeFromBuffer(data, registry);
  factory StartReviewResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      StartReviewResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartReviewResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: StartReviewResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reviewId')
    ..aOB(2, _omitFieldNames ? '' : 'started')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartReviewResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartReviewResponse copyWith(void Function(StartReviewResponse) updates) =>
      super.copyWith((message) => updates(message as StartReviewResponse))
          as StartReviewResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use StartReviewResponse() / StartReviewResponse.new instead')
  static StartReviewResponse create() => StartReviewResponse._();
  static $pb.GeneratedMessage $_createMessage() => StartReviewResponse._();
  @$core.override
  StartReviewResponse createEmptyInstance() => StartReviewResponse._();
  @$core.pragma('dart2js:noInline')
  static StartReviewResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartReviewResponse>(
          StartReviewResponse.$_createMessage);
  static StartReviewResponse? _defaultInstance;

  /// 承载本次审查的 codex 线程 id(实测 inline 模式 = 原线程 id,与请求线程
  /// 一致)。手机端只拿它做诊断回指,审查结果完全依赖事件流展示。
  @$pb.TagNumber(1)
  $core.String get reviewId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reviewId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReviewId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReviewId() => $_clearField(1);

  /// 发起成功(失败走 gRPC 错误,不会出现 false;字段留给未来异步语义)。
  @$pb.TagNumber(2)
  $core.bool get started => $_getBF(1);
  @$pb.TagNumber(2)
  set started($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStarted() => $_has(1);
  @$pb.TagNumber(2)
  void clearStarted() => $_clearField(2);
}

class DelegateSubTaskRequest extends $pb.GeneratedMessage {
  factory DelegateSubTaskRequest({
    $core.String? agentId,
    $core.String? task,
    $core.String? workspace,
    $core.String? model,
    $core.String? effort,
    $core.String? parentThreadId,
    $core.String? roleId,
  }) {
    final result = DelegateSubTaskRequest._();
    if (agentId != null) result.agentId = agentId;
    if (task != null) result.task = task;
    if (workspace != null) result.workspace = workspace;
    if (model != null) result.model = model;
    if (effort != null) result.effort = effort;
    if (parentThreadId != null) result.parentThreadId = parentThreadId;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  DelegateSubTaskRequest._();

  factory DelegateSubTaskRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegateSubTaskRequest()..mergeFromBuffer(data, registry);
  factory DelegateSubTaskRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegateSubTaskRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DelegateSubTaskRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DelegateSubTaskRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'task')
    ..aOS(3, _omitFieldNames ? '' : 'workspace')
    ..aOS(4, _omitFieldNames ? '' : 'model')
    ..aOS(5, _omitFieldNames ? '' : 'effort')
    ..aOS(6, _omitFieldNames ? '' : 'parentThreadId')
    ..aOS(7, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegateSubTaskRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegateSubTaskRequest copyWith(
          void Function(DelegateSubTaskRequest) updates) =>
      super.copyWith((message) => updates(message as DelegateSubTaskRequest))
          as DelegateSubTaskRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DelegateSubTaskRequest() / DelegateSubTaskRequest.new instead')
  static DelegateSubTaskRequest create() => DelegateSubTaskRequest._();
  static $pb.GeneratedMessage $_createMessage() => DelegateSubTaskRequest._();
  @$core.override
  DelegateSubTaskRequest createEmptyInstance() => DelegateSubTaskRequest._();
  @$core.pragma('dart2js:noInline')
  static DelegateSubTaskRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DelegateSubTaskRequest>(
          DelegateSubTaskRequest.$_createMessage);
  static DelegateSubTaskRequest? _defaultInstance;

  /// 目标 agent 的 id(如 "echo"/"pi";空或未知 → error 字段人话报错,
  /// daemon 不猜、不路由)。
  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  /// 要委派的有界子任务描述(必填)。
  @$pb.TagNumber(2)
  $core.String get task => $_getSZ(1);
  @$pb.TagNumber(2)
  set task($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTask() => $_has(1);
  @$pb.TagNumber(2)
  void clearTask() => $_clearField(2);

  /// 工作区覆盖;空 = daemon 默认工作区。
  @$pb.TagNumber(3)
  $core.String get workspace => $_getSZ(2);
  @$pb.TagNumber(3)
  set workspace($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWorkspace() => $_has(2);
  @$pb.TagNumber(3)
  void clearWorkspace() => $_clearField(3);

  /// 模型覆盖(形如 "agent/model-id");空 = 该 agent 当前默认。
  @$pb.TagNumber(4)
  $core.String get model => $_getSZ(3);
  @$pb.TagNumber(4)
  set model($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasModel() => $_has(3);
  @$pb.TagNumber(4)
  void clearModel() => $_clearField(4);

  /// 思考强度覆盖;空 = 该 agent 当前默认。
  @$pb.TagNumber(5)
  $core.String get effort => $_getSZ(4);
  @$pb.TagNumber(5)
  set effort($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasEffort() => $_has(4);
  @$pb.TagNumber(5)
  void clearEffort() => $_clearField(5);

  /// 发起方(父)线程的 daemon 本地 id(T-S1Pa,只增,可选):非空 = 委派建的
  /// 子线程落库 parent_thread_id = 该值,手机端据此点亮子代理内联卡;空 =
  /// 子线程父链为空(诚实变暗,卡不亮不误报),与旧行为一致。
  /// 【v1 限制】pi 侧经 pi-mcp-adapter 的委派暂填不出该值(mcp.json 是静态
  /// 共享文件、MCP 服务器进程懒启动跨会话共享,无会话上下文注入机制);
  /// 未来 piapp 具备会话感知注入时,由薄代理把当前线程 id 填进请求本字段,
  /// 链路即一行接通。
  @$pb.TagNumber(6)
  $core.String get parentThreadId => $_getSZ(5);
  @$pb.TagNumber(6)
  set parentThreadId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasParentThreadId() => $_has(5);
  @$pb.TagNumber(6)
  void clearParentThreadId() => $_clearField(6);

  /// 引用的协同角色 id(可选):非空 = daemon 取出该角色,把它的人设注入子任务
  /// prompt、并用它的 model/effort/workspace 兜底本请求的空字段
  /// (**本请求显式给的值优先级更高**,角色只填空)。
  /// 【为什么只传一个 id 而不是整段人设】父模型应当在委派时说明"这次具体做什么";
  /// "这次具体做什么";角色回答"我是谁、长期怎么干"。两者职责不同,都传才完整。
  /// 未知 id → error 字段人话报错(**不猜、不回落到默认**,否则父模型永远发现
  /// 不了自己写错了角色名)。
  @$pb.TagNumber(7)
  $core.String get roleId => $_getSZ(6);
  @$pb.TagNumber(7)
  set roleId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRoleId() => $_has(6);
  @$pb.TagNumber(7)
  void clearRoleId() => $_clearField(7);
}

class DelegateSubTaskResponse extends $pb.GeneratedMessage {
  factory DelegateSubTaskResponse({
    $core.String? childThreadId,
    $core.String? summary,
    $core.String? state,
    $core.String? error,
  }) {
    final result = DelegateSubTaskResponse._();
    if (childThreadId != null) result.childThreadId = childThreadId;
    if (summary != null) result.summary = summary;
    if (state != null) result.state = state;
    if (error != null) result.error = error;
    return result;
  }

  DelegateSubTaskResponse._();

  factory DelegateSubTaskResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegateSubTaskResponse()..mergeFromBuffer(data, registry);
  factory DelegateSubTaskResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DelegateSubTaskResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DelegateSubTaskResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DelegateSubTaskResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'childThreadId')
    ..aOS(2, _omitFieldNames ? '' : 'summary')
    ..aOS(3, _omitFieldNames ? '' : 'state')
    ..aOS(4, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegateSubTaskResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DelegateSubTaskResponse copyWith(
          void Function(DelegateSubTaskResponse) updates) =>
      super.copyWith((message) => updates(message as DelegateSubTaskResponse))
          as DelegateSubTaskResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DelegateSubTaskResponse() / DelegateSubTaskResponse.new instead')
  static DelegateSubTaskResponse create() => DelegateSubTaskResponse._();
  static $pb.GeneratedMessage $_createMessage() => DelegateSubTaskResponse._();
  @$core.override
  DelegateSubTaskResponse createEmptyInstance() => DelegateSubTaskResponse._();
  @$core.pragma('dart2js:noInline')
  static DelegateSubTaskResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DelegateSubTaskResponse>(
          DelegateSubTaskResponse.$_createMessage);
  static DelegateSubTaskResponse? _defaultInstance;

  /// 委派子线程 id(委派被引擎入口拒绝时为空;子线程已建但轮次失败时仍回填,
  /// 便于调用方事后 agent_status 查询)。
  @$pb.TagNumber(1)
  $core.String get childThreadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set childThreadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChildThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChildThreadId() => $_clearField(1);

  /// 子任务终稿摘要(失败时为空,不伪造)。
  @$pb.TagNumber(2)
  $core.String get summary => $_getSZ(1);
  @$pb.TagNumber(2)
  set summary($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSummary() => $_has(1);
  @$pb.TagNumber(2)
  void clearSummary() => $_clearField(2);

  /// 子线程在委派返回时点的状态(engine 口径:idle/failed/running/queued/…;
  /// 无法判定时留空)。子线程后续状态变化不在本 RPC 的跟踪范围内。
  @$pb.TagNumber(3)
  $core.String get state => $_getSZ(2);
  @$pb.TagNumber(3)
  set state($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasState() => $_has(2);
  @$pb.TagNumber(3)
  void clearState() => $_clearField(3);

  /// 业务失败的人话说明(agent 不存在 / 深度锁拒绝 / 子轮次失败等);
  /// 空 = 成功。不靠 gRPC status 承载业务失败(与既有风格对齐)。
  @$pb.TagNumber(4)
  $core.String get error => $_getSZ(3);
  @$pb.TagNumber(4)
  set error($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasError() => $_has(3);
  @$pb.TagNumber(4)
  void clearError() => $_clearField(4);
}

class CollabRole extends $pb.GeneratedMessage {
  factory CollabRole({
    $core.String? id,
    $core.String? name,
    $core.String? agentId,
    $core.String? description,
    $core.String? instructions,
    $core.String? model,
    $core.String? effort,
    $core.String? workspace,
  }) {
    final result = CollabRole._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (agentId != null) result.agentId = agentId;
    if (description != null) result.description = description;
    if (instructions != null) result.instructions = instructions;
    if (model != null) result.model = model;
    if (effort != null) result.effort = effort;
    if (workspace != null) result.workspace = workspace;
    return result;
  }

  CollabRole._();

  factory CollabRole.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabRole()..mergeFromBuffer(data, registry);
  factory CollabRole.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabRole()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CollabRole',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CollabRole.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'agentId')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aOS(5, _omitFieldNames ? '' : 'instructions')
    ..aOS(6, _omitFieldNames ? '' : 'model')
    ..aOS(7, _omitFieldNames ? '' : 'effort')
    ..aOS(8, _omitFieldNames ? '' : 'workspace')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabRole clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabRole copyWith(void Function(CollabRole) updates) =>
      super.copyWith((message) => updates(message as CollabRole)) as CollabRole;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CollabRole() / CollabRole.new instead')
  static CollabRole create() => CollabRole._();
  static $pb.GeneratedMessage $_createMessage() => CollabRole._();
  @$core.override
  CollabRole createEmptyInstance() => CollabRole._();
  @$core.pragma('dart2js:noInline')
  static CollabRole getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CollabRole>(CollabRole.$_createMessage);
  static CollabRole? _defaultInstance;

  /// 稳定 id(自定义 slug,如 "reviewer";同一 daemon 内唯一)。set 时用幂等键。
  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 显示名(手机端展示用,如 "安全审查者")。
  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  /// 落到哪个原生 agent(pi / codex / dsh / zcode ...),与委派的 agent_id 同域。
  @$pb.TagNumber(3)
  $core.String get agentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set agentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAgentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAgentId() => $_clearField(3);

  /// 这个岗位是干什么的(职责边界)。会前置注入到**每次**委派的子任务 prompt。
  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  /// 长期生效的追加要求(输出格式 / 只做不改 / 必须列证据 …),同样前置注入。
  @$pb.TagNumber(5)
  $core.String get instructions => $_getSZ(4);
  @$pb.TagNumber(5)
  set instructions($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasInstructions() => $_has(4);
  @$pb.TagNumber(5)
  void clearInstructions() => $_clearField(5);

  /// 模型覆盖(形如 "agent/model-id");空 = 用该 agent 当前默认。
  @$pb.TagNumber(6)
  $core.String get model => $_getSZ(5);
  @$pb.TagNumber(6)
  set model($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasModel() => $_has(5);
  @$pb.TagNumber(6)
  void clearModel() => $_clearField(6);

  /// 思考强度覆盖;空 = 用该 agent 当前默认。
  @$pb.TagNumber(7)
  $core.String get effort => $_getSZ(6);
  @$pb.TagNumber(7)
  set effort($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasEffort() => $_has(6);
  @$pb.TagNumber(7)
  void clearEffort() => $_clearField(7);

  /// 工作区覆盖;空 = daemon 默认工作区。
  @$pb.TagNumber(8)
  $core.String get workspace => $_getSZ(7);
  @$pb.TagNumber(8)
  set workspace($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasWorkspace() => $_has(7);
  @$pb.TagNumber(8)
  void clearWorkspace() => $_clearField(8);
}

class ListCollabRolesRequest extends $pb.GeneratedMessage {
  factory ListCollabRolesRequest() => ListCollabRolesRequest._();

  ListCollabRolesRequest._();

  factory ListCollabRolesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabRolesRequest()..mergeFromBuffer(data, registry);
  factory ListCollabRolesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabRolesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCollabRolesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCollabRolesRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabRolesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabRolesRequest copyWith(
          void Function(ListCollabRolesRequest) updates) =>
      super.copyWith((message) => updates(message as ListCollabRolesRequest))
          as ListCollabRolesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCollabRolesRequest() / ListCollabRolesRequest.new instead')
  static ListCollabRolesRequest create() => ListCollabRolesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListCollabRolesRequest._();
  @$core.override
  ListCollabRolesRequest createEmptyInstance() => ListCollabRolesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCollabRolesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCollabRolesRequest>(
          ListCollabRolesRequest.$_createMessage);
  static ListCollabRolesRequest? _defaultInstance;
}

class ListCollabRolesResponse extends $pb.GeneratedMessage {
  factory ListCollabRolesResponse({
    $core.Iterable<CollabRole>? roles,
  }) {
    final result = ListCollabRolesResponse._();
    if (roles != null) result.roles.addAll(roles);
    return result;
  }

  ListCollabRolesResponse._();

  factory ListCollabRolesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabRolesResponse()..mergeFromBuffer(data, registry);
  factory ListCollabRolesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabRolesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCollabRolesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCollabRolesResponse.$_createMessage)
    ..pPM<CollabRole>(1, _omitFieldNames ? '' : 'roles',
        subBuilder: CollabRole.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabRolesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabRolesResponse copyWith(
          void Function(ListCollabRolesResponse) updates) =>
      super.copyWith((message) => updates(message as ListCollabRolesResponse))
          as ListCollabRolesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCollabRolesResponse() / ListCollabRolesResponse.new instead')
  static ListCollabRolesResponse create() => ListCollabRolesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListCollabRolesResponse._();
  @$core.override
  ListCollabRolesResponse createEmptyInstance() => ListCollabRolesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCollabRolesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCollabRolesResponse>(
          ListCollabRolesResponse.$_createMessage);
  static ListCollabRolesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<CollabRole> get roles => $_getList(0);
}

class SetCollabRoleRequest extends $pb.GeneratedMessage {
  factory SetCollabRoleRequest({
    CollabRole? role,
  }) {
    final result = SetCollabRoleRequest._();
    if (role != null) result.role = role;
    return result;
  }

  SetCollabRoleRequest._();

  factory SetCollabRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCollabRoleRequest()..mergeFromBuffer(data, registry);
  factory SetCollabRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCollabRoleRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCollabRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SetCollabRoleRequest.$_createMessage)
    ..aOM<CollabRole>(1, _omitFieldNames ? '' : 'role',
        subBuilder: CollabRole.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCollabRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCollabRoleRequest copyWith(void Function(SetCollabRoleRequest) updates) =>
      super.copyWith((message) => updates(message as SetCollabRoleRequest))
          as SetCollabRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetCollabRoleRequest() / SetCollabRoleRequest.new instead')
  static SetCollabRoleRequest create() => SetCollabRoleRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetCollabRoleRequest._();
  @$core.override
  SetCollabRoleRequest createEmptyInstance() => SetCollabRoleRequest._();
  @$core.pragma('dart2js:noInline')
  static SetCollabRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCollabRoleRequest>(
          SetCollabRoleRequest.$_createMessage);
  static SetCollabRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  CollabRole get role => $_getN(0);
  @$pb.TagNumber(1)
  set role(CollabRole value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRole() => $_has(0);
  @$pb.TagNumber(1)
  void clearRole() => $_clearField(1);
  @$pb.TagNumber(1)
  CollabRole ensureRole() => $_ensure(0);
}

class SetCollabRoleResponse extends $pb.GeneratedMessage {
  factory SetCollabRoleResponse({
    CollabRole? role,
  }) {
    final result = SetCollabRoleResponse._();
    if (role != null) result.role = role;
    return result;
  }

  SetCollabRoleResponse._();

  factory SetCollabRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCollabRoleResponse()..mergeFromBuffer(data, registry);
  factory SetCollabRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCollabRoleResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCollabRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SetCollabRoleResponse.$_createMessage)
    ..aOM<CollabRole>(1, _omitFieldNames ? '' : 'role',
        subBuilder: CollabRole.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCollabRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCollabRoleResponse copyWith(
          void Function(SetCollabRoleResponse) updates) =>
      super.copyWith((message) => updates(message as SetCollabRoleResponse))
          as SetCollabRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetCollabRoleResponse() / SetCollabRoleResponse.new instead')
  static SetCollabRoleResponse create() => SetCollabRoleResponse._();
  static $pb.GeneratedMessage $_createMessage() => SetCollabRoleResponse._();
  @$core.override
  SetCollabRoleResponse createEmptyInstance() => SetCollabRoleResponse._();
  @$core.pragma('dart2js:noInline')
  static SetCollabRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCollabRoleResponse>(
          SetCollabRoleResponse.$_createMessage);
  static SetCollabRoleResponse? _defaultInstance;

  /// 回显落库后的角色(id 可能被 daemon 补生成),便于手机端立刻刷新成真值。
  @$pb.TagNumber(1)
  CollabRole get role => $_getN(0);
  @$pb.TagNumber(1)
  set role(CollabRole value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRole() => $_has(0);
  @$pb.TagNumber(1)
  void clearRole() => $_clearField(1);
  @$pb.TagNumber(1)
  CollabRole ensureRole() => $_ensure(0);
}

class DeleteCollabRoleRequest extends $pb.GeneratedMessage {
  factory DeleteCollabRoleRequest({
    $core.String? id,
  }) {
    final result = DeleteCollabRoleRequest._();
    if (id != null) result.id = id;
    return result;
  }

  DeleteCollabRoleRequest._();

  factory DeleteCollabRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCollabRoleRequest()..mergeFromBuffer(data, registry);
  factory DeleteCollabRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCollabRoleRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteCollabRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DeleteCollabRoleRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCollabRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCollabRoleRequest copyWith(
          void Function(DeleteCollabRoleRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteCollabRoleRequest))
          as DeleteCollabRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteCollabRoleRequest() / DeleteCollabRoleRequest.new instead')
  static DeleteCollabRoleRequest create() => DeleteCollabRoleRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteCollabRoleRequest._();
  @$core.override
  DeleteCollabRoleRequest createEmptyInstance() => DeleteCollabRoleRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteCollabRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteCollabRoleRequest>(
          DeleteCollabRoleRequest.$_createMessage);
  static DeleteCollabRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);
}

class DeleteCollabRoleResponse extends $pb.GeneratedMessage {
  factory DeleteCollabRoleResponse() => DeleteCollabRoleResponse._();

  DeleteCollabRoleResponse._();

  factory DeleteCollabRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCollabRoleResponse()..mergeFromBuffer(data, registry);
  factory DeleteCollabRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteCollabRoleResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteCollabRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DeleteCollabRoleResponse.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCollabRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCollabRoleResponse copyWith(
          void Function(DeleteCollabRoleResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteCollabRoleResponse))
          as DeleteCollabRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteCollabRoleResponse() / DeleteCollabRoleResponse.new instead')
  static DeleteCollabRoleResponse create() => DeleteCollabRoleResponse._();
  static $pb.GeneratedMessage $_createMessage() => DeleteCollabRoleResponse._();
  @$core.override
  DeleteCollabRoleResponse createEmptyInstance() =>
      DeleteCollabRoleResponse._();
  @$core.pragma('dart2js:noInline')
  static DeleteCollabRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteCollabRoleResponse>(
          DeleteCollabRoleResponse.$_createMessage);
  static DeleteCollabRoleResponse? _defaultInstance;
}

/// 参与本次协同的一员:落到哪个 agent、扮演哪个角色(角色可空)。
class CollabMember extends $pb.GeneratedMessage {
  factory CollabMember({
    $core.String? agentId,
    $core.String? roleId,
  }) {
    final result = CollabMember._();
    if (agentId != null) result.agentId = agentId;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  CollabMember._();

  factory CollabMember.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabMember()..mergeFromBuffer(data, registry);
  factory CollabMember.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabMember()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CollabMember',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CollabMember.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabMember clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabMember copyWith(void Function(CollabMember) updates) =>
      super.copyWith((message) => updates(message as CollabMember))
          as CollabMember;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CollabMember() / CollabMember.new instead')
  static CollabMember create() => CollabMember._();
  static $pb.GeneratedMessage $_createMessage() => CollabMember._();
  @$core.override
  CollabMember createEmptyInstance() => CollabMember._();
  @$core.pragma('dart2js:noInline')
  static CollabMember getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CollabMember>(
          CollabMember.$_createMessage);
  static CollabMember? _defaultInstance;

  /// 目标 agent id;为空时取 role_id 对应角色里配的 agent(两者都空 = 人话报错)。
  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  /// 协同角色 id(可空):非空则取该角色的人设与 model/effort/workspace 兜底。
  @$pb.TagNumber(2)
  $core.String get roleId => $_getSZ(1);
  @$pb.TagNumber(2)
  set roleId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRoleId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRoleId() => $_clearField(2);
}

class DispatchCollabRequest extends $pb.GeneratedMessage {
  factory DispatchCollabRequest({
    $core.String? instruction,
    $core.Iterable<CollabMember>? members,
    $core.String? workspace,
  }) {
    final result = DispatchCollabRequest._();
    if (instruction != null) result.instruction = instruction;
    if (members != null) result.members.addAll(members);
    if (workspace != null) result.workspace = workspace;
    return result;
  }

  DispatchCollabRequest._();

  factory DispatchCollabRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DispatchCollabRequest()..mergeFromBuffer(data, registry);
  factory DispatchCollabRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DispatchCollabRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DispatchCollabRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DispatchCollabRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'instruction')
    ..pPM<CollabMember>(2, _omitFieldNames ? '' : 'members',
        subBuilder: CollabMember.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'workspace')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DispatchCollabRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DispatchCollabRequest copyWith(
          void Function(DispatchCollabRequest) updates) =>
      super.copyWith((message) => updates(message as DispatchCollabRequest))
          as DispatchCollabRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DispatchCollabRequest() / DispatchCollabRequest.new instead')
  static DispatchCollabRequest create() => DispatchCollabRequest._();
  static $pb.GeneratedMessage $_createMessage() => DispatchCollabRequest._();
  @$core.override
  DispatchCollabRequest createEmptyInstance() => DispatchCollabRequest._();
  @$core.pragma('dart2js:noInline')
  static DispatchCollabRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DispatchCollabRequest>(
          DispatchCollabRequest.$_createMessage);
  static DispatchCollabRequest? _defaultInstance;

  /// 用户发出的指令原文(必填)。所有成员收到的是**同一条**指令 —— 这正是
  /// "共同完成一个任务"与"逐个委派不同子任务"的区别。
  @$pb.TagNumber(1)
  $core.String get instruction => $_getSZ(0);
  @$pb.TagNumber(1)
  set instruction($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInstruction() => $_has(0);
  @$pb.TagNumber(1)
  void clearInstruction() => $_clearField(1);

  /// 参与成员(至少一个)。
  @$pb.TagNumber(2)
  $pb.PbList<CollabMember> get members => $_getList(1);

  /// 工作区覆盖;空 = 各成员用角色值或 daemon 默认。
  @$pb.TagNumber(3)
  $core.String get workspace => $_getSZ(2);
  @$pb.TagNumber(3)
  set workspace($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWorkspace() => $_has(2);
  @$pb.TagNumber(3)
  void clearWorkspace() => $_clearField(3);
}

/// 单个成员的受理结果:一个成员失败不影响其它成员(并行扇出的应有语义)。
class DispatchCollabItem extends $pb.GeneratedMessage {
  factory DispatchCollabItem({
    $core.String? agentId,
    $core.String? threadId,
    $core.bool? ack,
    $core.String? error,
  }) {
    final result = DispatchCollabItem._();
    if (agentId != null) result.agentId = agentId;
    if (threadId != null) result.threadId = threadId;
    if (ack != null) result.ack = ack;
    if (error != null) result.error = error;
    return result;
  }

  DispatchCollabItem._();

  factory DispatchCollabItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DispatchCollabItem()..mergeFromBuffer(data, registry);
  factory DispatchCollabItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DispatchCollabItem()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DispatchCollabItem',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DispatchCollabItem.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'threadId')
    ..aOB(3, _omitFieldNames ? '' : 'ack')
    ..aOS(4, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DispatchCollabItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DispatchCollabItem copyWith(void Function(DispatchCollabItem) updates) =>
      super.copyWith((message) => updates(message as DispatchCollabItem))
          as DispatchCollabItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DispatchCollabItem() / DispatchCollabItem.new instead')
  static DispatchCollabItem create() => DispatchCollabItem._();
  static $pb.GeneratedMessage $_createMessage() => DispatchCollabItem._();
  @$core.override
  DispatchCollabItem createEmptyInstance() => DispatchCollabItem._();
  @$core.pragma('dart2js:noInline')
  static DispatchCollabItem getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DispatchCollabItem>(
          DispatchCollabItem.$_createMessage);
  static DispatchCollabItem? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  /// 为该成员建的子线程 id;受理失败时为空。
  @$pb.TagNumber(2)
  $core.String get threadId => $_getSZ(1);
  @$pb.TagNumber(2)
  set threadId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasThreadId() => $_has(1);
  @$pb.TagNumber(2)
  void clearThreadId() => $_clearField(2);

  /// 是否受理(子线程已建、轮次已起飞)。受理 ≠ 执行成功:结果看子线程状态。
  @$pb.TagNumber(3)
  $core.bool get ack => $_getBF(2);
  @$pb.TagNumber(3)
  set ack($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAck() => $_has(2);
  @$pb.TagNumber(3)
  void clearAck() => $_clearField(3);

  /// 受理失败的人话原因(agent 不存在 / 角色未定义等);成功时为空。
  @$pb.TagNumber(4)
  $core.String get error => $_getSZ(3);
  @$pb.TagNumber(4)
  set error($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasError() => $_has(3);
  @$pb.TagNumber(4)
  void clearError() => $_clearField(4);
}

class DispatchCollabResponse extends $pb.GeneratedMessage {
  factory DispatchCollabResponse({
    $core.String? sessionId,
    $core.Iterable<DispatchCollabItem>? items,
  }) {
    final result = DispatchCollabResponse._();
    if (sessionId != null) result.sessionId = sessionId;
    if (items != null) result.items.addAll(items);
    return result;
  }

  DispatchCollabResponse._();

  factory DispatchCollabResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DispatchCollabResponse()..mergeFromBuffer(data, registry);
  factory DispatchCollabResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DispatchCollabResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DispatchCollabResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DispatchCollabResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..pPM<DispatchCollabItem>(2, _omitFieldNames ? '' : 'items',
        subBuilder: DispatchCollabItem.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DispatchCollabResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DispatchCollabResponse copyWith(
          void Function(DispatchCollabResponse) updates) =>
      super.copyWith((message) => updates(message as DispatchCollabResponse))
          as DispatchCollabResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DispatchCollabResponse() / DispatchCollabResponse.new instead')
  static DispatchCollabResponse create() => DispatchCollabResponse._();
  static $pb.GeneratedMessage $_createMessage() => DispatchCollabResponse._();
  @$core.override
  DispatchCollabResponse createEmptyInstance() => DispatchCollabResponse._();
  @$core.pragma('dart2js:noInline')
  static DispatchCollabResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DispatchCollabResponse>(
          DispatchCollabResponse.$_createMessage);
  static DispatchCollabResponse? _defaultInstance;

  /// 本次协同的会话 id(便于前端归组与事后回溯;等于各成员子线程的父链)。
  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  /// 与请求 members **一一对应、顺序一致**(前端按位置回填每个 agent 的相位)。
  @$pb.TagNumber(2)
  $pb.PbList<DispatchCollabItem> get items => $_getList(1);
}

class ListSlashCommandsRequest extends $pb.GeneratedMessage {
  factory ListSlashCommandsRequest({
    $core.String? threadId,
    $core.bool? refresh,
  }) {
    final result = ListSlashCommandsRequest._();
    if (threadId != null) result.threadId = threadId;
    if (refresh != null) result.refresh = refresh;
    return result;
  }

  ListSlashCommandsRequest._();

  factory ListSlashCommandsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSlashCommandsRequest()..mergeFromBuffer(data, registry);
  factory ListSlashCommandsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSlashCommandsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSlashCommandsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListSlashCommandsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOB(2, _omitFieldNames ? '' : 'refresh')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsRequest copyWith(
          void Function(ListSlashCommandsRequest) updates) =>
      super.copyWith((message) => updates(message as ListSlashCommandsRequest))
          as ListSlashCommandsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSlashCommandsRequest() / ListSlashCommandsRequest.new instead')
  static ListSlashCommandsRequest create() => ListSlashCommandsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListSlashCommandsRequest._();
  @$core.override
  ListSlashCommandsRequest createEmptyInstance() =>
      ListSlashCommandsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListSlashCommandsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSlashCommandsRequest>(
          ListSlashCommandsRequest.$_createMessage);
  static ListSlashCommandsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get refresh => $_getBF(1);
  @$pb.TagNumber(2)
  set refresh($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRefresh() => $_has(1);
  @$pb.TagNumber(2)
  void clearRefresh() => $_clearField(2);
}

/// SlashCommand 快照顶层 slashCommands[] 的一项(元素形状为实测:
/// {"description":"…","inputHint":"/goal …","name":"goal"},见
/// daemon/docs/verify/evidence/zcode-p0-probe-raw.log)。
class SlashCommand extends $pb.GeneratedMessage {
  factory SlashCommand({
    $core.String? name,
    $core.String? description,
    $core.String? inputHint,
  }) {
    final result = SlashCommand._();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (inputHint != null) result.inputHint = inputHint;
    return result;
  }

  SlashCommand._();

  factory SlashCommand.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SlashCommand()..mergeFromBuffer(data, registry);
  factory SlashCommand.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SlashCommand()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SlashCommand',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SlashCommand.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'inputHint')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SlashCommand clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SlashCommand copyWith(void Function(SlashCommand) updates) =>
      super.copyWith((message) => updates(message as SlashCommand))
          as SlashCommand;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SlashCommand() / SlashCommand.new instead')
  static SlashCommand create() => SlashCommand._();
  static $pb.GeneratedMessage $_createMessage() => SlashCommand._();
  @$core.override
  SlashCommand createEmptyInstance() => SlashCommand._();
  @$core.pragma('dart2js:noInline')
  static SlashCommand getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SlashCommand>(
          SlashCommand.$_createMessage);
  static SlashCommand? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get inputHint => $_getSZ(2);
  @$pb.TagNumber(3)
  set inputHint($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInputHint() => $_has(2);
  @$pb.TagNumber(3)
  void clearInputHint() => $_clearField(3);
}

class ListSlashCommandsResponse extends $pb.GeneratedMessage {
  factory ListSlashCommandsResponse({
    $core.Iterable<SlashCommand>? commands,
  }) {
    final result = ListSlashCommandsResponse._();
    if (commands != null) result.commands.addAll(commands);
    return result;
  }

  ListSlashCommandsResponse._();

  factory ListSlashCommandsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSlashCommandsResponse()..mergeFromBuffer(data, registry);
  factory ListSlashCommandsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListSlashCommandsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSlashCommandsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListSlashCommandsResponse.$_createMessage)
    ..pPM<SlashCommand>(1, _omitFieldNames ? '' : 'commands',
        subBuilder: SlashCommand.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSlashCommandsResponse copyWith(
          void Function(ListSlashCommandsResponse) updates) =>
      super.copyWith((message) => updates(message as ListSlashCommandsResponse))
          as ListSlashCommandsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListSlashCommandsResponse() / ListSlashCommandsResponse.new instead')
  static ListSlashCommandsResponse create() => ListSlashCommandsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListSlashCommandsResponse._();
  @$core.override
  ListSlashCommandsResponse createEmptyInstance() =>
      ListSlashCommandsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListSlashCommandsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSlashCommandsResponse>(
          ListSlashCommandsResponse.$_createMessage);
  static ListSlashCommandsResponse? _defaultInstance;

  /// 空 = 非 zcode 会话或快照暂不可用(手机端不显示候选条)。
  @$pb.TagNumber(1)
  $pb.PbList<SlashCommand> get commands => $_getList(0);
}

class OptimizePromptRequest extends $pb.GeneratedMessage {
  factory OptimizePromptRequest({
    $core.String? text,
    $core.String? style,
    $core.String? agentId,
    $core.String? modelId,
    $core.String? effort,
  }) {
    final result = OptimizePromptRequest._();
    if (text != null) result.text = text;
    if (style != null) result.style = style;
    if (agentId != null) result.agentId = agentId;
    if (modelId != null) result.modelId = modelId;
    if (effort != null) result.effort = effort;
    return result;
  }

  OptimizePromptRequest._();

  factory OptimizePromptRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptimizePromptRequest()..mergeFromBuffer(data, registry);
  factory OptimizePromptRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptimizePromptRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OptimizePromptRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: OptimizePromptRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'text')
    ..aOS(2, _omitFieldNames ? '' : 'style')
    ..aOS(3, _omitFieldNames ? '' : 'agentId')
    ..aOS(4, _omitFieldNames ? '' : 'modelId')
    ..aOS(5, _omitFieldNames ? '' : 'effort')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptimizePromptRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptimizePromptRequest copyWith(
          void Function(OptimizePromptRequest) updates) =>
      super.copyWith((message) => updates(message as OptimizePromptRequest))
          as OptimizePromptRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use OptimizePromptRequest() / OptimizePromptRequest.new instead')
  static OptimizePromptRequest create() => OptimizePromptRequest._();
  static $pb.GeneratedMessage $_createMessage() => OptimizePromptRequest._();
  @$core.override
  OptimizePromptRequest createEmptyInstance() => OptimizePromptRequest._();
  @$core.pragma('dart2js:noInline')
  static OptimizePromptRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OptimizePromptRequest>(
          OptimizePromptRequest.$_createMessage);
  static OptimizePromptRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get text => $_getSZ(0);
  @$pb.TagNumber(1)
  set text($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasText() => $_has(0);
  @$pb.TagNumber(1)
  void clearText() => $_clearField(1);

  /// 预留的改写风格:concise = 压缩到一句话;detailed = 展开细节;
  /// 空 = 默认(保持长度量级,把意思说清楚)。daemon 侧按此拼优化指令。
  @$pb.TagNumber(2)
  $core.String get style => $_getSZ(1);
  @$pb.TagNumber(2)
  set style($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStyle() => $_has(1);
  @$pb.TagNumber(2)
  void clearStyle() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get agentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set agentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAgentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAgentId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get modelId => $_getSZ(3);
  @$pb.TagNumber(4)
  set modelId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasModelId() => $_has(3);
  @$pb.TagNumber(4)
  void clearModelId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get effort => $_getSZ(4);
  @$pb.TagNumber(5)
  set effort($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasEffort() => $_has(4);
  @$pb.TagNumber(5)
  void clearEffort() => $_clearField(5);
}

class OptimizePromptResponse extends $pb.GeneratedMessage {
  factory OptimizePromptResponse({
    $core.String? optimized,
  }) {
    final result = OptimizePromptResponse._();
    if (optimized != null) result.optimized = optimized;
    return result;
  }

  OptimizePromptResponse._();

  factory OptimizePromptResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptimizePromptResponse()..mergeFromBuffer(data, registry);
  factory OptimizePromptResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OptimizePromptResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OptimizePromptResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: OptimizePromptResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'optimized')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptimizePromptResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OptimizePromptResponse copyWith(
          void Function(OptimizePromptResponse) updates) =>
      super.copyWith((message) => updates(message as OptimizePromptResponse))
          as OptimizePromptResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use OptimizePromptResponse() / OptimizePromptResponse.new instead')
  static OptimizePromptResponse create() => OptimizePromptResponse._();
  static $pb.GeneratedMessage $_createMessage() => OptimizePromptResponse._();
  @$core.override
  OptimizePromptResponse createEmptyInstance() => OptimizePromptResponse._();
  @$core.pragma('dart2js:noInline')
  static OptimizePromptResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OptimizePromptResponse>(
          OptimizePromptResponse.$_createMessage);
  static OptimizePromptResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get optimized => $_getSZ(0);
  @$pb.TagNumber(1)
  set optimized($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOptimized() => $_has(0);
  @$pb.TagNumber(1)
  void clearOptimized() => $_clearField(1);
}

class ExportThreadHtmlRequest extends $pb.GeneratedMessage {
  factory ExportThreadHtmlRequest({
    $core.String? threadId,
  }) {
    final result = ExportThreadHtmlRequest._();
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  ExportThreadHtmlRequest._();

  factory ExportThreadHtmlRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExportThreadHtmlRequest()..mergeFromBuffer(data, registry);
  factory ExportThreadHtmlRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExportThreadHtmlRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExportThreadHtmlRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ExportThreadHtmlRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportThreadHtmlRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportThreadHtmlRequest copyWith(
          void Function(ExportThreadHtmlRequest) updates) =>
      super.copyWith((message) => updates(message as ExportThreadHtmlRequest))
          as ExportThreadHtmlRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExportThreadHtmlRequest() / ExportThreadHtmlRequest.new instead')
  static ExportThreadHtmlRequest create() => ExportThreadHtmlRequest._();
  static $pb.GeneratedMessage $_createMessage() => ExportThreadHtmlRequest._();
  @$core.override
  ExportThreadHtmlRequest createEmptyInstance() => ExportThreadHtmlRequest._();
  @$core.pragma('dart2js:noInline')
  static ExportThreadHtmlRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExportThreadHtmlRequest>(
          ExportThreadHtmlRequest.$_createMessage);
  static ExportThreadHtmlRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);
}

class ExportThreadHtmlResponse extends $pb.GeneratedMessage {
  factory ExportThreadHtmlResponse({
    $core.String? path,
  }) {
    final result = ExportThreadHtmlResponse._();
    if (path != null) result.path = path;
    return result;
  }

  ExportThreadHtmlResponse._();

  factory ExportThreadHtmlResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExportThreadHtmlResponse()..mergeFromBuffer(data, registry);
  factory ExportThreadHtmlResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExportThreadHtmlResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExportThreadHtmlResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ExportThreadHtmlResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportThreadHtmlResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportThreadHtmlResponse copyWith(
          void Function(ExportThreadHtmlResponse) updates) =>
      super.copyWith((message) => updates(message as ExportThreadHtmlResponse))
          as ExportThreadHtmlResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ExportThreadHtmlResponse() / ExportThreadHtmlResponse.new instead')
  static ExportThreadHtmlResponse create() => ExportThreadHtmlResponse._();
  static $pb.GeneratedMessage $_createMessage() => ExportThreadHtmlResponse._();
  @$core.override
  ExportThreadHtmlResponse createEmptyInstance() =>
      ExportThreadHtmlResponse._();
  @$core.pragma('dart2js:noInline')
  static ExportThreadHtmlResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExportThreadHtmlResponse>(
          ExportThreadHtmlResponse.$_createMessage);
  static ExportThreadHtmlResponse? _defaultInstance;

  /// 电脑上的 HTML 绝对路径(会话工作区/.pocketagent-exports/<threadId>.html)。
  /// 手机端拿它走现有 ReadFile(FileChunk 流)下载;同线程重复导出会覆盖旧文件。
  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);
}

class ApproveGuardianDeniedActionRequest extends $pb.GeneratedMessage {
  factory ApproveGuardianDeniedActionRequest({
    $core.String? threadId,
    $core.String? reviewId,
    $core.String? targetItemId,
  }) {
    final result = ApproveGuardianDeniedActionRequest._();
    if (threadId != null) result.threadId = threadId;
    if (reviewId != null) result.reviewId = reviewId;
    if (targetItemId != null) result.targetItemId = targetItemId;
    return result;
  }

  ApproveGuardianDeniedActionRequest._();

  factory ApproveGuardianDeniedActionRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApproveGuardianDeniedActionRequest()..mergeFromBuffer(data, registry);
  factory ApproveGuardianDeniedActionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApproveGuardianDeniedActionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApproveGuardianDeniedActionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ApproveGuardianDeniedActionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'reviewId')
    ..aOS(3, _omitFieldNames ? '' : 'targetItemId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApproveGuardianDeniedActionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApproveGuardianDeniedActionRequest copyWith(
          void Function(ApproveGuardianDeniedActionRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ApproveGuardianDeniedActionRequest))
          as ApproveGuardianDeniedActionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ApproveGuardianDeniedActionRequest() / ApproveGuardianDeniedActionRequest.new instead')
  static ApproveGuardianDeniedActionRequest create() =>
      ApproveGuardianDeniedActionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ApproveGuardianDeniedActionRequest._();
  @$core.override
  ApproveGuardianDeniedActionRequest createEmptyInstance() =>
      ApproveGuardianDeniedActionRequest._();
  @$core.pragma('dart2js:noInline')
  static ApproveGuardianDeniedActionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApproveGuardianDeniedActionRequest>(
          ApproveGuardianDeniedActionRequest.$_createMessage);
  static ApproveGuardianDeniedActionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reviewId => $_getSZ(1);
  @$pb.TagNumber(2)
  set reviewId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReviewId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReviewId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get targetItemId => $_getSZ(2);
  @$pb.TagNumber(3)
  set targetItemId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTargetItemId() => $_has(2);
  @$pb.TagNumber(3)
  void clearTargetItemId() => $_clearField(3);
}

class ForkThreadRequest extends $pb.GeneratedMessage {
  factory ForkThreadRequest({
    $core.String? threadId,
    $core.String? lastTurnId,
  }) {
    final result = ForkThreadRequest._();
    if (threadId != null) result.threadId = threadId;
    if (lastTurnId != null) result.lastTurnId = lastTurnId;
    return result;
  }

  ForkThreadRequest._();

  factory ForkThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ForkThreadRequest()..mergeFromBuffer(data, registry);
  factory ForkThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ForkThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ForkThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ForkThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'lastTurnId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForkThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ForkThreadRequest copyWith(void Function(ForkThreadRequest) updates) =>
      super.copyWith((message) => updates(message as ForkThreadRequest))
          as ForkThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ForkThreadRequest() / ForkThreadRequest.new instead')
  static ForkThreadRequest create() => ForkThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => ForkThreadRequest._();
  @$core.override
  ForkThreadRequest createEmptyInstance() => ForkThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static ForkThreadRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ForkThreadRequest>(
          ForkThreadRequest.$_createMessage);
  static ForkThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get lastTurnId => $_getSZ(1);
  @$pb.TagNumber(2)
  set lastTurnId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLastTurnId() => $_has(1);
  @$pb.TagNumber(2)
  void clearLastTurnId() => $_clearField(2);
}

class RollbackThreadRequest extends $pb.GeneratedMessage {
  factory RollbackThreadRequest({
    $core.String? threadId,
    $core.int? numTurns,
  }) {
    final result = RollbackThreadRequest._();
    if (threadId != null) result.threadId = threadId;
    if (numTurns != null) result.numTurns = numTurns;
    return result;
  }

  RollbackThreadRequest._();

  factory RollbackThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RollbackThreadRequest()..mergeFromBuffer(data, registry);
  factory RollbackThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RollbackThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RollbackThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RollbackThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aI(2, _omitFieldNames ? '' : 'numTurns')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RollbackThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RollbackThreadRequest copyWith(
          void Function(RollbackThreadRequest) updates) =>
      super.copyWith((message) => updates(message as RollbackThreadRequest))
          as RollbackThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RollbackThreadRequest() / RollbackThreadRequest.new instead')
  static RollbackThreadRequest create() => RollbackThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => RollbackThreadRequest._();
  @$core.override
  RollbackThreadRequest createEmptyInstance() => RollbackThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static RollbackThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RollbackThreadRequest>(
          RollbackThreadRequest.$_createMessage);
  static RollbackThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get numTurns => $_getIZ(1);
  @$pb.TagNumber(2)
  set numTurns($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNumTurns() => $_has(1);
  @$pb.TagNumber(2)
  void clearNumTurns() => $_clearField(2);
}

class ThreadSection extends $pb.GeneratedMessage {
  factory ThreadSection({
    $core.String? id,
    $core.String? name,
    $core.String? color,
    $core.String? icon,
    $core.int? threadCount,
  }) {
    final result = ThreadSection._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (color != null) result.color = color;
    if (icon != null) result.icon = icon;
    if (threadCount != null) result.threadCount = threadCount;
    return result;
  }

  ThreadSection._();

  factory ThreadSection.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadSection()..mergeFromBuffer(data, registry);
  factory ThreadSection.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadSection()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThreadSection',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ThreadSection.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'color')
    ..aOS(4, _omitFieldNames ? '' : 'icon')
    ..aI(5, _omitFieldNames ? '' : 'threadCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadSection clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadSection copyWith(void Function(ThreadSection) updates) =>
      super.copyWith((message) => updates(message as ThreadSection))
          as ThreadSection;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ThreadSection() / ThreadSection.new instead')
  static ThreadSection create() => ThreadSection._();
  static $pb.GeneratedMessage $_createMessage() => ThreadSection._();
  @$core.override
  ThreadSection createEmptyInstance() => ThreadSection._();
  @$core.pragma('dart2js:noInline')
  static ThreadSection getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ThreadSection>(
          ThreadSection.$_createMessage);
  static ThreadSection? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get color => $_getSZ(2);
  @$pb.TagNumber(3)
  set color($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasColor() => $_has(2);
  @$pb.TagNumber(3)
  void clearColor() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get icon => $_getSZ(3);
  @$pb.TagNumber(4)
  set icon($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIcon() => $_has(3);
  @$pb.TagNumber(4)
  void clearIcon() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get threadCount => $_getIZ(4);
  @$pb.TagNumber(5)
  set threadCount($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasThreadCount() => $_has(4);
  @$pb.TagNumber(5)
  void clearThreadCount() => $_clearField(5);
}

class ListThreadSectionsRequest extends $pb.GeneratedMessage {
  factory ListThreadSectionsRequest() => ListThreadSectionsRequest._();

  ListThreadSectionsRequest._();

  factory ListThreadSectionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadSectionsRequest()..mergeFromBuffer(data, registry);
  factory ListThreadSectionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadSectionsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadSectionsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListThreadSectionsRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadSectionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadSectionsRequest copyWith(
          void Function(ListThreadSectionsRequest) updates) =>
      super.copyWith((message) => updates(message as ListThreadSectionsRequest))
          as ListThreadSectionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListThreadSectionsRequest() / ListThreadSectionsRequest.new instead')
  static ListThreadSectionsRequest create() => ListThreadSectionsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListThreadSectionsRequest._();
  @$core.override
  ListThreadSectionsRequest createEmptyInstance() =>
      ListThreadSectionsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListThreadSectionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadSectionsRequest>(
          ListThreadSectionsRequest.$_createMessage);
  static ListThreadSectionsRequest? _defaultInstance;
}

class ListThreadSectionsResponse extends $pb.GeneratedMessage {
  factory ListThreadSectionsResponse({
    $core.Iterable<ThreadSection>? sections,
  }) {
    final result = ListThreadSectionsResponse._();
    if (sections != null) result.sections.addAll(sections);
    return result;
  }

  ListThreadSectionsResponse._();

  factory ListThreadSectionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadSectionsResponse()..mergeFromBuffer(data, registry);
  factory ListThreadSectionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadSectionsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadSectionsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListThreadSectionsResponse.$_createMessage)
    ..pPM<ThreadSection>(1, _omitFieldNames ? '' : 'sections',
        subBuilder: ThreadSection.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadSectionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadSectionsResponse copyWith(
          void Function(ListThreadSectionsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListThreadSectionsResponse))
          as ListThreadSectionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListThreadSectionsResponse() / ListThreadSectionsResponse.new instead')
  static ListThreadSectionsResponse create() => ListThreadSectionsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListThreadSectionsResponse._();
  @$core.override
  ListThreadSectionsResponse createEmptyInstance() =>
      ListThreadSectionsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListThreadSectionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadSectionsResponse>(
          ListThreadSectionsResponse.$_createMessage);
  static ListThreadSectionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ThreadSection> get sections => $_getList(0);
}

class CreateThreadSectionRequest extends $pb.GeneratedMessage {
  factory CreateThreadSectionRequest({
    $core.String? name,
  }) {
    final result = CreateThreadSectionRequest._();
    if (name != null) result.name = name;
    return result;
  }

  CreateThreadSectionRequest._();

  factory CreateThreadSectionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateThreadSectionRequest()..mergeFromBuffer(data, registry);
  factory CreateThreadSectionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateThreadSectionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateThreadSectionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CreateThreadSectionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateThreadSectionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateThreadSectionRequest copyWith(
          void Function(CreateThreadSectionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CreateThreadSectionRequest))
          as CreateThreadSectionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateThreadSectionRequest() / CreateThreadSectionRequest.new instead')
  static CreateThreadSectionRequest create() => CreateThreadSectionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      CreateThreadSectionRequest._();
  @$core.override
  CreateThreadSectionRequest createEmptyInstance() =>
      CreateThreadSectionRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateThreadSectionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateThreadSectionRequest>(
          CreateThreadSectionRequest.$_createMessage);
  static CreateThreadSectionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);
}

class RenameThreadSectionRequest extends $pb.GeneratedMessage {
  factory RenameThreadSectionRequest({
    $core.String? sectionId,
    $core.String? name,
  }) {
    final result = RenameThreadSectionRequest._();
    if (sectionId != null) result.sectionId = sectionId;
    if (name != null) result.name = name;
    return result;
  }

  RenameThreadSectionRequest._();

  factory RenameThreadSectionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameThreadSectionRequest()..mergeFromBuffer(data, registry);
  factory RenameThreadSectionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameThreadSectionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RenameThreadSectionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RenameThreadSectionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sectionId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameThreadSectionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameThreadSectionRequest copyWith(
          void Function(RenameThreadSectionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as RenameThreadSectionRequest))
          as RenameThreadSectionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RenameThreadSectionRequest() / RenameThreadSectionRequest.new instead')
  static RenameThreadSectionRequest create() => RenameThreadSectionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      RenameThreadSectionRequest._();
  @$core.override
  RenameThreadSectionRequest createEmptyInstance() =>
      RenameThreadSectionRequest._();
  @$core.pragma('dart2js:noInline')
  static RenameThreadSectionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RenameThreadSectionRequest>(
          RenameThreadSectionRequest.$_createMessage);
  static RenameThreadSectionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sectionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sectionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSectionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSectionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);
}

class DeleteThreadSectionRequest extends $pb.GeneratedMessage {
  factory DeleteThreadSectionRequest({
    $core.String? sectionId,
  }) {
    final result = DeleteThreadSectionRequest._();
    if (sectionId != null) result.sectionId = sectionId;
    return result;
  }

  DeleteThreadSectionRequest._();

  factory DeleteThreadSectionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteThreadSectionRequest()..mergeFromBuffer(data, registry);
  factory DeleteThreadSectionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteThreadSectionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteThreadSectionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DeleteThreadSectionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sectionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteThreadSectionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteThreadSectionRequest copyWith(
          void Function(DeleteThreadSectionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as DeleteThreadSectionRequest))
          as DeleteThreadSectionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DeleteThreadSectionRequest() / DeleteThreadSectionRequest.new instead')
  static DeleteThreadSectionRequest create() => DeleteThreadSectionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      DeleteThreadSectionRequest._();
  @$core.override
  DeleteThreadSectionRequest createEmptyInstance() =>
      DeleteThreadSectionRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteThreadSectionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteThreadSectionRequest>(
          DeleteThreadSectionRequest.$_createMessage);
  static DeleteThreadSectionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sectionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sectionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSectionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSectionId() => $_clearField(1);
}

class MoveThreadToSectionRequest extends $pb.GeneratedMessage {
  factory MoveThreadToSectionRequest({
    $core.String? threadId,
    $core.String? sectionId,
  }) {
    final result = MoveThreadToSectionRequest._();
    if (threadId != null) result.threadId = threadId;
    if (sectionId != null) result.sectionId = sectionId;
    return result;
  }

  MoveThreadToSectionRequest._();

  factory MoveThreadToSectionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MoveThreadToSectionRequest()..mergeFromBuffer(data, registry);
  factory MoveThreadToSectionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MoveThreadToSectionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveThreadToSectionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: MoveThreadToSectionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'sectionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveThreadToSectionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveThreadToSectionRequest copyWith(
          void Function(MoveThreadToSectionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as MoveThreadToSectionRequest))
          as MoveThreadToSectionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use MoveThreadToSectionRequest() / MoveThreadToSectionRequest.new instead')
  static MoveThreadToSectionRequest create() => MoveThreadToSectionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      MoveThreadToSectionRequest._();
  @$core.override
  MoveThreadToSectionRequest createEmptyInstance() =>
      MoveThreadToSectionRequest._();
  @$core.pragma('dart2js:noInline')
  static MoveThreadToSectionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MoveThreadToSectionRequest>(
          MoveThreadToSectionRequest.$_createMessage);
  static MoveThreadToSectionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sectionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sectionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSectionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSectionId() => $_clearField(2);
}

class ListCodexThreadsRequest extends $pb.GeneratedMessage {
  factory ListCodexThreadsRequest() => ListCodexThreadsRequest._();

  ListCodexThreadsRequest._();

  factory ListCodexThreadsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexThreadsRequest()..mergeFromBuffer(data, registry);
  factory ListCodexThreadsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexThreadsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCodexThreadsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCodexThreadsRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexThreadsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexThreadsRequest copyWith(
          void Function(ListCodexThreadsRequest) updates) =>
      super.copyWith((message) => updates(message as ListCodexThreadsRequest))
          as ListCodexThreadsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCodexThreadsRequest() / ListCodexThreadsRequest.new instead')
  static ListCodexThreadsRequest create() => ListCodexThreadsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListCodexThreadsRequest._();
  @$core.override
  ListCodexThreadsRequest createEmptyInstance() => ListCodexThreadsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCodexThreadsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCodexThreadsRequest>(
          ListCodexThreadsRequest.$_createMessage);
  static ListCodexThreadsRequest? _defaultInstance;
}

class CodexThreadInfo extends $pb.GeneratedMessage {
  factory CodexThreadInfo({
    $core.String? id,
    $core.String? title,
    $core.String? cwd,
    $fixnum.Int64? updatedAtUnixMs,
  }) {
    final result = CodexThreadInfo._();
    if (id != null) result.id = id;
    if (title != null) result.title = title;
    if (cwd != null) result.cwd = cwd;
    if (updatedAtUnixMs != null) result.updatedAtUnixMs = updatedAtUnixMs;
    return result;
  }

  CodexThreadInfo._();

  factory CodexThreadInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexThreadInfo()..mergeFromBuffer(data, registry);
  factory CodexThreadInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexThreadInfo()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexThreadInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexThreadInfo.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOS(3, _omitFieldNames ? '' : 'cwd')
    ..aInt64(4, _omitFieldNames ? '' : 'updatedAtUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexThreadInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexThreadInfo copyWith(void Function(CodexThreadInfo) updates) =>
      super.copyWith((message) => updates(message as CodexThreadInfo))
          as CodexThreadInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexThreadInfo() / CodexThreadInfo.new instead')
  static CodexThreadInfo create() => CodexThreadInfo._();
  static $pb.GeneratedMessage $_createMessage() => CodexThreadInfo._();
  @$core.override
  CodexThreadInfo createEmptyInstance() => CodexThreadInfo._();
  @$core.pragma('dart2js:noInline')
  static CodexThreadInfo getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CodexThreadInfo>(
          CodexThreadInfo.$_createMessage);
  static CodexThreadInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get cwd => $_getSZ(2);
  @$pb.TagNumber(3)
  set cwd($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCwd() => $_has(2);
  @$pb.TagNumber(3)
  void clearCwd() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get updatedAtUnixMs => $_getI64(3);
  @$pb.TagNumber(4)
  set updatedAtUnixMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUpdatedAtUnixMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearUpdatedAtUnixMs() => $_clearField(4);
}

class ListCodexThreadsResponse extends $pb.GeneratedMessage {
  factory ListCodexThreadsResponse({
    $core.Iterable<CodexThreadInfo>? threads,
  }) {
    final result = ListCodexThreadsResponse._();
    if (threads != null) result.threads.addAll(threads);
    return result;
  }

  ListCodexThreadsResponse._();

  factory ListCodexThreadsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexThreadsResponse()..mergeFromBuffer(data, registry);
  factory ListCodexThreadsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexThreadsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCodexThreadsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCodexThreadsResponse.$_createMessage)
    ..pPM<CodexThreadInfo>(1, _omitFieldNames ? '' : 'threads',
        subBuilder: CodexThreadInfo.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexThreadsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexThreadsResponse copyWith(
          void Function(ListCodexThreadsResponse) updates) =>
      super.copyWith((message) => updates(message as ListCodexThreadsResponse))
          as ListCodexThreadsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCodexThreadsResponse() / ListCodexThreadsResponse.new instead')
  static ListCodexThreadsResponse create() => ListCodexThreadsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListCodexThreadsResponse._();
  @$core.override
  ListCodexThreadsResponse createEmptyInstance() =>
      ListCodexThreadsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCodexThreadsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCodexThreadsResponse>(
          ListCodexThreadsResponse.$_createMessage);
  static ListCodexThreadsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<CodexThreadInfo> get threads => $_getList(0);
}

class ImportCodexThreadRequest extends $pb.GeneratedMessage {
  factory ImportCodexThreadRequest({
    $core.String? codexThreadId,
  }) {
    final result = ImportCodexThreadRequest._();
    if (codexThreadId != null) result.codexThreadId = codexThreadId;
    return result;
  }

  ImportCodexThreadRequest._();

  factory ImportCodexThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportCodexThreadRequest()..mergeFromBuffer(data, registry);
  factory ImportCodexThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportCodexThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportCodexThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ImportCodexThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'codexThreadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCodexThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCodexThreadRequest copyWith(
          void Function(ImportCodexThreadRequest) updates) =>
      super.copyWith((message) => updates(message as ImportCodexThreadRequest))
          as ImportCodexThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ImportCodexThreadRequest() / ImportCodexThreadRequest.new instead')
  static ImportCodexThreadRequest create() => ImportCodexThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => ImportCodexThreadRequest._();
  @$core.override
  ImportCodexThreadRequest createEmptyInstance() =>
      ImportCodexThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static ImportCodexThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImportCodexThreadRequest>(
          ImportCodexThreadRequest.$_createMessage);
  static ImportCodexThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get codexThreadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set codexThreadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCodexThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCodexThreadId() => $_clearField(1);
}

class ImportCodexThreadResponse extends $pb.GeneratedMessage {
  factory ImportCodexThreadResponse({
    $core.String? threadId,
  }) {
    final result = ImportCodexThreadResponse._();
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  ImportCodexThreadResponse._();

  factory ImportCodexThreadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportCodexThreadResponse()..mergeFromBuffer(data, registry);
  factory ImportCodexThreadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportCodexThreadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportCodexThreadResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ImportCodexThreadResponse.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCodexThreadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportCodexThreadResponse copyWith(
          void Function(ImportCodexThreadResponse) updates) =>
      super.copyWith((message) => updates(message as ImportCodexThreadResponse))
          as ImportCodexThreadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ImportCodexThreadResponse() / ImportCodexThreadResponse.new instead')
  static ImportCodexThreadResponse create() => ImportCodexThreadResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ImportCodexThreadResponse._();
  @$core.override
  ImportCodexThreadResponse createEmptyInstance() =>
      ImportCodexThreadResponse._();
  @$core.pragma('dart2js:noInline')
  static ImportCodexThreadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImportCodexThreadResponse>(
          ImportCodexThreadResponse.$_createMessage);
  static ImportCodexThreadResponse? _defaultInstance;

  @$pb.TagNumber(2)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(2)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(2)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(2)
  void clearThreadId() => $_clearField(2);
}

class ListAgentThreadsRequest extends $pb.GeneratedMessage {
  factory ListAgentThreadsRequest({
    $core.String? agentId,
  }) {
    final result = ListAgentThreadsRequest._();
    if (agentId != null) result.agentId = agentId;
    return result;
  }

  ListAgentThreadsRequest._();

  factory ListAgentThreadsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentThreadsRequest()..mergeFromBuffer(data, registry);
  factory ListAgentThreadsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentThreadsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAgentThreadsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListAgentThreadsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentThreadsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentThreadsRequest copyWith(
          void Function(ListAgentThreadsRequest) updates) =>
      super.copyWith((message) => updates(message as ListAgentThreadsRequest))
          as ListAgentThreadsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListAgentThreadsRequest() / ListAgentThreadsRequest.new instead')
  static ListAgentThreadsRequest create() => ListAgentThreadsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListAgentThreadsRequest._();
  @$core.override
  ListAgentThreadsRequest createEmptyInstance() => ListAgentThreadsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListAgentThreadsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAgentThreadsRequest>(
          ListAgentThreadsRequest.$_createMessage);
  static ListAgentThreadsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);
}

class AgentThreadInfo extends $pb.GeneratedMessage {
  factory AgentThreadInfo({
    $core.String? agentId,
    $core.String? sessionId,
    $core.String? sessionFile,
    $core.String? cwd,
    $core.String? title,
    $fixnum.Int64? updatedAtUnixMs,
  }) {
    final result = AgentThreadInfo._();
    if (agentId != null) result.agentId = agentId;
    if (sessionId != null) result.sessionId = sessionId;
    if (sessionFile != null) result.sessionFile = sessionFile;
    if (cwd != null) result.cwd = cwd;
    if (title != null) result.title = title;
    if (updatedAtUnixMs != null) result.updatedAtUnixMs = updatedAtUnixMs;
    return result;
  }

  AgentThreadInfo._();

  factory AgentThreadInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentThreadInfo()..mergeFromBuffer(data, registry);
  factory AgentThreadInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentThreadInfo()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentThreadInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentThreadInfo.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'sessionId')
    ..aOS(3, _omitFieldNames ? '' : 'sessionFile')
    ..aOS(4, _omitFieldNames ? '' : 'cwd')
    ..aOS(5, _omitFieldNames ? '' : 'title')
    ..aInt64(6, _omitFieldNames ? '' : 'updatedAtUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentThreadInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentThreadInfo copyWith(void Function(AgentThreadInfo) updates) =>
      super.copyWith((message) => updates(message as AgentThreadInfo))
          as AgentThreadInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AgentThreadInfo() / AgentThreadInfo.new instead')
  static AgentThreadInfo create() => AgentThreadInfo._();
  static $pb.GeneratedMessage $_createMessage() => AgentThreadInfo._();
  @$core.override
  AgentThreadInfo createEmptyInstance() => AgentThreadInfo._();
  @$core.pragma('dart2js:noInline')
  static AgentThreadInfo getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AgentThreadInfo>(
          AgentThreadInfo.$_createMessage);
  static AgentThreadInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sessionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sessionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSessionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSessionId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sessionFile => $_getSZ(2);
  @$pb.TagNumber(3)
  set sessionFile($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSessionFile() => $_has(2);
  @$pb.TagNumber(3)
  void clearSessionFile() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get cwd => $_getSZ(3);
  @$pb.TagNumber(4)
  set cwd($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCwd() => $_has(3);
  @$pb.TagNumber(4)
  void clearCwd() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get title => $_getSZ(4);
  @$pb.TagNumber(5)
  set title($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTitle() => $_has(4);
  @$pb.TagNumber(5)
  void clearTitle() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get updatedAtUnixMs => $_getI64(5);
  @$pb.TagNumber(6)
  set updatedAtUnixMs($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasUpdatedAtUnixMs() => $_has(5);
  @$pb.TagNumber(6)
  void clearUpdatedAtUnixMs() => $_clearField(6);
}

class ListAgentThreadsResponse extends $pb.GeneratedMessage {
  factory ListAgentThreadsResponse({
    $core.Iterable<AgentThreadInfo>? threads,
  }) {
    final result = ListAgentThreadsResponse._();
    if (threads != null) result.threads.addAll(threads);
    return result;
  }

  ListAgentThreadsResponse._();

  factory ListAgentThreadsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentThreadsResponse()..mergeFromBuffer(data, registry);
  factory ListAgentThreadsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentThreadsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAgentThreadsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListAgentThreadsResponse.$_createMessage)
    ..pPM<AgentThreadInfo>(1, _omitFieldNames ? '' : 'threads',
        subBuilder: AgentThreadInfo.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentThreadsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentThreadsResponse copyWith(
          void Function(ListAgentThreadsResponse) updates) =>
      super.copyWith((message) => updates(message as ListAgentThreadsResponse))
          as ListAgentThreadsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListAgentThreadsResponse() / ListAgentThreadsResponse.new instead')
  static ListAgentThreadsResponse create() => ListAgentThreadsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListAgentThreadsResponse._();
  @$core.override
  ListAgentThreadsResponse createEmptyInstance() =>
      ListAgentThreadsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListAgentThreadsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAgentThreadsResponse>(
          ListAgentThreadsResponse.$_createMessage);
  static ListAgentThreadsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<AgentThreadInfo> get threads => $_getList(0);
}

/// 导入请求:手机端把 ListAgentThreads 里选中的一项原样回传即可。
/// session_id / session_file 至少有一个非空,daemon 才能在续聊时重挂载外部会话。
class ImportAgentThreadRequest extends $pb.GeneratedMessage {
  factory ImportAgentThreadRequest({
    $core.String? agentId,
    $core.String? sessionId,
    $core.String? sessionFile,
    $core.String? cwd,
    $core.String? title,
  }) {
    final result = ImportAgentThreadRequest._();
    if (agentId != null) result.agentId = agentId;
    if (sessionId != null) result.sessionId = sessionId;
    if (sessionFile != null) result.sessionFile = sessionFile;
    if (cwd != null) result.cwd = cwd;
    if (title != null) result.title = title;
    return result;
  }

  ImportAgentThreadRequest._();

  factory ImportAgentThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportAgentThreadRequest()..mergeFromBuffer(data, registry);
  factory ImportAgentThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportAgentThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportAgentThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ImportAgentThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'sessionId')
    ..aOS(3, _omitFieldNames ? '' : 'sessionFile')
    ..aOS(4, _omitFieldNames ? '' : 'cwd')
    ..aOS(5, _omitFieldNames ? '' : 'title')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportAgentThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportAgentThreadRequest copyWith(
          void Function(ImportAgentThreadRequest) updates) =>
      super.copyWith((message) => updates(message as ImportAgentThreadRequest))
          as ImportAgentThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ImportAgentThreadRequest() / ImportAgentThreadRequest.new instead')
  static ImportAgentThreadRequest create() => ImportAgentThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => ImportAgentThreadRequest._();
  @$core.override
  ImportAgentThreadRequest createEmptyInstance() =>
      ImportAgentThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static ImportAgentThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImportAgentThreadRequest>(
          ImportAgentThreadRequest.$_createMessage);
  static ImportAgentThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sessionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sessionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSessionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSessionId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sessionFile => $_getSZ(2);
  @$pb.TagNumber(3)
  set sessionFile($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSessionFile() => $_has(2);
  @$pb.TagNumber(3)
  void clearSessionFile() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get cwd => $_getSZ(3);
  @$pb.TagNumber(4)
  set cwd($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCwd() => $_has(3);
  @$pb.TagNumber(4)
  void clearCwd() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get title => $_getSZ(4);
  @$pb.TagNumber(5)
  set title($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTitle() => $_has(4);
  @$pb.TagNumber(5)
  void clearTitle() => $_clearField(5);
}

class ImportAgentThreadResponse extends $pb.GeneratedMessage {
  factory ImportAgentThreadResponse({
    $core.String? threadId,
  }) {
    final result = ImportAgentThreadResponse._();
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  ImportAgentThreadResponse._();

  factory ImportAgentThreadResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportAgentThreadResponse()..mergeFromBuffer(data, registry);
  factory ImportAgentThreadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImportAgentThreadResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImportAgentThreadResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ImportAgentThreadResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportAgentThreadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImportAgentThreadResponse copyWith(
          void Function(ImportAgentThreadResponse) updates) =>
      super.copyWith((message) => updates(message as ImportAgentThreadResponse))
          as ImportAgentThreadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ImportAgentThreadResponse() / ImportAgentThreadResponse.new instead')
  static ImportAgentThreadResponse create() => ImportAgentThreadResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ImportAgentThreadResponse._();
  @$core.override
  ImportAgentThreadResponse createEmptyInstance() =>
      ImportAgentThreadResponse._();
  @$core.pragma('dart2js:noInline')
  static ImportAgentThreadResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImportAgentThreadResponse>(
          ImportAgentThreadResponse.$_createMessage);
  static ImportAgentThreadResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);
}

class GetThreadHistoryRequest extends $pb.GeneratedMessage {
  factory GetThreadHistoryRequest({
    $core.String? threadId,
    $core.int? limit,
    $core.int? rounds,
  }) {
    final result = GetThreadHistoryRequest._();
    if (threadId != null) result.threadId = threadId;
    if (limit != null) result.limit = limit;
    if (rounds != null) result.rounds = rounds;
    return result;
  }

  GetThreadHistoryRequest._();

  factory GetThreadHistoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadHistoryRequest()..mergeFromBuffer(data, registry);
  factory GetThreadHistoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadHistoryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetThreadHistoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetThreadHistoryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aI(2, _omitFieldNames ? '' : 'limit')
    ..aI(3, _omitFieldNames ? '' : 'rounds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadHistoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadHistoryRequest copyWith(
          void Function(GetThreadHistoryRequest) updates) =>
      super.copyWith((message) => updates(message as GetThreadHistoryRequest))
          as GetThreadHistoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetThreadHistoryRequest() / GetThreadHistoryRequest.new instead')
  static GetThreadHistoryRequest create() => GetThreadHistoryRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetThreadHistoryRequest._();
  @$core.override
  GetThreadHistoryRequest createEmptyInstance() => GetThreadHistoryRequest._();
  @$core.pragma('dart2js:noInline')
  static GetThreadHistoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetThreadHistoryRequest>(
          GetThreadHistoryRequest.$_createMessage);
  static GetThreadHistoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get limit => $_getIZ(1);
  @$pb.TagNumber(2)
  set limit($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLimit() => $_has(1);
  @$pb.TagNumber(2)
  void clearLimit() => $_clearField(2);

  /// 按轮截断:0 = 不限(兼容旧客户端:存量 APK 不传本字段,必须仍返回全量,
  /// 导出全量与断流恢复回填都依赖这个语义)。一轮 = 一条 user 起始,到下一条
  /// user 之前(失败轮可能只有 user 无 assistant;导入/fork 种子首条不保证
  /// user)。与 limit 并存时先按 limit 尾截,再按 rounds 叠加收窄;默认值
  /// (最近 20 轮)由 android 端显式传,服务端不内置默认。
  @$pb.TagNumber(3)
  $core.int get rounds => $_getIZ(2);
  @$pb.TagNumber(3)
  set rounds($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRounds() => $_has(2);
  @$pb.TagNumber(3)
  void clearRounds() => $_clearField(3);
}

class GetThreadHistoryResponse extends $pb.GeneratedMessage {
  factory GetThreadHistoryResponse({
    $core.Iterable<ThreadMessage>? messages,
    CodexGoalDetail? goalDetail,
  }) {
    final result = GetThreadHistoryResponse._();
    if (messages != null) result.messages.addAll(messages);
    if (goalDetail != null) result.goalDetail = goalDetail;
    return result;
  }

  GetThreadHistoryResponse._();

  factory GetThreadHistoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadHistoryResponse()..mergeFromBuffer(data, registry);
  factory GetThreadHistoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadHistoryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetThreadHistoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetThreadHistoryResponse.$_createMessage)
    ..pPM<ThreadMessage>(1, _omitFieldNames ? '' : 'messages',
        subBuilder: ThreadMessage.$_createMessage)
    ..aOM<CodexGoalDetail>(2, _omitFieldNames ? '' : 'goalDetail',
        subBuilder: CodexGoalDetail.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadHistoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadHistoryResponse copyWith(
          void Function(GetThreadHistoryResponse) updates) =>
      super.copyWith((message) => updates(message as GetThreadHistoryResponse))
          as GetThreadHistoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetThreadHistoryResponse() / GetThreadHistoryResponse.new instead')
  static GetThreadHistoryResponse create() => GetThreadHistoryResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetThreadHistoryResponse._();
  @$core.override
  GetThreadHistoryResponse createEmptyInstance() =>
      GetThreadHistoryResponse._();
  @$core.pragma('dart2js:noInline')
  static GetThreadHistoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetThreadHistoryResponse>(
          GetThreadHistoryResponse.$_createMessage);
  static GetThreadHistoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ThreadMessage> get messages => $_getList(0);

  /// 打开会话时顺带回读的 codex 目标态(仅 codex 线程;非 codex / 读不到 = 缺省,
  /// 手机端有则显示"目标:xxx",无则不占位)。
  @$pb.TagNumber(2)
  CodexGoalDetail get goalDetail => $_getN(1);
  @$pb.TagNumber(2)
  set goalDetail(CodexGoalDetail value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasGoalDetail() => $_has(1);
  @$pb.TagNumber(2)
  void clearGoalDetail() => $_clearField(2);
  @$pb.TagNumber(2)
  CodexGoalDetail ensureGoalDetail() => $_ensure(1);
}

class ThreadMessage extends $pb.GeneratedMessage {
  factory ThreadMessage({
    $core.String? role,
    $core.Iterable<$1.ContentBlock>? blocks,
    $fixnum.Int64? createdAtUnixMs,
    $core.String? messageId,
    $core.String? turnId,
  }) {
    final result = ThreadMessage._();
    if (role != null) result.role = role;
    if (blocks != null) result.blocks.addAll(blocks);
    if (createdAtUnixMs != null) result.createdAtUnixMs = createdAtUnixMs;
    if (messageId != null) result.messageId = messageId;
    if (turnId != null) result.turnId = turnId;
    return result;
  }

  ThreadMessage._();

  factory ThreadMessage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadMessage()..mergeFromBuffer(data, registry);
  factory ThreadMessage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadMessage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThreadMessage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ThreadMessage.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'role')
    ..pPM<$1.ContentBlock>(2, _omitFieldNames ? '' : 'blocks',
        subBuilder: $1.ContentBlock.$_createMessage)
    ..aInt64(3, _omitFieldNames ? '' : 'createdAtUnixMs')
    ..aOS(4, _omitFieldNames ? '' : 'messageId')
    ..aOS(5, _omitFieldNames ? '' : 'turnId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadMessage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadMessage copyWith(void Function(ThreadMessage) updates) =>
      super.copyWith((message) => updates(message as ThreadMessage))
          as ThreadMessage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ThreadMessage() / ThreadMessage.new instead')
  static ThreadMessage create() => ThreadMessage._();
  static $pb.GeneratedMessage $_createMessage() => ThreadMessage._();
  @$core.override
  ThreadMessage createEmptyInstance() => ThreadMessage._();
  @$core.pragma('dart2js:noInline')
  static ThreadMessage getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ThreadMessage>(
          ThreadMessage.$_createMessage);
  static ThreadMessage? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get role => $_getSZ(0);
  @$pb.TagNumber(1)
  set role($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRole() => $_has(0);
  @$pb.TagNumber(1)
  void clearRole() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$1.ContentBlock> get blocks => $_getList(1);

  @$pb.TagNumber(3)
  $fixnum.Int64 get createdAtUnixMs => $_getI64(2);
  @$pb.TagNumber(3)
  set createdAtUnixMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCreatedAtUnixMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearCreatedAtUnixMs() => $_clearField(3);

  /// ---- 消息稳定身份(只增:4~5,2026-10-04 会话选择与历史体验计划 Task 5;
  /// 分配登记见 proto/FIELD_ALLOCATIONS.md)----
  /// 【为什么需要稳定 id】历史搜索/跳转原来只能靠 sequence(数组下标)定位,而
  /// 加载更早、fork、rollback 都会改变下标 → 定位漂移。message_id 是 daemon 侧
  /// 稳定身份:新消息落库时生成、永久不变;旧数据由 daemon **启动时**幂等迁移补齐
  /// (thread + index + created_at + 内容 hash;重复启动不改变已有 id)。
  /// fork/rollback/import 对 id 是保持还是重新生成的语义见 store(store.go)。
  /// 手机端历史定位一律优先用它;sequence 降级为兼容字段。空 = 旧 daemon 未带,
  /// 客户端退回 sequence 兜底,不报错。
  @$pb.TagNumber(4)
  $core.String get messageId => $_getSZ(3);
  @$pb.TagNumber(4)
  set messageId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMessageId() => $_has(3);
  @$pb.TagNumber(4)
  void clearMessageId() => $_clearField(4);

  /// 消息所属轮次的 id(daemon 侧口径,与 TurnStarted.turn_id 同域);
  /// 空 = 未知/旧数据未迁移出轮次归属。
  @$pb.TagNumber(5)
  $core.String get turnId => $_getSZ(4);
  @$pb.TagNumber(5)
  set turnId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTurnId() => $_has(4);
  @$pb.TagNumber(5)
  void clearTurnId() => $_clearField(5);
}

class Thread extends $pb.GeneratedMessage {
  factory Thread({
    $core.String? id,
    $core.String? title,
    $core.String? workspace,
    $core.String? goal,
    $core.String? modelId,
    ThreadStatus? status,
    $fixnum.Int64? createdAtUnixMs,
    $fixnum.Int64? updatedAtUnixMs,
    $core.int? messageCount,
    $core.String? codexThreadId,
    $core.String? sectionId,
    $core.bool? archived,
    $core.String? guardianDeniedReviewId,
    $core.String? guardianDeniedItemId,
    $core.String? guardianDeniedReason,
    $core.String? agentId,
    ExternalSessionRef? externalSession,
    CodexGoalDetail? goalDetail,
    $core.String? effort,
    $core.String? lastTurnError,
    $core.Iterable<ApprovalRequest>? pendingApprovals,
    $core.String? parentThreadId,
    $fixnum.Int64? turnStartedUnixMs,
    $2.EffectiveModelSelection? effectiveSelection,
  }) {
    final result = Thread._();
    if (id != null) result.id = id;
    if (title != null) result.title = title;
    if (workspace != null) result.workspace = workspace;
    if (goal != null) result.goal = goal;
    if (modelId != null) result.modelId = modelId;
    if (status != null) result.status = status;
    if (createdAtUnixMs != null) result.createdAtUnixMs = createdAtUnixMs;
    if (updatedAtUnixMs != null) result.updatedAtUnixMs = updatedAtUnixMs;
    if (messageCount != null) result.messageCount = messageCount;
    if (codexThreadId != null) result.codexThreadId = codexThreadId;
    if (sectionId != null) result.sectionId = sectionId;
    if (archived != null) result.archived = archived;
    if (guardianDeniedReviewId != null)
      result.guardianDeniedReviewId = guardianDeniedReviewId;
    if (guardianDeniedItemId != null)
      result.guardianDeniedItemId = guardianDeniedItemId;
    if (guardianDeniedReason != null)
      result.guardianDeniedReason = guardianDeniedReason;
    if (agentId != null) result.agentId = agentId;
    if (externalSession != null) result.externalSession = externalSession;
    if (goalDetail != null) result.goalDetail = goalDetail;
    if (effort != null) result.effort = effort;
    if (lastTurnError != null) result.lastTurnError = lastTurnError;
    if (pendingApprovals != null)
      result.pendingApprovals.addAll(pendingApprovals);
    if (parentThreadId != null) result.parentThreadId = parentThreadId;
    if (turnStartedUnixMs != null) result.turnStartedUnixMs = turnStartedUnixMs;
    if (effectiveSelection != null)
      result.effectiveSelection = effectiveSelection;
    return result;
  }

  Thread._();

  factory Thread.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Thread()..mergeFromBuffer(data, registry);
  factory Thread.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Thread()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Thread',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: Thread.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOS(3, _omitFieldNames ? '' : 'workspace')
    ..aOS(4, _omitFieldNames ? '' : 'goal')
    ..aOS(5, _omitFieldNames ? '' : 'modelId')
    ..aE<ThreadStatus>(6, _omitFieldNames ? '' : 'status',
        enumValues: ThreadStatus.values)
    ..aInt64(7, _omitFieldNames ? '' : 'createdAtUnixMs')
    ..aInt64(8, _omitFieldNames ? '' : 'updatedAtUnixMs')
    ..aI(9, _omitFieldNames ? '' : 'messageCount')
    ..aOS(10, _omitFieldNames ? '' : 'codexThreadId')
    ..aOS(11, _omitFieldNames ? '' : 'sectionId')
    ..aOB(12, _omitFieldNames ? '' : 'archived')
    ..aOS(13, _omitFieldNames ? '' : 'guardianDeniedReviewId')
    ..aOS(14, _omitFieldNames ? '' : 'guardianDeniedItemId')
    ..aOS(15, _omitFieldNames ? '' : 'guardianDeniedReason')
    ..aOS(16, _omitFieldNames ? '' : 'agentId')
    ..aOM<ExternalSessionRef>(17, _omitFieldNames ? '' : 'externalSession',
        subBuilder: ExternalSessionRef.$_createMessage)
    ..aOM<CodexGoalDetail>(18, _omitFieldNames ? '' : 'goalDetail',
        subBuilder: CodexGoalDetail.$_createMessage)
    ..aOS(19, _omitFieldNames ? '' : 'effort')
    ..aOS(20, _omitFieldNames ? '' : 'lastTurnError')
    ..pPM<ApprovalRequest>(21, _omitFieldNames ? '' : 'pendingApprovals',
        subBuilder: ApprovalRequest.$_createMessage)
    ..aOS(22, _omitFieldNames ? '' : 'parentThreadId')
    ..aInt64(23, _omitFieldNames ? '' : 'turnStartedUnixMs')
    ..aOM<$2.EffectiveModelSelection>(
        24, _omitFieldNames ? '' : 'effectiveSelection',
        subBuilder: $2.EffectiveModelSelection.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Thread clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Thread copyWith(void Function(Thread) updates) =>
      super.copyWith((message) => updates(message as Thread)) as Thread;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Thread() / Thread.new instead')
  static Thread create() => Thread._();
  static $pb.GeneratedMessage $_createMessage() => Thread._();
  @$core.override
  Thread createEmptyInstance() => Thread._();
  @$core.pragma('dart2js:noInline')
  static Thread getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Thread>(Thread.$_createMessage);
  static Thread? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get workspace => $_getSZ(2);
  @$pb.TagNumber(3)
  set workspace($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWorkspace() => $_has(2);
  @$pb.TagNumber(3)
  void clearWorkspace() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get goal => $_getSZ(3);
  @$pb.TagNumber(4)
  set goal($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGoal() => $_has(3);
  @$pb.TagNumber(4)
  void clearGoal() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get modelId => $_getSZ(4);
  @$pb.TagNumber(5)
  set modelId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasModelId() => $_has(4);
  @$pb.TagNumber(5)
  void clearModelId() => $_clearField(5);

  @$pb.TagNumber(6)
  ThreadStatus get status => $_getN(5);
  @$pb.TagNumber(6)
  set status(ThreadStatus value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasStatus() => $_has(5);
  @$pb.TagNumber(6)
  void clearStatus() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get createdAtUnixMs => $_getI64(6);
  @$pb.TagNumber(7)
  set createdAtUnixMs($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCreatedAtUnixMs() => $_has(6);
  @$pb.TagNumber(7)
  void clearCreatedAtUnixMs() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get updatedAtUnixMs => $_getI64(7);
  @$pb.TagNumber(8)
  set updatedAtUnixMs($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasUpdatedAtUnixMs() => $_has(7);
  @$pb.TagNumber(8)
  void clearUpdatedAtUnixMs() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.int get messageCount => $_getIZ(8);
  @$pb.TagNumber(9)
  set messageCount($core.int value) => $_setSignedInt32(8, value);
  @$pb.TagNumber(9)
  $core.bool hasMessageCount() => $_has(8);
  @$pb.TagNumber(9)
  void clearMessageCount() => $_clearField(9);

  /// 桥接的 codex 线程 id:非空表示本会话来自电脑上的 codex 会话。
  /// 手机端用它把"活跃"与"电脑"两处同一个会话合并成一条(不去重会出现两份)。
  @$pb.TagNumber(10)
  $core.String get codexThreadId => $_getSZ(9);
  @$pb.TagNumber(10)
  set codexThreadId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasCodexThreadId() => $_has(9);
  @$pb.TagNumber(10)
  void clearCodexThreadId() => $_clearField(10);

  /// ---- 会话组织(字段号从 11 起,旧字段语义一字未动) ----
  @$pb.TagNumber(11)
  $core.String get sectionId => $_getSZ(10);
  @$pb.TagNumber(11)
  set sectionId($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasSectionId() => $_has(10);
  @$pb.TagNumber(11)
  void clearSectionId() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.bool get archived => $_getBF(11);
  @$pb.TagNumber(12)
  set archived($core.bool value) => $_setBool(11, value);
  @$pb.TagNumber(12)
  $core.bool hasArchived() => $_has(11);
  @$pb.TagNumber(12)
  void clearArchived() => $_clearField(12);

  /// 审查者(guardian)最近一次驳回:非空表示"自动审查已经拒了这个动作,
  /// 用户可以推翻"。手机端据此显示"仍然执行"入口。
  @$pb.TagNumber(13)
  $core.String get guardianDeniedReviewId => $_getSZ(12);
  @$pb.TagNumber(13)
  set guardianDeniedReviewId($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasGuardianDeniedReviewId() => $_has(12);
  @$pb.TagNumber(13)
  void clearGuardianDeniedReviewId() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get guardianDeniedItemId => $_getSZ(13);
  @$pb.TagNumber(14)
  set guardianDeniedItemId($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasGuardianDeniedItemId() => $_has(13);
  @$pb.TagNumber(14)
  void clearGuardianDeniedItemId() => $_clearField(14);

  @$pb.TagNumber(15)
  $core.String get guardianDeniedReason => $_getSZ(14);
  @$pb.TagNumber(15)
  set guardianDeniedReason($core.String value) => $_setString(14, value);
  @$pb.TagNumber(15)
  $core.bool hasGuardianDeniedReason() => $_has(14);
  @$pb.TagNumber(15)
  void clearGuardianDeniedReason() => $_clearField(15);

  /// ---- 多 agent 接入(只增:16~17)----
  /// 会话归属的 agent:"codex" / "pi" / …。老记录缺省 = "codex"(见 store 兼容规则)。
  /// providerFor 按它显式选择 provider,不再静默回退默认。
  @$pb.TagNumber(16)
  $core.String get agentId => $_getSZ(15);
  @$pb.TagNumber(16)
  set agentId($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasAgentId() => $_has(15);
  @$pb.TagNumber(16)
  void clearAgentId() => $_clearField(16);

  /// 外部 agent 的原生会话引用(Pi/ZCode 通用):daemon 重启后据此 switch_session
  /// 重挂载子进程会话(调研 §8.3 的握手依据)。
  @$pb.TagNumber(17)
  ExternalSessionRef get externalSession => $_getN(16);
  @$pb.TagNumber(17)
  set externalSession(ExternalSessionRef value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasExternalSession() => $_has(16);
  @$pb.TagNumber(17)
  void clearExternalSession() => $_clearField(17);
  @$pb.TagNumber(17)
  ExternalSessionRef ensureExternalSession() => $_ensure(16);

  /// ---- codex 目标态回读(只增:18)----
  /// 【与字段 4 goal 的分工】4 是 daemon 本库落库的常驻目标(手机设置后写的值);
  /// goal_detail 是打开会话时 daemon 从电脑端 codex 线程**现读**的目标态
  /// (thread/goal/get),带状态机与 token 消耗 —— 重连/换端后以它为准刷新。
  /// 仅 codex 线程填充;读不到/非 codex 会话缺省(手机端"有则显示,无则不占位")。
  @$pb.TagNumber(18)
  CodexGoalDetail get goalDetail => $_getN(17);
  @$pb.TagNumber(18)
  set goalDetail(CodexGoalDetail value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasGoalDetail() => $_has(17);
  @$pb.TagNumber(18)
  void clearGoalDetail() => $_clearField(18);
  @$pb.TagNumber(18)
  CodexGoalDetail ensureGoalDetail() => $_ensure(17);

  @$pb.TagNumber(19)
  $core.String get effort => $_getSZ(18);
  @$pb.TagNumber(19)
  set effort($core.String value) => $_setString(18, value);
  @$pb.TagNumber(19)
  $core.bool hasEffort() => $_has(18);
  @$pb.TagNumber(19)
  void clearEffort() => $_clearField(19);

  @$pb.TagNumber(20)
  $core.String get lastTurnError => $_getSZ(19);
  @$pb.TagNumber(20)
  set lastTurnError($core.String value) => $_setString(19, value);
  @$pb.TagNumber(20)
  $core.bool hasLastTurnError() => $_has(19);
  @$pb.TagNumber(20)
  void clearLastTurnError() => $_clearField(20);

  @$pb.TagNumber(21)
  $pb.PbList<ApprovalRequest> get pendingApprovals => $_getList(20);

  /// ---- 子代理线程归属(只增:22)----
  /// 非空 = 该线程是某线程的子代理线程,值为**父线程的 daemon 本地线程 id**
  /// (手机端按它与当前打开会话的 id 匹配,点亮子代理内联卡/来源行;空 =
  /// 普通顶层会话,门控关闭不误报)。两条填充路:collab 委派子线程在建线程时
  /// 落库;codex 原生子线程在 thread/started 通知(带 parentThreadId)到达时以
  /// 轻量存根登记落库。
  @$pb.TagNumber(22)
  $core.String get parentThreadId => $_getSZ(21);
  @$pb.TagNumber(22)
  set parentThreadId($core.String value) => $_setString(21, value);
  @$pb.TagNumber(22)
  $core.bool hasParentThreadId() => $_has(21);
  @$pb.TagNumber(22)
  void clearParentThreadId() => $_clearField(22);

  /// ---- 轮次开始时刻(只增:23)----
  /// 当前 RUNNING 段的开始时刻。手机端对运行中会话显示"运行 N 分钟",
  /// 不再退化为"最近活动·相对时间"。optional 显式缺席 = 未知:旧 daemon
  /// 不带、尚未开轮的会话不带;daemon 重启后仍处于 RUNNING 的会话由 daemon
  /// 记**重启时刻**(真实开始不可追溯,仅作展示锚点)。
  @$pb.TagNumber(23)
  $fixnum.Int64 get turnStartedUnixMs => $_getI64(22);
  @$pb.TagNumber(23)
  set turnStartedUnixMs($fixnum.Int64 value) => $_setInt64(22, value);
  @$pb.TagNumber(23)
  $core.bool hasTurnStartedUnixMs() => $_has(22);
  @$pb.TagNumber(23)
  void clearTurnStartedUnixMs() => $_clearField(23);

  /// ---- 有效模型选择(只增:24,2026-10-04 会话选择与历史体验计划 Task 1;
  /// 分配登记见 proto/FIELD_ALLOCATIONS.md)----
  /// 本会话解析后的选择真值(四级来源收敛结论 + 来源 + 生效时机 + 人话解释,
  /// 形状见 model.proto 的 EffectiveModelSelection)。旧 daemon 缺省 = 客户端按
  /// model_id(5)/effort(19) 兜底显示,不报错。
  @$pb.TagNumber(24)
  $2.EffectiveModelSelection get effectiveSelection => $_getN(23);
  @$pb.TagNumber(24)
  set effectiveSelection($2.EffectiveModelSelection value) =>
      $_setField(24, value);
  @$pb.TagNumber(24)
  $core.bool hasEffectiveSelection() => $_has(23);
  @$pb.TagNumber(24)
  void clearEffectiveSelection() => $_clearField(24);
  @$pb.TagNumber(24)
  $2.EffectiveModelSelection ensureEffectiveSelection() => $_ensure(23);
}

/// CodexGoalDetail codex 线程目标态(thread/goal/get 应答的归一)。
/// daemon 侧现读失败但本地库有目标时,objective 兜底为本地值(status 留空)。
class CodexGoalDetail extends $pb.GeneratedMessage {
  factory CodexGoalDetail({
    $core.String? objective,
    $core.String? status,
    $fixnum.Int64? tokensUsed,
    $fixnum.Int64? tokenBudget,
    $fixnum.Int64? timeUsedSeconds,
  }) {
    final result = CodexGoalDetail._();
    if (objective != null) result.objective = objective;
    if (status != null) result.status = status;
    if (tokensUsed != null) result.tokensUsed = tokensUsed;
    if (tokenBudget != null) result.tokenBudget = tokenBudget;
    if (timeUsedSeconds != null) result.timeUsedSeconds = timeUsedSeconds;
    return result;
  }

  CodexGoalDetail._();

  factory CodexGoalDetail.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexGoalDetail()..mergeFromBuffer(data, registry);
  factory CodexGoalDetail.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexGoalDetail()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexGoalDetail',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexGoalDetail.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'objective')
    ..aOS(2, _omitFieldNames ? '' : 'status')
    ..aInt64(3, _omitFieldNames ? '' : 'tokensUsed')
    ..aInt64(4, _omitFieldNames ? '' : 'tokenBudget')
    ..aInt64(5, _omitFieldNames ? '' : 'timeUsedSeconds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexGoalDetail clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexGoalDetail copyWith(void Function(CodexGoalDetail) updates) =>
      super.copyWith((message) => updates(message as CodexGoalDetail))
          as CodexGoalDetail;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexGoalDetail() / CodexGoalDetail.new instead')
  static CodexGoalDetail create() => CodexGoalDetail._();
  static $pb.GeneratedMessage $_createMessage() => CodexGoalDetail._();
  @$core.override
  CodexGoalDetail createEmptyInstance() => CodexGoalDetail._();
  @$core.pragma('dart2js:noInline')
  static CodexGoalDetail getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CodexGoalDetail>(
          CodexGoalDetail.$_createMessage);
  static CodexGoalDetail? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get objective => $_getSZ(0);
  @$pb.TagNumber(1)
  set objective($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasObjective() => $_has(0);
  @$pb.TagNumber(1)
  void clearObjective() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get status => $_getSZ(1);
  @$pb.TagNumber(2)
  set status($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get tokensUsed => $_getI64(2);
  @$pb.TagNumber(3)
  set tokensUsed($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTokensUsed() => $_has(2);
  @$pb.TagNumber(3)
  void clearTokensUsed() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get tokenBudget => $_getI64(3);
  @$pb.TagNumber(4)
  set tokenBudget($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTokenBudget() => $_has(3);
  @$pb.TagNumber(4)
  void clearTokenBudget() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get timeUsedSeconds => $_getI64(4);
  @$pb.TagNumber(5)
  set timeUsedSeconds($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTimeUsedSeconds() => $_has(4);
  @$pb.TagNumber(5)
  void clearTimeUsedSeconds() => $_clearField(5);
}

/// ExternalSessionRef 指向外部 agent 自己管理的会话事实来源。
/// Pi:session_file 是 ~/.pi/agent/sessions 下的 .jsonl;ZCode 同样以"外部会话 id
/// + 文件路径"重挂载。内容由外部 agent 持有,我们只存引用。
class ExternalSessionRef extends $pb.GeneratedMessage {
  factory ExternalSessionRef({
    $core.String? sessionId,
    $core.String? sessionFile,
    $core.String? cwd,
  }) {
    final result = ExternalSessionRef._();
    if (sessionId != null) result.sessionId = sessionId;
    if (sessionFile != null) result.sessionFile = sessionFile;
    if (cwd != null) result.cwd = cwd;
    return result;
  }

  ExternalSessionRef._();

  factory ExternalSessionRef.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExternalSessionRef()..mergeFromBuffer(data, registry);
  factory ExternalSessionRef.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ExternalSessionRef()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExternalSessionRef',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ExternalSessionRef.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'sessionFile')
    ..aOS(3, _omitFieldNames ? '' : 'cwd')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExternalSessionRef clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExternalSessionRef copyWith(void Function(ExternalSessionRef) updates) =>
      super.copyWith((message) => updates(message as ExternalSessionRef))
          as ExternalSessionRef;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ExternalSessionRef() / ExternalSessionRef.new instead')
  static ExternalSessionRef create() => ExternalSessionRef._();
  static $pb.GeneratedMessage $_createMessage() => ExternalSessionRef._();
  @$core.override
  ExternalSessionRef createEmptyInstance() => ExternalSessionRef._();
  @$core.pragma('dart2js:noInline')
  static ExternalSessionRef getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExternalSessionRef>(
          ExternalSessionRef.$_createMessage);
  static ExternalSessionRef? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sessionFile => $_getSZ(1);
  @$pb.TagNumber(2)
  set sessionFile($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSessionFile() => $_has(1);
  @$pb.TagNumber(2)
  void clearSessionFile() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get cwd => $_getSZ(2);
  @$pb.TagNumber(3)
  set cwd($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCwd() => $_has(2);
  @$pb.TagNumber(3)
  void clearCwd() => $_clearField(3);
}

class CreateThreadRequest extends $pb.GeneratedMessage {
  factory CreateThreadRequest({
    $core.String? workspace,
    $core.String? title,
    $core.String? modelId,
    $core.String? agentId,
  }) {
    final result = CreateThreadRequest._();
    if (workspace != null) result.workspace = workspace;
    if (title != null) result.title = title;
    if (modelId != null) result.modelId = modelId;
    if (agentId != null) result.agentId = agentId;
    return result;
  }

  CreateThreadRequest._();

  factory CreateThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateThreadRequest()..mergeFromBuffer(data, registry);
  factory CreateThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CreateThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'workspace')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOS(3, _omitFieldNames ? '' : 'modelId')
    ..aOS(4, _omitFieldNames ? '' : 'agentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateThreadRequest copyWith(void Function(CreateThreadRequest) updates) =>
      super.copyWith((message) => updates(message as CreateThreadRequest))
          as CreateThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use CreateThreadRequest() / CreateThreadRequest.new instead')
  static CreateThreadRequest create() => CreateThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateThreadRequest._();
  @$core.override
  CreateThreadRequest createEmptyInstance() => CreateThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateThreadRequest>(
          CreateThreadRequest.$_createMessage);
  static CreateThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get workspace => $_getSZ(0);
  @$pb.TagNumber(1)
  set workspace($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasWorkspace() => $_has(0);
  @$pb.TagNumber(1)
  void clearWorkspace() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get modelId => $_getSZ(2);
  @$pb.TagNumber(3)
  set modelId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasModelId() => $_has(2);
  @$pb.TagNumber(3)
  void clearModelId() => $_clearField(3);

  /// 目标 agent:"codex" / "pi" / …;空 = 默认(codex)。多 agent 接入(只增)。
  @$pb.TagNumber(4)
  $core.String get agentId => $_getSZ(3);
  @$pb.TagNumber(4)
  set agentId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAgentId() => $_has(3);
  @$pb.TagNumber(4)
  void clearAgentId() => $_clearField(4);
}

class ListThreadsRequest extends $pb.GeneratedMessage {
  factory ListThreadsRequest({
    $core.bool? includeArchived,
    $core.int? limit,
    $core.String? searchTerm,
    $core.String? sectionId,
    $core.String? sortKey,
    $core.bool? sortAscending,
  }) {
    final result = ListThreadsRequest._();
    if (includeArchived != null) result.includeArchived = includeArchived;
    if (limit != null) result.limit = limit;
    if (searchTerm != null) result.searchTerm = searchTerm;
    if (sectionId != null) result.sectionId = sectionId;
    if (sortKey != null) result.sortKey = sortKey;
    if (sortAscending != null) result.sortAscending = sortAscending;
    return result;
  }

  ListThreadsRequest._();

  factory ListThreadsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadsRequest()..mergeFromBuffer(data, registry);
  factory ListThreadsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListThreadsRequest.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'includeArchived')
    ..aI(2, _omitFieldNames ? '' : 'limit')
    ..aOS(3, _omitFieldNames ? '' : 'searchTerm')
    ..aOS(4, _omitFieldNames ? '' : 'sectionId')
    ..aOS(5, _omitFieldNames ? '' : 'sortKey')
    ..aOB(6, _omitFieldNames ? '' : 'sortAscending')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsRequest copyWith(void Function(ListThreadsRequest) updates) =>
      super.copyWith((message) => updates(message as ListThreadsRequest))
          as ListThreadsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListThreadsRequest() / ListThreadsRequest.new instead')
  static ListThreadsRequest create() => ListThreadsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListThreadsRequest._();
  @$core.override
  ListThreadsRequest createEmptyInstance() => ListThreadsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListThreadsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadsRequest>(
          ListThreadsRequest.$_createMessage);
  static ListThreadsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get includeArchived => $_getBF(0);
  @$pb.TagNumber(1)
  set includeArchived($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasIncludeArchived() => $_has(0);
  @$pb.TagNumber(1)
  void clearIncludeArchived() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get limit => $_getIZ(1);
  @$pb.TagNumber(2)
  set limit($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLimit() => $_has(1);
  @$pb.TagNumber(2)
  void clearLimit() => $_clearField(2);

  /// ---- 服务端过滤(交给 codex 的 thread/list,比在手机上过滤整表更省) ----
  @$pb.TagNumber(3)
  $core.String get searchTerm => $_getSZ(2);
  @$pb.TagNumber(3)
  set searchTerm($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSearchTerm() => $_has(2);
  @$pb.TagNumber(3)
  void clearSearchTerm() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get sectionId => $_getSZ(3);
  @$pb.TagNumber(4)
  set sectionId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSectionId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSectionId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get sortKey => $_getSZ(4);
  @$pb.TagNumber(5)
  set sortKey($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSortKey() => $_has(4);
  @$pb.TagNumber(5)
  void clearSortKey() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get sortAscending => $_getBF(5);
  @$pb.TagNumber(6)
  set sortAscending($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSortAscending() => $_has(5);
  @$pb.TagNumber(6)
  void clearSortAscending() => $_clearField(6);
}

class ListThreadsResponse extends $pb.GeneratedMessage {
  factory ListThreadsResponse({
    $core.Iterable<Thread>? threads,
  }) {
    final result = ListThreadsResponse._();
    if (threads != null) result.threads.addAll(threads);
    return result;
  }

  ListThreadsResponse._();

  factory ListThreadsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadsResponse()..mergeFromBuffer(data, registry);
  factory ListThreadsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListThreadsResponse.$_createMessage)
    ..pPM<Thread>(1, _omitFieldNames ? '' : 'threads',
        subBuilder: Thread.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadsResponse copyWith(void Function(ListThreadsResponse) updates) =>
      super.copyWith((message) => updates(message as ListThreadsResponse))
          as ListThreadsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListThreadsResponse() / ListThreadsResponse.new instead')
  static ListThreadsResponse create() => ListThreadsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListThreadsResponse._();
  @$core.override
  ListThreadsResponse createEmptyInstance() => ListThreadsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListThreadsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadsResponse>(
          ListThreadsResponse.$_createMessage);
  static ListThreadsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Thread> get threads => $_getList(0);
}

class GetThreadRequest extends $pb.GeneratedMessage {
  factory GetThreadRequest({
    $core.String? threadId,
  }) {
    final result = GetThreadRequest._();
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  GetThreadRequest._();

  factory GetThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadRequest()..mergeFromBuffer(data, registry);
  factory GetThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadRequest copyWith(void Function(GetThreadRequest) updates) =>
      super.copyWith((message) => updates(message as GetThreadRequest))
          as GetThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetThreadRequest() / GetThreadRequest.new instead')
  static GetThreadRequest create() => GetThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetThreadRequest._();
  @$core.override
  GetThreadRequest createEmptyInstance() => GetThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static GetThreadRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetThreadRequest>(
          GetThreadRequest.$_createMessage);
  static GetThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);
}

class DeleteThreadRequest extends $pb.GeneratedMessage {
  factory DeleteThreadRequest({
    $core.String? threadId,
    $core.bool? localOnly,
  }) {
    final result = DeleteThreadRequest._();
    if (threadId != null) result.threadId = threadId;
    if (localOnly != null) result.localOnly = localOnly;
    return result;
  }

  DeleteThreadRequest._();

  factory DeleteThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteThreadRequest()..mergeFromBuffer(data, registry);
  factory DeleteThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DeleteThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOB(2, _omitFieldNames ? '' : 'localOnly')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteThreadRequest copyWith(void Function(DeleteThreadRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteThreadRequest))
          as DeleteThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use DeleteThreadRequest() / DeleteThreadRequest.new instead')
  static DeleteThreadRequest create() => DeleteThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteThreadRequest._();
  @$core.override
  DeleteThreadRequest createEmptyInstance() => DeleteThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteThreadRequest>(
          DeleteThreadRequest.$_createMessage);
  static DeleteThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get localOnly => $_getBF(1);
  @$pb.TagNumber(2)
  set localOnly($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLocalOnly() => $_has(1);
  @$pb.TagNumber(2)
  void clearLocalOnly() => $_clearField(2);
}

class ArchiveThreadRequest extends $pb.GeneratedMessage {
  factory ArchiveThreadRequest({
    $core.String? threadId,
    $core.bool? archived,
  }) {
    final result = ArchiveThreadRequest._();
    if (threadId != null) result.threadId = threadId;
    if (archived != null) result.archived = archived;
    return result;
  }

  ArchiveThreadRequest._();

  factory ArchiveThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ArchiveThreadRequest()..mergeFromBuffer(data, registry);
  factory ArchiveThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ArchiveThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ArchiveThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ArchiveThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOB(2, _omitFieldNames ? '' : 'archived')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ArchiveThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ArchiveThreadRequest copyWith(void Function(ArchiveThreadRequest) updates) =>
      super.copyWith((message) => updates(message as ArchiveThreadRequest))
          as ArchiveThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ArchiveThreadRequest() / ArchiveThreadRequest.new instead')
  static ArchiveThreadRequest create() => ArchiveThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => ArchiveThreadRequest._();
  @$core.override
  ArchiveThreadRequest createEmptyInstance() => ArchiveThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static ArchiveThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ArchiveThreadRequest>(
          ArchiveThreadRequest.$_createMessage);
  static ArchiveThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get archived => $_getBF(1);
  @$pb.TagNumber(2)
  set archived($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasArchived() => $_has(1);
  @$pb.TagNumber(2)
  void clearArchived() => $_clearField(2);
}

class RenameThreadRequest extends $pb.GeneratedMessage {
  factory RenameThreadRequest({
    $core.String? threadId,
    $core.String? title,
  }) {
    final result = RenameThreadRequest._();
    if (threadId != null) result.threadId = threadId;
    if (title != null) result.title = title;
    return result;
  }

  RenameThreadRequest._();

  factory RenameThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameThreadRequest()..mergeFromBuffer(data, registry);
  factory RenameThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RenameThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RenameThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RenameThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RenameThreadRequest copyWith(void Function(RenameThreadRequest) updates) =>
      super.copyWith((message) => updates(message as RenameThreadRequest))
          as RenameThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use RenameThreadRequest() / RenameThreadRequest.new instead')
  static RenameThreadRequest create() => RenameThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => RenameThreadRequest._();
  @$core.override
  RenameThreadRequest createEmptyInstance() => RenameThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static RenameThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RenameThreadRequest>(
          RenameThreadRequest.$_createMessage);
  static RenameThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);
}

class SetThreadGoalRequest extends $pb.GeneratedMessage {
  factory SetThreadGoalRequest({
    $core.String? threadId,
    $core.String? goal,
  }) {
    final result = SetThreadGoalRequest._();
    if (threadId != null) result.threadId = threadId;
    if (goal != null) result.goal = goal;
    return result;
  }

  SetThreadGoalRequest._();

  factory SetThreadGoalRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetThreadGoalRequest()..mergeFromBuffer(data, registry);
  factory SetThreadGoalRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetThreadGoalRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetThreadGoalRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SetThreadGoalRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'goal')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetThreadGoalRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetThreadGoalRequest copyWith(void Function(SetThreadGoalRequest) updates) =>
      super.copyWith((message) => updates(message as SetThreadGoalRequest))
          as SetThreadGoalRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetThreadGoalRequest() / SetThreadGoalRequest.new instead')
  static SetThreadGoalRequest create() => SetThreadGoalRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetThreadGoalRequest._();
  @$core.override
  SetThreadGoalRequest createEmptyInstance() => SetThreadGoalRequest._();
  @$core.pragma('dart2js:noInline')
  static SetThreadGoalRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetThreadGoalRequest>(
          SetThreadGoalRequest.$_createMessage);
  static SetThreadGoalRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get goal => $_getSZ(1);
  @$pb.TagNumber(2)
  set goal($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasGoal() => $_has(1);
  @$pb.TagNumber(2)
  void clearGoal() => $_clearField(2);
}

class CompactThreadRequest extends $pb.GeneratedMessage {
  factory CompactThreadRequest({
    $core.String? threadId,
  }) {
    final result = CompactThreadRequest._();
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  CompactThreadRequest._();

  factory CompactThreadRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompactThreadRequest()..mergeFromBuffer(data, registry);
  factory CompactThreadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CompactThreadRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompactThreadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CompactThreadRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompactThreadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompactThreadRequest copyWith(void Function(CompactThreadRequest) updates) =>
      super.copyWith((message) => updates(message as CompactThreadRequest))
          as CompactThreadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CompactThreadRequest() / CompactThreadRequest.new instead')
  static CompactThreadRequest create() => CompactThreadRequest._();
  static $pb.GeneratedMessage $_createMessage() => CompactThreadRequest._();
  @$core.override
  CompactThreadRequest createEmptyInstance() => CompactThreadRequest._();
  @$core.pragma('dart2js:noInline')
  static CompactThreadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompactThreadRequest>(
          CompactThreadRequest.$_createMessage);
  static CompactThreadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);
}

enum TurnClientMessage_Msg { input, supplement, approval, cancel, notSet }

class TurnClientMessage extends $pb.GeneratedMessage {
  factory TurnClientMessage({
    TurnInput? input,
    TurnSupplement? supplement,
    ApprovalDecision? approval,
    CancelSignal? cancel,
  }) {
    final result = TurnClientMessage._();
    if (input != null) result.input = input;
    if (supplement != null) result.supplement = supplement;
    if (approval != null) result.approval = approval;
    if (cancel != null) result.cancel = cancel;
    return result;
  }

  TurnClientMessage._();

  factory TurnClientMessage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnClientMessage()..mergeFromBuffer(data, registry);
  factory TurnClientMessage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnClientMessage()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, TurnClientMessage_Msg>
      _TurnClientMessage_MsgByTag = {
    1: TurnClientMessage_Msg.input,
    2: TurnClientMessage_Msg.supplement,
    3: TurnClientMessage_Msg.approval,
    4: TurnClientMessage_Msg.cancel,
    0: TurnClientMessage_Msg.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnClientMessage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnClientMessage.$_createMessage)
    ..oo(0, [1, 2, 3, 4])
    ..aOM<TurnInput>(1, _omitFieldNames ? '' : 'input',
        subBuilder: TurnInput.$_createMessage)
    ..aOM<TurnSupplement>(2, _omitFieldNames ? '' : 'supplement',
        subBuilder: TurnSupplement.$_createMessage)
    ..aOM<ApprovalDecision>(3, _omitFieldNames ? '' : 'approval',
        subBuilder: ApprovalDecision.$_createMessage)
    ..aOM<CancelSignal>(4, _omitFieldNames ? '' : 'cancel',
        subBuilder: CancelSignal.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnClientMessage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnClientMessage copyWith(void Function(TurnClientMessage) updates) =>
      super.copyWith((message) => updates(message as TurnClientMessage))
          as TurnClientMessage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnClientMessage() / TurnClientMessage.new instead')
  static TurnClientMessage create() => TurnClientMessage._();
  static $pb.GeneratedMessage $_createMessage() => TurnClientMessage._();
  @$core.override
  TurnClientMessage createEmptyInstance() => TurnClientMessage._();
  @$core.pragma('dart2js:noInline')
  static TurnClientMessage getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TurnClientMessage>(
          TurnClientMessage.$_createMessage);
  static TurnClientMessage? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  TurnClientMessage_Msg whichMsg() =>
      _TurnClientMessage_MsgByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  void clearMsg() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  TurnInput get input => $_getN(0);
  @$pb.TagNumber(1)
  set input(TurnInput value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInput() => $_has(0);
  @$pb.TagNumber(1)
  void clearInput() => $_clearField(1);
  @$pb.TagNumber(1)
  TurnInput ensureInput() => $_ensure(0);

  @$pb.TagNumber(2)
  TurnSupplement get supplement => $_getN(1);
  @$pb.TagNumber(2)
  set supplement(TurnSupplement value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSupplement() => $_has(1);
  @$pb.TagNumber(2)
  void clearSupplement() => $_clearField(2);
  @$pb.TagNumber(2)
  TurnSupplement ensureSupplement() => $_ensure(1);

  @$pb.TagNumber(3)
  ApprovalDecision get approval => $_getN(2);
  @$pb.TagNumber(3)
  set approval(ApprovalDecision value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasApproval() => $_has(2);
  @$pb.TagNumber(3)
  void clearApproval() => $_clearField(3);
  @$pb.TagNumber(3)
  ApprovalDecision ensureApproval() => $_ensure(2);

  @$pb.TagNumber(4)
  CancelSignal get cancel => $_getN(3);
  @$pb.TagNumber(4)
  set cancel(CancelSignal value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCancel() => $_has(3);
  @$pb.TagNumber(4)
  void clearCancel() => $_clearField(4);
  @$pb.TagNumber(4)
  CancelSignal ensureCancel() => $_ensure(3);
}

class TurnInput extends $pb.GeneratedMessage {
  factory TurnInput({
    $core.String? threadId,
    $core.Iterable<$1.ContentBlock>? blocks,
    $core.bool? background,
    $core.String? modelId,
    $core.String? effort,
  }) {
    final result = TurnInput._();
    if (threadId != null) result.threadId = threadId;
    if (blocks != null) result.blocks.addAll(blocks);
    if (background != null) result.background = background;
    if (modelId != null) result.modelId = modelId;
    if (effort != null) result.effort = effort;
    return result;
  }

  TurnInput._();

  factory TurnInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnInput()..mergeFromBuffer(data, registry);
  factory TurnInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnInput',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..pPM<$1.ContentBlock>(2, _omitFieldNames ? '' : 'blocks',
        subBuilder: $1.ContentBlock.$_createMessage)
    ..aOB(3, _omitFieldNames ? '' : 'background')
    ..aOS(4, _omitFieldNames ? '' : 'modelId')
    ..aOS(5, _omitFieldNames ? '' : 'effort')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnInput copyWith(void Function(TurnInput) updates) =>
      super.copyWith((message) => updates(message as TurnInput)) as TurnInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnInput() / TurnInput.new instead')
  static TurnInput create() => TurnInput._();
  static $pb.GeneratedMessage $_createMessage() => TurnInput._();
  @$core.override
  TurnInput createEmptyInstance() => TurnInput._();
  @$core.pragma('dart2js:noInline')
  static TurnInput getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TurnInput>(TurnInput.$_createMessage);
  static TurnInput? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$1.ContentBlock> get blocks => $_getList(1);

  @$pb.TagNumber(3)
  $core.bool get background => $_getBF(2);
  @$pb.TagNumber(3)
  set background($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBackground() => $_has(2);
  @$pb.TagNumber(3)
  void clearBackground() => $_clearField(3);

  /// 每轮覆盖(可选):切换模型/思考强度,只影响本轮及之后
  @$pb.TagNumber(4)
  $core.String get modelId => $_getSZ(3);
  @$pb.TagNumber(4)
  set modelId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasModelId() => $_has(3);
  @$pb.TagNumber(4)
  void clearModelId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get effort => $_getSZ(4);
  @$pb.TagNumber(5)
  set effort($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasEffort() => $_has(4);
  @$pb.TagNumber(5)
  void clearEffort() => $_clearField(5);
}

class TurnSupplement extends $pb.GeneratedMessage {
  factory TurnSupplement({
    $core.Iterable<$1.ContentBlock>? blocks,
  }) {
    final result = TurnSupplement._();
    if (blocks != null) result.blocks.addAll(blocks);
    return result;
  }

  TurnSupplement._();

  factory TurnSupplement.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnSupplement()..mergeFromBuffer(data, registry);
  factory TurnSupplement.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnSupplement()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnSupplement',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnSupplement.$_createMessage)
    ..pPM<$1.ContentBlock>(1, _omitFieldNames ? '' : 'blocks',
        subBuilder: $1.ContentBlock.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnSupplement clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnSupplement copyWith(void Function(TurnSupplement) updates) =>
      super.copyWith((message) => updates(message as TurnSupplement))
          as TurnSupplement;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnSupplement() / TurnSupplement.new instead')
  static TurnSupplement create() => TurnSupplement._();
  static $pb.GeneratedMessage $_createMessage() => TurnSupplement._();
  @$core.override
  TurnSupplement createEmptyInstance() => TurnSupplement._();
  @$core.pragma('dart2js:noInline')
  static TurnSupplement getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TurnSupplement>(
          TurnSupplement.$_createMessage);
  static TurnSupplement? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$1.ContentBlock> get blocks => $_getList(0);
}

class ApprovalDecision extends $pb.GeneratedMessage {
  factory ApprovalDecision({
    $core.String? requestId,
    $core.bool? approved,
    $core.String? note,
    $core.String? option,
  }) {
    final result = ApprovalDecision._();
    if (requestId != null) result.requestId = requestId;
    if (approved != null) result.approved = approved;
    if (note != null) result.note = note;
    if (option != null) result.option = option;
    return result;
  }

  ApprovalDecision._();

  factory ApprovalDecision.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalDecision()..mergeFromBuffer(data, registry);
  factory ApprovalDecision.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalDecision()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApprovalDecision',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ApprovalDecision.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOB(2, _omitFieldNames ? '' : 'approved')
    ..aOS(3, _omitFieldNames ? '' : 'note')
    ..aOS(4, _omitFieldNames ? '' : 'option')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalDecision clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalDecision copyWith(void Function(ApprovalDecision) updates) =>
      super.copyWith((message) => updates(message as ApprovalDecision))
          as ApprovalDecision;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ApprovalDecision() / ApprovalDecision.new instead')
  static ApprovalDecision create() => ApprovalDecision._();
  static $pb.GeneratedMessage $_createMessage() => ApprovalDecision._();
  @$core.override
  ApprovalDecision createEmptyInstance() => ApprovalDecision._();
  @$core.pragma('dart2js:noInline')
  static ApprovalDecision getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ApprovalDecision>(
          ApprovalDecision.$_createMessage);
  static ApprovalDecision? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get approved => $_getBF(1);
  @$pb.TagNumber(2)
  set approved($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApproved() => $_has(1);
  @$pb.TagNumber(2)
  void clearApproved() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get note => $_getSZ(2);
  @$pb.TagNumber(3)
  set note($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNote() => $_has(2);
  @$pb.TagNumber(3)
  void clearNote() => $_clearField(3);

  /// ---- C1 只增不改 ----
  /// 新端三按钮时填选项 id(与卡 options[].id 对应,如 acceptForSession /
  /// cancel);为空 = 旧端 approved/note 语义,daemon 行为一字不变。
  /// daemon 会把它与 note 包装成 {"option":..,"note":..} 信封并入 note 传给
  /// provider,provider 侧解析(见 agent.parseApprovalChoice)。
  @$pb.TagNumber(4)
  $core.String get option => $_getSZ(3);
  @$pb.TagNumber(4)
  set option($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOption() => $_has(3);
  @$pb.TagNumber(4)
  void clearOption() => $_clearField(4);
}

class CancelSignal extends $pb.GeneratedMessage {
  factory CancelSignal({
    $core.String? reason,
  }) {
    final result = CancelSignal._();
    if (reason != null) result.reason = reason;
    return result;
  }

  CancelSignal._();

  factory CancelSignal.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelSignal()..mergeFromBuffer(data, registry);
  factory CancelSignal.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelSignal()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelSignal',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CancelSignal.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSignal clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSignal copyWith(void Function(CancelSignal) updates) =>
      super.copyWith((message) => updates(message as CancelSignal))
          as CancelSignal;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CancelSignal() / CancelSignal.new instead')
  static CancelSignal create() => CancelSignal._();
  static $pb.GeneratedMessage $_createMessage() => CancelSignal._();
  @$core.override
  CancelSignal createEmptyInstance() => CancelSignal._();
  @$core.pragma('dart2js:noInline')
  static CancelSignal getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CancelSignal>(
          CancelSignal.$_createMessage);
  static CancelSignal? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reason => $_getSZ(0);
  @$pb.TagNumber(1)
  set reason($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);
}

enum TurnServerMessage_Msg {
  started,
  textDelta,
  thinkingDelta,
  toolCall,
  approvalRequest,
  usage,
  completed,
  failed,
  codexEvent,
  approvalResolved,
  steered,
  queued,
  queueFlushed,
  resyncRequired,
  notSet
}

class TurnServerMessage extends $pb.GeneratedMessage {
  factory TurnServerMessage({
    TurnStarted? started,
    TextDelta? textDelta,
    ThinkingDelta? thinkingDelta,
    ToolCallUpdate? toolCall,
    ApprovalRequest? approvalRequest,
    UsageUpdate? usage,
    TurnCompleted? completed,
    TurnFailed? failed,
    CodexEvent? codexEvent,
    ApprovalResolved? approvalResolved,
    Steered? steered,
    Queued? queued,
    QueueFlushed? queueFlushed,
    $fixnum.Int64? sequence,
    ResyncRequired? resyncRequired,
  }) {
    final result = TurnServerMessage._();
    if (started != null) result.started = started;
    if (textDelta != null) result.textDelta = textDelta;
    if (thinkingDelta != null) result.thinkingDelta = thinkingDelta;
    if (toolCall != null) result.toolCall = toolCall;
    if (approvalRequest != null) result.approvalRequest = approvalRequest;
    if (usage != null) result.usage = usage;
    if (completed != null) result.completed = completed;
    if (failed != null) result.failed = failed;
    if (codexEvent != null) result.codexEvent = codexEvent;
    if (approvalResolved != null) result.approvalResolved = approvalResolved;
    if (steered != null) result.steered = steered;
    if (queued != null) result.queued = queued;
    if (queueFlushed != null) result.queueFlushed = queueFlushed;
    if (sequence != null) result.sequence = sequence;
    if (resyncRequired != null) result.resyncRequired = resyncRequired;
    return result;
  }

  TurnServerMessage._();

  factory TurnServerMessage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnServerMessage()..mergeFromBuffer(data, registry);
  factory TurnServerMessage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnServerMessage()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, TurnServerMessage_Msg>
      _TurnServerMessage_MsgByTag = {
    1: TurnServerMessage_Msg.started,
    2: TurnServerMessage_Msg.textDelta,
    3: TurnServerMessage_Msg.thinkingDelta,
    4: TurnServerMessage_Msg.toolCall,
    5: TurnServerMessage_Msg.approvalRequest,
    6: TurnServerMessage_Msg.usage,
    7: TurnServerMessage_Msg.completed,
    8: TurnServerMessage_Msg.failed,
    9: TurnServerMessage_Msg.codexEvent,
    10: TurnServerMessage_Msg.approvalResolved,
    11: TurnServerMessage_Msg.steered,
    12: TurnServerMessage_Msg.queued,
    13: TurnServerMessage_Msg.queueFlushed,
    15: TurnServerMessage_Msg.resyncRequired,
    0: TurnServerMessage_Msg.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnServerMessage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnServerMessage.$_createMessage)
    ..oo(0, [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15])
    ..aOM<TurnStarted>(1, _omitFieldNames ? '' : 'started',
        subBuilder: TurnStarted.$_createMessage)
    ..aOM<TextDelta>(2, _omitFieldNames ? '' : 'textDelta',
        subBuilder: TextDelta.$_createMessage)
    ..aOM<ThinkingDelta>(3, _omitFieldNames ? '' : 'thinkingDelta',
        subBuilder: ThinkingDelta.$_createMessage)
    ..aOM<ToolCallUpdate>(4, _omitFieldNames ? '' : 'toolCall',
        subBuilder: ToolCallUpdate.$_createMessage)
    ..aOM<ApprovalRequest>(5, _omitFieldNames ? '' : 'approvalRequest',
        subBuilder: ApprovalRequest.$_createMessage)
    ..aOM<UsageUpdate>(6, _omitFieldNames ? '' : 'usage',
        subBuilder: UsageUpdate.$_createMessage)
    ..aOM<TurnCompleted>(7, _omitFieldNames ? '' : 'completed',
        subBuilder: TurnCompleted.$_createMessage)
    ..aOM<TurnFailed>(8, _omitFieldNames ? '' : 'failed',
        subBuilder: TurnFailed.$_createMessage)
    ..aOM<CodexEvent>(9, _omitFieldNames ? '' : 'codexEvent',
        subBuilder: CodexEvent.$_createMessage)
    ..aOM<ApprovalResolved>(10, _omitFieldNames ? '' : 'approvalResolved',
        subBuilder: ApprovalResolved.$_createMessage)
    ..aOM<Steered>(11, _omitFieldNames ? '' : 'steered',
        subBuilder: Steered.$_createMessage)
    ..aOM<Queued>(12, _omitFieldNames ? '' : 'queued',
        subBuilder: Queued.$_createMessage)
    ..aOM<QueueFlushed>(13, _omitFieldNames ? '' : 'queueFlushed',
        subBuilder: QueueFlushed.$_createMessage)
    ..aInt64(14, _omitFieldNames ? '' : 'sequence')
    ..aOM<ResyncRequired>(15, _omitFieldNames ? '' : 'resyncRequired',
        subBuilder: ResyncRequired.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnServerMessage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnServerMessage copyWith(void Function(TurnServerMessage) updates) =>
      super.copyWith((message) => updates(message as TurnServerMessage))
          as TurnServerMessage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnServerMessage() / TurnServerMessage.new instead')
  static TurnServerMessage create() => TurnServerMessage._();
  static $pb.GeneratedMessage $_createMessage() => TurnServerMessage._();
  @$core.override
  TurnServerMessage createEmptyInstance() => TurnServerMessage._();
  @$core.pragma('dart2js:noInline')
  static TurnServerMessage getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TurnServerMessage>(
          TurnServerMessage.$_createMessage);
  static TurnServerMessage? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  @$pb.TagNumber(8)
  @$pb.TagNumber(9)
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(15)
  TurnServerMessage_Msg whichMsg() =>
      _TurnServerMessage_MsgByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  @$pb.TagNumber(8)
  @$pb.TagNumber(9)
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(15)
  void clearMsg() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  TurnStarted get started => $_getN(0);
  @$pb.TagNumber(1)
  set started(TurnStarted value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStarted() => $_has(0);
  @$pb.TagNumber(1)
  void clearStarted() => $_clearField(1);
  @$pb.TagNumber(1)
  TurnStarted ensureStarted() => $_ensure(0);

  @$pb.TagNumber(2)
  TextDelta get textDelta => $_getN(1);
  @$pb.TagNumber(2)
  set textDelta(TextDelta value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasTextDelta() => $_has(1);
  @$pb.TagNumber(2)
  void clearTextDelta() => $_clearField(2);
  @$pb.TagNumber(2)
  TextDelta ensureTextDelta() => $_ensure(1);

  @$pb.TagNumber(3)
  ThinkingDelta get thinkingDelta => $_getN(2);
  @$pb.TagNumber(3)
  set thinkingDelta(ThinkingDelta value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasThinkingDelta() => $_has(2);
  @$pb.TagNumber(3)
  void clearThinkingDelta() => $_clearField(3);
  @$pb.TagNumber(3)
  ThinkingDelta ensureThinkingDelta() => $_ensure(2);

  @$pb.TagNumber(4)
  ToolCallUpdate get toolCall => $_getN(3);
  @$pb.TagNumber(4)
  set toolCall(ToolCallUpdate value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasToolCall() => $_has(3);
  @$pb.TagNumber(4)
  void clearToolCall() => $_clearField(4);
  @$pb.TagNumber(4)
  ToolCallUpdate ensureToolCall() => $_ensure(3);

  @$pb.TagNumber(5)
  ApprovalRequest get approvalRequest => $_getN(4);
  @$pb.TagNumber(5)
  set approvalRequest(ApprovalRequest value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasApprovalRequest() => $_has(4);
  @$pb.TagNumber(5)
  void clearApprovalRequest() => $_clearField(5);
  @$pb.TagNumber(5)
  ApprovalRequest ensureApprovalRequest() => $_ensure(4);

  @$pb.TagNumber(6)
  UsageUpdate get usage => $_getN(5);
  @$pb.TagNumber(6)
  set usage(UsageUpdate value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasUsage() => $_has(5);
  @$pb.TagNumber(6)
  void clearUsage() => $_clearField(6);
  @$pb.TagNumber(6)
  UsageUpdate ensureUsage() => $_ensure(5);

  @$pb.TagNumber(7)
  TurnCompleted get completed => $_getN(6);
  @$pb.TagNumber(7)
  set completed(TurnCompleted value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasCompleted() => $_has(6);
  @$pb.TagNumber(7)
  void clearCompleted() => $_clearField(7);
  @$pb.TagNumber(7)
  TurnCompleted ensureCompleted() => $_ensure(6);

  @$pb.TagNumber(8)
  TurnFailed get failed => $_getN(7);
  @$pb.TagNumber(8)
  set failed(TurnFailed value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasFailed() => $_has(7);
  @$pb.TagNumber(8)
  void clearFailed() => $_clearField(8);
  @$pb.TagNumber(8)
  TurnFailed ensureFailed() => $_ensure(7);

  @$pb.TagNumber(9)
  CodexEvent get codexEvent => $_getN(8);
  @$pb.TagNumber(9)
  set codexEvent(CodexEvent value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCodexEvent() => $_has(8);
  @$pb.TagNumber(9)
  void clearCodexEvent() => $_clearField(9);
  @$pb.TagNumber(9)
  CodexEvent ensureCodexEvent() => $_ensure(8);

  /// 审批请求已被桌面端/codex 侧处理(serverRequest/resolved):
  /// 手机端必须立即关闭正在展示的审批卡,否则会残留"幽灵审批卡"
  /// (桌面端已放行/拒绝,手机上却还挂着一张点不动也没意义的卡)。
  @$pb.TagNumber(10)
  ApprovalResolved get approvalResolved => $_getN(9);
  @$pb.TagNumber(10)
  set approvalResolved(ApprovalResolved value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasApprovalResolved() => $_has(9);
  @$pb.TagNumber(10)
  void clearApprovalResolved() => $_clearField(10);
  @$pb.TagNumber(10)
  ApprovalResolved ensureApprovalResolved() => $_ensure(9);

  /// ---- P1-4 运行中继续发消息(只增不改:11~13 为新增帧)----
  /// 该输入已通过 codex 的 turn/steer 插入正在运行的轮次(不新开轮次)。
  @$pb.TagNumber(11)
  Steered get steered => $_getN(10);
  @$pb.TagNumber(11)
  set steered(Steered value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasSteered() => $_has(10);
  @$pb.TagNumber(11)
  void clearSteered() => $_clearField(11);
  @$pb.TagNumber(11)
  Steered ensureSteered() => $_ensure(10);

  /// 该输入已进入 daemon 内存队列,等当前轮次终局后自动执行。
  @$pb.TagNumber(12)
  Queued get queued => $_getN(11);
  @$pb.TagNumber(12)
  set queued(Queued value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasQueued() => $_has(11);
  @$pb.TagNumber(12)
  void clearQueued() => $_clearField(12);
  @$pb.TagNumber(12)
  Queued ensureQueued() => $_ensure(11);

  /// 队列开始执行:紧跟着会有该轮的 Started 帧与常规增量。
  @$pb.TagNumber(13)
  QueueFlushed get queueFlushed => $_getN(12);
  @$pb.TagNumber(13)
  set queueFlushed(QueueFlushed value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasQueueFlushed() => $_has(12);
  @$pb.TagNumber(13)
  void clearQueueFlushed() => $_clearField(13);
  @$pb.TagNumber(13)
  QueueFlushed ensureQueueFlushed() => $_ensure(12);

  /// ---- P1-5 事件断点续传(只增不改:14~15 为新增)----
  /// 全局单调事件序号(daemon events.jsonl 的 seq):daemon 发出的每一个真实
  /// 事件帧都带序号,手机记录收到的最大值;断线重连后用
  /// GetThreadEventsSince(thread_id, after_sequence=lastSequence) 增量补帧。
  /// 0 = 无序号(旧 daemon / 该帧未入事件库),手机端按无续传能力处理。
  @$pb.TagNumber(14)
  $fixnum.Int64 get sequence => $_getI64(13);
  @$pb.TagNumber(14)
  set sequence($fixnum.Int64 value) => $_setInt64(13, value);
  @$pb.TagNumber(14)
  $core.bool hasSequence() => $_has(13);
  @$pb.TagNumber(14)
  void clearSequence() => $_clearField(14);

  /// after_sequence 已超出保留窗口(轮转丢弃)或事件库不可用:立即走
  /// GetThreadHistory 全量回填,不要再增量拼接。本帧是流里的唯一一帧。
  @$pb.TagNumber(15)
  ResyncRequired get resyncRequired => $_getN(14);
  @$pb.TagNumber(15)
  set resyncRequired(ResyncRequired value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasResyncRequired() => $_has(14);
  @$pb.TagNumber(15)
  void clearResyncRequired() => $_clearField(15);
  @$pb.TagNumber(15)
  ResyncRequired ensureResyncRequired() => $_ensure(14);
}

class GetThreadEventsSinceRequest extends $pb.GeneratedMessage {
  factory GetThreadEventsSinceRequest({
    $core.String? threadId,
    $fixnum.Int64? afterSequence,
  }) {
    final result = GetThreadEventsSinceRequest._();
    if (threadId != null) result.threadId = threadId;
    if (afterSequence != null) result.afterSequence = afterSequence;
    return result;
  }

  GetThreadEventsSinceRequest._();

  factory GetThreadEventsSinceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadEventsSinceRequest()..mergeFromBuffer(data, registry);
  factory GetThreadEventsSinceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetThreadEventsSinceRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetThreadEventsSinceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetThreadEventsSinceRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aInt64(2, _omitFieldNames ? '' : 'afterSequence')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadEventsSinceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetThreadEventsSinceRequest copyWith(
          void Function(GetThreadEventsSinceRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetThreadEventsSinceRequest))
          as GetThreadEventsSinceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetThreadEventsSinceRequest() / GetThreadEventsSinceRequest.new instead')
  static GetThreadEventsSinceRequest create() =>
      GetThreadEventsSinceRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      GetThreadEventsSinceRequest._();
  @$core.override
  GetThreadEventsSinceRequest createEmptyInstance() =>
      GetThreadEventsSinceRequest._();
  @$core.pragma('dart2js:noInline')
  static GetThreadEventsSinceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetThreadEventsSinceRequest>(
          GetThreadEventsSinceRequest.$_createMessage);
  static GetThreadEventsSinceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get afterSequence => $_getI64(1);
  @$pb.TagNumber(2)
  set afterSequence($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAfterSequence() => $_has(1);
  @$pb.TagNumber(2)
  void clearAfterSequence() => $_clearField(2);
}

/// ResyncRequired:请求的区间已无法可靠回放(典型原因:events.jsonl 轮转把
/// 更早的事件删掉了)。收到本帧后手机端 lastSequence 作废,退回全量回填。
class ResyncRequired extends $pb.GeneratedMessage {
  factory ResyncRequired({
    $core.String? reason,
  }) {
    final result = ResyncRequired._();
    if (reason != null) result.reason = reason;
    return result;
  }

  ResyncRequired._();

  factory ResyncRequired.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResyncRequired()..mergeFromBuffer(data, registry);
  factory ResyncRequired.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResyncRequired()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResyncRequired',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ResyncRequired.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResyncRequired clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResyncRequired copyWith(void Function(ResyncRequired) updates) =>
      super.copyWith((message) => updates(message as ResyncRequired))
          as ResyncRequired;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ResyncRequired() / ResyncRequired.new instead')
  static ResyncRequired create() => ResyncRequired._();
  static $pb.GeneratedMessage $_createMessage() => ResyncRequired._();
  @$core.override
  ResyncRequired createEmptyInstance() => ResyncRequired._();
  @$core.pragma('dart2js:noInline')
  static ResyncRequired getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ResyncRequired>(
          ResyncRequired.$_createMessage);
  static ResyncRequired? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reason => $_getSZ(0);
  @$pb.TagNumber(1)
  set reason($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);
}

/// Steered 确认:这条输入已插入正在运行的轮次(codex turn/steer 成功)。
class Steered extends $pb.GeneratedMessage {
  factory Steered({
    $core.String? turnId,
    $core.String? codexTurnId,
  }) {
    final result = Steered._();
    if (turnId != null) result.turnId = turnId;
    if (codexTurnId != null) result.codexTurnId = codexTurnId;
    return result;
  }

  Steered._();

  factory Steered.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Steered()..mergeFromBuffer(data, registry);
  factory Steered.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Steered()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Steered',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: Steered.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'turnId')
    ..aOS(2, _omitFieldNames ? '' : 'codexTurnId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Steered clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Steered copyWith(void Function(Steered) updates) =>
      super.copyWith((message) => updates(message as Steered)) as Steered;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Steered() / Steered.new instead')
  static Steered create() => Steered._();
  static $pb.GeneratedMessage $_createMessage() => Steered._();
  @$core.override
  Steered createEmptyInstance() => Steered._();
  @$core.pragma('dart2js:noInline')
  static Steered getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Steered>(Steered.$_createMessage);
  static Steered? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get turnId => $_getSZ(0);
  @$pb.TagNumber(1)
  set turnId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTurnId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTurnId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get codexTurnId => $_getSZ(1);
  @$pb.TagNumber(2)
  set codexTurnId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCodexTurnId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCodexTurnId() => $_clearField(2);
}

/// Queued 确认:这条输入已排队。position 从 1 起;
/// position == 0 表示队列已满被拒收(上限 10 条),手机端应把该消息
/// 标记为"未送达"而不是"排队中"。
class Queued extends $pb.GeneratedMessage {
  factory Queued({
    $core.int? position,
    $core.int? depth,
  }) {
    final result = Queued._();
    if (position != null) result.position = position;
    if (depth != null) result.depth = depth;
    return result;
  }

  Queued._();

  factory Queued.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Queued()..mergeFromBuffer(data, registry);
  factory Queued.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Queued()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Queued',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: Queued.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'position')
    ..aI(2, _omitFieldNames ? '' : 'depth')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Queued clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Queued copyWith(void Function(Queued) updates) =>
      super.copyWith((message) => updates(message as Queued)) as Queued;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Queued() / Queued.new instead')
  static Queued create() => Queued._();
  static $pb.GeneratedMessage $_createMessage() => Queued._();
  @$core.override
  Queued createEmptyInstance() => Queued._();
  @$core.pragma('dart2js:noInline')
  static Queued getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Queued>(Queued.$_createMessage);
  static Queued? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get position => $_getIZ(0);
  @$pb.TagNumber(1)
  set position($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPosition() => $_has(0);
  @$pb.TagNumber(1)
  void clearPosition() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get depth => $_getIZ(1);
  @$pb.TagNumber(2)
  set depth($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDepth() => $_has(1);
  @$pb.TagNumber(2)
  void clearDepth() => $_clearField(2);
}

/// QueueFlushed:排队的输入开始执行。turn_id 是即将开始的那一轮的 id;
/// remaining 是出队后队列里还剩几条。
class QueueFlushed extends $pb.GeneratedMessage {
  factory QueueFlushed({
    $core.String? turnId,
    $core.int? remaining,
  }) {
    final result = QueueFlushed._();
    if (turnId != null) result.turnId = turnId;
    if (remaining != null) result.remaining = remaining;
    return result;
  }

  QueueFlushed._();

  factory QueueFlushed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QueueFlushed()..mergeFromBuffer(data, registry);
  factory QueueFlushed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      QueueFlushed()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'QueueFlushed',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: QueueFlushed.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'turnId')
    ..aI(2, _omitFieldNames ? '' : 'remaining')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QueueFlushed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  QueueFlushed copyWith(void Function(QueueFlushed) updates) =>
      super.copyWith((message) => updates(message as QueueFlushed))
          as QueueFlushed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use QueueFlushed() / QueueFlushed.new instead')
  static QueueFlushed create() => QueueFlushed._();
  static $pb.GeneratedMessage $_createMessage() => QueueFlushed._();
  @$core.override
  QueueFlushed createEmptyInstance() => QueueFlushed._();
  @$core.pragma('dart2js:noInline')
  static QueueFlushed getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<QueueFlushed>(
          QueueFlushed.$_createMessage);
  static QueueFlushed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get turnId => $_getSZ(0);
  @$pb.TagNumber(1)
  set turnId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTurnId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTurnId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get remaining => $_getIZ(1);
  @$pb.TagNumber(2)
  set remaining($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRemaining() => $_has(1);
  @$pb.TagNumber(2)
  void clearRemaining() => $_clearField(2);
}

class ApprovalResolved extends $pb.GeneratedMessage {
  factory ApprovalResolved({
    $core.String? requestId,
    $core.String? note,
  }) {
    final result = ApprovalResolved._();
    if (requestId != null) result.requestId = requestId;
    if (note != null) result.note = note;
    return result;
  }

  ApprovalResolved._();

  factory ApprovalResolved.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalResolved()..mergeFromBuffer(data, registry);
  factory ApprovalResolved.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalResolved()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApprovalResolved',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ApprovalResolved.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOS(2, _omitFieldNames ? '' : 'note')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalResolved clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalResolved copyWith(void Function(ApprovalResolved) updates) =>
      super.copyWith((message) => updates(message as ApprovalResolved))
          as ApprovalResolved;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ApprovalResolved() / ApprovalResolved.new instead')
  static ApprovalResolved create() => ApprovalResolved._();
  static $pb.GeneratedMessage $_createMessage() => ApprovalResolved._();
  @$core.override
  ApprovalResolved createEmptyInstance() => ApprovalResolved._();
  @$core.pragma('dart2js:noInline')
  static ApprovalResolved getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ApprovalResolved>(
          ApprovalResolved.$_createMessage);
  static ApprovalResolved? _defaultInstance;

  /// 对应 ApprovalRequest.request_id;为空 = "当前挂着的审批卡"(codex 的
  /// 审批请求在同一会话内是串行的,单槽位足够)。
  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  /// 人话说明,展示在时间线里,让用户明白卡为什么消失。
  @$pb.TagNumber(2)
  $core.String get note => $_getSZ(1);
  @$pb.TagNumber(2)
  set note($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNote() => $_has(1);
  @$pb.TagNumber(2)
  void clearNote() => $_clearField(2);
}

/// Codex app-server 的原生状态和错误事件。
/// 事件先实时发送给手机，再由最终 TurnFailed/TurnCompleted 收尾。
class CodexEvent extends $pb.GeneratedMessage {
  factory CodexEvent({
    CodexEvent_Kind? kind,
    $core.String? code,
    $core.String? title,
    $core.String? message,
    $core.String? url,
    $core.int? httpStatus,
    $core.bool? willRetry,
    $core.int? attempt,
    $fixnum.Int64? retryAfterMs,
    $core.String? threadId,
    $core.String? turnId,
    $core.String? riskLevel,
    $core.String? reviewStatus,
    $core.String? reviewId,
    $core.String? targetItemId,
    $core.String? rationale,
    $core.String? diff,
    $core.String? rawJson,
    $core.Iterable<$core.String>? planSteps,
  }) {
    final result = CodexEvent._();
    if (kind != null) result.kind = kind;
    if (code != null) result.code = code;
    if (title != null) result.title = title;
    if (message != null) result.message = message;
    if (url != null) result.url = url;
    if (httpStatus != null) result.httpStatus = httpStatus;
    if (willRetry != null) result.willRetry = willRetry;
    if (attempt != null) result.attempt = attempt;
    if (retryAfterMs != null) result.retryAfterMs = retryAfterMs;
    if (threadId != null) result.threadId = threadId;
    if (turnId != null) result.turnId = turnId;
    if (riskLevel != null) result.riskLevel = riskLevel;
    if (reviewStatus != null) result.reviewStatus = reviewStatus;
    if (reviewId != null) result.reviewId = reviewId;
    if (targetItemId != null) result.targetItemId = targetItemId;
    if (rationale != null) result.rationale = rationale;
    if (diff != null) result.diff = diff;
    if (rawJson != null) result.rawJson = rawJson;
    if (planSteps != null) result.planSteps.addAll(planSteps);
    return result;
  }

  CodexEvent._();

  factory CodexEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexEvent()..mergeFromBuffer(data, registry);
  factory CodexEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexEvent()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexEvent.$_createMessage)
    ..aE<CodexEvent_Kind>(1, _omitFieldNames ? '' : 'kind',
        enumValues: CodexEvent_Kind.values)
    ..aOS(2, _omitFieldNames ? '' : 'code')
    ..aOS(3, _omitFieldNames ? '' : 'title')
    ..aOS(4, _omitFieldNames ? '' : 'message')
    ..aOS(5, _omitFieldNames ? '' : 'url')
    ..aI(6, _omitFieldNames ? '' : 'httpStatus')
    ..aOB(7, _omitFieldNames ? '' : 'willRetry')
    ..aI(8, _omitFieldNames ? '' : 'attempt')
    ..aInt64(9, _omitFieldNames ? '' : 'retryAfterMs')
    ..aOS(10, _omitFieldNames ? '' : 'threadId')
    ..aOS(11, _omitFieldNames ? '' : 'turnId')
    ..aOS(12, _omitFieldNames ? '' : 'riskLevel')
    ..aOS(13, _omitFieldNames ? '' : 'reviewStatus')
    ..aOS(14, _omitFieldNames ? '' : 'reviewId')
    ..aOS(15, _omitFieldNames ? '' : 'targetItemId')
    ..aOS(16, _omitFieldNames ? '' : 'rationale')
    ..aOS(17, _omitFieldNames ? '' : 'diff')
    ..aOS(18, _omitFieldNames ? '' : 'rawJson')
    ..pPS(19, _omitFieldNames ? '' : 'planSteps')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexEvent copyWith(void Function(CodexEvent) updates) =>
      super.copyWith((message) => updates(message as CodexEvent)) as CodexEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexEvent() / CodexEvent.new instead')
  static CodexEvent create() => CodexEvent._();
  static $pb.GeneratedMessage $_createMessage() => CodexEvent._();
  @$core.override
  CodexEvent createEmptyInstance() => CodexEvent._();
  @$core.pragma('dart2js:noInline')
  static CodexEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CodexEvent>(CodexEvent.$_createMessage);
  static CodexEvent? _defaultInstance;

  @$pb.TagNumber(1)
  CodexEvent_Kind get kind => $_getN(0);
  @$pb.TagNumber(1)
  set kind(CodexEvent_Kind value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get code => $_getSZ(1);
  @$pb.TagNumber(2)
  set code($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearCode() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get title => $_getSZ(2);
  @$pb.TagNumber(3)
  set title($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get message => $_getSZ(3);
  @$pb.TagNumber(4)
  set message($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMessage() => $_has(3);
  @$pb.TagNumber(4)
  void clearMessage() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get url => $_getSZ(4);
  @$pb.TagNumber(5)
  set url($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasUrl() => $_has(4);
  @$pb.TagNumber(5)
  void clearUrl() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.int get httpStatus => $_getIZ(5);
  @$pb.TagNumber(6)
  set httpStatus($core.int value) => $_setSignedInt32(5, value);
  @$pb.TagNumber(6)
  $core.bool hasHttpStatus() => $_has(5);
  @$pb.TagNumber(6)
  void clearHttpStatus() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get willRetry => $_getBF(6);
  @$pb.TagNumber(7)
  set willRetry($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasWillRetry() => $_has(6);
  @$pb.TagNumber(7)
  void clearWillRetry() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get attempt => $_getIZ(7);
  @$pb.TagNumber(8)
  set attempt($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAttempt() => $_has(7);
  @$pb.TagNumber(8)
  void clearAttempt() => $_clearField(8);

  @$pb.TagNumber(9)
  $fixnum.Int64 get retryAfterMs => $_getI64(8);
  @$pb.TagNumber(9)
  set retryAfterMs($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRetryAfterMs() => $_has(8);
  @$pb.TagNumber(9)
  void clearRetryAfterMs() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get threadId => $_getSZ(9);
  @$pb.TagNumber(10)
  set threadId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasThreadId() => $_has(9);
  @$pb.TagNumber(10)
  void clearThreadId() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get turnId => $_getSZ(10);
  @$pb.TagNumber(11)
  set turnId($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasTurnId() => $_has(10);
  @$pb.TagNumber(11)
  void clearTurnId() => $_clearField(11);

  /// ---- 结构化附带信息(字段号从 12 起)----
  @$pb.TagNumber(12)
  $core.String get riskLevel => $_getSZ(11);
  @$pb.TagNumber(12)
  set riskLevel($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasRiskLevel() => $_has(11);
  @$pb.TagNumber(12)
  void clearRiskLevel() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get reviewStatus => $_getSZ(12);
  @$pb.TagNumber(13)
  set reviewStatus($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasReviewStatus() => $_has(12);
  @$pb.TagNumber(13)
  void clearReviewStatus() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get reviewId => $_getSZ(13);
  @$pb.TagNumber(14)
  set reviewId($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasReviewId() => $_has(13);
  @$pb.TagNumber(14)
  void clearReviewId() => $_clearField(14);

  @$pb.TagNumber(15)
  $core.String get targetItemId => $_getSZ(14);
  @$pb.TagNumber(15)
  set targetItemId($core.String value) => $_setString(14, value);
  @$pb.TagNumber(15)
  $core.bool hasTargetItemId() => $_has(14);
  @$pb.TagNumber(15)
  void clearTargetItemId() => $_clearField(15);

  @$pb.TagNumber(16)
  $core.String get rationale => $_getSZ(15);
  @$pb.TagNumber(16)
  set rationale($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasRationale() => $_has(15);
  @$pb.TagNumber(16)
  void clearRationale() => $_clearField(16);

  @$pb.TagNumber(17)
  $core.String get diff => $_getSZ(16);
  @$pb.TagNumber(17)
  set diff($core.String value) => $_setString(16, value);
  @$pb.TagNumber(17)
  $core.bool hasDiff() => $_has(16);
  @$pb.TagNumber(17)
  void clearDiff() => $_clearField(17);

  @$pb.TagNumber(18)
  $core.String get rawJson => $_getSZ(17);
  @$pb.TagNumber(18)
  set rawJson($core.String value) => $_setString(17, value);
  @$pb.TagNumber(18)
  $core.bool hasRawJson() => $_has(17);
  @$pb.TagNumber(18)
  void clearRawJson() => $_clearField(18);

  /// turn/plan/updated 的执行计划步骤,每项格式 "status|text"
  /// (status: pending/in_progress/completed)。UI 渲染为步骤卡。
  @$pb.TagNumber(19)
  $pb.PbList<$core.String> get planSteps => $_getList(18);
}

class TurnStarted extends $pb.GeneratedMessage {
  factory TurnStarted({
    $core.String? turnId,
    $core.String? modelId,
    $core.String? effort,
    $core.bool? configurationConfirmed,
    $core.String? source,
  }) {
    final result = TurnStarted._();
    if (turnId != null) result.turnId = turnId;
    if (modelId != null) result.modelId = modelId;
    if (effort != null) result.effort = effort;
    if (configurationConfirmed != null)
      result.configurationConfirmed = configurationConfirmed;
    if (source != null) result.source = source;
    return result;
  }

  TurnStarted._();

  factory TurnStarted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnStarted()..mergeFromBuffer(data, registry);
  factory TurnStarted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnStarted()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnStarted',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnStarted.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'turnId')
    ..aOS(2, _omitFieldNames ? '' : 'modelId')
    ..aOS(3, _omitFieldNames ? '' : 'effort')
    ..aOB(4, _omitFieldNames ? '' : 'configurationConfirmed')
    ..aOS(5, _omitFieldNames ? '' : 'source')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnStarted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnStarted copyWith(void Function(TurnStarted) updates) =>
      super.copyWith((message) => updates(message as TurnStarted))
          as TurnStarted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnStarted() / TurnStarted.new instead')
  static TurnStarted create() => TurnStarted._();
  static $pb.GeneratedMessage $_createMessage() => TurnStarted._();
  @$core.override
  TurnStarted createEmptyInstance() => TurnStarted._();
  @$core.pragma('dart2js:noInline')
  static TurnStarted getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TurnStarted>(
          TurnStarted.$_createMessage);
  static TurnStarted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get turnId => $_getSZ(0);
  @$pb.TagNumber(1)
  set turnId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTurnId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTurnId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelId => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasModelId() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get effort => $_getSZ(2);
  @$pb.TagNumber(3)
  set effort($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEffort() => $_has(2);
  @$pb.TagNumber(3)
  void clearEffort() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get configurationConfirmed => $_getBF(3);
  @$pb.TagNumber(4)
  set configurationConfirmed($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasConfigurationConfirmed() => $_has(3);
  @$pb.TagNumber(4)
  void clearConfigurationConfirmed() => $_clearField(4);

  /// ---- 本轮生效配置来源(只增:5,P3-T2;字段号分配登记见 proto/FIELD_ALLOCATIONS.md)----
  /// 本轮实际生效的模型/思考强度配置来自哪一级(与 model.proto
  /// EffectiveModelSelection.source 的四级来源词表一致,稳定字符串,新增来源不改 proto):
  ///   thread_override  — 本会话显式覆盖(手机端选择/委派指定)
  ///   app_default      — App 侧默认(daemon 落库的默认模型)
  ///   desktop_default  — 桌面端默认(如 codex config 的当前值)
  ///   native_current   — agent 原生当前值(会话实挂的模型/强度)
  /// 语义 = 本轮实际生效配置的来源,不是"用户最后一次在哪改的"。
  /// 旧 daemon 不带 = 空串:客户端按"未知来源"如实处理(不猜、不报错)。
  /// daemon 侧快照真值已备好,startedFrame 一行接线。
  @$pb.TagNumber(5)
  $core.String get source => $_getSZ(4);
  @$pb.TagNumber(5)
  set source($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSource() => $_has(4);
  @$pb.TagNumber(5)
  void clearSource() => $_clearField(5);
}

class TextDelta extends $pb.GeneratedMessage {
  factory TextDelta({
    $core.String? delta,
  }) {
    final result = TextDelta._();
    if (delta != null) result.delta = delta;
    return result;
  }

  TextDelta._();

  factory TextDelta.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TextDelta()..mergeFromBuffer(data, registry);
  factory TextDelta.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TextDelta()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TextDelta',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TextDelta.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'delta')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TextDelta clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TextDelta copyWith(void Function(TextDelta) updates) =>
      super.copyWith((message) => updates(message as TextDelta)) as TextDelta;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TextDelta() / TextDelta.new instead')
  static TextDelta create() => TextDelta._();
  static $pb.GeneratedMessage $_createMessage() => TextDelta._();
  @$core.override
  TextDelta createEmptyInstance() => TextDelta._();
  @$core.pragma('dart2js:noInline')
  static TextDelta getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TextDelta>(TextDelta.$_createMessage);
  static TextDelta? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get delta => $_getSZ(0);
  @$pb.TagNumber(1)
  set delta($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDelta() => $_has(0);
  @$pb.TagNumber(1)
  void clearDelta() => $_clearField(1);
}

/// ThinkingDelta 思考流的两类帧(一帧只带其一,不同时出现):
///   - delta    流式增量(与 TextDelta.delta 同语义,客户端追加拼接);
///   - snapshot 权威全文(pi 的 thinking_end 终局事件/历史回放一次性携带)。
/// 客户端收到 snapshot 即**整体替换**此前累积的思考文本 —— 它是丢帧/断连后的
/// 兜底真相,不依赖逐帧拼接的完整性。兼容性:旧客户端忽略未知字段 snapshot,
/// 增量行为不变;新客户端对 delta 为空、snapshot 非空的帧做替换、不追加。
class ThinkingDelta extends $pb.GeneratedMessage {
  factory ThinkingDelta({
    $core.String? delta,
    $core.String? snapshot,
  }) {
    final result = ThinkingDelta._();
    if (delta != null) result.delta = delta;
    if (snapshot != null) result.snapshot = snapshot;
    return result;
  }

  ThinkingDelta._();

  factory ThinkingDelta.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThinkingDelta()..mergeFromBuffer(data, registry);
  factory ThinkingDelta.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThinkingDelta()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThinkingDelta',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ThinkingDelta.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'delta')
    ..aOS(2, _omitFieldNames ? '' : 'snapshot')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThinkingDelta clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThinkingDelta copyWith(void Function(ThinkingDelta) updates) =>
      super.copyWith((message) => updates(message as ThinkingDelta))
          as ThinkingDelta;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ThinkingDelta() / ThinkingDelta.new instead')
  static ThinkingDelta create() => ThinkingDelta._();
  static $pb.GeneratedMessage $_createMessage() => ThinkingDelta._();
  @$core.override
  ThinkingDelta createEmptyInstance() => ThinkingDelta._();
  @$core.pragma('dart2js:noInline')
  static ThinkingDelta getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ThinkingDelta>(
          ThinkingDelta.$_createMessage);
  static ThinkingDelta? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get delta => $_getSZ(0);
  @$pb.TagNumber(1)
  set delta($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDelta() => $_has(0);
  @$pb.TagNumber(1)
  void clearDelta() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get snapshot => $_getSZ(1);
  @$pb.TagNumber(2)
  set snapshot($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSnapshot() => $_has(1);
  @$pb.TagNumber(2)
  void clearSnapshot() => $_clearField(2);
}

class ToolCallUpdate extends $pb.GeneratedMessage {
  factory ToolCallUpdate({
    ToolCallUpdate_Phase? phase,
    $1.ToolCallBlock? call,
    $1.ToolResultBlock? result,
  }) {
    final result$ = ToolCallUpdate._();
    if (phase != null) result$.phase = phase;
    if (call != null) result$.call = call;
    if (result != null) result$.result = result;
    return result$;
  }

  ToolCallUpdate._();

  factory ToolCallUpdate.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolCallUpdate()..mergeFromBuffer(data, registry);
  factory ToolCallUpdate.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolCallUpdate()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ToolCallUpdate',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ToolCallUpdate.$_createMessage)
    ..aE<ToolCallUpdate_Phase>(1, _omitFieldNames ? '' : 'phase',
        enumValues: ToolCallUpdate_Phase.values)
    ..aOM<$1.ToolCallBlock>(2, _omitFieldNames ? '' : 'call',
        subBuilder: $1.ToolCallBlock.$_createMessage)
    ..aOM<$1.ToolResultBlock>(3, _omitFieldNames ? '' : 'result',
        subBuilder: $1.ToolResultBlock.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolCallUpdate clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolCallUpdate copyWith(void Function(ToolCallUpdate) updates) =>
      super.copyWith((message) => updates(message as ToolCallUpdate))
          as ToolCallUpdate;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ToolCallUpdate() / ToolCallUpdate.new instead')
  static ToolCallUpdate create() => ToolCallUpdate._();
  static $pb.GeneratedMessage $_createMessage() => ToolCallUpdate._();
  @$core.override
  ToolCallUpdate createEmptyInstance() => ToolCallUpdate._();
  @$core.pragma('dart2js:noInline')
  static ToolCallUpdate getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ToolCallUpdate>(
          ToolCallUpdate.$_createMessage);
  static ToolCallUpdate? _defaultInstance;

  @$pb.TagNumber(1)
  ToolCallUpdate_Phase get phase => $_getN(0);
  @$pb.TagNumber(1)
  set phase(ToolCallUpdate_Phase value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPhase() => $_has(0);
  @$pb.TagNumber(1)
  void clearPhase() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.ToolCallBlock get call => $_getN(1);
  @$pb.TagNumber(2)
  set call($1.ToolCallBlock value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCall() => $_has(1);
  @$pb.TagNumber(2)
  void clearCall() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ToolCallBlock ensureCall() => $_ensure(1);

  @$pb.TagNumber(3)
  $1.ToolResultBlock get result => $_getN(2);
  @$pb.TagNumber(3)
  set result($1.ToolResultBlock value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasResult() => $_has(2);
  @$pb.TagNumber(3)
  void clearResult() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.ToolResultBlock ensureResult() => $_ensure(2);
}

/// ApprovalOption 审批卡上的一个可选项(C1:codex 0.159.2 三按钮审批增量)。
/// daemon 在审批卡 JSON(action.json_args)里同步带同样的 options/details;
/// 旧手机对未知 proto 字段与未知 JSON 键都忽略,仍按 approved 双按钮渲染,
/// 新手机读 options 渲染第三按钮 —— 新旧端互不破坏。
class ApprovalOption extends $pb.GeneratedMessage {
  factory ApprovalOption({
    $core.String? id,
    $core.String? label,
    $core.String? description,
  }) {
    final result = ApprovalOption._();
    if (id != null) result.id = id;
    if (label != null) result.label = label;
    if (description != null) result.description = description;
    return result;
  }

  ApprovalOption._();

  factory ApprovalOption.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalOption()..mergeFromBuffer(data, registry);
  factory ApprovalOption.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalOption()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApprovalOption',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ApprovalOption.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalOption clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalOption copyWith(void Function(ApprovalOption) updates) =>
      super.copyWith((message) => updates(message as ApprovalOption))
          as ApprovalOption;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ApprovalOption() / ApprovalOption.new instead')
  static ApprovalOption create() => ApprovalOption._();
  static $pb.GeneratedMessage $_createMessage() => ApprovalOption._();
  @$core.override
  ApprovalOption createEmptyInstance() => ApprovalOption._();
  @$core.pragma('dart2js:noInline')
  static ApprovalOption getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ApprovalOption>(
          ApprovalOption.$_createMessage);
  static ApprovalOption? _defaultInstance;

  /// 选项 id,codex 应答枚举原词(accept / acceptForSession / decline,
  /// 旧版 approved / approved_for_session / denied)。应答时原样回填
  /// ApprovalDecision.option,daemon 按协议版本翻译。
  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get label => $_getSZ(1);
  @$pb.TagNumber(2)
  set label($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLabel() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);
}

class ApprovalRequest extends $pb.GeneratedMessage {
  factory ApprovalRequest({
    $core.String? requestId,
    $1.ToolCallBlock? action,
    $core.String? reason,
    $core.Iterable<ApprovalOption>? options,
    $core.String? details,
  }) {
    final result = ApprovalRequest._();
    if (requestId != null) result.requestId = requestId;
    if (action != null) result.action = action;
    if (reason != null) result.reason = reason;
    if (options != null) result.options.addAll(options);
    if (details != null) result.details = details;
    return result;
  }

  ApprovalRequest._();

  factory ApprovalRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalRequest()..mergeFromBuffer(data, registry);
  factory ApprovalRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ApprovalRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApprovalRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ApprovalRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'requestId')
    ..aOM<$1.ToolCallBlock>(2, _omitFieldNames ? '' : 'action',
        subBuilder: $1.ToolCallBlock.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'reason')
    ..pPM<ApprovalOption>(4, _omitFieldNames ? '' : 'options',
        subBuilder: ApprovalOption.$_createMessage)
    ..aOS(5, _omitFieldNames ? '' : 'details')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApprovalRequest copyWith(void Function(ApprovalRequest) updates) =>
      super.copyWith((message) => updates(message as ApprovalRequest))
          as ApprovalRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ApprovalRequest() / ApprovalRequest.new instead')
  static ApprovalRequest create() => ApprovalRequest._();
  static $pb.GeneratedMessage $_createMessage() => ApprovalRequest._();
  @$core.override
  ApprovalRequest createEmptyInstance() => ApprovalRequest._();
  @$core.pragma('dart2js:noInline')
  static ApprovalRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ApprovalRequest>(
          ApprovalRequest.$_createMessage);
  static ApprovalRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requestId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requestId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequestId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequestId() => $_clearField(1);

  /// 需要审批的动作描述,如即将执行的命令 / 将写入的文件
  @$pb.TagNumber(2)
  $1.ToolCallBlock get action => $_getN(1);
  @$pb.TagNumber(2)
  set action($1.ToolCallBlock value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasAction() => $_has(1);
  @$pb.TagNumber(2)
  void clearAction() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ToolCallBlock ensureAction() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(3)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(3)
  void clearReason() => $_clearField(3);

  /// ---- C1 只增不改:4~5 为 codex 0.159.2 三按钮审批新增 ----
  @$pb.TagNumber(4)
  $pb.PbList<ApprovalOption> get options => $_getList(3);

  @$pb.TagNumber(5)
  $core.String get details => $_getSZ(4);
  @$pb.TagNumber(5)
  set details($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDetails() => $_has(4);
  @$pb.TagNumber(5)
  void clearDetails() => $_clearField(5);
}

class UsageUpdate extends $pb.GeneratedMessage {
  factory UsageUpdate({
    $1.Usage? usage,
    $1.Usage? totalUsage,
    $fixnum.Int64? modelContextWindow,
  }) {
    final result = UsageUpdate._();
    if (usage != null) result.usage = usage;
    if (totalUsage != null) result.totalUsage = totalUsage;
    if (modelContextWindow != null)
      result.modelContextWindow = modelContextWindow;
    return result;
  }

  UsageUpdate._();

  factory UsageUpdate.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UsageUpdate()..mergeFromBuffer(data, registry);
  factory UsageUpdate.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UsageUpdate()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UsageUpdate',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: UsageUpdate.$_createMessage)
    ..aOM<$1.Usage>(1, _omitFieldNames ? '' : 'usage',
        subBuilder: $1.Usage.$_createMessage)
    ..aOM<$1.Usage>(2, _omitFieldNames ? '' : 'totalUsage',
        subBuilder: $1.Usage.$_createMessage)
    ..aInt64(3, _omitFieldNames ? '' : 'modelContextWindow')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UsageUpdate clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UsageUpdate copyWith(void Function(UsageUpdate) updates) =>
      super.copyWith((message) => updates(message as UsageUpdate))
          as UsageUpdate;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UsageUpdate() / UsageUpdate.new instead')
  static UsageUpdate create() => UsageUpdate._();
  static $pb.GeneratedMessage $_createMessage() => UsageUpdate._();
  @$core.override
  UsageUpdate createEmptyInstance() => UsageUpdate._();
  @$core.pragma('dart2js:noInline')
  static UsageUpdate getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UsageUpdate>(
          UsageUpdate.$_createMessage);
  static UsageUpdate? _defaultInstance;

  @$pb.TagNumber(1)
  $1.Usage get usage => $_getN(0);
  @$pb.TagNumber(1)
  set usage($1.Usage value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasUsage() => $_has(0);
  @$pb.TagNumber(1)
  void clearUsage() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.Usage ensureUsage() => $_ensure(0);

  @$pb.TagNumber(2)
  $1.Usage get totalUsage => $_getN(1);
  @$pb.TagNumber(2)
  set totalUsage($1.Usage value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasTotalUsage() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotalUsage() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Usage ensureTotalUsage() => $_ensure(1);

  /// codex thread/tokenUsage/updated 上报的真实窗口大小;
  /// 模型列表拉不到各模型的窗口时,以这里为准(切模型后也能显示)。
  @$pb.TagNumber(3)
  $fixnum.Int64 get modelContextWindow => $_getI64(2);
  @$pb.TagNumber(3)
  set modelContextWindow($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasModelContextWindow() => $_has(2);
  @$pb.TagNumber(3)
  void clearModelContextWindow() => $_clearField(3);
}

class TurnCompleted extends $pb.GeneratedMessage {
  factory TurnCompleted({
    $core.String? turnId,
    $core.Iterable<$1.ContentBlock>? finalBlocks,
    $1.Usage? usage,
    $fixnum.Int64? elapsedMs,
    OutputState? outputState,
    $core.String? outputSource,
    $core.String? diagnosticId,
  }) {
    final result = TurnCompleted._();
    if (turnId != null) result.turnId = turnId;
    if (finalBlocks != null) result.finalBlocks.addAll(finalBlocks);
    if (usage != null) result.usage = usage;
    if (elapsedMs != null) result.elapsedMs = elapsedMs;
    if (outputState != null) result.outputState = outputState;
    if (outputSource != null) result.outputSource = outputSource;
    if (diagnosticId != null) result.diagnosticId = diagnosticId;
    return result;
  }

  TurnCompleted._();

  factory TurnCompleted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnCompleted()..mergeFromBuffer(data, registry);
  factory TurnCompleted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnCompleted()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnCompleted',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnCompleted.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'turnId')
    ..pPM<$1.ContentBlock>(2, _omitFieldNames ? '' : 'finalBlocks',
        subBuilder: $1.ContentBlock.$_createMessage)
    ..aOM<$1.Usage>(3, _omitFieldNames ? '' : 'usage',
        subBuilder: $1.Usage.$_createMessage)
    ..aInt64(4, _omitFieldNames ? '' : 'elapsedMs')
    ..aE<OutputState>(5, _omitFieldNames ? '' : 'outputState',
        enumValues: OutputState.values)
    ..aOS(6, _omitFieldNames ? '' : 'outputSource')
    ..aOS(7, _omitFieldNames ? '' : 'diagnosticId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnCompleted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnCompleted copyWith(void Function(TurnCompleted) updates) =>
      super.copyWith((message) => updates(message as TurnCompleted))
          as TurnCompleted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnCompleted() / TurnCompleted.new instead')
  static TurnCompleted create() => TurnCompleted._();
  static $pb.GeneratedMessage $_createMessage() => TurnCompleted._();
  @$core.override
  TurnCompleted createEmptyInstance() => TurnCompleted._();
  @$core.pragma('dart2js:noInline')
  static TurnCompleted getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TurnCompleted>(
          TurnCompleted.$_createMessage);
  static TurnCompleted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get turnId => $_getSZ(0);
  @$pb.TagNumber(1)
  set turnId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTurnId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTurnId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$1.ContentBlock> get finalBlocks => $_getList(1);

  @$pb.TagNumber(3)
  $1.Usage get usage => $_getN(2);
  @$pb.TagNumber(3)
  set usage($1.Usage value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasUsage() => $_has(2);
  @$pb.TagNumber(3)
  void clearUsage() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Usage ensureUsage() => $_ensure(2);

  @$pb.TagNumber(4)
  $fixnum.Int64 get elapsedMs => $_getI64(3);
  @$pb.TagNumber(4)
  set elapsedMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasElapsedMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearElapsedMs() => $_clearField(4);

  @$pb.TagNumber(5)
  OutputState get outputState => $_getN(4);
  @$pb.TagNumber(5)
  set outputState(OutputState value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasOutputState() => $_has(4);
  @$pb.TagNumber(5)
  void clearOutputState() => $_clearField(5);

  /// output_source 正文来源的可读标注("delta" / "item_completed" /
  /// "turn_items" / "native_history"),诊断用,手机端可忽略。
  @$pb.TagNumber(6)
  $core.String get outputSource => $_getSZ(5);
  @$pb.TagNumber(6)
  set outputSource($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasOutputSource() => $_has(5);
  @$pb.TagNumber(6)
  void clearOutputSource() => $_clearField(6);

  /// diagnostic_id 关联的诊断事件 id(有诊断可查时非空)。
  @$pb.TagNumber(7)
  $core.String get diagnosticId => $_getSZ(6);
  @$pb.TagNumber(7)
  set diagnosticId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasDiagnosticId() => $_has(6);
  @$pb.TagNumber(7)
  void clearDiagnosticId() => $_clearField(7);
}

class TurnFailed extends $pb.GeneratedMessage {
  factory TurnFailed({
    $core.String? turnId,
    $1.Error? error,
    $core.String? partialText,
  }) {
    final result = TurnFailed._();
    if (turnId != null) result.turnId = turnId;
    if (error != null) result.error = error;
    if (partialText != null) result.partialText = partialText;
    return result;
  }

  TurnFailed._();

  factory TurnFailed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnFailed()..mergeFromBuffer(data, registry);
  factory TurnFailed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnFailed()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnFailed',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnFailed.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'turnId')
    ..aOM<$1.Error>(2, _omitFieldNames ? '' : 'error',
        subBuilder: $1.Error.$_createMessage)
    ..aOS(3, _omitFieldNames ? '' : 'partialText')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnFailed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnFailed copyWith(void Function(TurnFailed) updates) =>
      super.copyWith((message) => updates(message as TurnFailed)) as TurnFailed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TurnFailed() / TurnFailed.new instead')
  static TurnFailed create() => TurnFailed._();
  static $pb.GeneratedMessage $_createMessage() => TurnFailed._();
  @$core.override
  TurnFailed createEmptyInstance() => TurnFailed._();
  @$core.pragma('dart2js:noInline')
  static TurnFailed getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TurnFailed>(TurnFailed.$_createMessage);
  static TurnFailed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get turnId => $_getSZ(0);
  @$pb.TagNumber(1)
  set turnId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTurnId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTurnId() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Error get error => $_getN(1);
  @$pb.TagNumber(2)
  set error($1.Error value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasError() => $_has(1);
  @$pb.TagNumber(2)
  void clearError() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Error ensureError() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get partialText => $_getSZ(2);
  @$pb.TagNumber(3)
  set partialText($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPartialText() => $_has(2);
  @$pb.TagNumber(3)
  void clearPartialText() => $_clearField(3);
}

class CancelTurnRequest extends $pb.GeneratedMessage {
  factory CancelTurnRequest({
    $core.String? threadId,
  }) {
    final result = CancelTurnRequest._();
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  CancelTurnRequest._();

  factory CancelTurnRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelTurnRequest()..mergeFromBuffer(data, registry);
  factory CancelTurnRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelTurnRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelTurnRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CancelTurnRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelTurnRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelTurnRequest copyWith(void Function(CancelTurnRequest) updates) =>
      super.copyWith((message) => updates(message as CancelTurnRequest))
          as CancelTurnRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CancelTurnRequest() / CancelTurnRequest.new instead')
  static CancelTurnRequest create() => CancelTurnRequest._();
  static $pb.GeneratedMessage $_createMessage() => CancelTurnRequest._();
  @$core.override
  CancelTurnRequest createEmptyInstance() => CancelTurnRequest._();
  @$core.pragma('dart2js:noInline')
  static CancelTurnRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CancelTurnRequest>(
          CancelTurnRequest.$_createMessage);
  static CancelTurnRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);
}

class ClearQueuedInputsRequest extends $pb.GeneratedMessage {
  factory ClearQueuedInputsRequest({
    $core.String? threadId,
  }) {
    final result = ClearQueuedInputsRequest._();
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  ClearQueuedInputsRequest._();

  factory ClearQueuedInputsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearQueuedInputsRequest()..mergeFromBuffer(data, registry);
  factory ClearQueuedInputsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearQueuedInputsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearQueuedInputsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ClearQueuedInputsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearQueuedInputsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearQueuedInputsRequest copyWith(
          void Function(ClearQueuedInputsRequest) updates) =>
      super.copyWith((message) => updates(message as ClearQueuedInputsRequest))
          as ClearQueuedInputsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ClearQueuedInputsRequest() / ClearQueuedInputsRequest.new instead')
  static ClearQueuedInputsRequest create() => ClearQueuedInputsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ClearQueuedInputsRequest._();
  @$core.override
  ClearQueuedInputsRequest createEmptyInstance() =>
      ClearQueuedInputsRequest._();
  @$core.pragma('dart2js:noInline')
  static ClearQueuedInputsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearQueuedInputsRequest>(
          ClearQueuedInputsRequest.$_createMessage);
  static ClearQueuedInputsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);
}

class ClearQueuedInputsResponse extends $pb.GeneratedMessage {
  factory ClearQueuedInputsResponse({
    $core.int? steered,
    $core.int? followUp,
  }) {
    final result = ClearQueuedInputsResponse._();
    if (steered != null) result.steered = steered;
    if (followUp != null) result.followUp = followUp;
    return result;
  }

  ClearQueuedInputsResponse._();

  factory ClearQueuedInputsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearQueuedInputsResponse()..mergeFromBuffer(data, registry);
  factory ClearQueuedInputsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearQueuedInputsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearQueuedInputsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ClearQueuedInputsResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'steered')
    ..aI(2, _omitFieldNames ? '' : 'followUp')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearQueuedInputsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearQueuedInputsResponse copyWith(
          void Function(ClearQueuedInputsResponse) updates) =>
      super.copyWith((message) => updates(message as ClearQueuedInputsResponse))
          as ClearQueuedInputsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ClearQueuedInputsResponse() / ClearQueuedInputsResponse.new instead')
  static ClearQueuedInputsResponse create() => ClearQueuedInputsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ClearQueuedInputsResponse._();
  @$core.override
  ClearQueuedInputsResponse createEmptyInstance() =>
      ClearQueuedInputsResponse._();
  @$core.pragma('dart2js:noInline')
  static ClearQueuedInputsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearQueuedInputsResponse>(
          ClearQueuedInputsResponse.$_createMessage);
  static ClearQueuedInputsResponse? _defaultInstance;

  /// 被撤销的条数,两个口径与 pi 的队列一致:steered = 插队(当前轮还没结束、
  /// 会被优先消费的),follow_up = 排队(当前轮结束后依次执行的)。
  @$pb.TagNumber(1)
  $core.int get steered => $_getIZ(0);
  @$pb.TagNumber(1)
  set steered($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSteered() => $_has(0);
  @$pb.TagNumber(1)
  void clearSteered() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get followUp => $_getIZ(1);
  @$pb.TagNumber(2)
  set followUp($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFollowUp() => $_has(1);
  @$pb.TagNumber(2)
  void clearFollowUp() => $_clearField(2);
}

class SubmitApprovalRequest extends $pb.GeneratedMessage {
  factory SubmitApprovalRequest({
    $core.String? threadId,
    ApprovalDecision? decision,
  }) {
    final result = SubmitApprovalRequest._();
    if (threadId != null) result.threadId = threadId;
    if (decision != null) result.decision = decision;
    return result;
  }

  SubmitApprovalRequest._();

  factory SubmitApprovalRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitApprovalRequest()..mergeFromBuffer(data, registry);
  factory SubmitApprovalRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SubmitApprovalRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitApprovalRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SubmitApprovalRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOM<ApprovalDecision>(2, _omitFieldNames ? '' : 'decision',
        subBuilder: ApprovalDecision.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitApprovalRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitApprovalRequest copyWith(
          void Function(SubmitApprovalRequest) updates) =>
      super.copyWith((message) => updates(message as SubmitApprovalRequest))
          as SubmitApprovalRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SubmitApprovalRequest() / SubmitApprovalRequest.new instead')
  static SubmitApprovalRequest create() => SubmitApprovalRequest._();
  static $pb.GeneratedMessage $_createMessage() => SubmitApprovalRequest._();
  @$core.override
  SubmitApprovalRequest createEmptyInstance() => SubmitApprovalRequest._();
  @$core.pragma('dart2js:noInline')
  static SubmitApprovalRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitApprovalRequest>(
          SubmitApprovalRequest.$_createMessage);
  static SubmitApprovalRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  ApprovalDecision get decision => $_getN(1);
  @$pb.TagNumber(2)
  set decision(ApprovalDecision value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasDecision() => $_has(1);
  @$pb.TagNumber(2)
  void clearDecision() => $_clearField(2);
  @$pb.TagNumber(2)
  ApprovalDecision ensureDecision() => $_ensure(1);
}

class SearchThreadMessagesRequest extends $pb.GeneratedMessage {
  factory SearchThreadMessagesRequest({
    $core.String? threadId,
    $core.String? query,
    $core.int? limit,
  }) {
    final result = SearchThreadMessagesRequest._();
    if (threadId != null) result.threadId = threadId;
    if (query != null) result.query = query;
    if (limit != null) result.limit = limit;
    return result;
  }

  SearchThreadMessagesRequest._();

  factory SearchThreadMessagesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchThreadMessagesRequest()..mergeFromBuffer(data, registry);
  factory SearchThreadMessagesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchThreadMessagesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchThreadMessagesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SearchThreadMessagesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'query')
    ..aI(3, _omitFieldNames ? '' : 'limit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchThreadMessagesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchThreadMessagesRequest copyWith(
          void Function(SearchThreadMessagesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SearchThreadMessagesRequest))
          as SearchThreadMessagesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SearchThreadMessagesRequest() / SearchThreadMessagesRequest.new instead')
  static SearchThreadMessagesRequest create() =>
      SearchThreadMessagesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      SearchThreadMessagesRequest._();
  @$core.override
  SearchThreadMessagesRequest createEmptyInstance() =>
      SearchThreadMessagesRequest._();
  @$core.pragma('dart2js:noInline')
  static SearchThreadMessagesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchThreadMessagesRequest>(
          SearchThreadMessagesRequest.$_createMessage);
  static SearchThreadMessagesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  /// 搜索词(trim 后必须非空):对消息正文(文本块)做大小写不敏感的包含匹配。
  @$pb.TagNumber(2)
  $core.String get query => $_getSZ(1);
  @$pb.TagNumber(2)
  set query($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasQuery() => $_has(1);
  @$pb.TagNumber(2)
  void clearQuery() => $_clearField(2);

  /// 返回命中条数上限:0 = 默认 20;负数同 0;超过 100 钳到 100(daemon 侧钳制,
  /// 手机端不必自行防御)。命中按消息序号升序返回(先出现的先回)。
  @$pb.TagNumber(3)
  $core.int get limit => $_getIZ(2);
  @$pb.TagNumber(3)
  set limit($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLimit() => $_has(2);
  @$pb.TagNumber(3)
  void clearLimit() => $_clearField(3);
}

/// MatchRange 一处命中在文本里的范围(2026-10-04 会话选择与历史体验计划 Task 6,
/// 分配登记见 proto/FIELD_ALLOCATIONS.md)。
/// [start, end) 半开区间,按 Unicode 码点(rune)计 —— 与 daemon 既有 snippet 的
/// 截断口径一致(中文按字算,不吃 UTF-8 字节错位亏)。客户端只拿它做高亮渲染,
/// 异常值按钳制处理,**不作为消息定位依据**(定位看 message_id)。
class MatchRange extends $pb.GeneratedMessage {
  factory MatchRange({
    $core.int? start,
    $core.int? end,
  }) {
    final result = MatchRange._();
    if (start != null) result.start = start;
    if (end != null) result.end = end;
    return result;
  }

  MatchRange._();

  factory MatchRange.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MatchRange()..mergeFromBuffer(data, registry);
  factory MatchRange.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MatchRange()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchRange',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: MatchRange.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'start')
    ..aI(2, _omitFieldNames ? '' : 'end')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchRange clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchRange copyWith(void Function(MatchRange) updates) =>
      super.copyWith((message) => updates(message as MatchRange)) as MatchRange;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MatchRange() / MatchRange.new instead')
  static MatchRange create() => MatchRange._();
  static $pb.GeneratedMessage $_createMessage() => MatchRange._();
  @$core.override
  MatchRange createEmptyInstance() => MatchRange._();
  @$core.pragma('dart2js:noInline')
  static MatchRange getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchRange>(MatchRange.$_createMessage);
  static MatchRange? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get start => $_getIZ(0);
  @$pb.TagNumber(1)
  set start($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStart() => $_has(0);
  @$pb.TagNumber(1)
  void clearStart() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get end => $_getIZ(1);
  @$pb.TagNumber(2)
  set end($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEnd() => $_has(1);
  @$pb.TagNumber(2)
  void clearEnd() => $_clearField(2);
}

/// ThreadMessageHit 一条命中。sequence 是该消息在会话消息数组里的 0 基下标,
/// 与 GetThreadHistory 返回顺序一致 —— 它现在只是**兼容字段**:下标会随加载更早/
/// fork/rollback 漂移,定位一律优先用 message_id(6);旧 daemon 回包不带 6~9 时
/// 才退回 sequence 兜底。snippet 是命中处的摘录(匹配词 + 前后文各 40 字符,
/// 截断处以 … 标示),只含文本正文;思考块/工具调用参数刻意不搜(那是过程不是正文)。
class ThreadMessageHit extends $pb.GeneratedMessage {
  factory ThreadMessageHit({
    $core.int? sequence,
    $core.String? role,
    $core.String? snippet,
    $fixnum.Int64? createdAtUnixMs,
    $core.String? messageId,
    $core.String? turnId,
    $core.Iterable<MatchRange>? ranges,
    $core.bool? fromLoadedHistory,
  }) {
    final result = ThreadMessageHit._();
    if (sequence != null) result.sequence = sequence;
    if (role != null) result.role = role;
    if (snippet != null) result.snippet = snippet;
    if (createdAtUnixMs != null) result.createdAtUnixMs = createdAtUnixMs;
    if (messageId != null) result.messageId = messageId;
    if (turnId != null) result.turnId = turnId;
    if (ranges != null) result.ranges.addAll(ranges);
    if (fromLoadedHistory != null) result.fromLoadedHistory = fromLoadedHistory;
    return result;
  }

  ThreadMessageHit._();

  factory ThreadMessageHit.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadMessageHit()..mergeFromBuffer(data, registry);
  factory ThreadMessageHit.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadMessageHit()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThreadMessageHit',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ThreadMessageHit.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'sequence')
    ..aOS(2, _omitFieldNames ? '' : 'role')
    ..aOS(3, _omitFieldNames ? '' : 'snippet')
    ..aInt64(4, _omitFieldNames ? '' : 'createdAtUnixMs')
    ..aOS(6, _omitFieldNames ? '' : 'messageId')
    ..aOS(7, _omitFieldNames ? '' : 'turnId')
    ..pPM<MatchRange>(8, _omitFieldNames ? '' : 'ranges',
        subBuilder: MatchRange.$_createMessage)
    ..aOB(9, _omitFieldNames ? '' : 'fromLoadedHistory')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadMessageHit clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadMessageHit copyWith(void Function(ThreadMessageHit) updates) =>
      super.copyWith((message) => updates(message as ThreadMessageHit))
          as ThreadMessageHit;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ThreadMessageHit() / ThreadMessageHit.new instead')
  static ThreadMessageHit create() => ThreadMessageHit._();
  static $pb.GeneratedMessage $_createMessage() => ThreadMessageHit._();
  @$core.override
  ThreadMessageHit createEmptyInstance() => ThreadMessageHit._();
  @$core.pragma('dart2js:noInline')
  static ThreadMessageHit getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ThreadMessageHit>(
          ThreadMessageHit.$_createMessage);
  static ThreadMessageHit? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get sequence => $_getIZ(0);
  @$pb.TagNumber(1)
  set sequence($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSequence() => $_has(0);
  @$pb.TagNumber(1)
  void clearSequence() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get role => $_getSZ(1);
  @$pb.TagNumber(2)
  set role($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRole() => $_has(1);
  @$pb.TagNumber(2)
  void clearRole() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get snippet => $_getSZ(2);
  @$pb.TagNumber(3)
  set snippet($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSnippet() => $_has(2);
  @$pb.TagNumber(3)
  void clearSnippet() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get createdAtUnixMs => $_getI64(3);
  @$pb.TagNumber(4)
  set createdAtUnixMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCreatedAtUnixMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearCreatedAtUnixMs() => $_clearField(4);

  /// 字段 5 刻意留空:号位按 2026-10-04 spec 原样分配(6~9,spec 误以为旧消息
  /// 占到 5),为防撞号此号位冻结不再启用;登记见 proto/FIELD_ALLOCATIONS.md。
  /// ---- 命中身份与匹配范围(只增:6~9)----
  /// 命中消息的稳定 id(与 ThreadMessage.message_id 同源),定位主键。
  @$pb.TagNumber(6)
  $core.String get messageId => $_getSZ(4);
  @$pb.TagNumber(6)
  set messageId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(6)
  $core.bool hasMessageId() => $_has(4);
  @$pb.TagNumber(6)
  void clearMessageId() => $_clearField(6);

  /// 命中消息所属轮次 id(与 ThreadMessage.turn_id 同源);空 = 未知。
  @$pb.TagNumber(7)
  $core.String get turnId => $_getSZ(5);
  @$pb.TagNumber(7)
  set turnId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(7)
  $core.bool hasTurnId() => $_has(5);
  @$pb.TagNumber(7)
  void clearTurnId() => $_clearField(7);

  /// 该消息内的命中范围,可多处(如一词多次出现);按出现顺序给。
  @$pb.TagNumber(8)
  $pb.PbList<MatchRange> get ranges => $_getList(6);

  /// true = 命中来自已加载进手机时间线的历史片段(点一下就在眼前);
  /// false = 命中仍在 daemon 侧未下载的历史里(客户端需先请求目标片段再跳)。
  /// 空 = 旧 daemon,客户端按未知处理,不猜。
  @$pb.TagNumber(9)
  $core.bool get fromLoadedHistory => $_getBF(7);
  @$pb.TagNumber(9)
  set fromLoadedHistory($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(9)
  $core.bool hasFromLoadedHistory() => $_has(7);
  @$pb.TagNumber(9)
  void clearFromLoadedHistory() => $_clearField(9);
}

class SearchThreadMessagesResponse extends $pb.GeneratedMessage {
  factory SearchThreadMessagesResponse({
    $core.Iterable<ThreadMessageHit>? hits,
    $core.bool? hasMore,
    $core.int? searchedMessages,
    $core.String? scope,
  }) {
    final result = SearchThreadMessagesResponse._();
    if (hits != null) result.hits.addAll(hits);
    if (hasMore != null) result.hasMore = hasMore;
    if (searchedMessages != null) result.searchedMessages = searchedMessages;
    if (scope != null) result.scope = scope;
    return result;
  }

  SearchThreadMessagesResponse._();

  factory SearchThreadMessagesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchThreadMessagesResponse()..mergeFromBuffer(data, registry);
  factory SearchThreadMessagesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchThreadMessagesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchThreadMessagesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SearchThreadMessagesResponse.$_createMessage)
    ..pPM<ThreadMessageHit>(1, _omitFieldNames ? '' : 'hits',
        subBuilder: ThreadMessageHit.$_createMessage)
    ..aOB(2, _omitFieldNames ? '' : 'hasMore')
    ..aI(3, _omitFieldNames ? '' : 'searchedMessages')
    ..aOS(4, _omitFieldNames ? '' : 'scope')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchThreadMessagesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchThreadMessagesResponse copyWith(
          void Function(SearchThreadMessagesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SearchThreadMessagesResponse))
          as SearchThreadMessagesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SearchThreadMessagesResponse() / SearchThreadMessagesResponse.new instead')
  static SearchThreadMessagesResponse create() =>
      SearchThreadMessagesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      SearchThreadMessagesResponse._();
  @$core.override
  SearchThreadMessagesResponse createEmptyInstance() =>
      SearchThreadMessagesResponse._();
  @$core.pragma('dart2js:noInline')
  static SearchThreadMessagesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchThreadMessagesResponse>(
          SearchThreadMessagesResponse.$_createMessage);
  static SearchThreadMessagesResponse? _defaultInstance;

  /// 无命中 = 空列表(不是错误);命中超出 limit 时截断。旧客户端拿"满 limit"
  /// 自行猜测可能还有更多 —— 新客户端改读 has_more(2,权威)与 searched_messages
  /// (3,本次实扫条数),不再靠启发式;回包规模始终有界(limit 钳制在 daemon 侧)。
  @$pb.TagNumber(1)
  $pb.PbList<ThreadMessageHit> get hits => $_getList(0);

  /// true = 还有未返回的命中(客户端可据此提示"还有更多",为未来 FTS 迁移
  /// 预留分页语义;当前 daemon 一次性回满 limit 内命中,has_more 恒 false ——
  /// 客户端不得假设它恒 false,协议上它是真分页位)。
  @$pb.TagNumber(2)
  $core.bool get hasMore => $_getBF(1);
  @$pb.TagNumber(2)
  set hasMore($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHasMore() => $_has(1);
  @$pb.TagNumber(2)
  void clearHasMore() => $_clearField(2);

  /// 本次实际扫描的消息条数(诊断/展示"搜了 N 条"用;与命中数无关)。
  @$pb.TagNumber(3)
  $core.int get searchedMessages => $_getIZ(2);
  @$pb.TagNumber(3)
  set searchedMessages($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSearchedMessages() => $_has(2);
  @$pb.TagNumber(3)
  void clearSearchedMessages() => $_clearField(3);

  /// 搜索覆盖范围(稳定字符串,新增口径不改 proto):
  ///   full_thread     — 全会话消息(daemon 内存 store 全量)
  ///   local_snapshot  — 仅本地快照/已加载片段(降级口径)
  /// 空 = 旧 daemon 未报,客户端按未知处理,不得臆断为 full_thread。
  @$pb.TagNumber(4)
  $core.String get scope => $_getSZ(3);
  @$pb.TagNumber(4)
  set scope($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasScope() => $_has(3);
  @$pb.TagNumber(4)
  void clearScope() => $_clearField(4);
}

/// CollabSessionSnapshot 一次协同会话的完整快照:会话级状态 + 全体成员卡片。
/// daemon 是协同状态的唯一事实来源 —— 手机端打开协同页先用本消息恢复整屏,
/// 再用 WatchCollabSession 增量跟随;两者给的必须一致。
class CollabSessionSnapshot extends $pb.GeneratedMessage {
  factory CollabSessionSnapshot({
    $core.String? sessionId,
    $core.String? instruction,
    $core.String? status,
    $core.String? workspace,
    $fixnum.Int64? createdAtUnixMs,
    $fixnum.Int64? updatedAtUnixMs,
    $core.Iterable<CollabMemberSnapshot>? members,
    $core.String? error,
  }) {
    final result = CollabSessionSnapshot._();
    if (sessionId != null) result.sessionId = sessionId;
    if (instruction != null) result.instruction = instruction;
    if (status != null) result.status = status;
    if (workspace != null) result.workspace = workspace;
    if (createdAtUnixMs != null) result.createdAtUnixMs = createdAtUnixMs;
    if (updatedAtUnixMs != null) result.updatedAtUnixMs = updatedAtUnixMs;
    if (members != null) result.members.addAll(members);
    if (error != null) result.error = error;
    return result;
  }

  CollabSessionSnapshot._();

  factory CollabSessionSnapshot.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabSessionSnapshot()..mergeFromBuffer(data, registry);
  factory CollabSessionSnapshot.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabSessionSnapshot()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CollabSessionSnapshot',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CollabSessionSnapshot.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'instruction')
    ..aOS(3, _omitFieldNames ? '' : 'status')
    ..aOS(4, _omitFieldNames ? '' : 'workspace')
    ..aInt64(5, _omitFieldNames ? '' : 'createdAtUnixMs')
    ..aInt64(6, _omitFieldNames ? '' : 'updatedAtUnixMs')
    ..pPM<CollabMemberSnapshot>(7, _omitFieldNames ? '' : 'members',
        subBuilder: CollabMemberSnapshot.$_createMessage)
    ..aOS(8, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabSessionSnapshot clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabSessionSnapshot copyWith(
          void Function(CollabSessionSnapshot) updates) =>
      super.copyWith((message) => updates(message as CollabSessionSnapshot))
          as CollabSessionSnapshot;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CollabSessionSnapshot() / CollabSessionSnapshot.new instead')
  static CollabSessionSnapshot create() => CollabSessionSnapshot._();
  static $pb.GeneratedMessage $_createMessage() => CollabSessionSnapshot._();
  @$core.override
  CollabSessionSnapshot createEmptyInstance() => CollabSessionSnapshot._();
  @$core.pragma('dart2js:noInline')
  static CollabSessionSnapshot getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CollabSessionSnapshot>(
          CollabSessionSnapshot.$_createMessage);
  static CollabSessionSnapshot? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get instruction => $_getSZ(1);
  @$pb.TagNumber(2)
  set instruction($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasInstruction() => $_has(1);
  @$pb.TagNumber(2)
  void clearInstruction() => $_clearField(2);

  /// 会话状态:string 口径,取值随 daemon 状态机(如 running / completed /
  /// failed / cancelled);旧 daemon 可能给空,客户端如实显示"未知",不编造。
  @$pb.TagNumber(3)
  $core.String get status => $_getSZ(2);
  @$pb.TagNumber(3)
  set status($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get workspace => $_getSZ(3);
  @$pb.TagNumber(4)
  set workspace($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasWorkspace() => $_has(3);
  @$pb.TagNumber(4)
  void clearWorkspace() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get createdAtUnixMs => $_getI64(4);
  @$pb.TagNumber(5)
  set createdAtUnixMs($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCreatedAtUnixMs() => $_has(4);
  @$pb.TagNumber(5)
  void clearCreatedAtUnixMs() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get updatedAtUnixMs => $_getI64(5);
  @$pb.TagNumber(6)
  set updatedAtUnixMs($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasUpdatedAtUnixMs() => $_has(5);
  @$pb.TagNumber(6)
  void clearUpdatedAtUnixMs() => $_clearField(6);

  @$pb.TagNumber(7)
  $pb.PbList<CollabMemberSnapshot> get members => $_getList(6);

  @$pb.TagNumber(8)
  $core.String get error => $_getSZ(7);
  @$pb.TagNumber(8)
  set error($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasError() => $_has(7);
  @$pb.TagNumber(8)
  void clearError() => $_clearField(8);
}

/// CollabMemberSnapshot 协同会话里一个成员的实时卡片:它是谁、跑到哪、等什么。
/// 【为什么 pending_approvals 内嵌在成员卡上】"一个成员等审批、其他成员继续跑"
/// 是协同的常态 —— 审批挂在自己的成员卡上,才不会把整个会话误报成卡死。
class CollabMemberSnapshot extends $pb.GeneratedMessage {
  factory CollabMemberSnapshot({
    $core.String? agentId,
    $core.String? roleId,
    $core.String? threadId,
    $core.String? status,
    $core.String? modelId,
    $core.String? effort,
    $core.String? progress,
    $core.String? summary,
    $core.String? error,
    $core.Iterable<ApprovalRequest>? pendingApprovals,
    $core.String? workspace,
  }) {
    final result = CollabMemberSnapshot._();
    if (agentId != null) result.agentId = agentId;
    if (roleId != null) result.roleId = roleId;
    if (threadId != null) result.threadId = threadId;
    if (status != null) result.status = status;
    if (modelId != null) result.modelId = modelId;
    if (effort != null) result.effort = effort;
    if (progress != null) result.progress = progress;
    if (summary != null) result.summary = summary;
    if (error != null) result.error = error;
    if (pendingApprovals != null)
      result.pendingApprovals.addAll(pendingApprovals);
    if (workspace != null) result.workspace = workspace;
    return result;
  }

  CollabMemberSnapshot._();

  factory CollabMemberSnapshot.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabMemberSnapshot()..mergeFromBuffer(data, registry);
  factory CollabMemberSnapshot.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabMemberSnapshot()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CollabMemberSnapshot',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CollabMemberSnapshot.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'roleId')
    ..aOS(3, _omitFieldNames ? '' : 'threadId')
    ..aOS(4, _omitFieldNames ? '' : 'status')
    ..aOS(5, _omitFieldNames ? '' : 'modelId')
    ..aOS(6, _omitFieldNames ? '' : 'effort')
    ..aOS(7, _omitFieldNames ? '' : 'progress')
    ..aOS(8, _omitFieldNames ? '' : 'summary')
    ..aOS(9, _omitFieldNames ? '' : 'error')
    ..pPM<ApprovalRequest>(10, _omitFieldNames ? '' : 'pendingApprovals',
        subBuilder: ApprovalRequest.$_createMessage)
    ..aOS(11, _omitFieldNames ? '' : 'workspace')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabMemberSnapshot clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabMemberSnapshot copyWith(void Function(CollabMemberSnapshot) updates) =>
      super.copyWith((message) => updates(message as CollabMemberSnapshot))
          as CollabMemberSnapshot;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CollabMemberSnapshot() / CollabMemberSnapshot.new instead')
  static CollabMemberSnapshot create() => CollabMemberSnapshot._();
  static $pb.GeneratedMessage $_createMessage() => CollabMemberSnapshot._();
  @$core.override
  CollabMemberSnapshot createEmptyInstance() => CollabMemberSnapshot._();
  @$core.pragma('dart2js:noInline')
  static CollabMemberSnapshot getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CollabMemberSnapshot>(
          CollabMemberSnapshot.$_createMessage);
  static CollabMemberSnapshot? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get roleId => $_getSZ(1);
  @$pb.TagNumber(2)
  set roleId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRoleId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRoleId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get threadId => $_getSZ(2);
  @$pb.TagNumber(3)
  set threadId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasThreadId() => $_has(2);
  @$pb.TagNumber(3)
  void clearThreadId() => $_clearField(3);

  /// 成员状态:string 口径(ACCEPTED / RUNNING / COMPLETED / FAILED /
  /// CANCELLED / WAITING_APPROVAL 等);空 = 未知,客户端如实显示。
  @$pb.TagNumber(4)
  $core.String get status => $_getSZ(3);
  @$pb.TagNumber(4)
  set status($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearStatus() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get modelId => $_getSZ(4);
  @$pb.TagNumber(5)
  set modelId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasModelId() => $_has(4);
  @$pb.TagNumber(5)
  void clearModelId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get effort => $_getSZ(5);
  @$pb.TagNumber(6)
  set effort($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasEffort() => $_has(5);
  @$pb.TagNumber(6)
  void clearEffort() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get progress => $_getSZ(6);
  @$pb.TagNumber(7)
  set progress($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasProgress() => $_has(6);
  @$pb.TagNumber(7)
  void clearProgress() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get summary => $_getSZ(7);
  @$pb.TagNumber(8)
  set summary($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSummary() => $_has(7);
  @$pb.TagNumber(8)
  void clearSummary() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get error => $_getSZ(8);
  @$pb.TagNumber(9)
  set error($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasError() => $_has(8);
  @$pb.TagNumber(9)
  void clearError() => $_clearField(9);

  /// 该成员当前挂起的审批:复用既有 ApprovalRequest(带真实 thread_id/request_id),
  /// 手机端继续用 SubmitApproval 应答,不新增审批通道。旧 daemon 为空。
  @$pb.TagNumber(10)
  $pb.PbList<ApprovalRequest> get pendingApprovals => $_getList(9);

  @$pb.TagNumber(11)
  $core.String get workspace => $_getSZ(10);
  @$pb.TagNumber(11)
  set workspace($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasWorkspace() => $_has(10);
  @$pb.TagNumber(11)
  void clearWorkspace() => $_clearField(11);
}

/// WatchCollabSessionRequest 订阅单个协同会话的事件流。
class WatchCollabSessionRequest extends $pb.GeneratedMessage {
  factory WatchCollabSessionRequest({
    $core.String? sessionId,
    $fixnum.Int64? afterSequence,
  }) {
    final result = WatchCollabSessionRequest._();
    if (sessionId != null) result.sessionId = sessionId;
    if (afterSequence != null) result.afterSequence = afterSequence;
    return result;
  }

  WatchCollabSessionRequest._();

  factory WatchCollabSessionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchCollabSessionRequest()..mergeFromBuffer(data, registry);
  factory WatchCollabSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchCollabSessionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchCollabSessionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: WatchCollabSessionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aInt64(2, _omitFieldNames ? '' : 'afterSequence')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchCollabSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchCollabSessionRequest copyWith(
          void Function(WatchCollabSessionRequest) updates) =>
      super.copyWith((message) => updates(message as WatchCollabSessionRequest))
          as WatchCollabSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use WatchCollabSessionRequest() / WatchCollabSessionRequest.new instead')
  static WatchCollabSessionRequest create() => WatchCollabSessionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      WatchCollabSessionRequest._();
  @$core.override
  WatchCollabSessionRequest createEmptyInstance() =>
      WatchCollabSessionRequest._();
  @$core.pragma('dart2js:noInline')
  static WatchCollabSessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WatchCollabSessionRequest>(
          WatchCollabSessionRequest.$_createMessage);
  static WatchCollabSessionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  /// 续传水位:只回放 seq 更大的事件(与 GetThreadEventsSince 同一口径);
  /// 0 = 从头回放。超出保留窗口时由服务端改发一帧全量 snapshot 兜底。
  @$pb.TagNumber(2)
  $fixnum.Int64 get afterSequence => $_getI64(1);
  @$pb.TagNumber(2)
  set afterSequence($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAfterSequence() => $_has(1);
  @$pb.TagNumber(2)
  void clearAfterSequence() => $_clearField(2);
}

/// CollabSessionEvent 协同会话的一帧事件:要么是细粒度变化(kind + member),
/// 要么是整屏快照(snapshot)。sequence 全局单调递增,客户端记录最大值做续传。
class CollabSessionEvent extends $pb.GeneratedMessage {
  factory CollabSessionEvent({
    $fixnum.Int64? sequence,
    $core.String? sessionId,
    $core.String? memberThreadId,
    $core.String? kind,
    CollabSessionSnapshot? snapshot,
    CollabMemberSnapshot? member,
  }) {
    final result = CollabSessionEvent._();
    if (sequence != null) result.sequence = sequence;
    if (sessionId != null) result.sessionId = sessionId;
    if (memberThreadId != null) result.memberThreadId = memberThreadId;
    if (kind != null) result.kind = kind;
    if (snapshot != null) result.snapshot = snapshot;
    if (member != null) result.member = member;
    return result;
  }

  CollabSessionEvent._();

  factory CollabSessionEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabSessionEvent()..mergeFromBuffer(data, registry);
  factory CollabSessionEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabSessionEvent()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CollabSessionEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CollabSessionEvent.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'sequence')
    ..aOS(2, _omitFieldNames ? '' : 'sessionId')
    ..aOS(3, _omitFieldNames ? '' : 'memberThreadId')
    ..aOS(4, _omitFieldNames ? '' : 'kind')
    ..aOM<CollabSessionSnapshot>(5, _omitFieldNames ? '' : 'snapshot',
        subBuilder: CollabSessionSnapshot.$_createMessage)
    ..aOM<CollabMemberSnapshot>(6, _omitFieldNames ? '' : 'member',
        subBuilder: CollabMemberSnapshot.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabSessionEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabSessionEvent copyWith(void Function(CollabSessionEvent) updates) =>
      super.copyWith((message) => updates(message as CollabSessionEvent))
          as CollabSessionEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CollabSessionEvent() / CollabSessionEvent.new instead')
  static CollabSessionEvent create() => CollabSessionEvent._();
  static $pb.GeneratedMessage $_createMessage() => CollabSessionEvent._();
  @$core.override
  CollabSessionEvent createEmptyInstance() => CollabSessionEvent._();
  @$core.pragma('dart2js:noInline')
  static CollabSessionEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CollabSessionEvent>(
          CollabSessionEvent.$_createMessage);
  static CollabSessionEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get sequence => $_getI64(0);
  @$pb.TagNumber(1)
  set sequence($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSequence() => $_has(0);
  @$pb.TagNumber(1)
  void clearSequence() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sessionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sessionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSessionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSessionId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get memberThreadId => $_getSZ(2);
  @$pb.TagNumber(3)
  set memberThreadId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMemberThreadId() => $_has(2);
  @$pb.TagNumber(3)
  void clearMemberThreadId() => $_clearField(3);

  /// 事件种类:string 口径(ACCEPTED / RUNNING / COMPLETED / FAILED /
  /// CANCELLED / WAITING_APPROVAL / SNAPSHOT 等);客户端遇到不认识的取值
  /// 必须按 snapshot 兜底刷新,不得报错。
  @$pb.TagNumber(4)
  $core.String get kind => $_getSZ(3);
  @$pb.TagNumber(4)
  set kind($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasKind() => $_has(3);
  @$pb.TagNumber(4)
  void clearKind() => $_clearField(4);

  /// 整屏快照:重连恢复 / 续传超窗 / 客户端不认识 kind 时的兜底真值。
  @$pb.TagNumber(5)
  CollabSessionSnapshot get snapshot => $_getN(4);
  @$pb.TagNumber(5)
  set snapshot(CollabSessionSnapshot value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasSnapshot() => $_has(4);
  @$pb.TagNumber(5)
  void clearSnapshot() => $_clearField(5);
  @$pb.TagNumber(5)
  CollabSessionSnapshot ensureSnapshot() => $_ensure(4);

  /// 单成员变化:与 member_thread_id 对应的成员卡片(整卡真值,非 diff)。
  @$pb.TagNumber(6)
  CollabMemberSnapshot get member => $_getN(5);
  @$pb.TagNumber(6)
  set member(CollabMemberSnapshot value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasMember() => $_has(5);
  @$pb.TagNumber(6)
  void clearMember() => $_clearField(6);
  @$pb.TagNumber(6)
  CollabMemberSnapshot ensureMember() => $_ensure(5);
}

/// GetCollabSessionRequest 取单个协同会话快照。
class GetCollabSessionRequest extends $pb.GeneratedMessage {
  factory GetCollabSessionRequest({
    $core.String? sessionId,
  }) {
    final result = GetCollabSessionRequest._();
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  GetCollabSessionRequest._();

  factory GetCollabSessionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCollabSessionRequest()..mergeFromBuffer(data, registry);
  factory GetCollabSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCollabSessionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCollabSessionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetCollabSessionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCollabSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCollabSessionRequest copyWith(
          void Function(GetCollabSessionRequest) updates) =>
      super.copyWith((message) => updates(message as GetCollabSessionRequest))
          as GetCollabSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetCollabSessionRequest() / GetCollabSessionRequest.new instead')
  static GetCollabSessionRequest create() => GetCollabSessionRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetCollabSessionRequest._();
  @$core.override
  GetCollabSessionRequest createEmptyInstance() => GetCollabSessionRequest._();
  @$core.pragma('dart2js:noInline')
  static GetCollabSessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCollabSessionRequest>(
          GetCollabSessionRequest.$_createMessage);
  static GetCollabSessionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);
}

/// ListCollabSessionsRequest 列出协同会话(最简全量形态,不做分页 ——
/// 协同会话量级远小于普通会话,分页只会让手机端多一轮接线)。
class ListCollabSessionsRequest extends $pb.GeneratedMessage {
  factory ListCollabSessionsRequest({
    $core.bool? includeFinished,
  }) {
    final result = ListCollabSessionsRequest._();
    if (includeFinished != null) result.includeFinished = includeFinished;
    return result;
  }

  ListCollabSessionsRequest._();

  factory ListCollabSessionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabSessionsRequest()..mergeFromBuffer(data, registry);
  factory ListCollabSessionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabSessionsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCollabSessionsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCollabSessionsRequest.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'includeFinished')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabSessionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabSessionsRequest copyWith(
          void Function(ListCollabSessionsRequest) updates) =>
      super.copyWith((message) => updates(message as ListCollabSessionsRequest))
          as ListCollabSessionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCollabSessionsRequest() / ListCollabSessionsRequest.new instead')
  static ListCollabSessionsRequest create() => ListCollabSessionsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListCollabSessionsRequest._();
  @$core.override
  ListCollabSessionsRequest createEmptyInstance() =>
      ListCollabSessionsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCollabSessionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCollabSessionsRequest>(
          ListCollabSessionsRequest.$_createMessage);
  static ListCollabSessionsRequest? _defaultInstance;

  /// true = 连已结束的(completed / failed / cancelled)一起返回;
  /// false = 只看进行中的。
  @$pb.TagNumber(1)
  $core.bool get includeFinished => $_getBF(0);
  @$pb.TagNumber(1)
  set includeFinished($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasIncludeFinished() => $_has(0);
  @$pb.TagNumber(1)
  void clearIncludeFinished() => $_clearField(1);
}

class ListCollabSessionsResponse extends $pb.GeneratedMessage {
  factory ListCollabSessionsResponse({
    $core.Iterable<CollabSessionSnapshot>? sessions,
  }) {
    final result = ListCollabSessionsResponse._();
    if (sessions != null) result.sessions.addAll(sessions);
    return result;
  }

  ListCollabSessionsResponse._();

  factory ListCollabSessionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabSessionsResponse()..mergeFromBuffer(data, registry);
  factory ListCollabSessionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCollabSessionsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCollabSessionsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCollabSessionsResponse.$_createMessage)
    ..pPM<CollabSessionSnapshot>(1, _omitFieldNames ? '' : 'sessions',
        subBuilder: CollabSessionSnapshot.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabSessionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCollabSessionsResponse copyWith(
          void Function(ListCollabSessionsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListCollabSessionsResponse))
          as ListCollabSessionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCollabSessionsResponse() / ListCollabSessionsResponse.new instead')
  static ListCollabSessionsResponse create() => ListCollabSessionsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListCollabSessionsResponse._();
  @$core.override
  ListCollabSessionsResponse createEmptyInstance() =>
      ListCollabSessionsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCollabSessionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCollabSessionsResponse>(
          ListCollabSessionsResponse.$_createMessage);
  static ListCollabSessionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<CollabSessionSnapshot> get sessions => $_getList(0);
}

class CancelCollabSessionRequest extends $pb.GeneratedMessage {
  factory CancelCollabSessionRequest({
    $core.String? sessionId,
  }) {
    final result = CancelCollabSessionRequest._();
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  CancelCollabSessionRequest._();

  factory CancelCollabSessionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelCollabSessionRequest()..mergeFromBuffer(data, registry);
  factory CancelCollabSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelCollabSessionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelCollabSessionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CancelCollabSessionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelCollabSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelCollabSessionRequest copyWith(
          void Function(CancelCollabSessionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CancelCollabSessionRequest))
          as CancelCollabSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CancelCollabSessionRequest() / CancelCollabSessionRequest.new instead')
  static CancelCollabSessionRequest create() => CancelCollabSessionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      CancelCollabSessionRequest._();
  @$core.override
  CancelCollabSessionRequest createEmptyInstance() =>
      CancelCollabSessionRequest._();
  @$core.pragma('dart2js:noInline')
  static CancelCollabSessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelCollabSessionRequest>(
          CancelCollabSessionRequest.$_createMessage);
  static CancelCollabSessionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);
}

class CancelCollabMemberRequest extends $pb.GeneratedMessage {
  factory CancelCollabMemberRequest({
    $core.String? sessionId,
    $core.String? threadId,
  }) {
    final result = CancelCollabMemberRequest._();
    if (sessionId != null) result.sessionId = sessionId;
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  CancelCollabMemberRequest._();

  factory CancelCollabMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelCollabMemberRequest()..mergeFromBuffer(data, registry);
  factory CancelCollabMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CancelCollabMemberRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelCollabMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CancelCollabMemberRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelCollabMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelCollabMemberRequest copyWith(
          void Function(CancelCollabMemberRequest) updates) =>
      super.copyWith((message) => updates(message as CancelCollabMemberRequest))
          as CancelCollabMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CancelCollabMemberRequest() / CancelCollabMemberRequest.new instead')
  static CancelCollabMemberRequest create() => CancelCollabMemberRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      CancelCollabMemberRequest._();
  @$core.override
  CancelCollabMemberRequest createEmptyInstance() =>
      CancelCollabMemberRequest._();
  @$core.pragma('dart2js:noInline')
  static CancelCollabMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelCollabMemberRequest>(
          CancelCollabMemberRequest.$_createMessage);
  static CancelCollabMemberRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get threadId => $_getSZ(1);
  @$pb.TagNumber(2)
  set threadId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasThreadId() => $_has(1);
  @$pb.TagNumber(2)
  void clearThreadId() => $_clearField(2);
}

class RetryCollabMemberRequest extends $pb.GeneratedMessage {
  factory RetryCollabMemberRequest({
    $core.String? sessionId,
    $core.String? threadId,
  }) {
    final result = RetryCollabMemberRequest._();
    if (sessionId != null) result.sessionId = sessionId;
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  RetryCollabMemberRequest._();

  factory RetryCollabMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RetryCollabMemberRequest()..mergeFromBuffer(data, registry);
  factory RetryCollabMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RetryCollabMemberRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RetryCollabMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RetryCollabMemberRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RetryCollabMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RetryCollabMemberRequest copyWith(
          void Function(RetryCollabMemberRequest) updates) =>
      super.copyWith((message) => updates(message as RetryCollabMemberRequest))
          as RetryCollabMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RetryCollabMemberRequest() / RetryCollabMemberRequest.new instead')
  static RetryCollabMemberRequest create() => RetryCollabMemberRequest._();
  static $pb.GeneratedMessage $_createMessage() => RetryCollabMemberRequest._();
  @$core.override
  RetryCollabMemberRequest createEmptyInstance() =>
      RetryCollabMemberRequest._();
  @$core.pragma('dart2js:noInline')
  static RetryCollabMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RetryCollabMemberRequest>(
          RetryCollabMemberRequest.$_createMessage);
  static RetryCollabMemberRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get threadId => $_getSZ(1);
  @$pb.TagNumber(2)
  set threadId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasThreadId() => $_has(1);
  @$pb.TagNumber(2)
  void clearThreadId() => $_clearField(2);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
