// This is a generated file - do not edit.
//
// Generated from v1/model.proto.

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

class GetCodexUsageRequest extends $pb.GeneratedMessage {
  factory GetCodexUsageRequest() => GetCodexUsageRequest._();

  GetCodexUsageRequest._();

  factory GetCodexUsageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCodexUsageRequest()..mergeFromBuffer(data, registry);
  factory GetCodexUsageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCodexUsageRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCodexUsageRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetCodexUsageRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCodexUsageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCodexUsageRequest copyWith(void Function(GetCodexUsageRequest) updates) =>
      super.copyWith((message) => updates(message as GetCodexUsageRequest))
          as GetCodexUsageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetCodexUsageRequest() / GetCodexUsageRequest.new instead')
  static GetCodexUsageRequest create() => GetCodexUsageRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetCodexUsageRequest._();
  @$core.override
  GetCodexUsageRequest createEmptyInstance() => GetCodexUsageRequest._();
  @$core.pragma('dart2js:noInline')
  static GetCodexUsageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCodexUsageRequest>(
          GetCodexUsageRequest.$_createMessage);
  static GetCodexUsageRequest? _defaultInstance;
}

/// 一个限流窗口(primary / secondary 用的是同一个结构)。
class RateLimitWindow extends $pb.GeneratedMessage {
  factory RateLimitWindow({
    $core.int? usedPercent,
    $fixnum.Int64? resetsAtUnixS,
    $fixnum.Int64? windowDurationMins,
  }) {
    final result = RateLimitWindow._();
    if (usedPercent != null) result.usedPercent = usedPercent;
    if (resetsAtUnixS != null) result.resetsAtUnixS = resetsAtUnixS;
    if (windowDurationMins != null)
      result.windowDurationMins = windowDurationMins;
    return result;
  }

  RateLimitWindow._();

