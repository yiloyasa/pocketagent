// This is a generated file - do not edit.
//
// Generated from v1/runtime.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class GetRuntimePolicyRequest extends $pb.GeneratedMessage {
  factory GetRuntimePolicyRequest() => GetRuntimePolicyRequest._();

  GetRuntimePolicyRequest._();

  factory GetRuntimePolicyRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimePolicyRequest()..mergeFromBuffer(data, registry);
  factory GetRuntimePolicyRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimePolicyRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetRuntimePolicyRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetRuntimePolicyRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimePolicyRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimePolicyRequest copyWith(
          void Function(GetRuntimePolicyRequest) updates) =>
      super.copyWith((message) => updates(message as GetRuntimePolicyRequest))
          as GetRuntimePolicyRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetRuntimePolicyRequest() / GetRuntimePolicyRequest.new instead')
  static GetRuntimePolicyRequest create() => GetRuntimePolicyRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetRuntimePolicyRequest._();
  @$core.override
  GetRuntimePolicyRequest createEmptyInstance() => GetRuntimePolicyRequest._();
  @$core.pragma('dart2js:noInline')
  static GetRuntimePolicyRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetRuntimePolicyRequest>(
          GetRuntimePolicyRequest.$_createMessage);
  static GetRuntimePolicyRequest? _defaultInstance;
}

class SetRuntimePolicyRequest extends $pb.GeneratedMessage {
  factory SetRuntimePolicyRequest({
    RuntimePolicy? policy,
    $fixnum.Int64? expectedRevision,
  }) {
    final result = SetRuntimePolicyRequest._();
    if (policy != null) result.policy = policy;
    if (expectedRevision != null) result.expectedRevision = expectedRevision;
    return result;
  }

  SetRuntimePolicyRequest._();

  factory SetRuntimePolicyRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetRuntimePolicyRequest()..mergeFromBuffer(data, registry);
  factory SetRuntimePolicyRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetRuntimePolicyRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetRuntimePolicyRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SetRuntimePolicyRequest.$_createMessage)
    ..aOM<RuntimePolicy>(1, _omitFieldNames ? '' : 'policy',
        subBuilder: RuntimePolicy.$_createMessage)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'expectedRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetRuntimePolicyRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetRuntimePolicyRequest copyWith(
          void Function(SetRuntimePolicyRequest) updates) =>
      super.copyWith((message) => updates(message as SetRuntimePolicyRequest))
          as SetRuntimePolicyRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetRuntimePolicyRequest() / SetRuntimePolicyRequest.new instead')
  static SetRuntimePolicyRequest create() => SetRuntimePolicyRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetRuntimePolicyRequest._();
  @$core.override
  SetRuntimePolicyRequest createEmptyInstance() => SetRuntimePolicyRequest._();
  @$core.pragma('dart2js:noInline')
  static SetRuntimePolicyRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetRuntimePolicyRequest>(
          SetRuntimePolicyRequest.$_createMessage);
  static SetRuntimePolicyRequest? _defaultInstance;

  /// 【提交的是**完整策略**,不是补丁】缺席的项 = 未设置(沿用上一级/内置基线),
  /// 不是"不改这一项"。客户端改单项时请先 Get、在拿到的完整对象上改、再 Set
  /// (并带上 revision 做 CAS)。参见 SetCollabPolicyRequest 的同款说明。
  @$pb.TagNumber(1)
  RuntimePolicy get policy => $_getN(0);
  @$pb.TagNumber(1)
  set policy(RuntimePolicy value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPolicy() => $_has(0);
  @$pb.TagNumber(1)
  void clearPolicy() => $_clearField(1);
  @$pb.TagNumber(1)
  RuntimePolicy ensurePolicy() => $_ensure(0);

  /// 期望的当前 revision;0 = 不做 CAS。
  /// 【为什么需要 CAS】策略可能被两处同时改(手机设置页、电脑上直接编辑文件)。
  /// 没有它,后提交的一方会静默覆盖另一方 —— 用户看到的是"我明明改过"。
  @$pb.TagNumber(2)
  $fixnum.Int64 get expectedRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set expectedRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExpectedRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpectedRevision() => $_clearField(2);
}

class RuntimePolicyResponse extends $pb.GeneratedMessage {
  factory RuntimePolicyResponse({
    RuntimePolicy? policy,
    $fixnum.Int64? revision,
    $core.String? policyPath,
    RuntimeLimits? effectiveGlobal,
  }) {
    final result = RuntimePolicyResponse._();
    if (policy != null) result.policy = policy;
    if (revision != null) result.revision = revision;
    if (policyPath != null) result.policyPath = policyPath;
    if (effectiveGlobal != null) result.effectiveGlobal = effectiveGlobal;
    return result;
  }

  RuntimePolicyResponse._();

  factory RuntimePolicyResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimePolicyResponse()..mergeFromBuffer(data, registry);
  factory RuntimePolicyResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimePolicyResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RuntimePolicyResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RuntimePolicyResponse.$_createMessage)
    ..aOM<RuntimePolicy>(1, _omitFieldNames ? '' : 'policy',
        subBuilder: RuntimePolicy.$_createMessage)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(3, _omitFieldNames ? '' : 'policyPath')
    ..aOM<RuntimeLimits>(4, _omitFieldNames ? '' : 'effectiveGlobal',
        subBuilder: RuntimeLimits.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimePolicyResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimePolicyResponse copyWith(
          void Function(RuntimePolicyResponse) updates) =>
      super.copyWith((message) => updates(message as RuntimePolicyResponse))
          as RuntimePolicyResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RuntimePolicyResponse() / RuntimePolicyResponse.new instead')
  static RuntimePolicyResponse create() => RuntimePolicyResponse._();
  static $pb.GeneratedMessage $_createMessage() => RuntimePolicyResponse._();
  @$core.override
  RuntimePolicyResponse createEmptyInstance() => RuntimePolicyResponse._();
  @$core.pragma('dart2js:noInline')
  static RuntimePolicyResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RuntimePolicyResponse>(
          RuntimePolicyResponse.$_createMessage);
  static RuntimePolicyResponse? _defaultInstance;

  @$pb.TagNumber(1)
  RuntimePolicy get policy => $_getN(0);
  @$pb.TagNumber(1)
  set policy(RuntimePolicy value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPolicy() => $_has(0);
  @$pb.TagNumber(1)
  void clearPolicy() => $_clearField(1);
  @$pb.TagNumber(1)
  RuntimePolicy ensurePolicy() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get revision => $_getI64(1);
  @$pb.TagNumber(2)
  set revision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get policyPath => $_getSZ(2);
  @$pb.TagNumber(3)
  set policyPath($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPolicyPath() => $_has(2);
  @$pb.TagNumber(3)
  void clearPolicyPath() => $_clearField(3);

  /// effective_global 是"内置基线 + global 覆盖"之后的**实际生效值**(全字段齐备)。
  ///
  /// 【为什么必须单独给,而不是让客户端拿 policy 自己判】policy 是一个带缺席字段的
  /// 部分对象:用户只改了一项时,其余项在线上就是 0。客户端很容易把那个 0 显示成
  /// "封禁了这一类资源"或"未知" —— 而真实生效的是内置基线(例如 max_parallel_turns=3)。
  /// 客户端手里没有"内置基线"这份数据,合并只能由 daemon 做;把结果直接给出去,
  /// 才做得到"看到的就是生效的"。
  @$pb.TagNumber(4)
  RuntimeLimits get effectiveGlobal => $_getN(3);
  @$pb.TagNumber(4)
  set effectiveGlobal(RuntimeLimits value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasEffectiveGlobal() => $_has(3);
  @$pb.TagNumber(4)
  void clearEffectiveGlobal() => $_clearField(4);
  @$pb.TagNumber(4)
  RuntimeLimits ensureEffectiveGlobal() => $_ensure(3);
}

/// RuntimeLimits 是某一级(全局 / agent / 工作区 / 会话 / 协同)上的额度。
///
/// 【为什么每一项都是 optional】proto3 的零值分不清"这一项没设"与"显式设成 0",
/// 而这两件事在预算里必须分开表达:未设 = 继承上一级;显式 0 = 明令封禁该类资源
/// (与 daemon 侧 runtime.Limits 的指针语义一一对应)。混在一起的表现是:
/// 用户明明关了并发,系统却继承了上一级的 3 继续并行。
class RuntimeLimits extends $pb.GeneratedMessage {
  factory RuntimeLimits({
    $fixnum.Int64? ramWorkingBytes,
    $fixnum.Int64? ramEventBufferBytes,
    $fixnum.Int64? ramSessionIndexBytes,
    $fixnum.Int64? diskConversationBytes,
    $fixnum.Int64? diskCacheBytes,
    $fixnum.Int64? diskArtifactBytes,
    $core.int? maxParallelTurns,
  }) {
    final result = RuntimeLimits._();
    if (ramWorkingBytes != null) result.ramWorkingBytes = ramWorkingBytes;
    if (ramEventBufferBytes != null)
      result.ramEventBufferBytes = ramEventBufferBytes;
    if (ramSessionIndexBytes != null)
      result.ramSessionIndexBytes = ramSessionIndexBytes;
    if (diskConversationBytes != null)
      result.diskConversationBytes = diskConversationBytes;
    if (diskCacheBytes != null) result.diskCacheBytes = diskCacheBytes;
    if (diskArtifactBytes != null) result.diskArtifactBytes = diskArtifactBytes;
    if (maxParallelTurns != null) result.maxParallelTurns = maxParallelTurns;
    return result;
  }

  RuntimeLimits._();

  factory RuntimeLimits.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeLimits()..mergeFromBuffer(data, registry);
  factory RuntimeLimits.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeLimits()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RuntimeLimits',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RuntimeLimits.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'ramWorkingBytes')
    ..aInt64(2, _omitFieldNames ? '' : 'ramEventBufferBytes')
    ..aInt64(3, _omitFieldNames ? '' : 'ramSessionIndexBytes')
    ..aInt64(4, _omitFieldNames ? '' : 'diskConversationBytes')
    ..aInt64(5, _omitFieldNames ? '' : 'diskCacheBytes')
    ..aInt64(6, _omitFieldNames ? '' : 'diskArtifactBytes')
    ..aI(7, _omitFieldNames ? '' : 'maxParallelTurns')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeLimits clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeLimits copyWith(void Function(RuntimeLimits) updates) =>
      super.copyWith((message) => updates(message as RuntimeLimits))
          as RuntimeLimits;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RuntimeLimits() / RuntimeLimits.new instead')
  static RuntimeLimits create() => RuntimeLimits._();
  static $pb.GeneratedMessage $_createMessage() => RuntimeLimits._();
  @$core.override
  RuntimeLimits createEmptyInstance() => RuntimeLimits._();
  @$core.pragma('dart2js:noInline')
  static RuntimeLimits getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RuntimeLimits>(
          RuntimeLimits.$_createMessage);
  static RuntimeLimits? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get ramWorkingBytes => $_getI64(0);
  @$pb.TagNumber(1)
  set ramWorkingBytes($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRamWorkingBytes() => $_has(0);
  @$pb.TagNumber(1)
  void clearRamWorkingBytes() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get ramEventBufferBytes => $_getI64(1);
  @$pb.TagNumber(2)
  set ramEventBufferBytes($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRamEventBufferBytes() => $_has(1);
  @$pb.TagNumber(2)
  void clearRamEventBufferBytes() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get ramSessionIndexBytes => $_getI64(2);
  @$pb.TagNumber(3)
  set ramSessionIndexBytes($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRamSessionIndexBytes() => $_has(2);
  @$pb.TagNumber(3)
  void clearRamSessionIndexBytes() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get diskConversationBytes => $_getI64(3);
  @$pb.TagNumber(4)
  set diskConversationBytes($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDiskConversationBytes() => $_has(3);
  @$pb.TagNumber(4)
  void clearDiskConversationBytes() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get diskCacheBytes => $_getI64(4);
  @$pb.TagNumber(5)
  set diskCacheBytes($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDiskCacheBytes() => $_has(4);
  @$pb.TagNumber(5)
  void clearDiskCacheBytes() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get diskArtifactBytes => $_getI64(5);
  @$pb.TagNumber(6)
  set diskArtifactBytes($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDiskArtifactBytes() => $_has(5);
  @$pb.TagNumber(6)
  void clearDiskArtifactBytes() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.int get maxParallelTurns => $_getIZ(6);
  @$pb.TagNumber(7)
  set maxParallelTurns($core.int value) => $_setSignedInt32(6, value);
  @$pb.TagNumber(7)
  $core.bool hasMaxParallelTurns() => $_has(6);
  @$pb.TagNumber(7)
  void clearMaxParallelTurns() => $_clearField(7);
}

class RuntimePolicy extends $pb.GeneratedMessage {
  factory RuntimePolicy({
    RuntimeLimits? global,
    $core.Iterable<$core.MapEntry<$core.String, RuntimeLimits>>? agents,
    $core.Iterable<$core.MapEntry<$core.String, RuntimeLimits>>? workspaces,
    $core.Iterable<$core.MapEntry<$core.String, RuntimeLimits>>? threads,
    $core.Iterable<$core.MapEntry<$core.String, RuntimeLimits>>? collabs,
    $core.bool? unknownRequestPassthrough,
  }) {
    final result = RuntimePolicy._();
    if (global != null) result.global = global;
    if (agents != null) result.agents.addEntries(agents);
    if (workspaces != null) result.workspaces.addEntries(workspaces);
    if (threads != null) result.threads.addEntries(threads);
    if (collabs != null) result.collabs.addEntries(collabs);
    if (unknownRequestPassthrough != null)
      result.unknownRequestPassthrough = unknownRequestPassthrough;
    return result;
  }

  RuntimePolicy._();

  factory RuntimePolicy.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimePolicy()..mergeFromBuffer(data, registry);
  factory RuntimePolicy.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimePolicy()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RuntimePolicy',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RuntimePolicy.$_createMessage)
    ..aOM<RuntimeLimits>(1, _omitFieldNames ? '' : 'global',
        subBuilder: RuntimeLimits.$_createMessage)
    ..m<$core.String, RuntimeLimits>(2, _omitFieldNames ? '' : 'agents',
        entryClassName: 'RuntimePolicy.AgentsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: RuntimeLimits.$_createMessage,
        valueDefaultOrMaker: RuntimeLimits.getDefault,
        packageName: const $pb.PackageName('pocketagent.v1'))
    ..m<$core.String, RuntimeLimits>(3, _omitFieldNames ? '' : 'workspaces',
        entryClassName: 'RuntimePolicy.WorkspacesEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: RuntimeLimits.$_createMessage,
        valueDefaultOrMaker: RuntimeLimits.getDefault,
        packageName: const $pb.PackageName('pocketagent.v1'))
    ..m<$core.String, RuntimeLimits>(4, _omitFieldNames ? '' : 'threads',
        entryClassName: 'RuntimePolicy.ThreadsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: RuntimeLimits.$_createMessage,
        valueDefaultOrMaker: RuntimeLimits.getDefault,
        packageName: const $pb.PackageName('pocketagent.v1'))
    ..m<$core.String, RuntimeLimits>(5, _omitFieldNames ? '' : 'collabs',
        entryClassName: 'RuntimePolicy.CollabsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: RuntimeLimits.$_createMessage,
        valueDefaultOrMaker: RuntimeLimits.getDefault,
        packageName: const $pb.PackageName('pocketagent.v1'))
    ..aOB(6, _omitFieldNames ? '' : 'unknownRequestPassthrough')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimePolicy clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimePolicy copyWith(void Function(RuntimePolicy) updates) =>
      super.copyWith((message) => updates(message as RuntimePolicy))
          as RuntimePolicy;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RuntimePolicy() / RuntimePolicy.new instead')
  static RuntimePolicy create() => RuntimePolicy._();
  static $pb.GeneratedMessage $_createMessage() => RuntimePolicy._();
  @$core.override
  RuntimePolicy createEmptyInstance() => RuntimePolicy._();
  @$core.pragma('dart2js:noInline')
  static RuntimePolicy getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RuntimePolicy>(
          RuntimePolicy.$_createMessage);
  static RuntimePolicy? _defaultInstance;

  @$pb.TagNumber(1)
  RuntimeLimits get global => $_getN(0);
  @$pb.TagNumber(1)
  set global(RuntimeLimits value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGlobal() => $_has(0);
  @$pb.TagNumber(1)
  void clearGlobal() => $_clearField(1);
  @$pb.TagNumber(1)
  RuntimeLimits ensureGlobal() => $_ensure(0);

  @$pb.TagNumber(2)
  $pb.PbMap<$core.String, RuntimeLimits> get agents => $_getMap(1);

  @$pb.TagNumber(3)
  $pb.PbMap<$core.String, RuntimeLimits> get workspaces => $_getMap(2);

  @$pb.TagNumber(4)
  $pb.PbMap<$core.String, RuntimeLimits> get threads => $_getMap(3);

  @$pb.TagNumber(5)
  $pb.PbMap<$core.String, RuntimeLimits> get collabs => $_getMap(4);

  /// 【辅线 B】未知反向请求透传确认(全局开关,默认缺 = 关)。
  ///
  /// 【为什么是 optional】关是安全姿态:agent 发来我们不认识的反向请求时,缺省直接
  /// 回协议错误 -32601(绝不替用户做任何决定)。用户显式打开后,daemon 才会把未知
  /// 请求的脱敏原文弹成手机确认卡,由用户逐次决定"允许(回空对象 {})/拒绝"。
  /// optional 保住的是"用户从没碰过这个开关"这一事实 —— 设置页据此显示默认态,
  /// 而不是把"没设"渲染成"用户选过关"。
  @$pb.TagNumber(6)
  $core.bool get unknownRequestPassthrough => $_getBF(5);
  @$pb.TagNumber(6)
  set unknownRequestPassthrough($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasUnknownRequestPassthrough() => $_has(5);
  @$pb.TagNumber(6)
  void clearUnknownRequestPassthrough() => $_clearField(6);
}

class GetResourceUsageRequest extends $pb.GeneratedMessage {
  factory GetResourceUsageRequest({
    $core.String? scope,
  }) {
    final result = GetResourceUsageRequest._();
    if (scope != null) result.scope = scope;
    return result;
  }

  GetResourceUsageRequest._();

  factory GetResourceUsageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetResourceUsageRequest()..mergeFromBuffer(data, registry);
  factory GetResourceUsageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetResourceUsageRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetResourceUsageRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetResourceUsageRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'scope')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetResourceUsageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetResourceUsageRequest copyWith(
          void Function(GetResourceUsageRequest) updates) =>
      super.copyWith((message) => updates(message as GetResourceUsageRequest))
          as GetResourceUsageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetResourceUsageRequest() / GetResourceUsageRequest.new instead')
  static GetResourceUsageRequest create() => GetResourceUsageRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetResourceUsageRequest._();
  @$core.override
  GetResourceUsageRequest createEmptyInstance() => GetResourceUsageRequest._();
  @$core.pragma('dart2js:noInline')
  static GetResourceUsageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetResourceUsageRequest>(
          GetResourceUsageRequest.$_createMessage);
  static GetResourceUsageRequest? _defaultInstance;

  /// 作用域:"" = 全部 / global / agent:<id> / workspace:<路径> / thread:<id> / collab:<id>。
  /// 词汇表与 daemon 侧 runtime.Scope* 构造函数一致 —— 各处自己拼字符串必然拼出
  /// "agent/echo" 与 "agent:echo" 两种写法,然后被当成两个 scope 各记一份账。
  @$pb.TagNumber(1)
  $core.String get scope => $_getSZ(0);
  @$pb.TagNumber(1)
  set scope($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasScope() => $_has(0);
  @$pb.TagNumber(1)
  void clearScope() => $_clearField(1);
}

class ResourceUsageResponse extends $pb.GeneratedMessage {
  factory ResourceUsageResponse({
    $core.Iterable<ResourceMetric>? metrics,
    $fixnum.Int64? sampledAtUnixMs,
  }) {
    final result = ResourceUsageResponse._();
    if (metrics != null) result.metrics.addAll(metrics);
    if (sampledAtUnixMs != null) result.sampledAtUnixMs = sampledAtUnixMs;
    return result;
  }

  ResourceUsageResponse._();

  factory ResourceUsageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResourceUsageResponse()..mergeFromBuffer(data, registry);
  factory ResourceUsageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResourceUsageResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResourceUsageResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ResourceUsageResponse.$_createMessage)
    ..pPM<ResourceMetric>(1, _omitFieldNames ? '' : 'metrics',
        subBuilder: ResourceMetric.$_createMessage)
    ..aInt64(2, _omitFieldNames ? '' : 'sampledAtUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResourceUsageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResourceUsageResponse copyWith(
          void Function(ResourceUsageResponse) updates) =>
      super.copyWith((message) => updates(message as ResourceUsageResponse))
          as ResourceUsageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ResourceUsageResponse() / ResourceUsageResponse.new instead')
  static ResourceUsageResponse create() => ResourceUsageResponse._();
  static $pb.GeneratedMessage $_createMessage() => ResourceUsageResponse._();
  @$core.override
  ResourceUsageResponse createEmptyInstance() => ResourceUsageResponse._();
  @$core.pragma('dart2js:noInline')
  static ResourceUsageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResourceUsageResponse>(
          ResourceUsageResponse.$_createMessage);
  static ResourceUsageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ResourceMetric> get metrics => $_getList(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get sampledAtUnixMs => $_getI64(1);
  @$pb.TagNumber(2)
  set sampledAtUnixMs($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSampledAtUnixMs() => $_has(1);
  @$pb.TagNumber(2)
  void clearSampledAtUnixMs() => $_clearField(2);
}

/// ResourceMetric 是一条资源租约的**观察值**:额度与用量一起给。
///
/// 【为什么必须带上限额】只给用量,"3 个并发"到底是"上限 3"还是"上限 30"看不出来,
/// 用户也就无法判断该不该调;而限额的来历(哪一级给的)由 GetRuntimePolicy 回答。
class ResourceMetric extends $pb.GeneratedMessage {
  factory ResourceMetric({
    $core.String? leaseId,
    $core.String? scope,
    $core.String? kind,
    $fixnum.Int64? reservedBytes,
    $fixnum.Int64? usedBytes,
    $fixnum.Int64? softLimitBytes,
    $fixnum.Int64? hardLimitBytes,
    $core.String? state,
    $core.String? spillPolicy,
    $core.bool? optional,
  }) {
    final result = ResourceMetric._();
    if (leaseId != null) result.leaseId = leaseId;
    if (scope != null) result.scope = scope;
    if (kind != null) result.kind = kind;
    if (reservedBytes != null) result.reservedBytes = reservedBytes;
    if (usedBytes != null) result.usedBytes = usedBytes;
    if (softLimitBytes != null) result.softLimitBytes = softLimitBytes;
    if (hardLimitBytes != null) result.hardLimitBytes = hardLimitBytes;
    if (state != null) result.state = state;
    if (spillPolicy != null) result.spillPolicy = spillPolicy;
    if (optional != null) result.optional = optional;
    return result;
  }

  ResourceMetric._();

  factory ResourceMetric.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResourceMetric()..mergeFromBuffer(data, registry);
  factory ResourceMetric.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ResourceMetric()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResourceMetric',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ResourceMetric.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'leaseId')
    ..aOS(2, _omitFieldNames ? '' : 'scope')
    ..aOS(3, _omitFieldNames ? '' : 'kind')
    ..aInt64(4, _omitFieldNames ? '' : 'reservedBytes')
    ..aInt64(5, _omitFieldNames ? '' : 'usedBytes')
    ..aInt64(6, _omitFieldNames ? '' : 'softLimitBytes')
    ..aInt64(7, _omitFieldNames ? '' : 'hardLimitBytes')
    ..aOS(8, _omitFieldNames ? '' : 'state')
    ..aOS(9, _omitFieldNames ? '' : 'spillPolicy')
    ..aOB(10, _omitFieldNames ? '' : 'optional')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResourceMetric clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResourceMetric copyWith(void Function(ResourceMetric) updates) =>
      super.copyWith((message) => updates(message as ResourceMetric))
          as ResourceMetric;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ResourceMetric() / ResourceMetric.new instead')
  static ResourceMetric create() => ResourceMetric._();
  static $pb.GeneratedMessage $_createMessage() => ResourceMetric._();
  @$core.override
  ResourceMetric createEmptyInstance() => ResourceMetric._();
  @$core.pragma('dart2js:noInline')
  static ResourceMetric getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ResourceMetric>(
          ResourceMetric.$_createMessage);
  static ResourceMetric? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get leaseId => $_getSZ(0);
  @$pb.TagNumber(1)
  set leaseId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasLeaseId() => $_has(0);
  @$pb.TagNumber(1)
  void clearLeaseId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get scope => $_getSZ(1);
  @$pb.TagNumber(2)
  set scope($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasScope() => $_has(1);
  @$pb.TagNumber(2)
  void clearScope() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get kind => $_getSZ(2);
  @$pb.TagNumber(3)
  set kind($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasKind() => $_has(2);
  @$pb.TagNumber(3)
  void clearKind() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get reservedBytes => $_getI64(3);
  @$pb.TagNumber(4)
  set reservedBytes($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReservedBytes() => $_has(3);
  @$pb.TagNumber(4)
  void clearReservedBytes() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get usedBytes => $_getI64(4);
  @$pb.TagNumber(5)
  set usedBytes($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasUsedBytes() => $_has(4);
  @$pb.TagNumber(5)
  void clearUsedBytes() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get softLimitBytes => $_getI64(5);
  @$pb.TagNumber(6)
  set softLimitBytes($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSoftLimitBytes() => $_has(5);
  @$pb.TagNumber(6)
  void clearSoftLimitBytes() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get hardLimitBytes => $_getI64(6);
  @$pb.TagNumber(7)
  set hardLimitBytes($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasHardLimitBytes() => $_has(6);
  @$pb.TagNumber(7)
  void clearHardLimitBytes() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get state => $_getSZ(7);
  @$pb.TagNumber(8)
  set state($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasState() => $_has(7);
  @$pb.TagNumber(8)
  void clearState() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get spillPolicy => $_getSZ(8);
  @$pb.TagNumber(9)
  set spillPolicy($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasSpillPolicy() => $_has(8);
  @$pb.TagNumber(9)
  void clearSpillPolicy() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.bool get optional => $_getBF(9);
  @$pb.TagNumber(10)
  set optional($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasOptional() => $_has(9);
  @$pb.TagNumber(10)
  void clearOptional() => $_clearField(10);
}

class ListUnknownEventsRequest extends $pb.GeneratedMessage {
  factory ListUnknownEventsRequest({
    $core.String? kind,
    $core.String? agentId,
    $core.String? threadId,
    $core.String? processingStatus,
    $core.int? limit,
  }) {
    final result = ListUnknownEventsRequest._();
    if (kind != null) result.kind = kind;
    if (agentId != null) result.agentId = agentId;
    if (threadId != null) result.threadId = threadId;
    if (processingStatus != null) result.processingStatus = processingStatus;
    if (limit != null) result.limit = limit;
    return result;
  }

  ListUnknownEventsRequest._();

  factory ListUnknownEventsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListUnknownEventsRequest()..mergeFromBuffer(data, registry);
  factory ListUnknownEventsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListUnknownEventsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListUnknownEventsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListUnknownEventsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'kind')
    ..aOS(2, _omitFieldNames ? '' : 'agentId')
    ..aOS(3, _omitFieldNames ? '' : 'threadId')
    ..aOS(4, _omitFieldNames ? '' : 'processingStatus')
    ..aI(5, _omitFieldNames ? '' : 'limit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListUnknownEventsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListUnknownEventsRequest copyWith(
          void Function(ListUnknownEventsRequest) updates) =>
      super.copyWith((message) => updates(message as ListUnknownEventsRequest))
          as ListUnknownEventsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListUnknownEventsRequest() / ListUnknownEventsRequest.new instead')
  static ListUnknownEventsRequest create() => ListUnknownEventsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListUnknownEventsRequest._();
  @$core.override
  ListUnknownEventsRequest createEmptyInstance() =>
      ListUnknownEventsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListUnknownEventsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListUnknownEventsRequest>(
          ListUnknownEventsRequest.$_createMessage);
  static ListUnknownEventsRequest? _defaultInstance;

  /// normalized_kind:UNKNOWN_NOTIFICATION / UNKNOWN_REQUEST / UNKNOWN_RESPONSE;空 = 全部。
  @$pb.TagNumber(1)
  $core.String get kind => $_getSZ(0);
  @$pb.TagNumber(1)
  set kind($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get agentId => $_getSZ(1);
  @$pb.TagNumber(2)
  set agentId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAgentId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAgentId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get threadId => $_getSZ(2);
  @$pb.TagNumber(3)
  set threadId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasThreadId() => $_has(2);
  @$pb.TagNumber(3)
  void clearThreadId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get processingStatus => $_getSZ(3);
  @$pb.TagNumber(4)
  set processingStatus($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProcessingStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearProcessingStatus() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get limit => $_getIZ(4);
  @$pb.TagNumber(5)
  set limit($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasLimit() => $_has(4);
  @$pb.TagNumber(5)
  void clearLimit() => $_clearField(5);
}

/// UserActionDescriptor 是 daemon 判定为**可以给用户看**的一个动作按钮
/// (2026-10-04 未知事件用户处理中心计划,只增不改)。
/// 【为什么由 daemon 定义而不是 Android 猜】未知事件能不能执行、要不要确认,
/// 是这台电脑的安全姿态;手机端只按本描述渲染按钮,绝不自行发明"允许"。
class UserActionDescriptor extends $pb.GeneratedMessage {
  factory UserActionDescriptor({
    $core.String? actionId,
    $core.String? label,
    $core.String? description,
    $core.bool? safe,
    $core.bool? requiresConfirmation,
    $core.bool? destructive,
  }) {
    final result = UserActionDescriptor._();
    if (actionId != null) result.actionId = actionId;
    if (label != null) result.label = label;
    if (description != null) result.description = description;
    if (safe != null) result.safe = safe;
    if (requiresConfirmation != null)
      result.requiresConfirmation = requiresConfirmation;
    if (destructive != null) result.destructive = destructive;
    return result;
  }

  UserActionDescriptor._();

  factory UserActionDescriptor.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UserActionDescriptor()..mergeFromBuffer(data, registry);
  factory UserActionDescriptor.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UserActionDescriptor()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserActionDescriptor',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: UserActionDescriptor.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'actionId')
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOB(4, _omitFieldNames ? '' : 'safe')
    ..aOB(5, _omitFieldNames ? '' : 'requiresConfirmation')
    ..aOB(6, _omitFieldNames ? '' : 'destructive')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserActionDescriptor clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserActionDescriptor copyWith(void Function(UserActionDescriptor) updates) =>
      super.copyWith((message) => updates(message as UserActionDescriptor))
          as UserActionDescriptor;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UserActionDescriptor() / UserActionDescriptor.new instead')
  static UserActionDescriptor create() => UserActionDescriptor._();
  static $pb.GeneratedMessage $_createMessage() => UserActionDescriptor._();
  @$core.override
  UserActionDescriptor createEmptyInstance() => UserActionDescriptor._();
  @$core.pragma('dart2js:noInline')
  static UserActionDescriptor getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserActionDescriptor>(
          UserActionDescriptor.$_createMessage);
  static UserActionDescriptor? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get actionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set actionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasActionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearActionId() => $_clearField(1);

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

  /// 安全声明:只有 safe=true 的动作才允许渲染成可点按钮。
  /// 【红线】UNKNOWN_REQUEST 默认不出现 safe=true 的"允许/执行"类动作。
  @$pb.TagNumber(4)
  $core.bool get safe => $_getBF(3);
  @$pb.TagNumber(4)
  set safe($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSafe() => $_has(3);
  @$pb.TagNumber(4)
  void clearSafe() => $_clearField(4);

  /// true = 执行前必须再向用户要一次确认(幂等动作也可能要求确认)。
  @$pb.TagNumber(5)
  $core.bool get requiresConfirmation => $_getBF(4);
  @$pb.TagNumber(5)
  set requiresConfirmation($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRequiresConfirmation() => $_has(4);
  @$pb.TagNumber(5)
  void clearRequiresConfirmation() => $_clearField(5);

  /// true = 该动作有破坏性(如丢弃数据);UI 必须按危险样式渲染。
  @$pb.TagNumber(6)
  $core.bool get destructive => $_getBF(5);
  @$pb.TagNumber(6)
  set destructive($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDestructive() => $_has(5);
  @$pb.TagNumber(6)
  void clearDestructive() => $_clearField(6);
}

/// UnknownEventSummary 是列表项。原始载荷只在 GetUnknownEvent 才给 —— 列表页不需要
/// 背着一堆可能很大的 payload。
class UnknownEventSummary extends $pb.GeneratedMessage {
  factory UnknownEventSummary({
    $core.String? eventId,
    $fixnum.Int64? sequence,
    $fixnum.Int64? receivedAtUnixMs,
    $core.String? sourceAgent,
    $core.String? sourceMethod,
    $core.String? threadId,
    $core.String? normalizedKind,
    $core.String? safetyClass,
    $core.String? processingStatus,
    $fixnum.Int64? revision,
    $core.String? summary,
    $core.String? plainTitle,
    $core.String? plainExplanation,
    $core.String? impact,
    $core.bool? requiresUser,
    $core.bool? canOpenNative,
    $core.Iterable<UserActionDescriptor>? actions,
  }) {
    final result = UnknownEventSummary._();
    if (eventId != null) result.eventId = eventId;
    if (sequence != null) result.sequence = sequence;
    if (receivedAtUnixMs != null) result.receivedAtUnixMs = receivedAtUnixMs;
    if (sourceAgent != null) result.sourceAgent = sourceAgent;
    if (sourceMethod != null) result.sourceMethod = sourceMethod;
    if (threadId != null) result.threadId = threadId;
    if (normalizedKind != null) result.normalizedKind = normalizedKind;
    if (safetyClass != null) result.safetyClass = safetyClass;
    if (processingStatus != null) result.processingStatus = processingStatus;
    if (revision != null) result.revision = revision;
    if (summary != null) result.summary = summary;
    if (plainTitle != null) result.plainTitle = plainTitle;
    if (plainExplanation != null) result.plainExplanation = plainExplanation;
    if (impact != null) result.impact = impact;
    if (requiresUser != null) result.requiresUser = requiresUser;
    if (canOpenNative != null) result.canOpenNative = canOpenNative;
    if (actions != null) result.actions.addAll(actions);
    return result;
  }

  UnknownEventSummary._();

  factory UnknownEventSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventSummary()..mergeFromBuffer(data, registry);
  factory UnknownEventSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventSummary()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnknownEventSummary',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: UnknownEventSummary.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aInt64(2, _omitFieldNames ? '' : 'sequence')
    ..aInt64(3, _omitFieldNames ? '' : 'receivedAtUnixMs')
    ..aOS(4, _omitFieldNames ? '' : 'sourceAgent')
    ..aOS(5, _omitFieldNames ? '' : 'sourceMethod')
    ..aOS(6, _omitFieldNames ? '' : 'threadId')
    ..aOS(7, _omitFieldNames ? '' : 'normalizedKind')
    ..aOS(8, _omitFieldNames ? '' : 'safetyClass')
    ..aOS(9, _omitFieldNames ? '' : 'processingStatus')
    ..a<$fixnum.Int64>(
        10, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(11, _omitFieldNames ? '' : 'summary')
    ..aOS(12, _omitFieldNames ? '' : 'plainTitle')
    ..aOS(13, _omitFieldNames ? '' : 'plainExplanation')
    ..aOS(14, _omitFieldNames ? '' : 'impact')
    ..aOB(15, _omitFieldNames ? '' : 'requiresUser')
    ..aOB(16, _omitFieldNames ? '' : 'canOpenNative')
    ..pPM<UserActionDescriptor>(17, _omitFieldNames ? '' : 'actions',
        subBuilder: UserActionDescriptor.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventSummary copyWith(void Function(UnknownEventSummary) updates) =>
      super.copyWith((message) => updates(message as UnknownEventSummary))
          as UnknownEventSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use UnknownEventSummary() / UnknownEventSummary.new instead')
  static UnknownEventSummary create() => UnknownEventSummary._();
  static $pb.GeneratedMessage $_createMessage() => UnknownEventSummary._();
  @$core.override
  UnknownEventSummary createEmptyInstance() => UnknownEventSummary._();
  @$core.pragma('dart2js:noInline')
  static UnknownEventSummary getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnknownEventSummary>(
          UnknownEventSummary.$_createMessage);
  static UnknownEventSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get sequence => $_getI64(1);
  @$pb.TagNumber(2)
  set sequence($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSequence() => $_has(1);
  @$pb.TagNumber(2)
  void clearSequence() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get receivedAtUnixMs => $_getI64(2);
  @$pb.TagNumber(3)
  set receivedAtUnixMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReceivedAtUnixMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearReceivedAtUnixMs() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get sourceAgent => $_getSZ(3);
  @$pb.TagNumber(4)
  set sourceAgent($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSourceAgent() => $_has(3);
  @$pb.TagNumber(4)
  void clearSourceAgent() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get sourceMethod => $_getSZ(4);
  @$pb.TagNumber(5)
  set sourceMethod($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSourceMethod() => $_has(4);
  @$pb.TagNumber(5)
  void clearSourceMethod() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get threadId => $_getSZ(5);
  @$pb.TagNumber(6)
  set threadId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasThreadId() => $_has(5);
  @$pb.TagNumber(6)
  void clearThreadId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get normalizedKind => $_getSZ(6);
  @$pb.TagNumber(7)
  set normalizedKind($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasNormalizedKind() => $_has(6);
  @$pb.TagNumber(7)
  void clearNormalizedKind() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get safetyClass => $_getSZ(7);
  @$pb.TagNumber(8)
  set safetyClass($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSafetyClass() => $_has(7);
  @$pb.TagNumber(8)
  void clearSafetyClass() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get processingStatus => $_getSZ(8);
  @$pb.TagNumber(9)
  set processingStatus($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasProcessingStatus() => $_has(8);
  @$pb.TagNumber(9)
  void clearProcessingStatus() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get revision => $_getI64(9);
  @$pb.TagNumber(10)
  set revision($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasRevision() => $_has(9);
  @$pb.TagNumber(10)
  void clearRevision() => $_clearField(10);

  /// daemon 生成的一句话人话翻译:说「发生了什么」,不复述术语。
  /// 翻译是摘要不是裁决 —— 原文永远可查(GetUnknownEvent.raw_json),冲突以原文为准。
  /// 旧 daemon / 旧台账条目可能为空:客户端空值时显示方法名原文并如实说明,不得编造。
  @$pb.TagNumber(11)
  $core.String get summary => $_getSZ(10);
  @$pb.TagNumber(11)
  set summary($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasSummary() => $_has(10);
  @$pb.TagNumber(11)
  void clearSummary() => $_clearField(11);

  /// ---- 用户处理中心(只增不改:12~17 为新增,旧 daemon 不带,字段号分配登记见
  /// proto/FIELD_ALLOCATIONS.md)----
  /// 以下均为 daemon 按事件种类套**固定文案**得到的确定性人话解释(不经 LLM);
  /// 旧端 / 旧台账条目可能为空,客户端空值时降级显示方法名原文,不编造。
  /// 一行标题(说「来了个什么东西」);列表与卡片共用。
  @$pb.TagNumber(12)
  $core.String get plainTitle => $_getSZ(11);
  @$pb.TagNumber(12)
  set plainTitle($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasPlainTitle() => $_has(11);
  @$pb.TagNumber(12)
  void clearPlainTitle() => $_clearField(12);

  /// 两三句话讲清「发生了什么、daemon 是怎么处理的」。
  @$pb.TagNumber(13)
  $core.String get plainExplanation => $_getSZ(12);
  @$pb.TagNumber(13)
  set plainExplanation($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasPlainExplanation() => $_has(12);
  @$pb.TagNumber(13)
  void clearPlainExplanation() => $_clearField(13);

  /// 对当前任务的影响(如「不影响正在进行的轮次」「该请求未自动执行」)。
  @$pb.TagNumber(14)
  $core.String get impact => $_getSZ(13);
  @$pb.TagNumber(14)
  set impact($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasImpact() => $_has(13);
  @$pb.TagNumber(14)
  void clearImpact() => $_clearField(14);

  /// true = 需要用户处理(进"需要我处理"分组);false = 仅供参考,不催办。
  @$pb.TagNumber(15)
  $core.bool get requiresUser => $_getBF(14);
  @$pb.TagNumber(15)
  set requiresUser($core.bool value) => $_setBool(14, value);
  @$pb.TagNumber(15)
  $core.bool hasRequiresUser() => $_has(14);
  @$pb.TagNumber(15)
  void clearRequiresUser() => $_clearField(15);

  /// true = 电脑端有原生界面,可发"在电脑端打开"类动作;false = 没有,别画入口。
  @$pb.TagNumber(16)
  $core.bool get canOpenNative => $_getBF(15);
  @$pb.TagNumber(16)
  set canOpenNative($core.bool value) => $_setBool(15, value);
  @$pb.TagNumber(16)
  $core.bool hasCanOpenNative() => $_has(15);
  @$pb.TagNumber(16)
  void clearCanOpenNative() => $_clearField(16);

  /// daemon 判定安全的可选动作(可为空 = 只能查看/复制技术原文,别造按钮)。
  @$pb.TagNumber(17)
  $pb.PbList<UserActionDescriptor> get actions => $_getList(16);
}

/// UnknownEventField 是字段级透析表的一行:把这个载荷「拆开读」的结果。
class UnknownEventField extends $pb.GeneratedMessage {
  factory UnknownEventField({
    $core.String? key,
    $core.String? kind,
    $core.String? preview,
    $core.String? meaning,
  }) {
    final result = UnknownEventField._();
    if (key != null) result.key = key;
    if (kind != null) result.kind = kind;
    if (preview != null) result.preview = preview;
    if (meaning != null) result.meaning = meaning;
    return result;
  }

  UnknownEventField._();

  factory UnknownEventField.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventField()..mergeFromBuffer(data, registry);
  factory UnknownEventField.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventField()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnknownEventField',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: UnknownEventField.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'key')
    ..aOS(2, _omitFieldNames ? '' : 'kind')
    ..aOS(3, _omitFieldNames ? '' : 'preview')
    ..aOS(4, _omitFieldNames ? '' : 'meaning')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventField clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventField copyWith(void Function(UnknownEventField) updates) =>
      super.copyWith((message) => updates(message as UnknownEventField))
          as UnknownEventField;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use UnknownEventField() / UnknownEventField.new instead')
  static UnknownEventField create() => UnknownEventField._();
  static $pb.GeneratedMessage $_createMessage() => UnknownEventField._();
  @$core.override
  UnknownEventField createEmptyInstance() => UnknownEventField._();
  @$core.pragma('dart2js:noInline')
  static UnknownEventField getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<UnknownEventField>(
          UnknownEventField.$_createMessage);
  static UnknownEventField? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get key => $_getSZ(0);
  @$pb.TagNumber(1)
  set key($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearKey() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get kind => $_getSZ(1);
  @$pb.TagNumber(2)
  set kind($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get preview => $_getSZ(2);
  @$pb.TagNumber(3)
  set preview($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPreview() => $_has(2);
  @$pb.TagNumber(3)
  void clearPreview() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get meaning => $_getSZ(3);
  @$pb.TagNumber(4)
  set meaning($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMeaning() => $_has(3);
  @$pb.TagNumber(4)
  void clearMeaning() => $_clearField(4);
}

class ListUnknownEventsResponse extends $pb.GeneratedMessage {
  factory ListUnknownEventsResponse({
    $core.Iterable<UnknownEventSummary>? items,
    $fixnum.Int64? total,
    $fixnum.Int64? dropped,
  }) {
    final result = ListUnknownEventsResponse._();
    if (items != null) result.items.addAll(items);
    if (total != null) result.total = total;
    if (dropped != null) result.dropped = dropped;
    return result;
  }

  ListUnknownEventsResponse._();

  factory ListUnknownEventsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListUnknownEventsResponse()..mergeFromBuffer(data, registry);
  factory ListUnknownEventsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListUnknownEventsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListUnknownEventsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListUnknownEventsResponse.$_createMessage)
    ..pPM<UnknownEventSummary>(1, _omitFieldNames ? '' : 'items',
        subBuilder: UnknownEventSummary.$_createMessage)
    ..aInt64(2, _omitFieldNames ? '' : 'total')
    ..aInt64(3, _omitFieldNames ? '' : 'dropped')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListUnknownEventsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListUnknownEventsResponse copyWith(
          void Function(ListUnknownEventsResponse) updates) =>
      super.copyWith((message) => updates(message as ListUnknownEventsResponse))
          as ListUnknownEventsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListUnknownEventsResponse() / ListUnknownEventsResponse.new instead')
  static ListUnknownEventsResponse create() => ListUnknownEventsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListUnknownEventsResponse._();
  @$core.override
  ListUnknownEventsResponse createEmptyInstance() =>
      ListUnknownEventsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListUnknownEventsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListUnknownEventsResponse>(
          ListUnknownEventsResponse.$_createMessage);
  static ListUnknownEventsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<UnknownEventSummary> get items => $_getList(0);

  /// 台账累计登记条数与因超上限丢弃的条数。
  /// 【为什么必须给】只回最近 N 条而不说,用户会以为"就这么几条";这两个数让
  /// "这里只留了最近 N 条、更早的已经丢了"变成明面上的事实。
  @$pb.TagNumber(2)
  $fixnum.Int64 get total => $_getI64(1);
  @$pb.TagNumber(2)
  set total($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTotal() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotal() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get dropped => $_getI64(2);
  @$pb.TagNumber(3)
  set dropped($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDropped() => $_has(2);
  @$pb.TagNumber(3)
  void clearDropped() => $_clearField(3);
}

class GetUnknownEventRequest extends $pb.GeneratedMessage {
  factory GetUnknownEventRequest({
    $core.String? eventId,
  }) {
    final result = GetUnknownEventRequest._();
    if (eventId != null) result.eventId = eventId;
    return result;
  }

  GetUnknownEventRequest._();

  factory GetUnknownEventRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnknownEventRequest()..mergeFromBuffer(data, registry);
  factory GetUnknownEventRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetUnknownEventRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetUnknownEventRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetUnknownEventRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnknownEventRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetUnknownEventRequest copyWith(
          void Function(GetUnknownEventRequest) updates) =>
      super.copyWith((message) => updates(message as GetUnknownEventRequest))
          as GetUnknownEventRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetUnknownEventRequest() / GetUnknownEventRequest.new instead')
  static GetUnknownEventRequest create() => GetUnknownEventRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetUnknownEventRequest._();
  @$core.override
  GetUnknownEventRequest createEmptyInstance() => GetUnknownEventRequest._();
  @$core.pragma('dart2js:noInline')
  static GetUnknownEventRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetUnknownEventRequest>(
          GetUnknownEventRequest.$_createMessage);
  static GetUnknownEventRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);
}

class UnknownEventResponse extends $pb.GeneratedMessage {
  factory UnknownEventResponse({
    UnknownEventSummary? summary,
    $core.String? rawJson,
    $core.String? decodedFieldsJson,
    $core.String? note,
    $core.Iterable<UnknownEventField>? digest,
    $core.String? translation,
    $core.String? recommendedNextStep,
    $core.String? supportBundleHint,
  }) {
    final result = UnknownEventResponse._();
    if (summary != null) result.summary = summary;
    if (rawJson != null) result.rawJson = rawJson;
    if (decodedFieldsJson != null) result.decodedFieldsJson = decodedFieldsJson;
    if (note != null) result.note = note;
    if (digest != null) result.digest.addAll(digest);
    if (translation != null) result.translation = translation;
    if (recommendedNextStep != null)
      result.recommendedNextStep = recommendedNextStep;
    if (supportBundleHint != null) result.supportBundleHint = supportBundleHint;
    return result;
  }

  UnknownEventResponse._();

  factory UnknownEventResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventResponse()..mergeFromBuffer(data, registry);
  factory UnknownEventResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnknownEventResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: UnknownEventResponse.$_createMessage)
    ..aOM<UnknownEventSummary>(1, _omitFieldNames ? '' : 'summary',
        subBuilder: UnknownEventSummary.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'rawJson')
    ..aOS(3, _omitFieldNames ? '' : 'decodedFieldsJson')
    ..aOS(4, _omitFieldNames ? '' : 'note')
    ..pPM<UnknownEventField>(5, _omitFieldNames ? '' : 'digest',
        subBuilder: UnknownEventField.$_createMessage)
    ..aOS(6, _omitFieldNames ? '' : 'translation')
    ..aOS(7, _omitFieldNames ? '' : 'recommendedNextStep')
    ..aOS(8, _omitFieldNames ? '' : 'supportBundleHint')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventResponse copyWith(void Function(UnknownEventResponse) updates) =>
      super.copyWith((message) => updates(message as UnknownEventResponse))
          as UnknownEventResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnknownEventResponse() / UnknownEventResponse.new instead')
  static UnknownEventResponse create() => UnknownEventResponse._();
  static $pb.GeneratedMessage $_createMessage() => UnknownEventResponse._();
  @$core.override
  UnknownEventResponse createEmptyInstance() => UnknownEventResponse._();
  @$core.pragma('dart2js:noInline')
  static UnknownEventResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnknownEventResponse>(
          UnknownEventResponse.$_createMessage);
  static UnknownEventResponse? _defaultInstance;

  @$pb.TagNumber(1)
  UnknownEventSummary get summary => $_getN(0);
  @$pb.TagNumber(1)
  set summary(UnknownEventSummary value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSummary() => $_has(0);
  @$pb.TagNumber(1)
  void clearSummary() => $_clearField(1);
  @$pb.TagNumber(1)
  UnknownEventSummary ensureSummary() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get rawJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set rawJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRawJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearRawJson() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get decodedFieldsJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set decodedFieldsJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDecodedFieldsJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearDecodedFieldsJson() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get note => $_getSZ(3);
  @$pb.TagNumber(4)
  set note($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasNote() => $_has(3);
  @$pb.TagNumber(4)
  void clearNote() => $_clearField(4);

  /// 字段级透析表:载荷拆开读的每一行(见 UnknownEventField)。旧 daemon 为空。
  @$pb.TagNumber(5)
  $pb.PbList<UnknownEventField> get digest => $_getList(4);

  /// 详细人话解读:比 summary 长 —— 字段怎么读、各部分像什么、daemon 打算怎么处理
  /// (已投影/只透传/安全拒绝)。旧 daemon 为空。
  @$pb.TagNumber(6)
  $core.String get translation => $_getSZ(5);
  @$pb.TagNumber(6)
  set translation($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasTranslation() => $_has(5);
  @$pb.TagNumber(6)
  void clearTranslation() => $_clearField(6);

  /// ---- 用户处理中心(只增不改:7~8 为新增,旧 daemon 不带,字段号分配登记见
  /// proto/FIELD_ALLOCATIONS.md)----
  /// 建议的下一步(人话,如「可在电脑端查看原文」);空 = 无建议,客户端不占位。
  @$pb.TagNumber(7)
  $core.String get recommendedNextStep => $_getSZ(6);
  @$pb.TagNumber(7)
  set recommendedNextStep($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasRecommendedNextStep() => $_has(6);
  @$pb.TagNumber(7)
  void clearRecommendedNextStep() => $_clearField(7);

  /// 导出支持包(support bundle)时与本事件相关的提示(如原文会一并打包);
  /// 空 = 无,客户端不占位。
  @$pb.TagNumber(8)
  $core.String get supportBundleHint => $_getSZ(7);
  @$pb.TagNumber(8)
  set supportBundleHint($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSupportBundleHint() => $_has(7);
  @$pb.TagNumber(8)
  void clearSupportBundleHint() => $_clearField(8);
}

class AcknowledgeUnknownEventRequest extends $pb.GeneratedMessage {
  factory AcknowledgeUnknownEventRequest({
    $core.String? eventId,
    $core.String? action,
    $core.String? note,
  }) {
    final result = AcknowledgeUnknownEventRequest._();
    if (eventId != null) result.eventId = eventId;
    if (action != null) result.action = action;
    if (note != null) result.note = note;
    return result;
  }

  AcknowledgeUnknownEventRequest._();

  factory AcknowledgeUnknownEventRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcknowledgeUnknownEventRequest()..mergeFromBuffer(data, registry);
  factory AcknowledgeUnknownEventRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcknowledgeUnknownEventRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcknowledgeUnknownEventRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AcknowledgeUnknownEventRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOS(2, _omitFieldNames ? '' : 'action')
    ..aOS(3, _omitFieldNames ? '' : 'note')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcknowledgeUnknownEventRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcknowledgeUnknownEventRequest copyWith(
          void Function(AcknowledgeUnknownEventRequest) updates) =>
      super.copyWith(
              (message) => updates(message as AcknowledgeUnknownEventRequest))
          as AcknowledgeUnknownEventRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AcknowledgeUnknownEventRequest() / AcknowledgeUnknownEventRequest.new instead')
  static AcknowledgeUnknownEventRequest create() =>
      AcknowledgeUnknownEventRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      AcknowledgeUnknownEventRequest._();
  @$core.override
  AcknowledgeUnknownEventRequest createEmptyInstance() =>
      AcknowledgeUnknownEventRequest._();
  @$core.pragma('dart2js:noInline')
  static AcknowledgeUnknownEventRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcknowledgeUnknownEventRequest>(
          AcknowledgeUnknownEventRequest.$_createMessage);
  static AcknowledgeUnknownEventRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);

  /// NEW / INSPECTED / IGNORED / RETRY_LATER / ESCALATED / NEEDS_NATIVE_SUPPORT。
  /// 非法取值回 InvalidArgument 并列出可用值 —— 静默写入客户端不认识的状态,
  /// 表现是"点了确认但没变化"。
  @$pb.TagNumber(2)
  $core.String get action => $_getSZ(1);
  @$pb.TagNumber(2)
  set action($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAction() => $_has(1);
  @$pb.TagNumber(2)
  void clearAction() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get note => $_getSZ(2);
  @$pb.TagNumber(3)
  set note($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNote() => $_has(2);
  @$pb.TagNumber(3)
  void clearNote() => $_clearField(3);
}

class AcknowledgeUnknownEventResponse extends $pb.GeneratedMessage {
  factory AcknowledgeUnknownEventResponse({
    UnknownEventSummary? item,
  }) {
    final result = AcknowledgeUnknownEventResponse._();
    if (item != null) result.item = item;
    return result;
  }

  AcknowledgeUnknownEventResponse._();

  factory AcknowledgeUnknownEventResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcknowledgeUnknownEventResponse()..mergeFromBuffer(data, registry);
  factory AcknowledgeUnknownEventResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AcknowledgeUnknownEventResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcknowledgeUnknownEventResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AcknowledgeUnknownEventResponse.$_createMessage)
    ..aOM<UnknownEventSummary>(1, _omitFieldNames ? '' : 'item',
        subBuilder: UnknownEventSummary.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcknowledgeUnknownEventResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcknowledgeUnknownEventResponse copyWith(
          void Function(AcknowledgeUnknownEventResponse) updates) =>
      super.copyWith(
              (message) => updates(message as AcknowledgeUnknownEventResponse))
          as AcknowledgeUnknownEventResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AcknowledgeUnknownEventResponse() / AcknowledgeUnknownEventResponse.new instead')
  static AcknowledgeUnknownEventResponse create() =>
      AcknowledgeUnknownEventResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      AcknowledgeUnknownEventResponse._();
  @$core.override
  AcknowledgeUnknownEventResponse createEmptyInstance() =>
      AcknowledgeUnknownEventResponse._();
  @$core.pragma('dart2js:noInline')
  static AcknowledgeUnknownEventResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcknowledgeUnknownEventResponse>(
          AcknowledgeUnknownEventResponse.$_createMessage);
  static AcknowledgeUnknownEventResponse? _defaultInstance;

  @$pb.TagNumber(1)
  UnknownEventSummary get item => $_getN(0);
  @$pb.TagNumber(1)
  set item(UnknownEventSummary value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasItem() => $_has(0);
  @$pb.TagNumber(1)
  void clearItem() => $_clearField(1);
  @$pb.TagNumber(1)
  UnknownEventSummary ensureItem() => $_ensure(0);
}

/// UnknownEventActionRequest 执行一个未知事件上的安全动作。
/// action_id 必须来自该事件 UnknownEventSummary.actions 里声明过的取值;
/// 客户端不得自造动作 —— daemon 按白名单(inspect / acknowledge / retry_later /
/// open_native)二次校验,白名单之外一律拒绝,绝不出现 approve/放行类动作。
class UnknownEventActionRequest extends $pb.GeneratedMessage {
  factory UnknownEventActionRequest({
    $core.String? eventId,
    $fixnum.Int64? expectedRevision,
    $core.String? actionId,
    $core.String? note,
  }) {
    final result = UnknownEventActionRequest._();
    if (eventId != null) result.eventId = eventId;
    if (expectedRevision != null) result.expectedRevision = expectedRevision;
    if (actionId != null) result.actionId = actionId;
    if (note != null) result.note = note;
    return result;
  }

  UnknownEventActionRequest._();

  factory UnknownEventActionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventActionRequest()..mergeFromBuffer(data, registry);
  factory UnknownEventActionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventActionRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnknownEventActionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: UnknownEventActionRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aInt64(2, _omitFieldNames ? '' : 'expectedRevision')
    ..aOS(3, _omitFieldNames ? '' : 'actionId')
    ..aOS(4, _omitFieldNames ? '' : 'note')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventActionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventActionRequest copyWith(
          void Function(UnknownEventActionRequest) updates) =>
      super.copyWith((message) => updates(message as UnknownEventActionRequest))
          as UnknownEventActionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnknownEventActionRequest() / UnknownEventActionRequest.new instead')
  static UnknownEventActionRequest create() => UnknownEventActionRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      UnknownEventActionRequest._();
  @$core.override
  UnknownEventActionRequest createEmptyInstance() =>
      UnknownEventActionRequest._();
  @$core.pragma('dart2js:noInline')
  static UnknownEventActionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnknownEventActionRequest>(
          UnknownEventActionRequest.$_createMessage);
  static UnknownEventActionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);

  /// 乐观并发控制:客户端最后一次看到的 UnknownEventSummary.revision。
  /// 不一致 = 事件已被别处处理过 → 结果 conflict=true(见 ActionResult,
  /// 不是 gRPC error)。0 = 不做 CAS(仅 inspect 这类纯读动作允许)。
  @$pb.TagNumber(2)
  $fixnum.Int64 get expectedRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set expectedRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExpectedRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpectedRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get actionId => $_getSZ(2);
  @$pb.TagNumber(3)
  set actionId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasActionId() => $_has(2);
  @$pb.TagNumber(3)
  void clearActionId() => $_clearField(3);

  /// 可选备注(如 acknowledge 时用户写的一句话);空 = 无。客户端有备注输入
  /// 时必须透传,不得始终传空(与 AcknowledgeUnknownEventRequest.note 同纪律)。
  @$pb.TagNumber(4)
  $core.String get note => $_getSZ(3);
  @$pb.TagNumber(4)
  set note($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasNote() => $_has(3);
  @$pb.TagNumber(4)
  void clearNote() => $_clearField(4);
}

/// UnknownEventActionResult 动作执行的结构化结果。
/// 【为什么是结构化结果而不是 gRPC error】"已被别处处理过"不是传输层故障:
/// 冲突走 conflict=true + 人话 detail,正常完成走 ok=true —— 客户端一眼分清
/// "该重拉详情"还是"成了",不会把冲突渲染成红屏报错。
class UnknownEventActionResult extends $pb.GeneratedMessage {
  factory UnknownEventActionResult({
    $core.bool? ok,
    $core.bool? conflict,
    $core.String? detail,
    UnknownEventSummary? summary,
    UnknownEventResponse? technical,
    $fixnum.Int64? newRevision,
  }) {
    final result = UnknownEventActionResult._();
    if (ok != null) result.ok = ok;
    if (conflict != null) result.conflict = conflict;
    if (detail != null) result.detail = detail;
    if (summary != null) result.summary = summary;
    if (technical != null) result.technical = technical;
    if (newRevision != null) result.newRevision = newRevision;
    return result;
  }

  UnknownEventActionResult._();

  factory UnknownEventActionResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventActionResult()..mergeFromBuffer(data, registry);
  factory UnknownEventActionResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      UnknownEventActionResult()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnknownEventActionResult',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: UnknownEventActionResult.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'ok')
    ..aOB(2, _omitFieldNames ? '' : 'conflict')
    ..aOS(3, _omitFieldNames ? '' : 'detail')
    ..aOM<UnknownEventSummary>(4, _omitFieldNames ? '' : 'summary',
        subBuilder: UnknownEventSummary.$_createMessage)
    ..aOM<UnknownEventResponse>(5, _omitFieldNames ? '' : 'technical',
        subBuilder: UnknownEventResponse.$_createMessage)
    ..aInt64(6, _omitFieldNames ? '' : 'newRevision')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventActionResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnknownEventActionResult copyWith(
          void Function(UnknownEventActionResult) updates) =>
      super.copyWith((message) => updates(message as UnknownEventActionResult))
          as UnknownEventActionResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use UnknownEventActionResult() / UnknownEventActionResult.new instead')
  static UnknownEventActionResult create() => UnknownEventActionResult._();
  static $pb.GeneratedMessage $_createMessage() => UnknownEventActionResult._();
  @$core.override
  UnknownEventActionResult createEmptyInstance() =>
      UnknownEventActionResult._();
  @$core.pragma('dart2js:noInline')
  static UnknownEventActionResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnknownEventActionResult>(
          UnknownEventActionResult.$_createMessage);
  static UnknownEventActionResult? _defaultInstance;

  /// 动作已执行成功(幂等重复提交返回当前状态时同样 ok=true)。
  @$pb.TagNumber(1)
  $core.bool get ok => $_getBF(0);
  @$pb.TagNumber(1)
  set ok($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOk() => $_has(0);
  @$pb.TagNumber(1)
  void clearOk() => $_clearField(1);

  /// true = expected_revision 不匹配,事件已被别处处理过;detail 说明现状,
  /// 客户端应重新拉取 GetUnknownEvent / 用 new_revision 重试。
  @$pb.TagNumber(2)
  $core.bool get conflict => $_getBF(1);
  @$pb.TagNumber(2)
  set conflict($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConflict() => $_has(1);
  @$pb.TagNumber(2)
  void clearConflict() => $_clearField(2);

  /// 人话说明:成功 = 实际做了什么;冲突 = 被处理成了什么状态;如实给,不编造。
  @$pb.TagNumber(3)
  $core.String get detail => $_getSZ(2);
  @$pb.TagNumber(3)
  set detail($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDetail() => $_has(2);
  @$pb.TagNumber(3)
  void clearDetail() => $_clearField(3);

  /// 执行后(冲突时为当前)的台账条目真值,客户端就地刷新列表与卡片。
  @$pb.TagNumber(4)
  UnknownEventSummary get summary => $_getN(3);
  @$pb.TagNumber(4)
  set summary(UnknownEventSummary value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSummary() => $_has(3);
  @$pb.TagNumber(4)
  void clearSummary() => $_clearField(4);
  @$pb.TagNumber(4)
  UnknownEventSummary ensureSummary() => $_ensure(3);

  /// 技术详情:复用既有 UnknownEventResponse 形态(raw_json / decoded_fields_json /
  /// digest / translation 等原样可用,详情页直接整包渲染)。
  /// 【复用说明】其内嵌的 summary 字段在本场景冗余(本消息字段 4 已给),
  /// 客户端以本消息顶层 summary 为准,忽略 technical.summary。
  @$pb.TagNumber(5)
  UnknownEventResponse get technical => $_getN(4);
  @$pb.TagNumber(5)
  set technical(UnknownEventResponse value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasTechnical() => $_has(4);
  @$pb.TagNumber(5)
  void clearTechnical() => $_clearField(5);
  @$pb.TagNumber(5)
  UnknownEventResponse ensureTechnical() => $_ensure(4);

  /// 执行后的台账 revision;冲突时返回当前值,客户端拿它做下一次 expected_revision。
  @$pb.TagNumber(6)
  $fixnum.Int64 get newRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set newRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasNewRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearNewRevision() => $_clearField(6);
}

/// ---- 协同策略(规格 §11)----------------------------------------------------
///
/// 【为什么只落这几项】规格 §11 还列了 auto_summarize / retain_artifacts /
/// max_session_bytes —— 那三项在 daemon 侧目前没有对应的执行点。先放进契约就只是
/// 一个"改了不生效"的开关,比不提供更糟:用户会以为功能坏了,而真正的原因是它
/// 尚未实现。等执行点存在时再加字段。
class CollabPolicy extends $pb.GeneratedMessage {
  factory CollabPolicy({
    $core.bool? collaborationEnabled,
    $core.bool? nativeDelegateEnabled,
    $core.bool? fanoutEnabled,
    $core.int? maxParallelMembers,
  }) {
    final result = CollabPolicy._();
    if (collaborationEnabled != null)
      result.collaborationEnabled = collaborationEnabled;
    if (nativeDelegateEnabled != null)
      result.nativeDelegateEnabled = nativeDelegateEnabled;
    if (fanoutEnabled != null) result.fanoutEnabled = fanoutEnabled;
    if (maxParallelMembers != null)
      result.maxParallelMembers = maxParallelMembers;
    return result;
  }

  CollabPolicy._();

  factory CollabPolicy.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabPolicy()..mergeFromBuffer(data, registry);
  factory CollabPolicy.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabPolicy()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CollabPolicy',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CollabPolicy.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'collaborationEnabled')
    ..aOB(2, _omitFieldNames ? '' : 'nativeDelegateEnabled')
    ..aOB(3, _omitFieldNames ? '' : 'fanoutEnabled')
    ..aI(4, _omitFieldNames ? '' : 'maxParallelMembers')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabPolicy clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabPolicy copyWith(void Function(CollabPolicy) updates) =>
      super.copyWith((message) => updates(message as CollabPolicy))
          as CollabPolicy;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CollabPolicy() / CollabPolicy.new instead')
  static CollabPolicy create() => CollabPolicy._();
  static $pb.GeneratedMessage $_createMessage() => CollabPolicy._();
  @$core.override
  CollabPolicy createEmptyInstance() => CollabPolicy._();
  @$core.pragma('dart2js:noInline')
  static CollabPolicy getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CollabPolicy>(
          CollabPolicy.$_createMessage);
  static CollabPolicy? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get collaborationEnabled => $_getBF(0);
  @$pb.TagNumber(1)
  set collaborationEnabled($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCollaborationEnabled() => $_has(0);
  @$pb.TagNumber(1)
  void clearCollaborationEnabled() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get nativeDelegateEnabled => $_getBF(1);
  @$pb.TagNumber(2)
  set nativeDelegateEnabled($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNativeDelegateEnabled() => $_has(1);
  @$pb.TagNumber(2)
  void clearNativeDelegateEnabled() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get fanoutEnabled => $_getBF(2);
  @$pb.TagNumber(3)
  set fanoutEnabled($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFanoutEnabled() => $_has(2);
  @$pb.TagNumber(3)
  void clearFanoutEnabled() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get maxParallelMembers => $_getIZ(3);
  @$pb.TagNumber(4)
  set maxParallelMembers($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMaxParallelMembers() => $_has(3);
  @$pb.TagNumber(4)
  void clearMaxParallelMembers() => $_clearField(4);
}

class GetCollabPolicyRequest extends $pb.GeneratedMessage {
  factory GetCollabPolicyRequest() => GetCollabPolicyRequest._();

  GetCollabPolicyRequest._();

  factory GetCollabPolicyRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCollabPolicyRequest()..mergeFromBuffer(data, registry);
  factory GetCollabPolicyRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCollabPolicyRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCollabPolicyRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetCollabPolicyRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCollabPolicyRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCollabPolicyRequest copyWith(
          void Function(GetCollabPolicyRequest) updates) =>
      super.copyWith((message) => updates(message as GetCollabPolicyRequest))
          as GetCollabPolicyRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetCollabPolicyRequest() / GetCollabPolicyRequest.new instead')
  static GetCollabPolicyRequest create() => GetCollabPolicyRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetCollabPolicyRequest._();
  @$core.override
  GetCollabPolicyRequest createEmptyInstance() => GetCollabPolicyRequest._();
  @$core.pragma('dart2js:noInline')
  static GetCollabPolicyRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCollabPolicyRequest>(
          GetCollabPolicyRequest.$_createMessage);
  static GetCollabPolicyRequest? _defaultInstance;
}

class SetCollabPolicyRequest extends $pb.GeneratedMessage {
  factory SetCollabPolicyRequest({
    CollabPolicy? policy,
    $fixnum.Int64? expectedRevision,
  }) {
    final result = SetCollabPolicyRequest._();
    if (policy != null) result.policy = policy;
    if (expectedRevision != null) result.expectedRevision = expectedRevision;
    return result;
  }

  SetCollabPolicyRequest._();

  factory SetCollabPolicyRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCollabPolicyRequest()..mergeFromBuffer(data, registry);
  factory SetCollabPolicyRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCollabPolicyRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCollabPolicyRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SetCollabPolicyRequest.$_createMessage)
    ..aOM<CollabPolicy>(1, _omitFieldNames ? '' : 'policy',
        subBuilder: CollabPolicy.$_createMessage)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'expectedRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCollabPolicyRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCollabPolicyRequest copyWith(
          void Function(SetCollabPolicyRequest) updates) =>
      super.copyWith((message) => updates(message as SetCollabPolicyRequest))
          as SetCollabPolicyRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetCollabPolicyRequest() / SetCollabPolicyRequest.new instead')
  static SetCollabPolicyRequest create() => SetCollabPolicyRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetCollabPolicyRequest._();
  @$core.override
  SetCollabPolicyRequest createEmptyInstance() => SetCollabPolicyRequest._();
  @$core.pragma('dart2js:noInline')
  static SetCollabPolicyRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCollabPolicyRequest>(
          SetCollabPolicyRequest.$_createMessage);
  static SetCollabPolicyRequest? _defaultInstance;

  /// 【提交的是**完整策略**,不是补丁】缺席的项 = 未设置(沿用默认),不是"不改这一项"。
  /// 客户端改单项时请先 Get、在拿到的完整对象上改、再 Set(并带上 revision 做 CAS);
  /// 只提交一个字段会把其它字段重置回默认 —— 实测踩到过:只关扇出会让成员上限
  /// 从 2 变回"不限"。
  @$pb.TagNumber(1)
  CollabPolicy get policy => $_getN(0);
  @$pb.TagNumber(1)
  set policy(CollabPolicy value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPolicy() => $_has(0);
  @$pb.TagNumber(1)
  void clearPolicy() => $_clearField(1);
  @$pb.TagNumber(1)
  CollabPolicy ensurePolicy() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get expectedRevision => $_getI64(1);
  @$pb.TagNumber(2)
  set expectedRevision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExpectedRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpectedRevision() => $_clearField(2);
}

class CollabPolicyResponse extends $pb.GeneratedMessage {
  factory CollabPolicyResponse({
    CollabPolicy? policy,
    $fixnum.Int64? revision,
    $core.String? policyPath,
    CollabPolicy? effective,
  }) {
    final result = CollabPolicyResponse._();
    if (policy != null) result.policy = policy;
    if (revision != null) result.revision = revision;
    if (policyPath != null) result.policyPath = policyPath;
    if (effective != null) result.effective = effective;
    return result;
  }

  CollabPolicyResponse._();

  factory CollabPolicyResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabPolicyResponse()..mergeFromBuffer(data, registry);
  factory CollabPolicyResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CollabPolicyResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CollabPolicyResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CollabPolicyResponse.$_createMessage)
    ..aOM<CollabPolicy>(1, _omitFieldNames ? '' : 'policy',
        subBuilder: CollabPolicy.$_createMessage)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(3, _omitFieldNames ? '' : 'policyPath')
    ..aOM<CollabPolicy>(4, _omitFieldNames ? '' : 'effective',
        subBuilder: CollabPolicy.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabPolicyResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CollabPolicyResponse copyWith(void Function(CollabPolicyResponse) updates) =>
      super.copyWith((message) => updates(message as CollabPolicyResponse))
          as CollabPolicyResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CollabPolicyResponse() / CollabPolicyResponse.new instead')
  static CollabPolicyResponse create() => CollabPolicyResponse._();
  static $pb.GeneratedMessage $_createMessage() => CollabPolicyResponse._();
  @$core.override
  CollabPolicyResponse createEmptyInstance() => CollabPolicyResponse._();
  @$core.pragma('dart2js:noInline')
  static CollabPolicyResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CollabPolicyResponse>(
          CollabPolicyResponse.$_createMessage);
  static CollabPolicyResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CollabPolicy get policy => $_getN(0);
  @$pb.TagNumber(1)
  set policy(CollabPolicy value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPolicy() => $_has(0);
  @$pb.TagNumber(1)
  void clearPolicy() => $_clearField(1);
  @$pb.TagNumber(1)
  CollabPolicy ensurePolicy() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get revision => $_getI64(1);
  @$pb.TagNumber(2)
  set revision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get policyPath => $_getSZ(2);
  @$pb.TagNumber(3)
  set policyPath($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPolicyPath() => $_has(2);
  @$pb.TagNumber(3)
  void clearPolicyPath() => $_clearField(3);

  @$pb.TagNumber(4)
  CollabPolicy get effective => $_getN(3);
  @$pb.TagNumber(4)
  set effective(CollabPolicy value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasEffective() => $_has(3);
  @$pb.TagNumber(4)
  void clearEffective() => $_clearField(4);
  @$pb.TagNumber(4)
  CollabPolicy ensureEffective() => $_ensure(3);
}

class GetRuntimeStateRequest extends $pb.GeneratedMessage {
  factory GetRuntimeStateRequest() => GetRuntimeStateRequest._();

  GetRuntimeStateRequest._();

  factory GetRuntimeStateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimeStateRequest()..mergeFromBuffer(data, registry);
  factory GetRuntimeStateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetRuntimeStateRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetRuntimeStateRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetRuntimeStateRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimeStateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRuntimeStateRequest copyWith(
          void Function(GetRuntimeStateRequest) updates) =>
      super.copyWith((message) => updates(message as GetRuntimeStateRequest))
          as GetRuntimeStateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetRuntimeStateRequest() / GetRuntimeStateRequest.new instead')
  static GetRuntimeStateRequest create() => GetRuntimeStateRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetRuntimeStateRequest._();
  @$core.override
  GetRuntimeStateRequest createEmptyInstance() => GetRuntimeStateRequest._();
  @$core.pragma('dart2js:noInline')
  static GetRuntimeStateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetRuntimeStateRequest>(
          GetRuntimeStateRequest.$_createMessage);
  static GetRuntimeStateRequest? _defaultInstance;
}

class RuntimeStateResponse extends $pb.GeneratedMessage {
  factory RuntimeStateResponse({
    $core.String? installationId,
    $core.String? hostId,
    $core.String? daemonVersion,
    $fixnum.Int64? budgetPolicyRevision,
    $fixnum.Int64? collabPolicyRevision,
    $core.String? runtimeState,
    $fixnum.Int64? sampledAtUnixMs,
    $core.Iterable<RuntimeContractReport>? contracts,
    $core.String? dataDir,
  }) {
    final result = RuntimeStateResponse._();
    if (installationId != null) result.installationId = installationId;
    if (hostId != null) result.hostId = hostId;
    if (daemonVersion != null) result.daemonVersion = daemonVersion;
    if (budgetPolicyRevision != null)
      result.budgetPolicyRevision = budgetPolicyRevision;
    if (collabPolicyRevision != null)
      result.collabPolicyRevision = collabPolicyRevision;
    if (runtimeState != null) result.runtimeState = runtimeState;
    if (sampledAtUnixMs != null) result.sampledAtUnixMs = sampledAtUnixMs;
    if (contracts != null) result.contracts.addAll(contracts);
    if (dataDir != null) result.dataDir = dataDir;
    return result;
  }

  RuntimeStateResponse._();

  factory RuntimeStateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeStateResponse()..mergeFromBuffer(data, registry);
  factory RuntimeStateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeStateResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RuntimeStateResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RuntimeStateResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'installationId')
    ..aOS(2, _omitFieldNames ? '' : 'hostId')
    ..aOS(3, _omitFieldNames ? '' : 'daemonVersion')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'budgetPolicyRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        5, _omitFieldNames ? '' : 'collabPolicyRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(6, _omitFieldNames ? '' : 'runtimeState')
    ..aInt64(7, _omitFieldNames ? '' : 'sampledAtUnixMs')
    ..pPM<RuntimeContractReport>(8, _omitFieldNames ? '' : 'contracts',
        subBuilder: RuntimeContractReport.$_createMessage)
    ..aOS(9, _omitFieldNames ? '' : 'dataDir')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeStateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeStateResponse copyWith(void Function(RuntimeStateResponse) updates) =>
      super.copyWith((message) => updates(message as RuntimeStateResponse))
          as RuntimeStateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RuntimeStateResponse() / RuntimeStateResponse.new instead')
  static RuntimeStateResponse create() => RuntimeStateResponse._();
  static $pb.GeneratedMessage $_createMessage() => RuntimeStateResponse._();
  @$core.override
  RuntimeStateResponse createEmptyInstance() => RuntimeStateResponse._();
  @$core.pragma('dart2js:noInline')
  static RuntimeStateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RuntimeStateResponse>(
          RuntimeStateResponse.$_createMessage);
  static RuntimeStateResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get installationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set installationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInstallationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearInstallationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get hostId => $_getSZ(1);
  @$pb.TagNumber(2)
  set hostId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHostId() => $_has(1);
  @$pb.TagNumber(2)
  void clearHostId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get daemonVersion => $_getSZ(2);
  @$pb.TagNumber(3)
  set daemonVersion($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDaemonVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearDaemonVersion() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get budgetPolicyRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set budgetPolicyRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBudgetPolicyRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearBudgetPolicyRevision() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get collabPolicyRevision => $_getI64(4);
  @$pb.TagNumber(5)
  set collabPolicyRevision($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCollabPolicyRevision() => $_has(4);
  @$pb.TagNumber(5)
  void clearCollabPolicyRevision() => $_clearField(5);

  /// runtime_state:ready / degraded。degraded = 至少有一个 agent 的形态申报不合格
  /// —— "有东西没交代清楚"本身就是需要用户知道的状态,不该只在日志里。
  @$pb.TagNumber(6)
  $core.String get runtimeState => $_getSZ(5);
  @$pb.TagNumber(6)
  set runtimeState($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRuntimeState() => $_has(5);
  @$pb.TagNumber(6)
  void clearRuntimeState() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get sampledAtUnixMs => $_getI64(6);
  @$pb.TagNumber(7)
  set sampledAtUnixMs($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSampledAtUnixMs() => $_has(6);
  @$pb.TagNumber(7)
  void clearSampledAtUnixMs() => $_clearField(7);

  @$pb.TagNumber(8)
  $pb.PbList<RuntimeContractReport> get contracts => $_getList(7);

  @$pb.TagNumber(9)
  $core.String get dataDir => $_getSZ(8);
  @$pb.TagNumber(9)
  set dataDir($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasDataDir() => $_has(8);
  @$pb.TagNumber(9)
  void clearDataDir() => $_clearField(9);
}

/// RuntimeContractReport 是一个 agent 的运行时形态申报 + 核验结果。
///
/// 【为什么把 descriptor 的六个 model 字段摊平给客户端】它们是"这家 agent 的会话
/// 存在哪、能不能恢复、审批从哪来"的机器可读答案。手机端据此决定显示什么入口,
/// 而不是按 agent 名字分支(那是项目红线)。
class RuntimeContractReport extends $pb.GeneratedMessage {
  factory RuntimeContractReport({
    $core.String? agentId,
    $core.String? displayName,
    $core.String? version,
    $core.bool? declared,
    $core.Iterable<$core.String>? capabilities,
    $core.Iterable<RuntimeContractIssue>? issues,
    $core.Iterable<RuntimeLimitation>? limitations,
    $core.String? sessionModel,
    $core.String? resumeModel,
    $core.String? approvalModel,
    $core.String? subagentModel,
    $core.String? memoryModel,
    $core.String? cacheModel,
  }) {
    final result = RuntimeContractReport._();
    if (agentId != null) result.agentId = agentId;
    if (displayName != null) result.displayName = displayName;
    if (version != null) result.version = version;
    if (declared != null) result.declared = declared;
    if (capabilities != null) result.capabilities.addAll(capabilities);
    if (issues != null) result.issues.addAll(issues);
    if (limitations != null) result.limitations.addAll(limitations);
    if (sessionModel != null) result.sessionModel = sessionModel;
    if (resumeModel != null) result.resumeModel = resumeModel;
    if (approvalModel != null) result.approvalModel = approvalModel;
    if (subagentModel != null) result.subagentModel = subagentModel;
    if (memoryModel != null) result.memoryModel = memoryModel;
    if (cacheModel != null) result.cacheModel = cacheModel;
    return result;
  }

  RuntimeContractReport._();

  factory RuntimeContractReport.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeContractReport()..mergeFromBuffer(data, registry);
  factory RuntimeContractReport.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeContractReport()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RuntimeContractReport',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RuntimeContractReport.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'displayName')
    ..aOS(3, _omitFieldNames ? '' : 'version')
    ..aOB(4, _omitFieldNames ? '' : 'declared')
    ..pPS(5, _omitFieldNames ? '' : 'capabilities')
    ..pPM<RuntimeContractIssue>(6, _omitFieldNames ? '' : 'issues',
        subBuilder: RuntimeContractIssue.$_createMessage)
    ..pPM<RuntimeLimitation>(7, _omitFieldNames ? '' : 'limitations',
        subBuilder: RuntimeLimitation.$_createMessage)
    ..aOS(8, _omitFieldNames ? '' : 'sessionModel')
    ..aOS(9, _omitFieldNames ? '' : 'resumeModel')
    ..aOS(10, _omitFieldNames ? '' : 'approvalModel')
    ..aOS(11, _omitFieldNames ? '' : 'subagentModel')
    ..aOS(12, _omitFieldNames ? '' : 'memoryModel')
    ..aOS(13, _omitFieldNames ? '' : 'cacheModel')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeContractReport clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeContractReport copyWith(
          void Function(RuntimeContractReport) updates) =>
      super.copyWith((message) => updates(message as RuntimeContractReport))
          as RuntimeContractReport;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RuntimeContractReport() / RuntimeContractReport.new instead')
  static RuntimeContractReport create() => RuntimeContractReport._();
  static $pb.GeneratedMessage $_createMessage() => RuntimeContractReport._();
  @$core.override
  RuntimeContractReport createEmptyInstance() => RuntimeContractReport._();
  @$core.pragma('dart2js:noInline')
  static RuntimeContractReport getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RuntimeContractReport>(
          RuntimeContractReport.$_createMessage);
  static RuntimeContractReport? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get displayName => $_getSZ(1);
  @$pb.TagNumber(2)
  set displayName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplayName() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplayName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get version => $_getSZ(2);
  @$pb.TagNumber(3)
  set version($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearVersion() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get declared => $_getBF(3);
  @$pb.TagNumber(4)
  set declared($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeclared() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeclared() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get capabilities => $_getList(4);

  @$pb.TagNumber(6)
  $pb.PbList<RuntimeContractIssue> get issues => $_getList(5);

  @$pb.TagNumber(7)
  $pb.PbList<RuntimeLimitation> get limitations => $_getList(6);

  @$pb.TagNumber(8)
  $core.String get sessionModel => $_getSZ(7);
  @$pb.TagNumber(8)
  set sessionModel($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSessionModel() => $_has(7);
  @$pb.TagNumber(8)
  void clearSessionModel() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get resumeModel => $_getSZ(8);
  @$pb.TagNumber(9)
  set resumeModel($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasResumeModel() => $_has(8);
  @$pb.TagNumber(9)
  void clearResumeModel() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get approvalModel => $_getSZ(9);
  @$pb.TagNumber(10)
  set approvalModel($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasApprovalModel() => $_has(9);
  @$pb.TagNumber(10)
  void clearApprovalModel() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get subagentModel => $_getSZ(10);
  @$pb.TagNumber(11)
  set subagentModel($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasSubagentModel() => $_has(10);
  @$pb.TagNumber(11)
  void clearSubagentModel() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get memoryModel => $_getSZ(11);
  @$pb.TagNumber(12)
  set memoryModel($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasMemoryModel() => $_has(11);
  @$pb.TagNumber(12)
  void clearMemoryModel() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get cacheModel => $_getSZ(12);
  @$pb.TagNumber(13)
  set cacheModel($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasCacheModel() => $_has(12);
  @$pb.TagNumber(13)
  void clearCacheModel() => $_clearField(13);
}

/// RuntimeContractIssue 一条契约违约(机器可读 code 见 runtime 包的 Violation*)。
class RuntimeContractIssue extends $pb.GeneratedMessage {
  factory RuntimeContractIssue({
    $core.String? code,
    $core.String? detail,
  }) {
    final result = RuntimeContractIssue._();
    if (code != null) result.code = code;
    if (detail != null) result.detail = detail;
    return result;
  }

  RuntimeContractIssue._();

  factory RuntimeContractIssue.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeContractIssue()..mergeFromBuffer(data, registry);
  factory RuntimeContractIssue.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeContractIssue()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RuntimeContractIssue',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RuntimeContractIssue.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..aOS(2, _omitFieldNames ? '' : 'detail')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeContractIssue clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeContractIssue copyWith(void Function(RuntimeContractIssue) updates) =>
      super.copyWith((message) => updates(message as RuntimeContractIssue))
          as RuntimeContractIssue;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use RuntimeContractIssue() / RuntimeContractIssue.new instead')
  static RuntimeContractIssue create() => RuntimeContractIssue._();
  static $pb.GeneratedMessage $_createMessage() => RuntimeContractIssue._();
  @$core.override
  RuntimeContractIssue createEmptyInstance() => RuntimeContractIssue._();
  @$core.pragma('dart2js:noInline')
  static RuntimeContractIssue getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RuntimeContractIssue>(
          RuntimeContractIssue.$_createMessage);
  static RuntimeContractIssue? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get detail => $_getSZ(1);
  @$pb.TagNumber(2)
  set detail($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDetail() => $_has(1);
  @$pb.TagNumber(2)
  void clearDetail() => $_clearField(2);
}

/// RuntimeLimitation 一条"我明确不支持"的声明及其原因。
class RuntimeLimitation extends $pb.GeneratedMessage {
  factory RuntimeLimitation({
    $core.String? capability,
    $core.String? reason,
  }) {
    final result = RuntimeLimitation._();
    if (capability != null) result.capability = capability;
    if (reason != null) result.reason = reason;
    return result;
  }

  RuntimeLimitation._();

  factory RuntimeLimitation.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeLimitation()..mergeFromBuffer(data, registry);
  factory RuntimeLimitation.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RuntimeLimitation()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RuntimeLimitation',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RuntimeLimitation.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'capability')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeLimitation clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RuntimeLimitation copyWith(void Function(RuntimeLimitation) updates) =>
      super.copyWith((message) => updates(message as RuntimeLimitation))
          as RuntimeLimitation;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RuntimeLimitation() / RuntimeLimitation.new instead')
  static RuntimeLimitation create() => RuntimeLimitation._();
  static $pb.GeneratedMessage $_createMessage() => RuntimeLimitation._();
  @$core.override
  RuntimeLimitation createEmptyInstance() => RuntimeLimitation._();
  @$core.pragma('dart2js:noInline')
  static RuntimeLimitation getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RuntimeLimitation>(
          RuntimeLimitation.$_createMessage);
  static RuntimeLimitation? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get capability => $_getSZ(0);
  @$pb.TagNumber(1)
  set capability($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCapability() => $_has(0);
  @$pb.TagNumber(1)
  void clearCapability() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

class GetAgentRuntimeRequest extends $pb.GeneratedMessage {
  factory GetAgentRuntimeRequest({
    $core.String? agentId,
  }) {
    final result = GetAgentRuntimeRequest._();
    if (agentId != null) result.agentId = agentId;
    return result;
  }

  GetAgentRuntimeRequest._();

  factory GetAgentRuntimeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetAgentRuntimeRequest()..mergeFromBuffer(data, registry);
  factory GetAgentRuntimeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetAgentRuntimeRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAgentRuntimeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetAgentRuntimeRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAgentRuntimeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAgentRuntimeRequest copyWith(
          void Function(GetAgentRuntimeRequest) updates) =>
      super.copyWith((message) => updates(message as GetAgentRuntimeRequest))
          as GetAgentRuntimeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetAgentRuntimeRequest() / GetAgentRuntimeRequest.new instead')
  static GetAgentRuntimeRequest create() => GetAgentRuntimeRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetAgentRuntimeRequest._();
  @$core.override
  GetAgentRuntimeRequest createEmptyInstance() => GetAgentRuntimeRequest._();
  @$core.pragma('dart2js:noInline')
  static GetAgentRuntimeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAgentRuntimeRequest>(
          GetAgentRuntimeRequest.$_createMessage);
  static GetAgentRuntimeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);
}

class AgentRuntimeResponse extends $pb.GeneratedMessage {
  factory AgentRuntimeResponse({
    RuntimeContractReport? contract,
    $core.Iterable<ResourceMetric>? resources,
    $fixnum.Int64? sampledAtUnixMs,
    AgentMemoryUsage? memory,
    AgentCacheUsage? cache,
    AgentCheckpointUsage? checkpoint,
    AgentRecoveryState? recovery,
  }) {
    final result = AgentRuntimeResponse._();
    if (contract != null) result.contract = contract;
    if (resources != null) result.resources.addAll(resources);
    if (sampledAtUnixMs != null) result.sampledAtUnixMs = sampledAtUnixMs;
    if (memory != null) result.memory = memory;
    if (cache != null) result.cache = cache;
    if (checkpoint != null) result.checkpoint = checkpoint;
    if (recovery != null) result.recovery = recovery;
    return result;
  }

  AgentRuntimeResponse._();

  factory AgentRuntimeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentRuntimeResponse()..mergeFromBuffer(data, registry);
  factory AgentRuntimeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentRuntimeResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentRuntimeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentRuntimeResponse.$_createMessage)
    ..aOM<RuntimeContractReport>(1, _omitFieldNames ? '' : 'contract',
        subBuilder: RuntimeContractReport.$_createMessage)
    ..pPM<ResourceMetric>(2, _omitFieldNames ? '' : 'resources',
        subBuilder: ResourceMetric.$_createMessage)
    ..aInt64(3, _omitFieldNames ? '' : 'sampledAtUnixMs')
    ..aOM<AgentMemoryUsage>(4, _omitFieldNames ? '' : 'memory',
        subBuilder: AgentMemoryUsage.$_createMessage)
    ..aOM<AgentCacheUsage>(5, _omitFieldNames ? '' : 'cache',
        subBuilder: AgentCacheUsage.$_createMessage)
    ..aOM<AgentCheckpointUsage>(6, _omitFieldNames ? '' : 'checkpoint',
        subBuilder: AgentCheckpointUsage.$_createMessage)
    ..aOM<AgentRecoveryState>(7, _omitFieldNames ? '' : 'recovery',
        subBuilder: AgentRecoveryState.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentRuntimeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentRuntimeResponse copyWith(void Function(AgentRuntimeResponse) updates) =>
      super.copyWith((message) => updates(message as AgentRuntimeResponse))
          as AgentRuntimeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AgentRuntimeResponse() / AgentRuntimeResponse.new instead')
  static AgentRuntimeResponse create() => AgentRuntimeResponse._();
  static $pb.GeneratedMessage $_createMessage() => AgentRuntimeResponse._();
  @$core.override
  AgentRuntimeResponse createEmptyInstance() => AgentRuntimeResponse._();
  @$core.pragma('dart2js:noInline')
  static AgentRuntimeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AgentRuntimeResponse>(
          AgentRuntimeResponse.$_createMessage);
  static AgentRuntimeResponse? _defaultInstance;

  @$pb.TagNumber(1)
  RuntimeContractReport get contract => $_getN(0);
  @$pb.TagNumber(1)
  set contract(RuntimeContractReport value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasContract() => $_has(0);
  @$pb.TagNumber(1)
  void clearContract() => $_clearField(1);
  @$pb.TagNumber(1)
  RuntimeContractReport ensureContract() => $_ensure(0);

  /// 该 agent 作用域下当前活跃的资源租约(复用 GetResourceUsage 的同一个 message)。
  @$pb.TagNumber(2)
  $pb.PbList<ResourceMetric> get resources => $_getList(1);

  @$pb.TagNumber(3)
  $fixnum.Int64 get sampledAtUnixMs => $_getI64(2);
  @$pb.TagNumber(3)
  set sampledAtUnixMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSampledAtUnixMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearSampledAtUnixMs() => $_clearField(3);

  /// 规格 §9.5「每家都能显示 memory/cache/resource/checkpoint/recovery 状态」的
  /// 后四项。resource 见上面的 resources。
  @$pb.TagNumber(4)
  AgentMemoryUsage get memory => $_getN(3);
  @$pb.TagNumber(4)
  set memory(AgentMemoryUsage value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasMemory() => $_has(3);
  @$pb.TagNumber(4)
  void clearMemory() => $_clearField(4);
  @$pb.TagNumber(4)
  AgentMemoryUsage ensureMemory() => $_ensure(3);

  @$pb.TagNumber(5)
  AgentCacheUsage get cache => $_getN(4);
  @$pb.TagNumber(5)
  set cache(AgentCacheUsage value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCache() => $_has(4);
  @$pb.TagNumber(5)
  void clearCache() => $_clearField(5);
  @$pb.TagNumber(5)
  AgentCacheUsage ensureCache() => $_ensure(4);

  @$pb.TagNumber(6)
  AgentCheckpointUsage get checkpoint => $_getN(5);
  @$pb.TagNumber(6)
  set checkpoint(AgentCheckpointUsage value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasCheckpoint() => $_has(5);
  @$pb.TagNumber(6)
  void clearCheckpoint() => $_clearField(6);
  @$pb.TagNumber(6)
  AgentCheckpointUsage ensureCheckpoint() => $_ensure(5);

  @$pb.TagNumber(7)
  AgentRecoveryState get recovery => $_getN(6);
  @$pb.TagNumber(7)
  set recovery(AgentRecoveryState value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasRecovery() => $_has(6);
  @$pb.TagNumber(7)
  void clearRecovery() => $_clearField(7);
  @$pb.TagNumber(7)
  AgentRecoveryState ensureRecovery() => $_ensure(6);
}

/// AgentMemoryUsage 是一个 agent 名下的语义记忆(按 agent 隔离聚合)。
class AgentMemoryUsage extends $pb.GeneratedMessage {
  factory AgentMemoryUsage({
    $core.int? threads,
    $core.int? items,
    $fixnum.Int64? bytes,
    $core.Iterable<$core.MapEntry<$core.String, $core.int>>? kinds,
  }) {
    final result = AgentMemoryUsage._();
    if (threads != null) result.threads = threads;
    if (items != null) result.items = items;
    if (bytes != null) result.bytes = bytes;
    if (kinds != null) result.kinds.addEntries(kinds);
    return result;
  }

  AgentMemoryUsage._();

  factory AgentMemoryUsage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentMemoryUsage()..mergeFromBuffer(data, registry);
  factory AgentMemoryUsage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentMemoryUsage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentMemoryUsage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentMemoryUsage.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'threads')
    ..aI(2, _omitFieldNames ? '' : 'items')
    ..aInt64(3, _omitFieldNames ? '' : 'bytes')
    ..m<$core.String, $core.int>(4, _omitFieldNames ? '' : 'kinds',
        entryClassName: 'AgentMemoryUsage.KindsEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.O3,
        packageName: const $pb.PackageName('pocketagent.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentMemoryUsage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentMemoryUsage copyWith(void Function(AgentMemoryUsage) updates) =>
      super.copyWith((message) => updates(message as AgentMemoryUsage))
          as AgentMemoryUsage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AgentMemoryUsage() / AgentMemoryUsage.new instead')
  static AgentMemoryUsage create() => AgentMemoryUsage._();
  static $pb.GeneratedMessage $_createMessage() => AgentMemoryUsage._();
  @$core.override
  AgentMemoryUsage createEmptyInstance() => AgentMemoryUsage._();
  @$core.pragma('dart2js:noInline')
  static AgentMemoryUsage getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AgentMemoryUsage>(
          AgentMemoryUsage.$_createMessage);
  static AgentMemoryUsage? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get threads => $_getIZ(0);
  @$pb.TagNumber(1)
  set threads($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreads() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreads() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get items => $_getIZ(1);
  @$pb.TagNumber(2)
  set items($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasItems() => $_has(1);
  @$pb.TagNumber(2)
  void clearItems() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get bytes => $_getI64(2);
  @$pb.TagNumber(3)
  set bytes($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBytes() => $_has(2);
  @$pb.TagNumber(3)
  void clearBytes() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbMap<$core.String, $core.int> get kinds => $_getMap(3);
}

/// AgentCacheUsage 是一个 agent 名下的可再生缓存(按命名空间前缀聚合)。
class AgentCacheUsage extends $pb.GeneratedMessage {
  factory AgentCacheUsage({
    $core.int? namespaces,
    $core.int? entries,
    $fixnum.Int64? bytes,
    $fixnum.Int64? hits,
    $fixnum.Int64? misses,
    $fixnum.Int64? evictions,
  }) {
    final result = AgentCacheUsage._();
    if (namespaces != null) result.namespaces = namespaces;
    if (entries != null) result.entries = entries;
    if (bytes != null) result.bytes = bytes;
    if (hits != null) result.hits = hits;
    if (misses != null) result.misses = misses;
    if (evictions != null) result.evictions = evictions;
    return result;
  }

  AgentCacheUsage._();

  factory AgentCacheUsage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentCacheUsage()..mergeFromBuffer(data, registry);
  factory AgentCacheUsage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentCacheUsage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentCacheUsage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentCacheUsage.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'namespaces')
    ..aI(2, _omitFieldNames ? '' : 'entries')
    ..aInt64(3, _omitFieldNames ? '' : 'bytes')
    ..aInt64(4, _omitFieldNames ? '' : 'hits')
    ..aInt64(5, _omitFieldNames ? '' : 'misses')
    ..aInt64(6, _omitFieldNames ? '' : 'evictions')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentCacheUsage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentCacheUsage copyWith(void Function(AgentCacheUsage) updates) =>
      super.copyWith((message) => updates(message as AgentCacheUsage))
          as AgentCacheUsage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AgentCacheUsage() / AgentCacheUsage.new instead')
  static AgentCacheUsage create() => AgentCacheUsage._();
  static $pb.GeneratedMessage $_createMessage() => AgentCacheUsage._();
  @$core.override
  AgentCacheUsage createEmptyInstance() => AgentCacheUsage._();
  @$core.pragma('dart2js:noInline')
  static AgentCacheUsage getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AgentCacheUsage>(
          AgentCacheUsage.$_createMessage);
  static AgentCacheUsage? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get namespaces => $_getIZ(0);
  @$pb.TagNumber(1)
  set namespaces($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNamespaces() => $_has(0);
  @$pb.TagNumber(1)
  void clearNamespaces() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get entries => $_getIZ(1);
  @$pb.TagNumber(2)
  set entries($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEntries() => $_has(1);
  @$pb.TagNumber(2)
  void clearEntries() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get bytes => $_getI64(2);
  @$pb.TagNumber(3)
  set bytes($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBytes() => $_has(2);
  @$pb.TagNumber(3)
  void clearBytes() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get hits => $_getI64(3);
  @$pb.TagNumber(4)
  set hits($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasHits() => $_has(3);
  @$pb.TagNumber(4)
  void clearHits() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get misses => $_getI64(4);
  @$pb.TagNumber(5)
  set misses($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMisses() => $_has(4);
  @$pb.TagNumber(5)
  void clearMisses() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get evictions => $_getI64(5);
  @$pb.TagNumber(6)
  set evictions($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasEvictions() => $_has(5);
  @$pb.TagNumber(6)
  void clearEvictions() => $_clearField(6);
}

/// AgentCheckpointUsage 是一个 agent 名下的 checkpoint。
class AgentCheckpointUsage extends $pb.GeneratedMessage {
  factory AgentCheckpointUsage({
    $core.int? count,
    $fixnum.Int64? bytes,
  }) {
    final result = AgentCheckpointUsage._();
    if (count != null) result.count = count;
    if (bytes != null) result.bytes = bytes;
    return result;
  }

  AgentCheckpointUsage._();

  factory AgentCheckpointUsage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentCheckpointUsage()..mergeFromBuffer(data, registry);
  factory AgentCheckpointUsage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentCheckpointUsage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentCheckpointUsage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentCheckpointUsage.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'count')
    ..aInt64(2, _omitFieldNames ? '' : 'bytes')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentCheckpointUsage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentCheckpointUsage copyWith(void Function(AgentCheckpointUsage) updates) =>
      super.copyWith((message) => updates(message as AgentCheckpointUsage))
          as AgentCheckpointUsage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AgentCheckpointUsage() / AgentCheckpointUsage.new instead')
  static AgentCheckpointUsage create() => AgentCheckpointUsage._();
  static $pb.GeneratedMessage $_createMessage() => AgentCheckpointUsage._();
  @$core.override
  AgentCheckpointUsage createEmptyInstance() => AgentCheckpointUsage._();
  @$core.pragma('dart2js:noInline')
  static AgentCheckpointUsage getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AgentCheckpointUsage>(
          AgentCheckpointUsage.$_createMessage);
  static AgentCheckpointUsage? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get count => $_getIZ(0);
  @$pb.TagNumber(1)
  set count($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearCount() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get bytes => $_getI64(1);
  @$pb.TagNumber(2)
  set bytes($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBytes() => $_has(1);
  @$pb.TagNumber(2)
  void clearBytes() => $_clearField(2);
}

/// AgentRecoveryState 是最近一次恢复尝试的结果。
///
/// 【reported 为什么必须存在】目前 daemon 侧**还没有产出方**:恢复是跨重启的事件,
/// 而 RecoveryState 挂在 per-thread 的临时 AgentRuntime 上,没人能填它。所以这里
/// 恒为 reported=false —— 客户端据此显示"该 agent 未上报恢复状态",而不是把
/// "没这一项"当成"恢复失败"。等产出方接上之后这个字段自然转 true。
class AgentRecoveryState extends $pb.GeneratedMessage {
  factory AgentRecoveryState({
    $core.bool? reported,
    $core.bool? attempted,
    $core.bool? succeeded,
    $core.String? from,
    $core.String? reason,
    $core.String? at,
  }) {
    final result = AgentRecoveryState._();
    if (reported != null) result.reported = reported;
    if (attempted != null) result.attempted = attempted;
    if (succeeded != null) result.succeeded = succeeded;
    if (from != null) result.from = from;
    if (reason != null) result.reason = reason;
    if (at != null) result.at = at;
    return result;
  }

  AgentRecoveryState._();

  factory AgentRecoveryState.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentRecoveryState()..mergeFromBuffer(data, registry);
  factory AgentRecoveryState.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentRecoveryState()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentRecoveryState',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentRecoveryState.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'reported')
    ..aOB(2, _omitFieldNames ? '' : 'attempted')
    ..aOB(3, _omitFieldNames ? '' : 'succeeded')
    ..aOS(4, _omitFieldNames ? '' : 'from')
    ..aOS(5, _omitFieldNames ? '' : 'reason')
    ..aOS(6, _omitFieldNames ? '' : 'at')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentRecoveryState clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentRecoveryState copyWith(void Function(AgentRecoveryState) updates) =>
      super.copyWith((message) => updates(message as AgentRecoveryState))
          as AgentRecoveryState;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AgentRecoveryState() / AgentRecoveryState.new instead')
  static AgentRecoveryState create() => AgentRecoveryState._();
  static $pb.GeneratedMessage $_createMessage() => AgentRecoveryState._();
  @$core.override
  AgentRecoveryState createEmptyInstance() => AgentRecoveryState._();
  @$core.pragma('dart2js:noInline')
  static AgentRecoveryState getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AgentRecoveryState>(
          AgentRecoveryState.$_createMessage);
  static AgentRecoveryState? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get reported => $_getBF(0);
  @$pb.TagNumber(1)
  set reported($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReported() => $_has(0);
  @$pb.TagNumber(1)
  void clearReported() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get attempted => $_getBF(1);
  @$pb.TagNumber(2)
  set attempted($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAttempted() => $_has(1);
  @$pb.TagNumber(2)
  void clearAttempted() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get succeeded => $_getBF(2);
  @$pb.TagNumber(3)
  set succeeded($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSucceeded() => $_has(2);
  @$pb.TagNumber(3)
  void clearSucceeded() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get from => $_getSZ(3);
  @$pb.TagNumber(4)
  set from($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFrom() => $_has(3);
  @$pb.TagNumber(4)
  void clearFrom() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get reason => $_getSZ(4);
  @$pb.TagNumber(5)
  set reason($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasReason() => $_has(4);
  @$pb.TagNumber(5)
  void clearReason() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get at => $_getSZ(5);
  @$pb.TagNumber(6)
  set at($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasAt() => $_has(5);
  @$pb.TagNumber(6)
  void clearAt() => $_clearField(6);
}

class ListThreadMemoriesRequest extends $pb.GeneratedMessage {
  factory ListThreadMemoriesRequest({
    $core.String? agentId,
  }) {
    final result = ListThreadMemoriesRequest._();
    if (agentId != null) result.agentId = agentId;
    return result;
  }

  ListThreadMemoriesRequest._();

  factory ListThreadMemoriesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadMemoriesRequest()..mergeFromBuffer(data, registry);
  factory ListThreadMemoriesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadMemoriesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadMemoriesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListThreadMemoriesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadMemoriesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadMemoriesRequest copyWith(
          void Function(ListThreadMemoriesRequest) updates) =>
      super.copyWith((message) => updates(message as ListThreadMemoriesRequest))
          as ListThreadMemoriesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListThreadMemoriesRequest() / ListThreadMemoriesRequest.new instead')
  static ListThreadMemoriesRequest create() => ListThreadMemoriesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListThreadMemoriesRequest._();
  @$core.override
  ListThreadMemoriesRequest createEmptyInstance() =>
      ListThreadMemoriesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListThreadMemoriesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadMemoriesRequest>(
          ListThreadMemoriesRequest.$_createMessage);
  static ListThreadMemoriesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);
}

/// ThreadMemoryRow 是清单里一个会话的记忆合计。
class ThreadMemoryRow extends $pb.GeneratedMessage {
  factory ThreadMemoryRow({
    $core.String? threadId,
    $core.int? entries,
    $fixnum.Int64? bytes,
    $fixnum.Int64? seq,
    $fixnum.Int64? lastCheckpointUnixMs,
    $core.bool? loaded,
  }) {
    final result = ThreadMemoryRow._();
    if (threadId != null) result.threadId = threadId;
    if (entries != null) result.entries = entries;
    if (bytes != null) result.bytes = bytes;
    if (seq != null) result.seq = seq;
    if (lastCheckpointUnixMs != null)
      result.lastCheckpointUnixMs = lastCheckpointUnixMs;
    if (loaded != null) result.loaded = loaded;
    return result;
  }

  ThreadMemoryRow._();

  factory ThreadMemoryRow.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadMemoryRow()..mergeFromBuffer(data, registry);
  factory ThreadMemoryRow.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThreadMemoryRow()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThreadMemoryRow',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ThreadMemoryRow.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aI(2, _omitFieldNames ? '' : 'entries')
    ..aInt64(3, _omitFieldNames ? '' : 'bytes')
    ..aInt64(4, _omitFieldNames ? '' : 'seq')
    ..aInt64(5, _omitFieldNames ? '' : 'lastCheckpointUnixMs')
    ..aOB(6, _omitFieldNames ? '' : 'loaded')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadMemoryRow clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThreadMemoryRow copyWith(void Function(ThreadMemoryRow) updates) =>
      super.copyWith((message) => updates(message as ThreadMemoryRow))
          as ThreadMemoryRow;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ThreadMemoryRow() / ThreadMemoryRow.new instead')
  static ThreadMemoryRow create() => ThreadMemoryRow._();
  static $pb.GeneratedMessage $_createMessage() => ThreadMemoryRow._();
  @$core.override
  ThreadMemoryRow createEmptyInstance() => ThreadMemoryRow._();
  @$core.pragma('dart2js:noInline')
  static ThreadMemoryRow getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ThreadMemoryRow>(
          ThreadMemoryRow.$_createMessage);
  static ThreadMemoryRow? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get entries => $_getIZ(1);
  @$pb.TagNumber(2)
  set entries($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEntries() => $_has(1);
  @$pb.TagNumber(2)
  void clearEntries() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get bytes => $_getI64(2);
  @$pb.TagNumber(3)
  set bytes($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBytes() => $_has(2);
  @$pb.TagNumber(3)
  void clearBytes() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get seq => $_getI64(3);
  @$pb.TagNumber(4)
  set seq($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSeq() => $_has(3);
  @$pb.TagNumber(4)
  void clearSeq() => $_clearField(4);

  /// 最后一次 checkpoint 落盘时刻(文件 mtime);0 = 从未落盘。
  @$pb.TagNumber(5)
  $fixnum.Int64 get lastCheckpointUnixMs => $_getI64(4);
  @$pb.TagNumber(5)
  set lastCheckpointUnixMs($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasLastCheckpointUnixMs() => $_has(4);
  @$pb.TagNumber(5)
  void clearLastCheckpointUnixMs() => $_clearField(5);

  /// true = 本进程已装载(内存实时值);false = 重启后从未触碰,值来自盘上落盘那一刻
  /// 的快照。两种口径必须分开:客户端把快照当实时值展示,就是在编一个"看起来更新"的数。
  @$pb.TagNumber(6)
  $core.bool get loaded => $_getBF(5);
  @$pb.TagNumber(6)
  set loaded($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasLoaded() => $_has(5);
  @$pb.TagNumber(6)
  void clearLoaded() => $_clearField(6);
}

class ListThreadMemoriesResponse extends $pb.GeneratedMessage {
  factory ListThreadMemoriesResponse({
    $core.Iterable<ThreadMemoryRow>? rows,
    $core.int? totalThreads,
    $core.int? totalEntries,
    $fixnum.Int64? totalBytes,
    $fixnum.Int64? sampledAtUnixMs,
  }) {
    final result = ListThreadMemoriesResponse._();
    if (rows != null) result.rows.addAll(rows);
    if (totalThreads != null) result.totalThreads = totalThreads;
    if (totalEntries != null) result.totalEntries = totalEntries;
    if (totalBytes != null) result.totalBytes = totalBytes;
    if (sampledAtUnixMs != null) result.sampledAtUnixMs = sampledAtUnixMs;
    return result;
  }

  ListThreadMemoriesResponse._();

  factory ListThreadMemoriesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadMemoriesResponse()..mergeFromBuffer(data, registry);
  factory ListThreadMemoriesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListThreadMemoriesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListThreadMemoriesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListThreadMemoriesResponse.$_createMessage)
    ..pPM<ThreadMemoryRow>(1, _omitFieldNames ? '' : 'rows',
        subBuilder: ThreadMemoryRow.$_createMessage)
    ..aI(2, _omitFieldNames ? '' : 'totalThreads')
    ..aI(3, _omitFieldNames ? '' : 'totalEntries')
    ..aInt64(4, _omitFieldNames ? '' : 'totalBytes')
    ..aInt64(5, _omitFieldNames ? '' : 'sampledAtUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadMemoriesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListThreadMemoriesResponse copyWith(
          void Function(ListThreadMemoriesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListThreadMemoriesResponse))
          as ListThreadMemoriesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListThreadMemoriesResponse() / ListThreadMemoriesResponse.new instead')
  static ListThreadMemoriesResponse create() => ListThreadMemoriesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListThreadMemoriesResponse._();
  @$core.override
  ListThreadMemoriesResponse createEmptyInstance() =>
      ListThreadMemoriesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListThreadMemoriesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListThreadMemoriesResponse>(
          ListThreadMemoriesResponse.$_createMessage);
  static ListThreadMemoriesResponse? _defaultInstance;

  /// 按字节降序(daemon 侧排好;同字节按 thread_id 字典序),客户端保持次序不再重排。
  @$pb.TagNumber(1)
  $pb.PbList<ThreadMemoryRow> get rows => $_getList(0);

  @$pb.TagNumber(2)
  $core.int get totalThreads => $_getIZ(1);
  @$pb.TagNumber(2)
  set totalThreads($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTotalThreads() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotalThreads() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get totalEntries => $_getIZ(2);
  @$pb.TagNumber(3)
  set totalEntries($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalEntries() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalEntries() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get totalBytes => $_getI64(3);
  @$pb.TagNumber(4)
  set totalBytes($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTotalBytes() => $_has(3);
  @$pb.TagNumber(4)
  void clearTotalBytes() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get sampledAtUnixMs => $_getI64(4);
  @$pb.TagNumber(5)
  set sampledAtUnixMs($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSampledAtUnixMs() => $_has(4);
  @$pb.TagNumber(5)
  void clearSampledAtUnixMs() => $_clearField(5);
}

class ClearThreadMemoryRequest extends $pb.GeneratedMessage {
  factory ClearThreadMemoryRequest({
    $core.String? agentId,
    $core.String? threadId,
  }) {
    final result = ClearThreadMemoryRequest._();
    if (agentId != null) result.agentId = agentId;
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  ClearThreadMemoryRequest._();

  factory ClearThreadMemoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearThreadMemoryRequest()..mergeFromBuffer(data, registry);
  factory ClearThreadMemoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearThreadMemoryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearThreadMemoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ClearThreadMemoryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearThreadMemoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearThreadMemoryRequest copyWith(
          void Function(ClearThreadMemoryRequest) updates) =>
      super.copyWith((message) => updates(message as ClearThreadMemoryRequest))
          as ClearThreadMemoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ClearThreadMemoryRequest() / ClearThreadMemoryRequest.new instead')
  static ClearThreadMemoryRequest create() => ClearThreadMemoryRequest._();
  static $pb.GeneratedMessage $_createMessage() => ClearThreadMemoryRequest._();
  @$core.override
  ClearThreadMemoryRequest createEmptyInstance() =>
      ClearThreadMemoryRequest._();
  @$core.pragma('dart2js:noInline')
  static ClearThreadMemoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearThreadMemoryRequest>(
          ClearThreadMemoryRequest.$_createMessage);
  static ClearThreadMemoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get threadId => $_getSZ(1);
  @$pb.TagNumber(2)
  set threadId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasThreadId() => $_has(1);
  @$pb.TagNumber(2)
  void clearThreadId() => $_clearField(2);
}

class ClearThreadMemoryResponse extends $pb.GeneratedMessage {
  factory ClearThreadMemoryResponse({
    $core.int? totalThreads,
    $core.int? totalEntries,
    $fixnum.Int64? totalBytes,
  }) {
    final result = ClearThreadMemoryResponse._();
    if (totalThreads != null) result.totalThreads = totalThreads;
    if (totalEntries != null) result.totalEntries = totalEntries;
    if (totalBytes != null) result.totalBytes = totalBytes;
    return result;
  }

  ClearThreadMemoryResponse._();

  factory ClearThreadMemoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearThreadMemoryResponse()..mergeFromBuffer(data, registry);
  factory ClearThreadMemoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClearThreadMemoryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearThreadMemoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ClearThreadMemoryResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'totalThreads')
    ..aI(2, _omitFieldNames ? '' : 'totalEntries')
    ..aInt64(3, _omitFieldNames ? '' : 'totalBytes')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearThreadMemoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearThreadMemoryResponse copyWith(
          void Function(ClearThreadMemoryResponse) updates) =>
      super.copyWith((message) => updates(message as ClearThreadMemoryResponse))
          as ClearThreadMemoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ClearThreadMemoryResponse() / ClearThreadMemoryResponse.new instead')
  static ClearThreadMemoryResponse create() => ClearThreadMemoryResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ClearThreadMemoryResponse._();
  @$core.override
  ClearThreadMemoryResponse createEmptyInstance() =>
      ClearThreadMemoryResponse._();
  @$core.pragma('dart2js:noInline')
  static ClearThreadMemoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearThreadMemoryResponse>(
          ClearThreadMemoryResponse.$_createMessage);
  static ClearThreadMemoryResponse? _defaultInstance;

  /// 清除后该 agent 名下的剩余合计(客户端拿它刷新合计行,不必再发一次 List)。
  @$pb.TagNumber(1)
  $core.int get totalThreads => $_getIZ(0);
  @$pb.TagNumber(1)
  set totalThreads($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTotalThreads() => $_has(0);
  @$pb.TagNumber(1)
  void clearTotalThreads() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get totalEntries => $_getIZ(1);
  @$pb.TagNumber(2)
  set totalEntries($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTotalEntries() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotalEntries() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get totalBytes => $_getI64(2);
  @$pb.TagNumber(3)
  set totalBytes($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTotalBytes() => $_has(2);
  @$pb.TagNumber(3)
  void clearTotalBytes() => $_clearField(3);
}

class GetControlPlaneRequest extends $pb.GeneratedMessage {
  factory GetControlPlaneRequest() => GetControlPlaneRequest._();

  GetControlPlaneRequest._();

  factory GetControlPlaneRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetControlPlaneRequest()..mergeFromBuffer(data, registry);
  factory GetControlPlaneRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetControlPlaneRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetControlPlaneRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetControlPlaneRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetControlPlaneRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetControlPlaneRequest copyWith(
          void Function(GetControlPlaneRequest) updates) =>
      super.copyWith((message) => updates(message as GetControlPlaneRequest))
          as GetControlPlaneRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetControlPlaneRequest() / GetControlPlaneRequest.new instead')
  static GetControlPlaneRequest create() => GetControlPlaneRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetControlPlaneRequest._();
  @$core.override
  GetControlPlaneRequest createEmptyInstance() => GetControlPlaneRequest._();
  @$core.pragma('dart2js:noInline')
  static GetControlPlaneRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetControlPlaneRequest>(
          GetControlPlaneRequest.$_createMessage);
  static GetControlPlaneRequest? _defaultInstance;
}

/// ControlPlane 是 daemon 全部可控点的一次性快照。
///
/// 【为什么是"域"的列表而不是大平铺】预算、协同、记忆……各有各的控制通道与
/// 展示形态;按域分组后,客户端一页按域渲染,新域上线只是多一个条目,不必改版面。
/// 【为什么全部人话由 daemon 给】summary/label/hint 与事件翻译(event_translate)
/// 同一条纪律:daemon 给中文人话、不编造,状态取不到就如实写"未检测到/未报告";
/// 客户端只按 id 匹配与排版,不做任何术语翻译。
class ControlPlane extends $pb.GeneratedMessage {
  factory ControlPlane({
    $core.String? daemonVersion,
    $core.String? hostName,
    $core.String? os,
    $fixnum.Int64? uptimeUnixMs,
    $core.Iterable<ControlDomain>? domains,
    $core.String? dataDirLabel,
  }) {
    final result = ControlPlane._();
    if (daemonVersion != null) result.daemonVersion = daemonVersion;
    if (hostName != null) result.hostName = hostName;
    if (os != null) result.os = os;
    if (uptimeUnixMs != null) result.uptimeUnixMs = uptimeUnixMs;
    if (domains != null) result.domains.addAll(domains);
    if (dataDirLabel != null) result.dataDirLabel = dataDirLabel;
    return result;
  }

  ControlPlane._();

  factory ControlPlane.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ControlPlane()..mergeFromBuffer(data, registry);
  factory ControlPlane.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ControlPlane()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ControlPlane',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ControlPlane.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'daemonVersion')
    ..aOS(2, _omitFieldNames ? '' : 'hostName')
    ..aOS(3, _omitFieldNames ? '' : 'os')
    ..aInt64(4, _omitFieldNames ? '' : 'uptimeUnixMs')
    ..pPM<ControlDomain>(5, _omitFieldNames ? '' : 'domains',
        subBuilder: ControlDomain.$_createMessage)
    ..aOS(6, _omitFieldNames ? '' : 'dataDirLabel')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ControlPlane clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ControlPlane copyWith(void Function(ControlPlane) updates) =>
      super.copyWith((message) => updates(message as ControlPlane))
          as ControlPlane;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ControlPlane() / ControlPlane.new instead')
  static ControlPlane create() => ControlPlane._();
  static $pb.GeneratedMessage $_createMessage() => ControlPlane._();
  @$core.override
  ControlPlane createEmptyInstance() => ControlPlane._();
  @$core.pragma('dart2js:noInline')
  static ControlPlane getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ControlPlane>(
          ControlPlane.$_createMessage);
  static ControlPlane? _defaultInstance;

  /// 服务与身份。
  @$pb.TagNumber(1)
  $core.String get daemonVersion => $_getSZ(0);
  @$pb.TagNumber(1)
  set daemonVersion($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDaemonVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearDaemonVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get hostName => $_getSZ(1);
  @$pb.TagNumber(2)
  set hostName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHostName() => $_has(1);
  @$pb.TagNumber(2)
  void clearHostName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get os => $_getSZ(2);
  @$pb.TagNumber(3)
  set os($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOs() => $_has(2);
  @$pb.TagNumber(3)
  void clearOs() => $_clearField(3);

  /// daemon 进程的启动时刻(unix 毫秒)。
  /// 【为什么给"时刻"不给"时长"]客户端拿当前时刻一减即可显示"已运行多久",
  /// 且不受快照往返延迟影响;时区与"多久前"的排版都留给客户端。
  @$pb.TagNumber(4)
  $fixnum.Int64 get uptimeUnixMs => $_getI64(3);
  @$pb.TagNumber(4)
  set uptimeUnixMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasUptimeUnixMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearUptimeUnixMs() => $_clearField(4);

  /// 各功能域状态与控制句柄(每域一个条目,客户端按 ControlDomain.id 呈现)。
  @$pb.TagNumber(5)
  $pb.PbList<ControlDomain> get domains => $_getList(4);

  /// 数据目录落在哪个标准位置(如 LOCALAPPDATA / 家目录/.pocketagentd)。
  /// 【为什么只给标签不给路径】绝对路径里的用户名属于敏感信息,诊断事实用不着它;
  /// 要看落盘位置时,各策略响应里的 policy_path 已有完整路径。
  @$pb.TagNumber(6)
  $core.String get dataDirLabel => $_getSZ(5);
  @$pb.TagNumber(6)
  set dataDirLabel($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDataDirLabel() => $_has(5);
  @$pb.TagNumber(6)
  void clearDataDirLabel() => $_clearField(6);
}

/// ControlDomain 是一个功能域的当前状态。
class ControlDomain extends $pb.GeneratedMessage {
  factory ControlDomain({
    $core.String? id,
    $core.String? title,
    $core.String? summary,
    $core.bool? writable,
    $core.int? statusCode,
    $core.Iterable<ControlItem>? items,
  }) {
    final result = ControlDomain._();
    if (id != null) result.id = id;
    if (title != null) result.title = title;
    if (summary != null) result.summary = summary;
    if (writable != null) result.writable = writable;
    if (statusCode != null) result.statusCode = statusCode;
    if (items != null) result.items.addAll(items);
    return result;
  }

  ControlDomain._();

  factory ControlDomain.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ControlDomain()..mergeFromBuffer(data, registry);
  factory ControlDomain.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ControlDomain()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ControlDomain',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ControlDomain.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'title')
    ..aOS(3, _omitFieldNames ? '' : 'summary')
    ..aOB(4, _omitFieldNames ? '' : 'writable')
    ..aI(5, _omitFieldNames ? '' : 'statusCode')
    ..pPM<ControlItem>(6, _omitFieldNames ? '' : 'items',
        subBuilder: ControlItem.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ControlDomain clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ControlDomain copyWith(void Function(ControlDomain) updates) =>
      super.copyWith((message) => updates(message as ControlDomain))
          as ControlDomain;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ControlDomain() / ControlDomain.new instead')
  static ControlDomain create() => ControlDomain._();
  static $pb.GeneratedMessage $_createMessage() => ControlDomain._();
  @$core.override
  ControlDomain createEmptyInstance() => ControlDomain._();
  @$core.pragma('dart2js:noInline')
  static ControlDomain getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ControlDomain>(
          ControlDomain.$_createMessage);
  static ControlDomain? _defaultInstance;

  /// 域稳定键:budget / collab / agents / memory / dialysis / approvals /
  /// transports / resources。客户端按 id 匹配,不按 title(title 是给人看的)。
  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 人话标题(daemon 给,客户端不翻译)。
  @$pb.TagNumber(2)
  $core.String get title => $_getSZ(1);
  @$pb.TagNumber(2)
  set title($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearTitle() => $_clearField(2);

  /// 一句话当前状态(人话;如实描述,不编造)。
  @$pb.TagNumber(3)
  $core.String get summary => $_getSZ(2);
  @$pb.TagNumber(3)
  set summary($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSummary() => $_has(2);
  @$pb.TagNumber(3)
  void clearSummary() => $_clearField(3);

  /// 该域是否有写通道。写仍走各自既有 RPC —— 这里只是让客户端知道"可改",
  /// 不必为每个域再实现一遍能力探测。
  @$pb.TagNumber(4)
  $core.bool get writable => $_getBF(3);
  @$pb.TagNumber(4)
  set writable($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasWritable() => $_has(3);
  @$pb.TagNumber(4)
  void clearWritable() => $_clearField(4);

  /// 0=正常 1=有注意项 2=受限。如实映射:2 可能只是"未装配/取不到",
  /// 客户端把 summary 原文显示出来即可,不要自行把 2 渲染成"故障"。
  @$pb.TagNumber(5)
  $core.int get statusCode => $_getIZ(4);
  @$pb.TagNumber(5)
  set statusCode($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStatusCode() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatusCode() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<ControlItem> get items => $_getList(5);
}

/// ControlItem 是域内的一个可控点或事实点。
class ControlItem extends $pb.GeneratedMessage {
  factory ControlItem({
    $core.String? id,
    $core.String? label,
    $core.String? current,
    $core.String? kind,
    $core.Iterable<$core.String>? options,
    $core.String? hint,
  }) {
    final result = ControlItem._();
    if (id != null) result.id = id;
    if (label != null) result.label = label;
    if (current != null) result.current = current;
    if (kind != null) result.kind = kind;
    if (options != null) result.options.addAll(options);
    if (hint != null) result.hint = hint;
    return result;
  }

  ControlItem._();

  factory ControlItem.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ControlItem()..mergeFromBuffer(data, registry);
  factory ControlItem.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ControlItem()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ControlItem',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ControlItem.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'label')
    ..aOS(3, _omitFieldNames ? '' : 'current')
    ..aOS(4, _omitFieldNames ? '' : 'kind')
    ..pPS(5, _omitFieldNames ? '' : 'options')
    ..aOS(6, _omitFieldNames ? '' : 'hint')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ControlItem clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ControlItem copyWith(void Function(ControlItem) updates) =>
      super.copyWith((message) => updates(message as ControlItem))
          as ControlItem;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ControlItem() / ControlItem.new instead')
  static ControlItem create() => ControlItem._();
  static $pb.GeneratedMessage $_createMessage() => ControlItem._();
  @$core.override
  ControlItem createEmptyInstance() => ControlItem._();
  @$core.pragma('dart2js:noInline')
  static ControlItem getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ControlItem>(
          ControlItem.$_createMessage);
  static ControlItem? _defaultInstance;

  /// 域内稳定键,如 budget.global.ram_event_buffer_bytes。
  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  /// 人话名。
  @$pb.TagNumber(2)
  $core.String get label => $_getSZ(1);
  @$pb.TagNumber(2)
  set label($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLabel() => $_has(1);
  @$pb.TagNumber(2)
  void clearLabel() => $_clearField(2);

  /// 当前值(字符串化:数值/开关/原文)。
  @$pb.TagNumber(3)
  $core.String get current => $_getSZ(2);
  @$pb.TagNumber(3)
  set current($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCurrent() => $_has(2);
  @$pb.TagNumber(3)
  void clearCurrent() => $_clearField(3);

  /// 呈现形态:toggle / select / number / info(只读事实)。
  @$pb.TagNumber(4)
  $core.String get kind => $_getSZ(3);
  @$pb.TagNumber(4)
  set kind($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasKind() => $_has(3);
  @$pb.TagNumber(4)
  void clearKind() => $_clearField(4);

  /// select 型候选;其它 kind 为空。
  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get options => $_getList(4);

  /// 一句说明(为什么有这个限制/改动影响什么)。
  @$pb.TagNumber(6)
  $core.String get hint => $_getSZ(5);
  @$pb.TagNumber(6)
  set hint($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasHint() => $_has(5);
  @$pb.TagNumber(6)
  void clearHint() => $_clearField(6);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