  factory RateLimitWindow.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RateLimitWindow()..mergeFromBuffer(data, registry);
  factory RateLimitWindow.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      RateLimitWindow()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RateLimitWindow',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: RateLimitWindow.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'usedPercent')
    ..aInt64(2, _omitFieldNames ? '' : 'resetsAtUnixS')
    ..aInt64(3, _omitFieldNames ? '' : 'windowDurationMins')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RateLimitWindow clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RateLimitWindow copyWith(void Function(RateLimitWindow) updates) =>
      super.copyWith((message) => updates(message as RateLimitWindow))
          as RateLimitWindow;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use RateLimitWindow() / RateLimitWindow.new instead')
  static RateLimitWindow create() => RateLimitWindow._();
  static $pb.GeneratedMessage $_createMessage() => RateLimitWindow._();
  @$core.override
  RateLimitWindow createEmptyInstance() => RateLimitWindow._();
  @$core.pragma('dart2js:noInline')
  static RateLimitWindow getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RateLimitWindow>(
          RateLimitWindow.$_createMessage);
  static RateLimitWindow? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get usedPercent => $_getIZ(0);
  @$pb.TagNumber(1)
  set usedPercent($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUsedPercent() => $_has(0);
  @$pb.TagNumber(1)
  void clearUsedPercent() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get resetsAtUnixS => $_getI64(1);
  @$pb.TagNumber(2)
  set resetsAtUnixS($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasResetsAtUnixS() => $_has(1);
  @$pb.TagNumber(2)
  void clearResetsAtUnixS() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get windowDurationMins => $_getI64(2);
  @$pb.TagNumber(3)
  set windowDurationMins($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWindowDurationMins() => $_has(2);
  @$pb.TagNumber(3)
  void clearWindowDurationMins() => $_clearField(3);
}

class CodexUsage extends $pb.GeneratedMessage {
  factory CodexUsage({
    $core.String? planType,
    RateLimitWindow? primary,
    RateLimitWindow? secondary,
    $core.bool? hasCredits,
    $core.bool? unlimitedCredits,
    $core.String? creditBalance,
    $core.bool? spendControlReached,
    $core.String? rateLimitReachedType,
    $fixnum.Int64? lifetimeTokens,
    $fixnum.Int64? peakDailyTokens,
    $core.int? currentStreakDays,
    $fixnum.Int64? fetchedAtUnixMs,
    $core.String? error,
  }) {
    final result = CodexUsage._();
    if (planType != null) result.planType = planType;
    if (primary != null) result.primary = primary;
    if (secondary != null) result.secondary = secondary;
    if (hasCredits != null) result.hasCredits = hasCredits;
    if (unlimitedCredits != null) result.unlimitedCredits = unlimitedCredits;
    if (creditBalance != null) result.creditBalance = creditBalance;
    if (spendControlReached != null)
      result.spendControlReached = spendControlReached;
    if (rateLimitReachedType != null)
      result.rateLimitReachedType = rateLimitReachedType;
    if (lifetimeTokens != null) result.lifetimeTokens = lifetimeTokens;
    if (peakDailyTokens != null) result.peakDailyTokens = peakDailyTokens;
    if (currentStreakDays != null) result.currentStreakDays = currentStreakDays;
    if (fetchedAtUnixMs != null) result.fetchedAtUnixMs = fetchedAtUnixMs;
    if (error != null) result.error = error;
    return result;
  }

  CodexUsage._();

  factory CodexUsage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexUsage()..mergeFromBuffer(data, registry);
  factory CodexUsage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexUsage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexUsage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexUsage.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'planType')
    ..aOM<RateLimitWindow>(2, _omitFieldNames ? '' : 'primary',
        subBuilder: RateLimitWindow.$_createMessage)
    ..aOM<RateLimitWindow>(3, _omitFieldNames ? '' : 'secondary',
        subBuilder: RateLimitWindow.$_createMessage)
    ..aOB(4, _omitFieldNames ? '' : 'hasCredits')
    ..aOB(5, _omitFieldNames ? '' : 'unlimitedCredits')
    ..aOS(6, _omitFieldNames ? '' : 'creditBalance')
    ..aOB(7, _omitFieldNames ? '' : 'spendControlReached')
    ..aOS(8, _omitFieldNames ? '' : 'rateLimitReachedType')
    ..aInt64(9, _omitFieldNames ? '' : 'lifetimeTokens')
    ..aInt64(10, _omitFieldNames ? '' : 'peakDailyTokens')
    ..aI(11, _omitFieldNames ? '' : 'currentStreakDays')
    ..aInt64(12, _omitFieldNames ? '' : 'fetchedAtUnixMs')
    ..aOS(13, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexUsage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexUsage copyWith(void Function(CodexUsage) updates) =>
      super.copyWith((message) => updates(message as CodexUsage)) as CodexUsage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexUsage() / CodexUsage.new instead')
  static CodexUsage create() => CodexUsage._();
  static $pb.GeneratedMessage $_createMessage() => CodexUsage._();
  @$core.override
  CodexUsage createEmptyInstance() => CodexUsage._();
  @$core.pragma('dart2js:noInline')
  static CodexUsage getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CodexUsage>(CodexUsage.$_createMessage);
  static CodexUsage? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get planType => $_getSZ(0);
  @$pb.TagNumber(1)
  set planType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPlanType() => $_has(0);
  @$pb.TagNumber(1)
  void clearPlanType() => $_clearField(1);

  @$pb.TagNumber(2)
  RateLimitWindow get primary => $_getN(1);
  @$pb.TagNumber(2)
  set primary(RateLimitWindow value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPrimary() => $_has(1);
  @$pb.TagNumber(2)
  void clearPrimary() => $_clearField(2);
  @$pb.TagNumber(2)
  RateLimitWindow ensurePrimary() => $_ensure(1);

  @$pb.TagNumber(3)
  RateLimitWindow get secondary => $_getN(2);
  @$pb.TagNumber(3)
  set secondary(RateLimitWindow value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSecondary() => $_has(2);
  @$pb.TagNumber(3)
  void clearSecondary() => $_clearField(3);
  @$pb.TagNumber(3)
  RateLimitWindow ensureSecondary() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.bool get hasCredits => $_getBF(3);
  @$pb.TagNumber(4)
  set hasCredits($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasHasCredits() => $_has(3);
  @$pb.TagNumber(4)
  void clearHasCredits() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get unlimitedCredits => $_getBF(4);
  @$pb.TagNumber(5)
  set unlimitedCredits($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasUnlimitedCredits() => $_has(4);
  @$pb.TagNumber(5)
  void clearUnlimitedCredits() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get creditBalance => $_getSZ(5);
  @$pb.TagNumber(6)
  set creditBalance($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCreditBalance() => $_has(5);
  @$pb.TagNumber(6)
  void clearCreditBalance() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get spendControlReached => $_getBF(6);
  @$pb.TagNumber(7)
  set spendControlReached($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSpendControlReached() => $_has(6);
  @$pb.TagNumber(7)
  void clearSpendControlReached() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get rateLimitReachedType => $_getSZ(7);
  @$pb.TagNumber(8)
  set rateLimitReachedType($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasRateLimitReachedType() => $_has(7);
  @$pb.TagNumber(8)
  void clearRateLimitReachedType() => $_clearField(8);

  @$pb.TagNumber(9)
  $fixnum.Int64 get lifetimeTokens => $_getI64(8);
  @$pb.TagNumber(9)
  set lifetimeTokens($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasLifetimeTokens() => $_has(8);
  @$pb.TagNumber(9)
  void clearLifetimeTokens() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get peakDailyTokens => $_getI64(9);
  @$pb.TagNumber(10)
  set peakDailyTokens($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasPeakDailyTokens() => $_has(9);
  @$pb.TagNumber(10)
  void clearPeakDailyTokens() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.int get currentStreakDays => $_getIZ(10);
  @$pb.TagNumber(11)
  set currentStreakDays($core.int value) => $_setSignedInt32(10, value);
  @$pb.TagNumber(11)
  $core.bool hasCurrentStreakDays() => $_has(10);
  @$pb.TagNumber(11)
  void clearCurrentStreakDays() => $_clearField(11);

  @$pb.TagNumber(12)
  $fixnum.Int64 get fetchedAtUnixMs => $_getI64(11);
  @$pb.TagNumber(12)
  set fetchedAtUnixMs($fixnum.Int64 value) => $_setInt64(11, value);
  @$pb.TagNumber(12)
  $core.bool hasFetchedAtUnixMs() => $_has(11);
  @$pb.TagNumber(12)
  void clearFetchedAtUnixMs() => $_clearField(12);

  /// 拉取失败的原因(未登录 / 地区不支持 / 网络)。非空时手机端显示"暂时读不到 + 原因",
  /// 而不是展示一堆 0 —— 0 会被误读成"额度用完了"。
  @$pb.TagNumber(13)
  $core.String get error => $_getSZ(12);
  @$pb.TagNumber(13)
  set error($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasError() => $_has(12);
  @$pb.TagNumber(13)
  void clearError() => $_clearField(13);
}

class ListCodexCapabilitiesRequest extends $pb.GeneratedMessage {
  factory ListCodexCapabilitiesRequest() => ListCodexCapabilitiesRequest._();

  ListCodexCapabilitiesRequest._();

  factory ListCodexCapabilitiesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexCapabilitiesRequest()..mergeFromBuffer(data, registry);
  factory ListCodexCapabilitiesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexCapabilitiesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCodexCapabilitiesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCodexCapabilitiesRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexCapabilitiesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexCapabilitiesRequest copyWith(
          void Function(ListCodexCapabilitiesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListCodexCapabilitiesRequest))
          as ListCodexCapabilitiesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCodexCapabilitiesRequest() / ListCodexCapabilitiesRequest.new instead')
  static ListCodexCapabilitiesRequest create() =>
      ListCodexCapabilitiesRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListCodexCapabilitiesRequest._();
  @$core.override
  ListCodexCapabilitiesRequest createEmptyInstance() =>
      ListCodexCapabilitiesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListCodexCapabilitiesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCodexCapabilitiesRequest>(
          ListCodexCapabilitiesRequest.$_createMessage);
  static ListCodexCapabilitiesRequest? _defaultInstance;
}

/// 一条能力(skill / hook / plugin 共用)。
class CodexCapability extends $pb.GeneratedMessage {
  factory CodexCapability({
    $core.String? kind,
    $core.String? name,
    $core.String? description,
    $core.bool? enabled,
    $core.String? path,
    $core.String? disabledReason,
    $core.String? version,
    $core.String? source,
    $core.String? status,
  }) {
    final result = CodexCapability._();
    if (kind != null) result.kind = kind;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (enabled != null) result.enabled = enabled;
    if (path != null) result.path = path;
    if (disabledReason != null) result.disabledReason = disabledReason;
    if (version != null) result.version = version;
    if (source != null) result.source = source;
    if (status != null) result.status = status;
    return result;
  }

  CodexCapability._();

  factory CodexCapability.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexCapability()..mergeFromBuffer(data, registry);
  factory CodexCapability.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexCapability()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexCapability',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexCapability.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'kind')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOB(4, _omitFieldNames ? '' : 'enabled')
    ..aOS(5, _omitFieldNames ? '' : 'path')
    ..aOS(6, _omitFieldNames ? '' : 'disabledReason')
    ..aOS(7, _omitFieldNames ? '' : 'version')
    ..aOS(8, _omitFieldNames ? '' : 'source')
    ..aOS(9, _omitFieldNames ? '' : 'status')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexCapability clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexCapability copyWith(void Function(CodexCapability) updates) =>
      super.copyWith((message) => updates(message as CodexCapability))
          as CodexCapability;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexCapability() / CodexCapability.new instead')
  static CodexCapability create() => CodexCapability._();
  static $pb.GeneratedMessage $_createMessage() => CodexCapability._();
  @$core.override
  CodexCapability createEmptyInstance() => CodexCapability._();
  @$core.pragma('dart2js:noInline')
  static CodexCapability getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CodexCapability>(
          CodexCapability.$_createMessage);
  static CodexCapability? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get kind => $_getSZ(0);
  @$pb.TagNumber(1)
  set kind($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasKind() => $_has(0);
  @$pb.TagNumber(1)
  void clearKind() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get enabled => $_getBF(3);
  @$pb.TagNumber(4)
  set enabled($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEnabled() => $_has(3);
  @$pb.TagNumber(4)
  void clearEnabled() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get path => $_getSZ(4);
  @$pb.TagNumber(5)
  set path($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPath() => $_has(4);
  @$pb.TagNumber(5)
  void clearPath() => $_clearField(5);

  /// disabled_reason 直接回答"为什么用不了",比 enabled=false 有用得多。
  @$pb.TagNumber(6)
  $core.String get disabledReason => $_getSZ(5);
  @$pb.TagNumber(6)
  set disabledReason($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDisabledReason() => $_has(5);
  @$pb.TagNumber(6)
  void clearDisabledReason() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get version => $_getSZ(6);
  @$pb.TagNumber(7)
  set version($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasVersion() => $_has(6);
  @$pb.TagNumber(7)
  void clearVersion() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get source => $_getSZ(7);
  @$pb.TagNumber(8)
  set source($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasSource() => $_has(7);
  @$pb.TagNumber(8)
  void clearSource() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get status => $_getSZ(8);
  @$pb.TagNumber(9)
  set status($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasStatus() => $_has(8);
  @$pb.TagNumber(9)
  void clearStatus() => $_clearField(9);
}

class ListCodexCapabilitiesResponse extends $pb.GeneratedMessage {
  factory ListCodexCapabilitiesResponse({
    $core.Iterable<CodexCapability>? items,
    $core.String? error,
  }) {
    final result = ListCodexCapabilitiesResponse._();
    if (items != null) result.items.addAll(items);
    if (error != null) result.error = error;
    return result;
  }

  ListCodexCapabilitiesResponse._();

  factory ListCodexCapabilitiesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexCapabilitiesResponse()..mergeFromBuffer(data, registry);
  factory ListCodexCapabilitiesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListCodexCapabilitiesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListCodexCapabilitiesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListCodexCapabilitiesResponse.$_createMessage)
    ..pPM<CodexCapability>(1, _omitFieldNames ? '' : 'items',
        subBuilder: CodexCapability.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexCapabilitiesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListCodexCapabilitiesResponse copyWith(
          void Function(ListCodexCapabilitiesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListCodexCapabilitiesResponse))
          as ListCodexCapabilitiesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListCodexCapabilitiesResponse() / ListCodexCapabilitiesResponse.new instead')
  static ListCodexCapabilitiesResponse create() =>
      ListCodexCapabilitiesResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListCodexCapabilitiesResponse._();
  @$core.override
  ListCodexCapabilitiesResponse createEmptyInstance() =>
      ListCodexCapabilitiesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListCodexCapabilitiesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListCodexCapabilitiesResponse>(
          ListCodexCapabilitiesResponse.$_createMessage);
  static ListCodexCapabilitiesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<CodexCapability> get items => $_getList(0);

  /// 非空表示本次读取失败或部分失败;空列表不再被误解为“没有能力”。
  @$pb.TagNumber(2)
  $core.String get error => $_getSZ(1);
  @$pb.TagNumber(2)
  set error($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasError() => $_has(1);
  @$pb.TagNumber(2)
  void clearError() => $_clearField(2);
}

class GetCodexSettingsRequest extends $pb.GeneratedMessage {
  factory GetCodexSettingsRequest({
    $core.String? agentId,
    $core.String? threadId,
  }) {
    final result = GetCodexSettingsRequest._();
    if (agentId != null) result.agentId = agentId;
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  GetCodexSettingsRequest._();

  factory GetCodexSettingsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCodexSettingsRequest()..mergeFromBuffer(data, registry);
  factory GetCodexSettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetCodexSettingsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCodexSettingsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetCodexSettingsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCodexSettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCodexSettingsRequest copyWith(
          void Function(GetCodexSettingsRequest) updates) =>
      super.copyWith((message) => updates(message as GetCodexSettingsRequest))
          as GetCodexSettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetCodexSettingsRequest() / GetCodexSettingsRequest.new instead')
  static GetCodexSettingsRequest create() => GetCodexSettingsRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetCodexSettingsRequest._();
  @$core.override
  GetCodexSettingsRequest createEmptyInstance() => GetCodexSettingsRequest._();
  @$core.pragma('dart2js:noInline')
  static GetCodexSettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCodexSettingsRequest>(
          GetCodexSettingsRequest.$_createMessage);
  static GetCodexSettingsRequest? _defaultInstance;

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

class CodexSettings extends $pb.GeneratedMessage {
  factory CodexSettings({
    $core.String? approvalPolicy,
    $core.String? sandbox,
    $core.String? account,
    $core.String? model,
    $core.String? effort,
    $core.bool? hideAgentReasoning,
    $core.String? modelVerbosity,
    $core.String? modelReasoningSummary,
    $core.bool? disablePasteBurst,
    $core.Iterable<$core.String>? allowedApprovalPolicies,
    $core.Iterable<$core.String>? allowedSandboxModes,
    $core.Iterable<$core.String>? allowedWebSearchModes,
    $core.bool? managed,
    $core.String? managedNote,
    $core.Iterable<CodexSettingStatus>? settingStatus,
    CodexDesktopConfigSummary? desktopConfigSummary,
    CodexProviderCaps? providerCaps,
    $core.String? sandboxReadiness,
    $core.String? desktopEchoError,
    $core.int? appliedThreads,
    $core.String? nativeMode,
    $core.Iterable<$core.String>? nativeModes,
    $core.String? nativePermissionsNote,
  }) {
    final result = CodexSettings._();
    if (approvalPolicy != null) result.approvalPolicy = approvalPolicy;
    if (sandbox != null) result.sandbox = sandbox;
    if (account != null) result.account = account;
    if (model != null) result.model = model;
    if (effort != null) result.effort = effort;
    if (hideAgentReasoning != null)
      result.hideAgentReasoning = hideAgentReasoning;
    if (modelVerbosity != null) result.modelVerbosity = modelVerbosity;
    if (modelReasoningSummary != null)
      result.modelReasoningSummary = modelReasoningSummary;
    if (disablePasteBurst != null) result.disablePasteBurst = disablePasteBurst;
    if (allowedApprovalPolicies != null)
      result.allowedApprovalPolicies.addAll(allowedApprovalPolicies);
    if (allowedSandboxModes != null)
      result.allowedSandboxModes.addAll(allowedSandboxModes);
    if (allowedWebSearchModes != null)
      result.allowedWebSearchModes.addAll(allowedWebSearchModes);
    if (managed != null) result.managed = managed;
    if (managedNote != null) result.managedNote = managedNote;
    if (settingStatus != null) result.settingStatus.addAll(settingStatus);
    if (desktopConfigSummary != null)
      result.desktopConfigSummary = desktopConfigSummary;
    if (providerCaps != null) result.providerCaps = providerCaps;
    if (sandboxReadiness != null) result.sandboxReadiness = sandboxReadiness;
    if (desktopEchoError != null) result.desktopEchoError = desktopEchoError;
    if (appliedThreads != null) result.appliedThreads = appliedThreads;
    if (nativeMode != null) result.nativeMode = nativeMode;
    if (nativeModes != null) result.nativeModes.addAll(nativeModes);
    if (nativePermissionsNote != null)
      result.nativePermissionsNote = nativePermissionsNote;
    return result;
  }

  CodexSettings._();

  factory CodexSettings.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexSettings()..mergeFromBuffer(data, registry);
  factory CodexSettings.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexSettings()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexSettings',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexSettings.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'approvalPolicy')
    ..aOS(2, _omitFieldNames ? '' : 'sandbox')
    ..aOS(3, _omitFieldNames ? '' : 'account')
    ..aOS(4, _omitFieldNames ? '' : 'model')
    ..aOS(5, _omitFieldNames ? '' : 'effort')
    ..aOB(6, _omitFieldNames ? '' : 'hideAgentReasoning')
    ..aOS(7, _omitFieldNames ? '' : 'modelVerbosity')
    ..aOS(8, _omitFieldNames ? '' : 'modelReasoningSummary')
    ..aOB(9, _omitFieldNames ? '' : 'disablePasteBurst')
    ..pPS(10, _omitFieldNames ? '' : 'allowedApprovalPolicies')
    ..pPS(11, _omitFieldNames ? '' : 'allowedSandboxModes')
    ..pPS(12, _omitFieldNames ? '' : 'allowedWebSearchModes')
    ..aOB(13, _omitFieldNames ? '' : 'managed')
    ..aOS(14, _omitFieldNames ? '' : 'managedNote')
    ..pPM<CodexSettingStatus>(15, _omitFieldNames ? '' : 'settingStatus',
        subBuilder: CodexSettingStatus.$_createMessage)
    ..aOM<CodexDesktopConfigSummary>(
        16, _omitFieldNames ? '' : 'desktopConfigSummary',
        subBuilder: CodexDesktopConfigSummary.$_createMessage)
    ..aOM<CodexProviderCaps>(17, _omitFieldNames ? '' : 'providerCaps',
        subBuilder: CodexProviderCaps.$_createMessage)
    ..aOS(18, _omitFieldNames ? '' : 'sandboxReadiness')
    ..aOS(19, _omitFieldNames ? '' : 'desktopEchoError')
    ..aI(20, _omitFieldNames ? '' : 'appliedThreads')
    ..aOS(21, _omitFieldNames ? '' : 'nativeMode')
    ..pPS(22, _omitFieldNames ? '' : 'nativeModes')
    ..aOS(23, _omitFieldNames ? '' : 'nativePermissionsNote')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexSettings clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexSettings copyWith(void Function(CodexSettings) updates) =>
      super.copyWith((message) => updates(message as CodexSettings))
          as CodexSettings;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexSettings() / CodexSettings.new instead')
  static CodexSettings create() => CodexSettings._();
  static $pb.GeneratedMessage $_createMessage() => CodexSettings._();
  @$core.override
  CodexSettings createEmptyInstance() => CodexSettings._();
  @$core.pragma('dart2js:noInline')
  static CodexSettings getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CodexSettings>(
          CodexSettings.$_createMessage);
  static CodexSettings? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get approvalPolicy => $_getSZ(0);
  @$pb.TagNumber(1)
  set approvalPolicy($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasApprovalPolicy() => $_has(0);
  @$pb.TagNumber(1)
  void clearApprovalPolicy() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sandbox => $_getSZ(1);
  @$pb.TagNumber(2)
  set sandbox($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSandbox() => $_has(1);
  @$pb.TagNumber(2)
  void clearSandbox() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get account => $_getSZ(2);
  @$pb.TagNumber(3)
  set account($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAccount() => $_has(2);
  @$pb.TagNumber(3)
  void clearAccount() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get model => $_getSZ(3);
  @$pb.TagNumber(4)
  set model($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasModel() => $_has(3);
  @$pb.TagNumber(4)
  void clearModel() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get effort => $_getSZ(4);
  @$pb.TagNumber(5)
  set effort($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasEffort() => $_has(4);
  @$pb.TagNumber(5)
  void clearEffort() => $_clearField(5);

  /// ---- 行为与降噪(第 27 轮新增,字段号从 6 起,旧字段语义一字未动) ----
  /// 【边界】这些值只作用于 daemon 拉起的 app-server 子进程(内存态,不落盘),
  /// 因此只影响从 App 发起的任务,绝不会改写用户桌面 codex 的 config.toml。
  @$pb.TagNumber(6)
  $core.bool get hideAgentReasoning => $_getBF(5);
  @$pb.TagNumber(6)
  set hideAgentReasoning($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasHideAgentReasoning() => $_has(5);
  @$pb.TagNumber(6)
  void clearHideAgentReasoning() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get modelVerbosity => $_getSZ(6);
  @$pb.TagNumber(7)
  set modelVerbosity($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasModelVerbosity() => $_has(6);
  @$pb.TagNumber(7)
  void clearModelVerbosity() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get modelReasoningSummary => $_getSZ(7);
  @$pb.TagNumber(8)
  set modelReasoningSummary($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasModelReasoningSummary() => $_has(7);
  @$pb.TagNumber(8)
  void clearModelReasoningSummary() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get disablePasteBurst => $_getBF(8);
  @$pb.TagNumber(9)
  set disablePasteBurst($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasDisablePasteBurst() => $_has(8);
  @$pb.TagNumber(9)
  void clearDisablePasteBurst() => $_clearField(9);

  /// ---- 托管策略(configRequirements/read)。字段号从 10 起,旧字段语义一字未动 ----
  /// 【为什么必须带回来】用户桌面上的 codex 可能被管理员/MDM 策略约束
  /// (requirements.toml),某些取值是被禁的。手机上照旧渲染出来,用户就会
  /// "点了 → codex 拒绝 → App 无反馈"。这三个列表是空的表示该项无限制;
  /// 非空时手机端**只渲染列表里的取值**。
  @$pb.TagNumber(10)
  $pb.PbList<$core.String> get allowedApprovalPolicies => $_getList(9);

  @$pb.TagNumber(11)
  $pb.PbList<$core.String> get allowedSandboxModes => $_getList(10);

  @$pb.TagNumber(12)
  $pb.PbList<$core.String> get allowedWebSearchModes => $_getList(11);

  @$pb.TagNumber(13)
  $core.bool get managed => $_getBF(12);
  @$pb.TagNumber(13)
  set managed($core.bool value) => $_setBool(12, value);
  @$pb.TagNumber(13)
  $core.bool hasManaged() => $_has(12);
  @$pb.TagNumber(13)
  void clearManaged() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get managedNote => $_getSZ(13);
  @$pb.TagNumber(14)
  set managedNote($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasManagedNote() => $_has(13);
  @$pb.TagNumber(14)
  void clearManagedNote() => $_clearField(14);

  /// ---- 逐项真实状态(第 2 轮新增,字段号从 15 起,旧字段语义一字未动)----
  /// 【为什么必须逐项回报】"设置页所有开关点了都没用"这个用户反馈的根子,是
  /// 手机端**无法区分**四种截然不同的状态:已生效 / 已保存但没打到 codex /
  /// 当前 codex 版本压根不支持 / 连接失败。以前一律显示"已保存",用户以为生效了。
  /// 现在由 daemon 按真实能力逐项回报,手机端据此如实渲染,不再假装成功。
  @$pb.TagNumber(15)
  $pb.PbList<CodexSettingStatus> get settingStatus => $_getList(14);

  /// ---- 桌面只读回显(新增,字段号从 16 起,旧字段语义一字未动)----
  /// 【与上面 1~15 的分工】上面的值是 daemon 自己落盘/下发的取值;下面三个字段
  /// 回答另一件事:"电脑端 codex 此刻实际生效的是什么"。数据全部来自只读桌面口子
  /// (config/read、modelProvider/capabilities/read、windowsSandbox/readiness,
  /// 见 daemon internal/agent/codex_read.go),daemon 绝不借它们写任何桌面配置。
  /// 读取失败时 desktop_echo_error 说明原因,各值留空 —— "读不到"不与"没有"混淆。
  @$pb.TagNumber(16)
  CodexDesktopConfigSummary get desktopConfigSummary => $_getN(15);
  @$pb.TagNumber(16)
  set desktopConfigSummary(CodexDesktopConfigSummary value) =>
      $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasDesktopConfigSummary() => $_has(15);
  @$pb.TagNumber(16)
  void clearDesktopConfigSummary() => $_clearField(16);
  @$pb.TagNumber(16)
  CodexDesktopConfigSummary ensureDesktopConfigSummary() => $_ensure(15);

  @$pb.TagNumber(17)
  CodexProviderCaps get providerCaps => $_getN(16);
  @$pb.TagNumber(17)
  set providerCaps(CodexProviderCaps value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasProviderCaps() => $_has(16);
  @$pb.TagNumber(17)
  void clearProviderCaps() => $_clearField(17);
  @$pb.TagNumber(17)
  CodexProviderCaps ensureProviderCaps() => $_ensure(16);

  @$pb.TagNumber(18)
  $core.String get sandboxReadiness => $_getSZ(17);
  @$pb.TagNumber(18)
  set sandboxReadiness($core.String value) => $_setString(17, value);
  @$pb.TagNumber(18)
  $core.bool hasSandboxReadiness() => $_has(17);
  @$pb.TagNumber(18)
  void clearSandboxReadiness() => $_clearField(18);

  @$pb.TagNumber(19)
  $core.String get desktopEchoError => $_getSZ(18);
  @$pb.TagNumber(19)
  set desktopEchoError($core.String value) => $_setString(18, value);
  @$pb.TagNumber(19)
  $core.bool hasDesktopEchoError() => $_has(18);
  @$pb.TagNumber(19)
  void clearDesktopEchoError() => $_clearField(19);

  /// ---- "应用到所有会话(含已有)"的回执(仅 SetCodexSettings 返回时非零)----
  /// 请求里 apply_to_existing_threads=true 时,daemon 把新值批量写进所有存量
  /// codex 会话的设置快照,这里是实际写到的会话数;GetCodexSettings / 未勾选时为 0。
  @$pb.TagNumber(20)
  $core.int get appliedThreads => $_getIZ(19);
  @$pb.TagNumber(20)
  set appliedThreads($core.int value) => $_setSignedInt32(19, value);
  @$pb.TagNumber(20)
  $core.bool hasAppliedThreads() => $_has(19);
  @$pb.TagNumber(20)
  void clearAppliedThreads() => $_clearField(20);

  @$pb.TagNumber(21)
  $core.String get nativeMode => $_getSZ(20);
  @$pb.TagNumber(21)
  set nativeMode($core.String value) => $_setString(20, value);
  @$pb.TagNumber(21)
  $core.bool hasNativeMode() => $_has(20);
  @$pb.TagNumber(21)
  void clearNativeMode() => $_clearField(21);

  @$pb.TagNumber(22)
  $pb.PbList<$core.String> get nativeModes => $_getList(21);

  @$pb.TagNumber(23)
  $core.String get nativePermissionsNote => $_getSZ(22);
  @$pb.TagNumber(23)
  set nativePermissionsNote($core.String value) => $_setString(22, value);
  @$pb.TagNumber(23)
  $core.bool hasNativePermissionsNote() => $_has(22);
  @$pb.TagNumber(23)
  void clearNativePermissionsNote() => $_clearField(23);
}

/// CodexDesktopConfigSummary 电脑端 config/read 的归一快照。
///
/// 只挑与手机设置页七项可调项能对照的值(原始 config 对象里的 Raw 不透传,
/// 避免把 desktop 大对象刷到手机上)。空字段 = 电脑端该项未显式配置
/// (codex 使用内置默认),手机端对该项不显示"电脑端当前"副文案。
class CodexDesktopConfigSummary extends $pb.GeneratedMessage {
  factory CodexDesktopConfigSummary({
    $core.String? model,
    $core.String? provider,
    $core.String? effort,
    $core.String? summary,
    $core.String? verbosity,
    $core.String? sandboxMode,
    $core.String? approvalPolicy,
  }) {
    final result = CodexDesktopConfigSummary._();
    if (model != null) result.model = model;
    if (provider != null) result.provider = provider;
    if (effort != null) result.effort = effort;
    if (summary != null) result.summary = summary;
    if (verbosity != null) result.verbosity = verbosity;
    if (sandboxMode != null) result.sandboxMode = sandboxMode;
    if (approvalPolicy != null) result.approvalPolicy = approvalPolicy;
    return result;
  }

  CodexDesktopConfigSummary._();

  factory CodexDesktopConfigSummary.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexDesktopConfigSummary()..mergeFromBuffer(data, registry);
  factory CodexDesktopConfigSummary.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexDesktopConfigSummary()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexDesktopConfigSummary',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexDesktopConfigSummary.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'model')
    ..aOS(2, _omitFieldNames ? '' : 'provider')
    ..aOS(3, _omitFieldNames ? '' : 'effort')
    ..aOS(4, _omitFieldNames ? '' : 'summary')
    ..aOS(5, _omitFieldNames ? '' : 'verbosity')
    ..aOS(6, _omitFieldNames ? '' : 'sandboxMode')
    ..aOS(7, _omitFieldNames ? '' : 'approvalPolicy')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexDesktopConfigSummary clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexDesktopConfigSummary copyWith(
          void Function(CodexDesktopConfigSummary) updates) =>
      super.copyWith((message) => updates(message as CodexDesktopConfigSummary))
          as CodexDesktopConfigSummary;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CodexDesktopConfigSummary() / CodexDesktopConfigSummary.new instead')
  static CodexDesktopConfigSummary create() => CodexDesktopConfigSummary._();
  static $pb.GeneratedMessage $_createMessage() =>
      CodexDesktopConfigSummary._();
  @$core.override
  CodexDesktopConfigSummary createEmptyInstance() =>
      CodexDesktopConfigSummary._();
  @$core.pragma('dart2js:noInline')
  static CodexDesktopConfigSummary getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CodexDesktopConfigSummary>(
          CodexDesktopConfigSummary.$_createMessage);
  static CodexDesktopConfigSummary? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get model => $_getSZ(0);
  @$pb.TagNumber(1)
  set model($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasModel() => $_has(0);
  @$pb.TagNumber(1)
  void clearModel() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get provider => $_getSZ(1);
  @$pb.TagNumber(2)
  set provider($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProvider() => $_has(1);
  @$pb.TagNumber(2)
  void clearProvider() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get effort => $_getSZ(2);
  @$pb.TagNumber(3)
  set effort($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEffort() => $_has(2);
  @$pb.TagNumber(3)
  void clearEffort() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get summary => $_getSZ(3);
  @$pb.TagNumber(4)
  set summary($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSummary() => $_has(3);
  @$pb.TagNumber(4)
  void clearSummary() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get verbosity => $_getSZ(4);
  @$pb.TagNumber(5)
  set verbosity($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVerbosity() => $_has(4);
  @$pb.TagNumber(5)
  void clearVerbosity() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get sandboxMode => $_getSZ(5);
  @$pb.TagNumber(6)
  set sandboxMode($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSandboxMode() => $_has(5);
  @$pb.TagNumber(6)
  void clearSandboxMode() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get approvalPolicy => $_getSZ(6);
  @$pb.TagNumber(7)
  set approvalPolicy($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasApprovalPolicy() => $_has(6);
  @$pb.TagNumber(7)
  void clearApprovalPolicy() => $_clearField(7);
}

/// CodexProviderCaps modelProvider/capabilities/read 的应答(provider 级能力声明)。
/// 用途:模型页解释"为什么某模型不可用"(如图像生成/联网搜索当前 provider 给不给)。
class CodexProviderCaps extends $pb.GeneratedMessage {
  factory CodexProviderCaps({
    $core.bool? imageGeneration,
    $core.bool? webSearch,
    $core.bool? namespaceTools,
  }) {
    final result = CodexProviderCaps._();
    if (imageGeneration != null) result.imageGeneration = imageGeneration;
    if (webSearch != null) result.webSearch = webSearch;
    if (namespaceTools != null) result.namespaceTools = namespaceTools;
    return result;
  }

  CodexProviderCaps._();

  factory CodexProviderCaps.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexProviderCaps()..mergeFromBuffer(data, registry);
  factory CodexProviderCaps.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexProviderCaps()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexProviderCaps',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexProviderCaps.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'imageGeneration')
    ..aOB(2, _omitFieldNames ? '' : 'webSearch')
    ..aOB(3, _omitFieldNames ? '' : 'namespaceTools')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexProviderCaps clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexProviderCaps copyWith(void Function(CodexProviderCaps) updates) =>
      super.copyWith((message) => updates(message as CodexProviderCaps))
          as CodexProviderCaps;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexProviderCaps() / CodexProviderCaps.new instead')
  static CodexProviderCaps create() => CodexProviderCaps._();
  static $pb.GeneratedMessage $_createMessage() => CodexProviderCaps._();
  @$core.override
  CodexProviderCaps createEmptyInstance() => CodexProviderCaps._();
  @$core.pragma('dart2js:noInline')
  static CodexProviderCaps getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CodexProviderCaps>(
          CodexProviderCaps.$_createMessage);
  static CodexProviderCaps? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get imageGeneration => $_getBF(0);
  @$pb.TagNumber(1)
  set imageGeneration($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasImageGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearImageGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get webSearch => $_getBF(1);
  @$pb.TagNumber(2)
  set webSearch($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasWebSearch() => $_has(1);
  @$pb.TagNumber(2)
  void clearWebSearch() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get namespaceTools => $_getBF(2);
  @$pb.TagNumber(3)
  set namespaceTools($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNamespaceTools() => $_has(2);
  @$pb.TagNumber(3)
  void clearNamespaceTools() => $_clearField(3);
}

/// 单个设置项的真实状态。
///
/// 【形状为什么定死成 key/state/detail 三条】
/// key 与 CodexSettings 的字段稳定对应(approval_policy / sandbox / model / effort /
/// hide_agent_reasoning / model_verbosity / model_reasoning_summary / disable_paste_burst)。
/// state 用**稳定字符串而不是 enum**:以后加状态(比如"等模型支持")不必动 proto,
/// 手机端遇到不认识的 state 时兜底显示原文而不是崩 —— 与 kind→显示名 同一套扩展纪律。
/// 以后新增设置项:daemon 侧状态表 + 手机端 文案表各加一处,proto 一个字不用改。
class CodexSettingStatus extends $pb.GeneratedMessage {
  factory CodexSettingStatus({
    $core.String? key,
    $core.String? state,
    $core.String? detail,
  }) {
    final result = CodexSettingStatus._();
    if (key != null) result.key = key;
    if (state != null) result.state = state;
    if (detail != null) result.detail = detail;
    return result;
  }

  CodexSettingStatus._();

  factory CodexSettingStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexSettingStatus()..mergeFromBuffer(data, registry);
  factory CodexSettingStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CodexSettingStatus()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CodexSettingStatus',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CodexSettingStatus.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'key')
    ..aOS(2, _omitFieldNames ? '' : 'state')
    ..aOS(3, _omitFieldNames ? '' : 'detail')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexSettingStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CodexSettingStatus copyWith(void Function(CodexSettingStatus) updates) =>
      super.copyWith((message) => updates(message as CodexSettingStatus))
          as CodexSettingStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CodexSettingStatus() / CodexSettingStatus.new instead')
  static CodexSettingStatus create() => CodexSettingStatus._();
  static $pb.GeneratedMessage $_createMessage() => CodexSettingStatus._();
  @$core.override
  CodexSettingStatus createEmptyInstance() => CodexSettingStatus._();
  @$core.pragma('dart2js:noInline')
  static CodexSettingStatus getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CodexSettingStatus>(
          CodexSettingStatus.$_createMessage);
  static CodexSettingStatus? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get key => $_getSZ(0);
  @$pb.TagNumber(1)
  set key($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasKey() => $_has(0);
  @$pb.TagNumber(1)
  void clearKey() => $_clearField(1);

  /// state 取值(稳定字符串,加状态不必动本文件):
  ///   applied / saved_not_applied / unsupported / failed / follow_desktop
  @$pb.TagNumber(2)
  $core.String get state => $_getSZ(1);
  @$pb.TagNumber(2)
  set state($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasState() => $_has(1);
  @$pb.TagNumber(2)
  void clearState() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get detail => $_getSZ(2);
  @$pb.TagNumber(3)
  set detail($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDetail() => $_has(2);
  @$pb.TagNumber(3)
  void clearDetail() => $_clearField(3);
}

class SetCodexSettingsRequest extends $pb.GeneratedMessage {
  factory SetCodexSettingsRequest({
    $core.String? approvalPolicy,
    $core.String? sandbox,
    $core.String? model,
    $core.String? effort,
    $core.bool? hideAgentReasoning,
    $core.String? modelVerbosity,
    $core.String? modelReasoningSummary,
    $core.bool? disablePasteBurst,
    $core.bool? applyToExistingThreads,
    $core.String? agentId,
    $core.String? nativeMode,
    $core.String? threadId,
    $core.String? nativeConfigOptionId,
    $core.String? nativeConfigValue,
  }) {
    final result = SetCodexSettingsRequest._();
    if (approvalPolicy != null) result.approvalPolicy = approvalPolicy;
    if (sandbox != null) result.sandbox = sandbox;
    if (model != null) result.model = model;
    if (effort != null) result.effort = effort;
    if (hideAgentReasoning != null)
      result.hideAgentReasoning = hideAgentReasoning;
    if (modelVerbosity != null) result.modelVerbosity = modelVerbosity;
    if (modelReasoningSummary != null)
      result.modelReasoningSummary = modelReasoningSummary;
    if (disablePasteBurst != null) result.disablePasteBurst = disablePasteBurst;
    if (applyToExistingThreads != null)
      result.applyToExistingThreads = applyToExistingThreads;
    if (agentId != null) result.agentId = agentId;
    if (nativeMode != null) result.nativeMode = nativeMode;
    if (threadId != null) result.threadId = threadId;
    if (nativeConfigOptionId != null)
      result.nativeConfigOptionId = nativeConfigOptionId;
    if (nativeConfigValue != null) result.nativeConfigValue = nativeConfigValue;
    return result;
  }

  SetCodexSettingsRequest._();

  factory SetCodexSettingsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCodexSettingsRequest()..mergeFromBuffer(data, registry);
  factory SetCodexSettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetCodexSettingsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCodexSettingsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SetCodexSettingsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'approvalPolicy')
    ..aOS(2, _omitFieldNames ? '' : 'sandbox')
    ..aOS(3, _omitFieldNames ? '' : 'model')
    ..aOS(4, _omitFieldNames ? '' : 'effort')
    ..aOB(5, _omitFieldNames ? '' : 'hideAgentReasoning')
    ..aOS(6, _omitFieldNames ? '' : 'modelVerbosity')
    ..aOS(7, _omitFieldNames ? '' : 'modelReasoningSummary')
    ..aOB(8, _omitFieldNames ? '' : 'disablePasteBurst')
    ..aOB(9, _omitFieldNames ? '' : 'applyToExistingThreads')
    ..aOS(10, _omitFieldNames ? '' : 'agentId')
    ..aOS(11, _omitFieldNames ? '' : 'nativeMode')
    ..aOS(12, _omitFieldNames ? '' : 'threadId')
    ..aOS(13, _omitFieldNames ? '' : 'nativeConfigOptionId')
    ..aOS(14, _omitFieldNames ? '' : 'nativeConfigValue')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCodexSettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCodexSettingsRequest copyWith(
          void Function(SetCodexSettingsRequest) updates) =>
      super.copyWith((message) => updates(message as SetCodexSettingsRequest))
          as SetCodexSettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetCodexSettingsRequest() / SetCodexSettingsRequest.new instead')
  static SetCodexSettingsRequest create() => SetCodexSettingsRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetCodexSettingsRequest._();
  @$core.override
  SetCodexSettingsRequest createEmptyInstance() => SetCodexSettingsRequest._();
  @$core.pragma('dart2js:noInline')
  static SetCodexSettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCodexSettingsRequest>(
          SetCodexSettingsRequest.$_createMessage);
  static SetCodexSettingsRequest? _defaultInstance;

  /// 【presence 语义 —— 8 个字段全部 optional,这是一条统一纪律】
  ///   present → 应用该值(空串是有意义的取值:跟随桌面 / 默认)
  ///   absent  → 不改该项
  ///
  /// 为什么不能用"零值 = 不改":字符串字段有一模一样的 bug ——
  ///   - 默认模型的「跟随桌面配置」写入的是 model=""，而 daemon 侧 `if req.Model != ""`
  ///     会直接跳过 → 用户**永远回不到"跟随桌面"**;
  ///   - 思考强度、审批策略、沙箱的"跟随桌面"取值同样是空串。
  /// bool 更是如此:普通 bool 的 false 既是"关掉"又是"没传"，用户**无法把开关关回去**。
  /// 客户端 writeCodex 每次从完整状态出发、8 个字段一律显式 set(含 setModel("")),
  /// 于是行为完全确定,旧的"零值即不改"歧义彻底消失。
  @$pb.TagNumber(1)
  $core.String get approvalPolicy => $_getSZ(0);
  @$pb.TagNumber(1)
  set approvalPolicy($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasApprovalPolicy() => $_has(0);
  @$pb.TagNumber(1)
  void clearApprovalPolicy() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sandbox => $_getSZ(1);
  @$pb.TagNumber(2)
  set sandbox($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSandbox() => $_has(1);
  @$pb.TagNumber(2)
  void clearSandbox() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get model => $_getSZ(2);
  @$pb.TagNumber(3)
  set model($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasModel() => $_has(2);
  @$pb.TagNumber(3)
  void clearModel() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get effort => $_getSZ(3);
  @$pb.TagNumber(4)
  set effort($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEffort() => $_has(3);
  @$pb.TagNumber(4)
  void clearEffort() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get hideAgentReasoning => $_getBF(4);
  @$pb.TagNumber(5)
  set hideAgentReasoning($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasHideAgentReasoning() => $_has(4);
  @$pb.TagNumber(5)
  void clearHideAgentReasoning() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get modelVerbosity => $_getSZ(5);
  @$pb.TagNumber(6)
  set modelVerbosity($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasModelVerbosity() => $_has(5);
  @$pb.TagNumber(6)
  void clearModelVerbosity() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get modelReasoningSummary => $_getSZ(6);
  @$pb.TagNumber(7)
  set modelReasoningSummary($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasModelReasoningSummary() => $_has(6);
  @$pb.TagNumber(7)
  void clearModelReasoningSummary() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get disablePasteBurst => $_getBF(7);
  @$pb.TagNumber(8)
  set disablePasteBurst($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasDisablePasteBurst() => $_has(7);
  @$pb.TagNumber(8)
  void clearDisablePasteBurst() => $_clearField(8);

  /// "应用到所有会话(含已有)":true 时,本次合并后的新值除了走"仅新建"的默认
  /// 语义外,还会被 daemon 批量写进**所有存量 codex 会话**的设置快照
  /// (store.Thread.SettingsSnapshot),让已有会话的后续轮次立刻按新值装配。
  /// false/缺省 = 保持"仅新建对话生效"的快照语义(默认行为不变)。
  @$pb.TagNumber(9)
  $core.bool get applyToExistingThreads => $_getBF(8);
  @$pb.TagNumber(9)
  set applyToExistingThreads($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasApplyToExistingThreads() => $_has(8);
  @$pb.TagNumber(9)
  void clearApplyToExistingThreads() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get agentId => $_getSZ(9);
  @$pb.TagNumber(10)
  set agentId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasAgentId() => $_has(9);
  @$pb.TagNumber(10)
  void clearAgentId() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get nativeMode => $_getSZ(10);
  @$pb.TagNumber(11)
  set nativeMode($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasNativeMode() => $_has(10);
  @$pb.TagNumber(11)
  void clearNativeMode() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get threadId => $_getSZ(11);
  @$pb.TagNumber(12)
  set threadId($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasThreadId() => $_has(11);
  @$pb.TagNumber(12)
  void clearThreadId() => $_clearField(12);

  /// ---- 原生配置项写通道(dsh 等;原生设置目录 ListAgentConfigOptions 的配套写)----
  /// 两个都 set 才是一次写请求:option_id 指项,value 是**原样**取值(候选叶子里的
  /// value 逐字节回传;dsh model 的双层编码串绝不能加工,裸模型名会被原生拒绝)。
  /// thread_id(字段 12)给了就当场写到那个会话;没给时按各 agent 自己的语义处理
  /// (dsh:记为"新会话初始值"—— 探针实证其 set_config_option 仅会话内生效、不持久)。
  @$pb.TagNumber(13)
  $core.String get nativeConfigOptionId => $_getSZ(12);
  @$pb.TagNumber(13)
  set nativeConfigOptionId($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasNativeConfigOptionId() => $_has(12);
  @$pb.TagNumber(13)
  void clearNativeConfigOptionId() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get nativeConfigValue => $_getSZ(13);
  @$pb.TagNumber(14)
  set nativeConfigValue($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasNativeConfigValue() => $_has(13);
  @$pb.TagNumber(14)
  void clearNativeConfigValue() => $_clearField(14);
}

class ListAgentConfigOptionsRequest extends $pb.GeneratedMessage {
  factory ListAgentConfigOptionsRequest({
    $core.String? agentId,
    $core.String? threadId,
  }) {
    final result = ListAgentConfigOptionsRequest._();
    if (agentId != null) result.agentId = agentId;
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  ListAgentConfigOptionsRequest._();

  factory ListAgentConfigOptionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentConfigOptionsRequest()..mergeFromBuffer(data, registry);
  factory ListAgentConfigOptionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentConfigOptionsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAgentConfigOptionsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListAgentConfigOptionsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentConfigOptionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentConfigOptionsRequest copyWith(
          void Function(ListAgentConfigOptionsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ListAgentConfigOptionsRequest))
          as ListAgentConfigOptionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListAgentConfigOptionsRequest() / ListAgentConfigOptionsRequest.new instead')
  static ListAgentConfigOptionsRequest create() =>
      ListAgentConfigOptionsRequest._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListAgentConfigOptionsRequest._();
  @$core.override
  ListAgentConfigOptionsRequest createEmptyInstance() =>
      ListAgentConfigOptionsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListAgentConfigOptionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAgentConfigOptionsRequest>(
          ListAgentConfigOptionsRequest.$_createMessage);
  static ListAgentConfigOptionsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  /// 会话上下文:给了就以那个会话的实时 configOptions 为准(dsh 走 session/resume);
  /// 空 = "新会话初始值"视角(dsh 用一次性瞬态会话取样,取完即毁)。
  @$pb.TagNumber(2)
  $core.String get threadId => $_getSZ(1);
  @$pb.TagNumber(2)
  set threadId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasThreadId() => $_has(1);
  @$pb.TagNumber(2)
  void clearThreadId() => $_clearField(2);
}

/// AgentConfigOptionLeaf 是一个候选叶子(dsh options 字段两种实测形状展平后统一到这;
/// group 非空 = 分组候选的组名,model 按 provider 分组时即 provider 名)。
class AgentConfigOptionLeaf extends $pb.GeneratedMessage {
  factory AgentConfigOptionLeaf({
    $core.String? value,
    $core.String? name,
    $core.String? description,
    $core.String? group,
  }) {
    final result = AgentConfigOptionLeaf._();
    if (value != null) result.value = value;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (group != null) result.group = group;
    return result;
  }

  AgentConfigOptionLeaf._();

  factory AgentConfigOptionLeaf.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentConfigOptionLeaf()..mergeFromBuffer(data, registry);
  factory AgentConfigOptionLeaf.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentConfigOptionLeaf()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentConfigOptionLeaf',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentConfigOptionLeaf.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'value')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOS(4, _omitFieldNames ? '' : 'group')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentConfigOptionLeaf clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentConfigOptionLeaf copyWith(
          void Function(AgentConfigOptionLeaf) updates) =>
      super.copyWith((message) => updates(message as AgentConfigOptionLeaf))
          as AgentConfigOptionLeaf;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use AgentConfigOptionLeaf() / AgentConfigOptionLeaf.new instead')
  static AgentConfigOptionLeaf create() => AgentConfigOptionLeaf._();
  static $pb.GeneratedMessage $_createMessage() => AgentConfigOptionLeaf._();
  @$core.override
  AgentConfigOptionLeaf createEmptyInstance() => AgentConfigOptionLeaf._();
  @$core.pragma('dart2js:noInline')
  static AgentConfigOptionLeaf getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AgentConfigOptionLeaf>(
          AgentConfigOptionLeaf.$_createMessage);
  static AgentConfigOptionLeaf? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get value => $_getSZ(0);
  @$pb.TagNumber(1)
  set value($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasValue() => $_has(0);
  @$pb.TagNumber(1)
  void clearValue() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get group => $_getSZ(3);
  @$pb.TagNumber(4)
  set group($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGroup() => $_has(3);
  @$pb.TagNumber(4)
  void clearGroup() => $_clearField(4);
}

/// AgentConfigOption 是「原生设置」分组里的一行。
///
/// 【字段纪律】id/name/category/type/current_value 全部原生原文(dsh 应答原样;pi 的
/// get_state 键名即 id)。current_value 是字符串化的原值(原生编码串不加工;布尔按
/// JSON 字面量 true/false)。options 空表示原生没报候选 —— 手机端渲染当前值原文,
/// 绝不编造选项。writable=false 的行是只读事实呈现(daemon 不提供写;原因见 note)。
class AgentConfigOption extends $pb.GeneratedMessage {
  factory AgentConfigOption({
    $core.String? id,
    $core.String? name,
    $core.String? category,
    $core.String? type,
    $core.String? currentValue,
    $core.Iterable<AgentConfigOptionLeaf>? options,
    $core.bool? writable,
    $core.String? note,
  }) {
    final result = AgentConfigOption._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (category != null) result.category = category;
    if (type != null) result.type = type;
    if (currentValue != null) result.currentValue = currentValue;
    if (options != null) result.options.addAll(options);
    if (writable != null) result.writable = writable;
    if (note != null) result.note = note;
    return result;
  }

  AgentConfigOption._();

  factory AgentConfigOption.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentConfigOption()..mergeFromBuffer(data, registry);
  factory AgentConfigOption.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AgentConfigOption()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AgentConfigOption',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AgentConfigOption.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'category')
    ..aOS(4, _omitFieldNames ? '' : 'type')
    ..aOS(5, _omitFieldNames ? '' : 'currentValue')
    ..pPM<AgentConfigOptionLeaf>(6, _omitFieldNames ? '' : 'options',
        subBuilder: AgentConfigOptionLeaf.$_createMessage)
    ..aOB(7, _omitFieldNames ? '' : 'writable')
    ..aOS(8, _omitFieldNames ? '' : 'note')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentConfigOption clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AgentConfigOption copyWith(void Function(AgentConfigOption) updates) =>
      super.copyWith((message) => updates(message as AgentConfigOption))
          as AgentConfigOption;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AgentConfigOption() / AgentConfigOption.new instead')
  static AgentConfigOption create() => AgentConfigOption._();
  static $pb.GeneratedMessage $_createMessage() => AgentConfigOption._();
  @$core.override
  AgentConfigOption createEmptyInstance() => AgentConfigOption._();
  @$core.pragma('dart2js:noInline')
  static AgentConfigOption getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AgentConfigOption>(
          AgentConfigOption.$_createMessage);
  static AgentConfigOption? _defaultInstance;

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
  $core.String get category => $_getSZ(2);
  @$pb.TagNumber(3)
  set category($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCategory() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategory() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get type => $_getSZ(3);
  @$pb.TagNumber(4)
  set type($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasType() => $_has(3);
  @$pb.TagNumber(4)
  void clearType() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get currentValue => $_getSZ(4);
  @$pb.TagNumber(5)
  set currentValue($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCurrentValue() => $_has(4);
  @$pb.TagNumber(5)
  void clearCurrentValue() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<AgentConfigOptionLeaf> get options => $_getList(5);

  @$pb.TagNumber(7)
  $core.bool get writable => $_getBF(6);
  @$pb.TagNumber(7)
  set writable($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasWritable() => $_has(6);
  @$pb.TagNumber(7)
  void clearWritable() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get note => $_getSZ(7);
  @$pb.TagNumber(8)
  set note($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasNote() => $_has(7);
  @$pb.TagNumber(8)
  void clearNote() => $_clearField(8);
}

class ListAgentConfigOptionsResponse extends $pb.GeneratedMessage {
  factory ListAgentConfigOptionsResponse({
    $core.Iterable<AgentConfigOption>? items,
    $core.String? error,
    $fixnum.Int64? sampledAtUnixMs,
  }) {
    final result = ListAgentConfigOptionsResponse._();
    if (items != null) result.items.addAll(items);
    if (error != null) result.error = error;
    if (sampledAtUnixMs != null) result.sampledAtUnixMs = sampledAtUnixMs;
    return result;
  }

  ListAgentConfigOptionsResponse._();

  factory ListAgentConfigOptionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentConfigOptionsResponse()..mergeFromBuffer(data, registry);
  factory ListAgentConfigOptionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListAgentConfigOptionsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListAgentConfigOptionsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListAgentConfigOptionsResponse.$_createMessage)
    ..pPM<AgentConfigOption>(1, _omitFieldNames ? '' : 'items',
        subBuilder: AgentConfigOption.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'error')
    ..aInt64(3, _omitFieldNames ? '' : 'sampledAtUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentConfigOptionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListAgentConfigOptionsResponse copyWith(
          void Function(ListAgentConfigOptionsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListAgentConfigOptionsResponse))
          as ListAgentConfigOptionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListAgentConfigOptionsResponse() / ListAgentConfigOptionsResponse.new instead')
  static ListAgentConfigOptionsResponse create() =>
      ListAgentConfigOptionsResponse._();
  static $pb.GeneratedMessage $_createMessage() =>
      ListAgentConfigOptionsResponse._();
  @$core.override
  ListAgentConfigOptionsResponse createEmptyInstance() =>
      ListAgentConfigOptionsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListAgentConfigOptionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListAgentConfigOptionsResponse>(
          ListAgentConfigOptionsResponse.$_createMessage);
  static ListAgentConfigOptionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<AgentConfigOption> get items => $_getList(0);

  /// 非空表示本次读取失败或部分失败;空列表 + 空 error = 该 agent 此刻无目录可示
  /// (如实,不伪造空成功之外的语义)。
  @$pb.TagNumber(2)
  $core.String get error => $_getSZ(1);
  @$pb.TagNumber(2)
  set error($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasError() => $_has(1);
  @$pb.TagNumber(2)
  void clearError() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get sampledAtUnixMs => $_getI64(2);
  @$pb.TagNumber(3)
  set sampledAtUnixMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSampledAtUnixMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearSampledAtUnixMs() => $_clearField(3);
}

class ListMcpServersRequest extends $pb.GeneratedMessage {
  factory ListMcpServersRequest() => ListMcpServersRequest._();

  ListMcpServersRequest._();

  factory ListMcpServersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMcpServersRequest()..mergeFromBuffer(data, registry);
  factory ListMcpServersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMcpServersRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMcpServersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListMcpServersRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMcpServersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMcpServersRequest copyWith(
          void Function(ListMcpServersRequest) updates) =>
      super.copyWith((message) => updates(message as ListMcpServersRequest))
          as ListMcpServersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMcpServersRequest() / ListMcpServersRequest.new instead')
  static ListMcpServersRequest create() => ListMcpServersRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListMcpServersRequest._();
  @$core.override
  ListMcpServersRequest createEmptyInstance() => ListMcpServersRequest._();
  @$core.pragma('dart2js:noInline')
  static ListMcpServersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMcpServersRequest>(
          ListMcpServersRequest.$_createMessage);
  static ListMcpServersRequest? _defaultInstance;
}

class McpServerStatus extends $pb.GeneratedMessage {
  factory McpServerStatus({
    $core.String? name,
    $core.int? tools,
    $core.String? status,
    $core.String? authStatus,
    $core.String? serverVersion,
    $core.String? serverTitle,
    $core.int? resourceCount,
    $core.int? resourceTemplateCount,
    $core.String? error,
  }) {
    final result = McpServerStatus._();
    if (name != null) result.name = name;
    if (tools != null) result.tools = tools;
    if (status != null) result.status = status;
    if (authStatus != null) result.authStatus = authStatus;
    if (serverVersion != null) result.serverVersion = serverVersion;
    if (serverTitle != null) result.serverTitle = serverTitle;
    if (resourceCount != null) result.resourceCount = resourceCount;
    if (resourceTemplateCount != null)
      result.resourceTemplateCount = resourceTemplateCount;
    if (error != null) result.error = error;
    return result;
  }

  McpServerStatus._();

  factory McpServerStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      McpServerStatus()..mergeFromBuffer(data, registry);
  factory McpServerStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      McpServerStatus()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'McpServerStatus',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: McpServerStatus.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aI(2, _omitFieldNames ? '' : 'tools')
    ..aOS(3, _omitFieldNames ? '' : 'status')
    ..aOS(4, _omitFieldNames ? '' : 'authStatus')
    ..aOS(5, _omitFieldNames ? '' : 'serverVersion')
    ..aOS(6, _omitFieldNames ? '' : 'serverTitle')
    ..aI(7, _omitFieldNames ? '' : 'resourceCount')
    ..aI(8, _omitFieldNames ? '' : 'resourceTemplateCount')
    ..aOS(9, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  McpServerStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  McpServerStatus copyWith(void Function(McpServerStatus) updates) =>
      super.copyWith((message) => updates(message as McpServerStatus))
          as McpServerStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use McpServerStatus() / McpServerStatus.new instead')
  static McpServerStatus create() => McpServerStatus._();
  static $pb.GeneratedMessage $_createMessage() => McpServerStatus._();
  @$core.override
  McpServerStatus createEmptyInstance() => McpServerStatus._();
  @$core.pragma('dart2js:noInline')
  static McpServerStatus getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<McpServerStatus>(
          McpServerStatus.$_createMessage);
  static McpServerStatus? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get tools => $_getIZ(1);
  @$pb.TagNumber(2)
  set tools($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTools() => $_has(1);
  @$pb.TagNumber(2)
  void clearTools() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get status => $_getSZ(2);
  @$pb.TagNumber(3)
  set status($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearStatus() => $_clearField(3);

  /// ---- 增强(字段号从 4 起)----
  /// 【以前把这两项丢了】codex 的 McpServerStatus 本来就带 authStatus 和 serverInfo,
  /// 只转发 name/tools/status 的话,"某个 MCP 用不了"这件事在手机上完全看不出来 ——
  /// 用户只会觉得"这个工具怎么不好使"。authStatus 直接回答是不是授权问题。
  @$pb.TagNumber(4)
  $core.String get authStatus => $_getSZ(3);
  @$pb.TagNumber(4)
  set authStatus($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAuthStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearAuthStatus() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get serverVersion => $_getSZ(4);
  @$pb.TagNumber(5)
  set serverVersion($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasServerVersion() => $_has(4);
  @$pb.TagNumber(5)
  void clearServerVersion() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get serverTitle => $_getSZ(5);
  @$pb.TagNumber(6)
  set serverTitle($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasServerTitle() => $_has(5);
  @$pb.TagNumber(6)
  void clearServerTitle() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.int get resourceCount => $_getIZ(6);
  @$pb.TagNumber(7)
  set resourceCount($core.int value) => $_setSignedInt32(6, value);
  @$pb.TagNumber(7)
  $core.bool hasResourceCount() => $_has(6);
  @$pb.TagNumber(7)
  void clearResourceCount() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get resourceTemplateCount => $_getIZ(7);
  @$pb.TagNumber(8)
  set resourceTemplateCount($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasResourceTemplateCount() => $_has(7);
  @$pb.TagNumber(8)
  void clearResourceTemplateCount() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get error => $_getSZ(8);
  @$pb.TagNumber(9)
  set error($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasError() => $_has(8);
  @$pb.TagNumber(9)
  void clearError() => $_clearField(9);
}

class ListMcpServersResponse extends $pb.GeneratedMessage {
  factory ListMcpServersResponse({
    $core.Iterable<McpServerStatus>? servers,
    $core.String? error,
  }) {
    final result = ListMcpServersResponse._();
    if (servers != null) result.servers.addAll(servers);
    if (error != null) result.error = error;
    return result;
  }

  ListMcpServersResponse._();

  factory ListMcpServersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMcpServersResponse()..mergeFromBuffer(data, registry);
  factory ListMcpServersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListMcpServersResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMcpServersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListMcpServersResponse.$_createMessage)
    ..pPM<McpServerStatus>(1, _omitFieldNames ? '' : 'servers',
        subBuilder: McpServerStatus.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMcpServersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMcpServersResponse copyWith(
          void Function(ListMcpServersResponse) updates) =>
      super.copyWith((message) => updates(message as ListMcpServersResponse))
          as ListMcpServersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListMcpServersResponse() / ListMcpServersResponse.new instead')
  static ListMcpServersResponse create() => ListMcpServersResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListMcpServersResponse._();
  @$core.override
  ListMcpServersResponse createEmptyInstance() => ListMcpServersResponse._();
  @$core.pragma('dart2js:noInline')
  static ListMcpServersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMcpServersResponse>(
          ListMcpServersResponse.$_createMessage);
  static ListMcpServersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<McpServerStatus> get servers => $_getList(0);

  /// 非空表示本次读取失败;与确实没有 MCP 服务器区分开。
  @$pb.TagNumber(2)
  $core.String get error => $_getSZ(1);
  @$pb.TagNumber(2)
  set error($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasError() => $_has(1);
  @$pb.TagNumber(2)
  void clearError() => $_clearField(2);
}

class ListModelsRequest extends $pb.GeneratedMessage {
  factory ListModelsRequest({
    $core.String? agentId,
    $core.bool? refresh,
    $core.String? threadId,
  }) {
    final result = ListModelsRequest._();
    if (agentId != null) result.agentId = agentId;
    if (refresh != null) result.refresh = refresh;
    if (threadId != null) result.threadId = threadId;
    return result;
  }

  ListModelsRequest._();

  factory ListModelsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListModelsRequest()..mergeFromBuffer(data, registry);
  factory ListModelsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListModelsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListModelsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListModelsRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOB(2, _omitFieldNames ? '' : 'refresh')
    ..aOS(3, _omitFieldNames ? '' : 'threadId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListModelsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListModelsRequest copyWith(void Function(ListModelsRequest) updates) =>
      super.copyWith((message) => updates(message as ListModelsRequest))
          as ListModelsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListModelsRequest() / ListModelsRequest.new instead')
  static ListModelsRequest create() => ListModelsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListModelsRequest._();
  @$core.override
  ListModelsRequest createEmptyInstance() => ListModelsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListModelsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListModelsRequest>(
          ListModelsRequest.$_createMessage);
  static ListModelsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get refresh => $_getBF(1);
  @$pb.TagNumber(2)
  set refresh($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRefresh() => $_has(1);
  @$pb.TagNumber(2)
  void clearRefresh() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get threadId => $_getSZ(2);
  @$pb.TagNumber(3)
  set threadId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasThreadId() => $_has(2);
  @$pb.TagNumber(3)
  void clearThreadId() => $_clearField(3);
}

class ListModelsResponse extends $pb.GeneratedMessage {
  factory ListModelsResponse({
    $core.Iterable<ModelInfo>? models,
    $core.String? currentModelId,
    $core.String? currentEffort,
    $core.String? stateError,
    EffectiveModelSelection? effectiveSelection,
  }) {
    final result = ListModelsResponse._();
    if (models != null) result.models.addAll(models);
    if (currentModelId != null) result.currentModelId = currentModelId;
    if (currentEffort != null) result.currentEffort = currentEffort;
    if (stateError != null) result.stateError = stateError;
    if (effectiveSelection != null)
      result.effectiveSelection = effectiveSelection;
    return result;
  }

  ListModelsResponse._();

  factory ListModelsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListModelsResponse()..mergeFromBuffer(data, registry);
  factory ListModelsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListModelsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListModelsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListModelsResponse.$_createMessage)
    ..pPM<ModelInfo>(1, _omitFieldNames ? '' : 'models',
        subBuilder: ModelInfo.$_createMessage)
    ..aOS(2, _omitFieldNames ? '' : 'currentModelId')
    ..aOS(3, _omitFieldNames ? '' : 'currentEffort')
    ..aOS(4, _omitFieldNames ? '' : 'stateError')
    ..aOM<EffectiveModelSelection>(
        5, _omitFieldNames ? '' : 'effectiveSelection',
        subBuilder: EffectiveModelSelection.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListModelsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListModelsResponse copyWith(void Function(ListModelsResponse) updates) =>
      super.copyWith((message) => updates(message as ListModelsResponse))
          as ListModelsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListModelsResponse() / ListModelsResponse.new instead')
  static ListModelsResponse create() => ListModelsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListModelsResponse._();
  @$core.override
  ListModelsResponse createEmptyInstance() => ListModelsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListModelsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListModelsResponse>(
          ListModelsResponse.$_createMessage);
  static ListModelsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<ModelInfo> get models => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get currentModelId => $_getSZ(1);
  @$pb.TagNumber(2)
  set currentModelId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCurrentModelId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCurrentModelId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get currentEffort => $_getSZ(2);
  @$pb.TagNumber(3)
  set currentEffort($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCurrentEffort() => $_has(2);
  @$pb.TagNumber(3)
  void clearCurrentEffort() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get stateError => $_getSZ(3);
  @$pb.TagNumber(4)
  set stateError($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStateError() => $_has(3);
  @$pb.TagNumber(4)
  void clearStateError() => $_clearField(4);

  /// ---- 有效模型选择(只增,字段号 5;分配登记见 proto/FIELD_ALLOCATIONS.md,
  /// 2026-10-04 会话选择与历史体验计划 Task 1)----
  /// 本请求上下文下解析后的选择真值:daemon 把"会话覆盖 → App 默认 → 桌面默认 →
  /// agent 原生当前值"四级来源收敛成一个结论(见 EffectiveModelSelection),
  /// 手机端只渲染它,不再让多个可变全局状态互相覆盖。消息缺席 = 旧 daemon,
  /// 客户端按 current_model_id(2)/current_effort(3) 兜底显示,不报错。
  @$pb.TagNumber(5)
  EffectiveModelSelection get effectiveSelection => $_getN(4);
  @$pb.TagNumber(5)
  set effectiveSelection(EffectiveModelSelection value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasEffectiveSelection() => $_has(4);
  @$pb.TagNumber(5)
  void clearEffectiveSelection() => $_clearField(5);
  @$pb.TagNumber(5)
  EffectiveModelSelection ensureEffectiveSelection() => $_ensure(4);
}

class ModelInfo extends $pb.GeneratedMessage {
  factory ModelInfo({
    $core.String? id,
    $core.String? provider,
    $core.String? displayName,
    $core.int? contextWindow,
    $core.bool? supportsThinking,
    $core.bool? supportsImages,
    $core.bool? available,
    $core.Iterable<$core.String>? availableEfforts,
  }) {
    final result = ModelInfo._();
    if (id != null) result.id = id;
    if (provider != null) result.provider = provider;
    if (displayName != null) result.displayName = displayName;
    if (contextWindow != null) result.contextWindow = contextWindow;
    if (supportsThinking != null) result.supportsThinking = supportsThinking;
    if (supportsImages != null) result.supportsImages = supportsImages;
    if (available != null) result.available = available;
    if (availableEfforts != null)
      result.availableEfforts.addAll(availableEfforts);
    return result;
  }

  ModelInfo._();

  factory ModelInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ModelInfo()..mergeFromBuffer(data, registry);
  factory ModelInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ModelInfo()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ModelInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ModelInfo.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'provider')
    ..aOS(3, _omitFieldNames ? '' : 'displayName')
    ..aI(4, _omitFieldNames ? '' : 'contextWindow')
    ..aOB(5, _omitFieldNames ? '' : 'supportsThinking')
    ..aOB(6, _omitFieldNames ? '' : 'supportsImages')
    ..aOB(7, _omitFieldNames ? '' : 'available')
    ..pPS(8, _omitFieldNames ? '' : 'availableEfforts')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ModelInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ModelInfo copyWith(void Function(ModelInfo) updates) =>
      super.copyWith((message) => updates(message as ModelInfo)) as ModelInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ModelInfo() / ModelInfo.new instead')
  static ModelInfo create() => ModelInfo._();
  static $pb.GeneratedMessage $_createMessage() => ModelInfo._();
  @$core.override
  ModelInfo createEmptyInstance() => ModelInfo._();
  @$core.pragma('dart2js:noInline')
  static ModelInfo getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ModelInfo>(ModelInfo.$_createMessage);
  static ModelInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get provider => $_getSZ(1);
  @$pb.TagNumber(2)
  set provider($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProvider() => $_has(1);
  @$pb.TagNumber(2)
  void clearProvider() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get displayName => $_getSZ(2);
  @$pb.TagNumber(3)
  set displayName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDisplayName() => $_has(2);
  @$pb.TagNumber(3)
  void clearDisplayName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get contextWindow => $_getIZ(3);
  @$pb.TagNumber(4)
  set contextWindow($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasContextWindow() => $_has(3);
  @$pb.TagNumber(4)
  void clearContextWindow() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get supportsThinking => $_getBF(4);
  @$pb.TagNumber(5)
  set supportsThinking($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSupportsThinking() => $_has(4);
  @$pb.TagNumber(5)
  void clearSupportsThinking() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get supportsImages => $_getBF(5);
  @$pb.TagNumber(6)
  set supportsImages($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSupportsImages() => $_has(5);
  @$pb.TagNumber(6)
  void clearSupportsImages() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get available => $_getBF(6);
  @$pb.TagNumber(7)
  set available($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasAvailable() => $_has(6);
  @$pb.TagNumber(7)
  void clearAvailable() => $_clearField(7);

  /// ---- 动态思考档位(只增,字段号顺延 8)----
  /// 该模型/agent 实际可用的思考强度取值,daemon 从 provider 的动态档位能力填充
  /// (pi:get_available_thinking_levels,随模型变化 —— 实测 glm-5.3 只有
  /// low/high/max;会话没挂载时回 pi 全集兜底)。codex/zcode 无动态档位 →
  /// 留空 = 手机端用内置默认档(low/medium/high/xhigh)。
  @$pb.TagNumber(8)
  $pb.PbList<$core.String> get availableEfforts => $_getList(7);
}

/// EffectiveModelSelection 一个上下文下"实际生效"的模型/思考强度选择及其来源。
///
/// 【为什么要有这条消息】手机端原来有 selectedModel/defaultModel/threadModels/
/// runtimeModel 好几个可变全局状态互相覆盖,谁也说不清最后生效的是哪个、为什么。
/// daemon 按优先级把四级来源解析成一个结论,连同"何时生效、用户是否确认过、
/// 为什么"一起下发;UI 只如实渲染,不再自己猜。
///
/// 【零值语义(钉死,勿改)】model_id / effort 为空串 = "跟随默认",是**合法状态**,
/// 不是错误,更不是"未设置":客户端不得把空串当异常,也不得擅自改写成具体值
/// (一旦写死,用户就永远回不到"跟随默认" —— 与 SetCodexSettings.model="" 的
/// 教训同源,见本文件 SetCodexSettings 的 presence 语义注释)。
class EffectiveModelSelection extends $pb.GeneratedMessage {
  factory EffectiveModelSelection({
    $core.String? agentId,
    $core.String? modelId,
    $core.String? effort,
    $core.String? source,
    $core.bool? appliesToNextTurn,
    $core.bool? configurationConfirmed,
    $core.String? explanation,
    $core.Iterable<$core.String>? availableEfforts,
  }) {
    final result = EffectiveModelSelection._();
    if (agentId != null) result.agentId = agentId;
    if (modelId != null) result.modelId = modelId;
    if (effort != null) result.effort = effort;
    if (source != null) result.source = source;
    if (appliesToNextTurn != null) result.appliesToNextTurn = appliesToNextTurn;
    if (configurationConfirmed != null)
      result.configurationConfirmed = configurationConfirmed;
    if (explanation != null) result.explanation = explanation;
    if (availableEfforts != null)
      result.availableEfforts.addAll(availableEfforts);
    return result;
  }

  EffectiveModelSelection._();

  factory EffectiveModelSelection.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EffectiveModelSelection()..mergeFromBuffer(data, registry);
  factory EffectiveModelSelection.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      EffectiveModelSelection()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EffectiveModelSelection',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: EffectiveModelSelection.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'agentId')
    ..aOS(2, _omitFieldNames ? '' : 'modelId')
    ..aOS(3, _omitFieldNames ? '' : 'effort')
    ..aOS(4, _omitFieldNames ? '' : 'source')
    ..aOB(5, _omitFieldNames ? '' : 'appliesToNextTurn')
    ..aOB(6, _omitFieldNames ? '' : 'configurationConfirmed')
    ..aOS(7, _omitFieldNames ? '' : 'explanation')
    ..pPS(8, _omitFieldNames ? '' : 'availableEfforts')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EffectiveModelSelection clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EffectiveModelSelection copyWith(
          void Function(EffectiveModelSelection) updates) =>
      super.copyWith((message) => updates(message as EffectiveModelSelection))
          as EffectiveModelSelection;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use EffectiveModelSelection() / EffectiveModelSelection.new instead')
  static EffectiveModelSelection create() => EffectiveModelSelection._();
  static $pb.GeneratedMessage $_createMessage() => EffectiveModelSelection._();
  @$core.override
  EffectiveModelSelection createEmptyInstance() => EffectiveModelSelection._();
  @$core.pragma('dart2js:noInline')
  static EffectiveModelSelection getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EffectiveModelSelection>(
          EffectiveModelSelection.$_createMessage);
  static EffectiveModelSelection? _defaultInstance;

  /// 归属的 agent id("codex" / "pi" / …);空 = 当前默认 agent。
  @$pb.TagNumber(1)
  $core.String get agentId => $_getSZ(0);
  @$pb.TagNumber(1)
  set agentId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAgentId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAgentId() => $_clearField(1);

  /// 生效的模型 id(形如 "agent/model-id");空 = 跟随默认(合法取值,见零值语义)。
  @$pb.TagNumber(2)
  $core.String get modelId => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasModelId() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelId() => $_clearField(2);

  /// 生效的思考强度(low / medium / high / …);空 = 跟随默认(合法取值)。
  @$pb.TagNumber(3)
  $core.String get effort => $_getSZ(2);
  @$pb.TagNumber(3)
  set effort($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEffort() => $_has(2);
  @$pb.TagNumber(3)
  void clearEffort() => $_clearField(3);

  /// 该结论来自哪一级来源(稳定字符串,新增来源不改 proto):
  ///   thread_override — 本会话显式覆盖(手机端选择/委派指定)
  ///   app_default     — App 侧默认(daemon 落库的默认模型)
  ///   desktop_default — 桌面端默认(如 codex config 的当前值)
  ///   native_current  — agent 原生当前值(会话实挂的模型/强度)
  /// 客户端遇到不认识的取值按"未知来源"如实显示原文,不得报错。
  @$pb.TagNumber(4)
  $core.String get source => $_getSZ(3);
  @$pb.TagNumber(4)
  set source($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSource() => $_has(3);
  @$pb.TagNumber(4)
  void clearSource() => $_clearField(4);

  /// true = 这份选择尚未落进任何已开始的轮次,将从**下一轮**生效
  /// (运行中切换的正确口径:绝不改写正在运行的 Turn)。
  @$pb.TagNumber(5)
  $core.bool get appliesToNextTurn => $_getBF(4);
  @$pb.TagNumber(5)
  set appliesToNextTurn($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAppliesToNextTurn() => $_has(4);
  @$pb.TagNumber(5)
  void clearAppliesToNextTurn() => $_clearField(5);

  /// 用户是否对这份配置显式确认过(区分"系统替你选的"与"你选过的")。
  @$pb.TagNumber(6)
  $core.bool get configurationConfirmed => $_getBF(5);
  @$pb.TagNumber(6)
  set configurationConfirmed($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasConfigurationConfirmed() => $_has(5);
  @$pb.TagNumber(6)
  void clearConfigurationConfirmed() => $_clearField(6);

  /// 人话解释:为什么生效的是这个(如"桌面默认 glm-5.3,本会话未覆盖")。
  /// UI 直接展示;空 = 无解释。
  @$pb.TagNumber(7)
  $core.String get explanation => $_getSZ(6);
  @$pb.TagNumber(7)
  set explanation($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasExplanation() => $_has(6);
  @$pb.TagNumber(7)
  void clearExplanation() => $_clearField(7);

  /// 当前模型实际可用的思考强度档位(daemon 从 provider 动态能力填充;
  /// 空 = 客户端用内置默认档)。与 ModelInfo.available_efforts(8) 同源口径。
  @$pb.TagNumber(8)
  $pb.PbList<$core.String> get availableEfforts => $_getList(7);
}

class SetDefaultModelRequest extends $pb.GeneratedMessage {
  factory SetDefaultModelRequest({
    $core.String? modelId,
  }) {
    final result = SetDefaultModelRequest._();
    if (modelId != null) result.modelId = modelId;
    return result;
  }

  SetDefaultModelRequest._();

  factory SetDefaultModelRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetDefaultModelRequest()..mergeFromBuffer(data, registry);
  factory SetDefaultModelRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SetDefaultModelRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetDefaultModelRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SetDefaultModelRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'modelId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetDefaultModelRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetDefaultModelRequest copyWith(
          void Function(SetDefaultModelRequest) updates) =>
      super.copyWith((message) => updates(message as SetDefaultModelRequest))
          as SetDefaultModelRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SetDefaultModelRequest() / SetDefaultModelRequest.new instead')
  static SetDefaultModelRequest create() => SetDefaultModelRequest._();
  static $pb.GeneratedMessage $_createMessage() => SetDefaultModelRequest._();
  @$core.override
  SetDefaultModelRequest createEmptyInstance() => SetDefaultModelRequest._();
  @$core.pragma('dart2js:noInline')
  static SetDefaultModelRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetDefaultModelRequest>(
          SetDefaultModelRequest.$_createMessage);
  static SetDefaultModelRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get modelId => $_getSZ(0);
  @$pb.TagNumber(1)
  set modelId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasModelId() => $_has(0);
  @$pb.TagNumber(1)
  void clearModelId() => $_clearField(1);
}

class TestModelRequest extends $pb.GeneratedMessage {
  factory TestModelRequest({
    $core.String? modelId,
    $core.String? prompt,
  }) {
    final result = TestModelRequest._();
    if (modelId != null) result.modelId = modelId;
    if (prompt != null) result.prompt = prompt;
    return result;
  }

  TestModelRequest._();

  factory TestModelRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TestModelRequest()..mergeFromBuffer(data, registry);
  factory TestModelRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TestModelRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TestModelRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TestModelRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'modelId')
    ..aOS(2, _omitFieldNames ? '' : 'prompt')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TestModelRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TestModelRequest copyWith(void Function(TestModelRequest) updates) =>
      super.copyWith((message) => updates(message as TestModelRequest))
          as TestModelRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TestModelRequest() / TestModelRequest.new instead')
  static TestModelRequest create() => TestModelRequest._();
  static $pb.GeneratedMessage $_createMessage() => TestModelRequest._();
  @$core.override
  TestModelRequest createEmptyInstance() => TestModelRequest._();
  @$core.pragma('dart2js:noInline')
  static TestModelRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TestModelRequest>(
          TestModelRequest.$_createMessage);
  static TestModelRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get modelId => $_getSZ(0);
  @$pb.TagNumber(1)
  set modelId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasModelId() => $_has(0);
  @$pb.TagNumber(1)
  void clearModelId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get prompt => $_getSZ(1);
  @$pb.TagNumber(2)
  set prompt($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPrompt() => $_has(1);
  @$pb.TagNumber(2)
  void clearPrompt() => $_clearField(2);
}

class TestModelResponse extends $pb.GeneratedMessage {
  factory TestModelResponse({
    $core.bool? ok,
    $core.String? reply,
    $fixnum.Int64? latencyMs,
    $core.String? error,
  }) {
    final result = TestModelResponse._();
    if (ok != null) result.ok = ok;
    if (reply != null) result.reply = reply;
    if (latencyMs != null) result.latencyMs = latencyMs;
    if (error != null) result.error = error;
    return result;
  }

  TestModelResponse._();

  factory TestModelResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TestModelResponse()..mergeFromBuffer(data, registry);
  factory TestModelResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TestModelResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TestModelResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TestModelResponse.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'ok')
    ..aOS(2, _omitFieldNames ? '' : 'reply')
    ..aInt64(3, _omitFieldNames ? '' : 'latencyMs')
    ..aOS(4, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TestModelResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TestModelResponse copyWith(void Function(TestModelResponse) updates) =>
      super.copyWith((message) => updates(message as TestModelResponse))
          as TestModelResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TestModelResponse() / TestModelResponse.new instead')
  static TestModelResponse create() => TestModelResponse._();
  static $pb.GeneratedMessage $_createMessage() => TestModelResponse._();
  @$core.override
  TestModelResponse createEmptyInstance() => TestModelResponse._();
  @$core.pragma('dart2js:noInline')
  static TestModelResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TestModelResponse>(
          TestModelResponse.$_createMessage);
  static TestModelResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get ok => $_getBF(0);
  @$pb.TagNumber(1)
  set ok($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOk() => $_has(0);
  @$pb.TagNumber(1)
  void clearOk() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reply => $_getSZ(1);
  @$pb.TagNumber(2)
  set reply($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReply() => $_has(1);
  @$pb.TagNumber(2)
  void clearReply() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get latencyMs => $_getI64(2);
  @$pb.TagNumber(3)
  set latencyMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLatencyMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearLatencyMs() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get error => $_getSZ(3);
  @$pb.TagNumber(4)
  set error($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasError() => $_has(3);
  @$pb.TagNumber(4)
  void clearError() => $_clearField(4);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
