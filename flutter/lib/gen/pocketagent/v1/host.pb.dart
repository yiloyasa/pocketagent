// This is a generated file - do not edit.
//
// Generated from v1/host.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'model.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class GetHostInfoRequest extends $pb.GeneratedMessage {
  factory GetHostInfoRequest() => GetHostInfoRequest._();

  GetHostInfoRequest._();

  factory GetHostInfoRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetHostInfoRequest()..mergeFromBuffer(data, registry);
  factory GetHostInfoRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetHostInfoRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetHostInfoRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetHostInfoRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetHostInfoRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetHostInfoRequest copyWith(void Function(GetHostInfoRequest) updates) =>
      super.copyWith((message) => updates(message as GetHostInfoRequest))
          as GetHostInfoRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GetHostInfoRequest() / GetHostInfoRequest.new instead')
  static GetHostInfoRequest create() => GetHostInfoRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetHostInfoRequest._();
  @$core.override
  GetHostInfoRequest createEmptyInstance() => GetHostInfoRequest._();
  @$core.pragma('dart2js:noInline')
  static GetHostInfoRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetHostInfoRequest>(
          GetHostInfoRequest.$_createMessage);
  static GetHostInfoRequest? _defaultInstance;
}

class HostInfo extends $pb.GeneratedMessage {
  factory HostInfo({
    $core.String? hostId,
    $core.String? displayName,
    $core.String? os,
    $core.String? arch,
    $core.String? daemonVersion,
    $core.Iterable<$core.String>? workspaces,
    $core.Iterable<$core.String>? capabilities,
    $core.bool? authRequired,
    $core.String? defaultShell,
    $core.String? tailscaleIpv4,
    $core.String? lanIpv4,
    HostRecoveryReport? recovery,
    $core.String? minAppVersion,
  }) {
    final result = HostInfo._();
    if (hostId != null) result.hostId = hostId;
    if (displayName != null) result.displayName = displayName;
    if (os != null) result.os = os;
    if (arch != null) result.arch = arch;
    if (daemonVersion != null) result.daemonVersion = daemonVersion;
    if (workspaces != null) result.workspaces.addAll(workspaces);
    if (capabilities != null) result.capabilities.addAll(capabilities);
    if (authRequired != null) result.authRequired = authRequired;
    if (defaultShell != null) result.defaultShell = defaultShell;
    if (tailscaleIpv4 != null) result.tailscaleIpv4 = tailscaleIpv4;
    if (lanIpv4 != null) result.lanIpv4 = lanIpv4;
    if (recovery != null) result.recovery = recovery;
    if (minAppVersion != null) result.minAppVersion = minAppVersion;
    return result;
  }

  HostInfo._();

  factory HostInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostInfo()..mergeFromBuffer(data, registry);
  factory HostInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostInfo()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostInfo.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'hostId')
    ..aOS(2, _omitFieldNames ? '' : 'displayName')
    ..aOS(3, _omitFieldNames ? '' : 'os')
    ..aOS(4, _omitFieldNames ? '' : 'arch')
    ..aOS(5, _omitFieldNames ? '' : 'daemonVersion')
    ..pPS(6, _omitFieldNames ? '' : 'workspaces')
    ..pPS(7, _omitFieldNames ? '' : 'capabilities')
    ..aOB(8, _omitFieldNames ? '' : 'authRequired')
    ..aOS(9, _omitFieldNames ? '' : 'defaultShell')
    ..aOS(10, _omitFieldNames ? '' : 'tailscaleIpv4')
    ..aOS(11, _omitFieldNames ? '' : 'lanIpv4')
    ..aOM<HostRecoveryReport>(12, _omitFieldNames ? '' : 'recovery',
        subBuilder: HostRecoveryReport.$_createMessage)
    ..aOS(13, _omitFieldNames ? '' : 'minAppVersion')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostInfo copyWith(void Function(HostInfo) updates) =>
      super.copyWith((message) => updates(message as HostInfo)) as HostInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HostInfo() / HostInfo.new instead')
  static HostInfo create() => HostInfo._();
  static $pb.GeneratedMessage $_createMessage() => HostInfo._();
  @$core.override
  HostInfo createEmptyInstance() => HostInfo._();
  @$core.pragma('dart2js:noInline')
  static HostInfo getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HostInfo>(HostInfo.$_createMessage);
  static HostInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get hostId => $_getSZ(0);
  @$pb.TagNumber(1)
  set hostId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHostId() => $_has(0);
  @$pb.TagNumber(1)
  void clearHostId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get displayName => $_getSZ(1);
  @$pb.TagNumber(2)
  set displayName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplayName() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplayName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get os => $_getSZ(2);
  @$pb.TagNumber(3)
  set os($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOs() => $_has(2);
  @$pb.TagNumber(3)
  void clearOs() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get arch => $_getSZ(3);
  @$pb.TagNumber(4)
  set arch($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasArch() => $_has(3);
  @$pb.TagNumber(4)
  void clearArch() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get daemonVersion => $_getSZ(4);
  @$pb.TagNumber(5)
  set daemonVersion($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDaemonVersion() => $_has(4);
  @$pb.TagNumber(5)
  void clearDaemonVersion() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get workspaces => $_getList(5);

  @$pb.TagNumber(7)
  $pb.PbList<$core.String> get capabilities => $_getList(6);

  @$pb.TagNumber(8)
  $core.bool get authRequired => $_getBF(7);
  @$pb.TagNumber(8)
  set authRequired($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAuthRequired() => $_has(7);
  @$pb.TagNumber(8)
  void clearAuthRequired() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get defaultShell => $_getSZ(8);
  @$pb.TagNumber(9)
  set defaultShell($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasDefaultShell() => $_has(8);
  @$pb.TagNumber(9)
  void clearDefaultShell() => $_clearField(9);

  /// 主机的 Tailscale IPv4(100.x.y.z),装了 Tailscale 才有;
  /// 手机在外面(4G/其他 WiFi)时用这个地址直连,无需公网
  @$pb.TagNumber(10)
  $core.String get tailscaleIpv4 => $_getSZ(9);
  @$pb.TagNumber(10)
  set tailscaleIpv4($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasTailscaleIpv4() => $_has(9);
  @$pb.TagNumber(10)
  void clearTailscaleIpv4() => $_clearField(10);

  /// 主机的局域网 IPv4(192.168.x.x 等):同一 WiFi 下直连最快,
  /// 不依赖 Tailscale 的 DERP 中继
  @$pb.TagNumber(11)
  $core.String get lanIpv4 => $_getSZ(10);
  @$pb.TagNumber(11)
  set lanIpv4($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasLanIpv4() => $_has(10);
  @$pb.TagNumber(11)
  void clearLanIpv4() => $_clearField(11);

  /// W6b:启动恢复诊断(规格 §8.2/§8.3)。把"这次启动走了哪条恢复路径、有没有数据
  /// 损失"经 RPC 送到手机 —— 此前它只落在 daemon 日志里,用户在手机端无从知道
  /// 「上次退出时还有几条排队输入没轮到执行」。旧 daemon 不带该字段 = 未报告,
  /// 手机端据此隐藏该区块(禁用"未知/错误"措辞,AUDIT §3.3)。
  @$pb.TagNumber(12)
  HostRecoveryReport get recovery => $_getN(11);
  @$pb.TagNumber(12)
  set recovery(HostRecoveryReport value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasRecovery() => $_has(11);
  @$pb.TagNumber(12)
  void clearRecovery() => $_clearField(12);
  @$pb.TagNumber(12)
  HostRecoveryReport ensureRecovery() => $_ensure(11);

  /// 版本协商闸(A5 最小实现,2026-10-09):本 daemon 要求的最低 App 版本,
  /// 语义化版本("0.3.0")。空 = 不设限(旧 daemon 恒空,手机端照常工作)。
  /// 手机端与本机 versionName 做分量数值比较,过低时在连接诊断置"App 版本过旧"
  /// 提示位(仅提示,不阻断 —— 旧 daemon 不带该字段,不能拿它当硬闸)。
  /// 常量真源:daemon/internal/version.MinAppVersion(升级 daemon 时同步抬)。
  @$pb.TagNumber(13)
  $core.String get minAppVersion => $_getSZ(12);
  @$pb.TagNumber(13)
  set minAppVersion($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasMinAppVersion() => $_has(12);
  @$pb.TagNumber(13)
  void clearMinAppVersion() => $_clearField(13);
}

enum AuthorizeRequest_Method {
  pairCode,
  token,
  challengeRequest,
  challengeAnswer,
  notSet
}

class AuthorizeRequest extends $pb.GeneratedMessage {
  factory AuthorizeRequest({
    $core.String? pairCode,
    $core.String? token,
    $core.String? deviceName,
    $core.String? deviceId,
    $core.String? devicePubKey,
    $core.String? challengeRequest,
    ChallengeAnswer? challengeAnswer,
  }) {
    final result = AuthorizeRequest._();
    if (pairCode != null) result.pairCode = pairCode;
    if (token != null) result.token = token;
    if (deviceName != null) result.deviceName = deviceName;
    if (deviceId != null) result.deviceId = deviceId;
    if (devicePubKey != null) result.devicePubKey = devicePubKey;
    if (challengeRequest != null) result.challengeRequest = challengeRequest;
    if (challengeAnswer != null) result.challengeAnswer = challengeAnswer;
    return result;
  }

  AuthorizeRequest._();

  factory AuthorizeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AuthorizeRequest()..mergeFromBuffer(data, registry);
  factory AuthorizeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AuthorizeRequest()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, AuthorizeRequest_Method>
      _AuthorizeRequest_MethodByTag = {
    1: AuthorizeRequest_Method.pairCode,
    2: AuthorizeRequest_Method.token,
    6: AuthorizeRequest_Method.challengeRequest,
    7: AuthorizeRequest_Method.challengeAnswer,
    0: AuthorizeRequest_Method.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthorizeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AuthorizeRequest.$_createMessage)
    ..oo(0, [1, 2, 6, 7])
    ..aOS(1, _omitFieldNames ? '' : 'pairCode')
    ..aOS(2, _omitFieldNames ? '' : 'token')
    ..aOS(3, _omitFieldNames ? '' : 'deviceName')
    ..aOS(4, _omitFieldNames ? '' : 'deviceId')
    ..aOS(5, _omitFieldNames ? '' : 'devicePubKey')
    ..aOS(6, _omitFieldNames ? '' : 'challengeRequest')
    ..aOM<ChallengeAnswer>(7, _omitFieldNames ? '' : 'challengeAnswer',
        subBuilder: ChallengeAnswer.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthorizeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthorizeRequest copyWith(void Function(AuthorizeRequest) updates) =>
      super.copyWith((message) => updates(message as AuthorizeRequest))
          as AuthorizeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AuthorizeRequest() / AuthorizeRequest.new instead')
  static AuthorizeRequest create() => AuthorizeRequest._();
  static $pb.GeneratedMessage $_createMessage() => AuthorizeRequest._();
  @$core.override
  AuthorizeRequest createEmptyInstance() => AuthorizeRequest._();
  @$core.pragma('dart2js:noInline')
  static AuthorizeRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AuthorizeRequest>(
          AuthorizeRequest.$_createMessage);
  static AuthorizeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  AuthorizeRequest_Method whichMethod() =>
      _AuthorizeRequest_MethodByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  void clearMethod() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get pairCode => $_getSZ(0);
  @$pb.TagNumber(1)
  set pairCode($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPairCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearPairCode() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get token => $_getSZ(1);
  @$pb.TagNumber(2)
  set token($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearToken() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get deviceName => $_getSZ(2);
  @$pb.TagNumber(3)
  set deviceName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeviceName() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeviceName() => $_clearField(3);

  /// 设备身份(P0-2):稳定的本机 UUID + base64 的 P-256 SPKI 公钥。
  /// 选 P-256 而非 Ed25519:Android Keystore 支持 Ed25519 要 API 33,本项目 minSdk 26。
  @$pb.TagNumber(4)
  $core.String get deviceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deviceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeviceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeviceId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get devicePubKey => $_getSZ(4);
  @$pb.TagNumber(5)
  set devicePubKey($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDevicePubKey() => $_has(4);
  @$pb.TagNumber(5)
  void clearDevicePubKey() => $_clearField(5);

  /// 设备身份认证(P0-2):先要一个挑战数,再交 Keystore 签名换新 token
  @$pb.TagNumber(6)
  $core.String get challengeRequest => $_getSZ(5);
  @$pb.TagNumber(6)
  set challengeRequest($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasChallengeRequest() => $_has(5);
  @$pb.TagNumber(6)
  void clearChallengeRequest() => $_clearField(6);

  @$pb.TagNumber(7)
  ChallengeAnswer get challengeAnswer => $_getN(6);
  @$pb.TagNumber(7)
  set challengeAnswer(ChallengeAnswer value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasChallengeAnswer() => $_has(6);
  @$pb.TagNumber(7)
  void clearChallengeAnswer() => $_clearField(7);
  @$pb.TagNumber(7)
  ChallengeAnswer ensureChallengeAnswer() => $_ensure(6);
}

/// ChallengeAnswer:challenge_request 拿到挑战数后,手机用 Keystore 私钥
/// 对 "device_id:challenge" 做 SHA256withECDSA 签名,签名走 base64。
class ChallengeAnswer extends $pb.GeneratedMessage {
  factory ChallengeAnswer({
    $core.String? challenge,
    $core.String? signature,
  }) {
    final result = ChallengeAnswer._();
    if (challenge != null) result.challenge = challenge;
    if (signature != null) result.signature = signature;
    return result;
  }

  ChallengeAnswer._();

  factory ChallengeAnswer.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ChallengeAnswer()..mergeFromBuffer(data, registry);
  factory ChallengeAnswer.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ChallengeAnswer()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChallengeAnswer',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ChallengeAnswer.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'challenge')
    ..aOS(2, _omitFieldNames ? '' : 'signature')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChallengeAnswer clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChallengeAnswer copyWith(void Function(ChallengeAnswer) updates) =>
      super.copyWith((message) => updates(message as ChallengeAnswer))
          as ChallengeAnswer;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ChallengeAnswer() / ChallengeAnswer.new instead')
  static ChallengeAnswer create() => ChallengeAnswer._();
  static $pb.GeneratedMessage $_createMessage() => ChallengeAnswer._();
  @$core.override
  ChallengeAnswer createEmptyInstance() => ChallengeAnswer._();
  @$core.pragma('dart2js:noInline')
  static ChallengeAnswer getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ChallengeAnswer>(
          ChallengeAnswer.$_createMessage);
  static ChallengeAnswer? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get challenge => $_getSZ(0);
  @$pb.TagNumber(1)
  set challenge($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChallenge() => $_has(0);
  @$pb.TagNumber(1)
  void clearChallenge() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get signature => $_getSZ(1);
  @$pb.TagNumber(2)
  set signature($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSignature() => $_has(1);
  @$pb.TagNumber(2)
  void clearSignature() => $_clearField(2);
}

class AuthorizeResponse extends $pb.GeneratedMessage {
  factory AuthorizeResponse({
    $core.String? token,
    $fixnum.Int64? expiresAtUnix,
    $core.String? challenge,
    $core.String? deviceId,
  }) {
    final result = AuthorizeResponse._();
    if (token != null) result.token = token;
    if (expiresAtUnix != null) result.expiresAtUnix = expiresAtUnix;
    if (challenge != null) result.challenge = challenge;
    if (deviceId != null) result.deviceId = deviceId;
    return result;
  }

  AuthorizeResponse._();

  factory AuthorizeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AuthorizeResponse()..mergeFromBuffer(data, registry);
  factory AuthorizeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      AuthorizeResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthorizeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: AuthorizeResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'token')
    ..aInt64(2, _omitFieldNames ? '' : 'expiresAtUnix')
    ..aOS(3, _omitFieldNames ? '' : 'challenge')
    ..aOS(4, _omitFieldNames ? '' : 'deviceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthorizeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthorizeResponse copyWith(void Function(AuthorizeResponse) updates) =>
      super.copyWith((message) => updates(message as AuthorizeResponse))
          as AuthorizeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use AuthorizeResponse() / AuthorizeResponse.new instead')
  static AuthorizeResponse create() => AuthorizeResponse._();
  static $pb.GeneratedMessage $_createMessage() => AuthorizeResponse._();
  @$core.override
  AuthorizeResponse createEmptyInstance() => AuthorizeResponse._();
  @$core.pragma('dart2js:noInline')
  static AuthorizeResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<AuthorizeResponse>(
          AuthorizeResponse.$_createMessage);
  static AuthorizeResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get token => $_getSZ(0);
  @$pb.TagNumber(1)
  set token($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearToken() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get expiresAtUnix => $_getI64(1);
  @$pb.TagNumber(2)
  set expiresAtUnix($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExpiresAtUnix() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpiresAtUnix() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get challenge => $_getSZ(2);
  @$pb.TagNumber(3)
  set challenge($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChallenge() => $_has(2);
  @$pb.TagNumber(3)
  void clearChallenge() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get deviceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set deviceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDeviceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearDeviceId() => $_clearField(4);
}

class PingRequest extends $pb.GeneratedMessage {
  factory PingRequest({
    $fixnum.Int64? clientTimeUnixMs,
  }) {
    final result = PingRequest._();
    if (clientTimeUnixMs != null) result.clientTimeUnixMs = clientTimeUnixMs;
    return result;
  }

  PingRequest._();

  factory PingRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PingRequest()..mergeFromBuffer(data, registry);
  factory PingRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PingRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PingRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: PingRequest.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'clientTimeUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PingRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PingRequest copyWith(void Function(PingRequest) updates) =>
      super.copyWith((message) => updates(message as PingRequest))
          as PingRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PingRequest() / PingRequest.new instead')
  static PingRequest create() => PingRequest._();
  static $pb.GeneratedMessage $_createMessage() => PingRequest._();
  @$core.override
  PingRequest createEmptyInstance() => PingRequest._();
  @$core.pragma('dart2js:noInline')
  static PingRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PingRequest>(
          PingRequest.$_createMessage);
  static PingRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get clientTimeUnixMs => $_getI64(0);
  @$pb.TagNumber(1)
  set clientTimeUnixMs($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasClientTimeUnixMs() => $_has(0);
  @$pb.TagNumber(1)
  void clearClientTimeUnixMs() => $_clearField(1);
}

class PingResponse extends $pb.GeneratedMessage {
  factory PingResponse({
    $fixnum.Int64? serverTimeUnixMs,
    $core.String? status,
  }) {
    final result = PingResponse._();
    if (serverTimeUnixMs != null) result.serverTimeUnixMs = serverTimeUnixMs;
    if (status != null) result.status = status;
    return result;
  }

  PingResponse._();

  factory PingResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PingResponse()..mergeFromBuffer(data, registry);
  factory PingResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      PingResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PingResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: PingResponse.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'serverTimeUnixMs')
    ..aOS(2, _omitFieldNames ? '' : 'status')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PingResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PingResponse copyWith(void Function(PingResponse) updates) =>
      super.copyWith((message) => updates(message as PingResponse))
          as PingResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use PingResponse() / PingResponse.new instead')
  static PingResponse create() => PingResponse._();
  static $pb.GeneratedMessage $_createMessage() => PingResponse._();
  @$core.override
  PingResponse createEmptyInstance() => PingResponse._();
  @$core.pragma('dart2js:noInline')
  static PingResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<PingResponse>(
          PingResponse.$_createMessage);
  static PingResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get serverTimeUnixMs => $_getI64(0);
  @$pb.TagNumber(1)
  set serverTimeUnixMs($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasServerTimeUnixMs() => $_has(0);
  @$pb.TagNumber(1)
  void clearServerTimeUnixMs() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get status => $_getSZ(1);
  @$pb.TagNumber(2)
  set status($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);
}

/// ---- P1-5:主机事件流断点续传(只增)----
class WatchEventsRequest extends $pb.GeneratedMessage {
  factory WatchEventsRequest({
    $fixnum.Int64? afterSequence,
  }) {
    final result = WatchEventsRequest._();
    if (afterSequence != null) result.afterSequence = afterSequence;
    return result;
  }

  WatchEventsRequest._();

  factory WatchEventsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchEventsRequest()..mergeFromBuffer(data, registry);
  factory WatchEventsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WatchEventsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WatchEventsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: WatchEventsRequest.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'afterSequence')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchEventsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WatchEventsRequest copyWith(void Function(WatchEventsRequest) updates) =>
      super.copyWith((message) => updates(message as WatchEventsRequest))
          as WatchEventsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use WatchEventsRequest() / WatchEventsRequest.new instead')
  static WatchEventsRequest create() => WatchEventsRequest._();
  static $pb.GeneratedMessage $_createMessage() => WatchEventsRequest._();
  @$core.override
  WatchEventsRequest createEmptyInstance() => WatchEventsRequest._();
  @$core.pragma('dart2js:noInline')
  static WatchEventsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WatchEventsRequest>(
          WatchEventsRequest.$_createMessage);
  static WatchEventsRequest? _defaultInstance;

  /// 手机已收到的最大主机事件 seq;daemon 先补发仍保留的、seq 更大的落盘事件,
  /// 再进入实时推送。0 = 不补发(只收实时)。
  @$pb.TagNumber(1)
  $fixnum.Int64 get afterSequence => $_getI64(0);
  @$pb.TagNumber(1)
  set afterSequence($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAfterSequence() => $_has(0);
  @$pb.TagNumber(1)
  void clearAfterSequence() => $_clearField(1);
}

enum HostEvent_Event { turnFinished, selectionChanged, notSet }

class HostEvent extends $pb.GeneratedMessage {
  factory HostEvent({
    $fixnum.Int64? timestampUnixMs,
    TurnFinishedNotification? turnFinished,
    $fixnum.Int64? sequence,
    SelectionChangedNotification? selectionChanged,
  }) {
    final result = HostEvent._();
    if (timestampUnixMs != null) result.timestampUnixMs = timestampUnixMs;
    if (turnFinished != null) result.turnFinished = turnFinished;
    if (sequence != null) result.sequence = sequence;
    if (selectionChanged != null) result.selectionChanged = selectionChanged;
    return result;
  }

  HostEvent._();

  factory HostEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostEvent()..mergeFromBuffer(data, registry);
  factory HostEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostEvent()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, HostEvent_Event> _HostEvent_EventByTag = {
    5: HostEvent_Event.turnFinished,
    7: HostEvent_Event.selectionChanged,
    0: HostEvent_Event.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostEvent.$_createMessage)
    ..oo(0, [5, 7])
    ..aInt64(1, _omitFieldNames ? '' : 'timestampUnixMs')
    ..aOM<TurnFinishedNotification>(5, _omitFieldNames ? '' : 'turnFinished',
        subBuilder: TurnFinishedNotification.$_createMessage)
    ..aInt64(6, _omitFieldNames ? '' : 'sequence')
    ..aOM<SelectionChangedNotification>(
        7, _omitFieldNames ? '' : 'selectionChanged',
        subBuilder: SelectionChangedNotification.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostEvent copyWith(void Function(HostEvent) updates) =>
      super.copyWith((message) => updates(message as HostEvent)) as HostEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HostEvent() / HostEvent.new instead')
  static HostEvent create() => HostEvent._();
  static $pb.GeneratedMessage $_createMessage() => HostEvent._();
  @$core.override
  HostEvent createEmptyInstance() => HostEvent._();
  @$core.pragma('dart2js:noInline')
  static HostEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HostEvent>(HostEvent.$_createMessage);
  static HostEvent? _defaultInstance;

  @$pb.TagNumber(5)
  @$pb.TagNumber(7)
  HostEvent_Event whichEvent() => _HostEvent_EventByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(5)
  @$pb.TagNumber(7)
  void clearEvent() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $fixnum.Int64 get timestampUnixMs => $_getI64(0);
  @$pb.TagNumber(1)
  set timestampUnixMs($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTimestampUnixMs() => $_has(0);
  @$pb.TagNumber(1)
  void clearTimestampUnixMs() => $_clearField(1);

  @$pb.TagNumber(5)
  TurnFinishedNotification get turnFinished => $_getN(1);
  @$pb.TagNumber(5)
  set turnFinished(TurnFinishedNotification value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasTurnFinished() => $_has(1);
  @$pb.TagNumber(5)
  void clearTurnFinished() => $_clearField(5);
  @$pb.TagNumber(5)
  TurnFinishedNotification ensureTurnFinished() => $_ensure(1);

  /// P1-5:全局单调事件序号(daemon events.jsonl),与 TurnServerMessage.sequence
  /// 同一序列空间;0 = 未入事件库(旧 daemon)。
  @$pb.TagNumber(6)
  $fixnum.Int64 get sequence => $_getI64(2);
  @$pb.TagNumber(6)
  set sequence($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(6)
  $core.bool hasSequence() => $_has(2);
  @$pb.TagNumber(6)
  void clearSequence() => $_clearField(6);

  /// ---- 线程有效选择变化通知(只增:7,P3-T2;字段号分配登记见 proto/FIELD_ALLOCATIONS.md)----
  /// 某线程的"实际生效"模型/思考强度选择发生变化(手机端改的、电脑端改的、
  /// 委派指定落定,都归一成这一条)。【推送纪律】daemon 重启后首轮**不发**本
  /// 事件 —— UI 的初始真值来自 TurnStarted.source / 查询型 RPC(effective_
  /// selection),通知只负责"之后的增量",两路真值不互相兜底。旧 daemon 不发
  /// 本分支,手机端按"无此通知"处理,不报错。
  @$pb.TagNumber(7)
  SelectionChangedNotification get selectionChanged => $_getN(3);
  @$pb.TagNumber(7)
  set selectionChanged(SelectionChangedNotification value) =>
      $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasSelectionChanged() => $_has(3);
  @$pb.TagNumber(7)
  void clearSelectionChanged() => $_clearField(7);
  @$pb.TagNumber(7)
  SelectionChangedNotification ensureSelectionChanged() => $_ensure(3);
}

class TurnFinishedNotification extends $pb.GeneratedMessage {
  factory TurnFinishedNotification({
    $core.String? threadId,
    $core.String? summary,
  }) {
    final result = TurnFinishedNotification._();
    if (threadId != null) result.threadId = threadId;
    if (summary != null) result.summary = summary;
    return result;
  }

  TurnFinishedNotification._();

  factory TurnFinishedNotification.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnFinishedNotification()..mergeFromBuffer(data, registry);
  factory TurnFinishedNotification.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TurnFinishedNotification()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TurnFinishedNotification',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TurnFinishedNotification.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'summary')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnFinishedNotification clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TurnFinishedNotification copyWith(
          void Function(TurnFinishedNotification) updates) =>
      super.copyWith((message) => updates(message as TurnFinishedNotification))
          as TurnFinishedNotification;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use TurnFinishedNotification() / TurnFinishedNotification.new instead')
  static TurnFinishedNotification create() => TurnFinishedNotification._();
  static $pb.GeneratedMessage $_createMessage() => TurnFinishedNotification._();
  @$core.override
  TurnFinishedNotification createEmptyInstance() =>
      TurnFinishedNotification._();
  @$core.pragma('dart2js:noInline')
  static TurnFinishedNotification getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TurnFinishedNotification>(
          TurnFinishedNotification.$_createMessage);
  static TurnFinishedNotification? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get summary => $_getSZ(1);
  @$pb.TagNumber(2)
  set summary($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSummary() => $_has(1);
  @$pb.TagNumber(2)
  void clearSummary() => $_clearField(2);
}

/// SelectionChangedNotification 某线程的"实际生效"模型/思考强度选择发生变化
/// (只增,P3-T2)。selection 是**整包真值**(EffectiveModelSelection 原样,含
/// 来源/确认态/下一轮生效标记),不是 diff —— 客户端整包覆盖本地缓存即可。
/// daemon 重启后首轮不发:初始真值以 TurnStarted.source / 查询型 RPC 为准,
/// 本通知只承担之后的增量(见 HostEvent.selection_changed 的推送纪律)。
class SelectionChangedNotification extends $pb.GeneratedMessage {
  factory SelectionChangedNotification({
    $core.String? threadId,
    $1.EffectiveModelSelection? selection,
  }) {
    final result = SelectionChangedNotification._();
    if (threadId != null) result.threadId = threadId;
    if (selection != null) result.selection = selection;
    return result;
  }

  SelectionChangedNotification._();

  factory SelectionChangedNotification.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SelectionChangedNotification()..mergeFromBuffer(data, registry);
  factory SelectionChangedNotification.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SelectionChangedNotification()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SelectionChangedNotification',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SelectionChangedNotification.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOM<$1.EffectiveModelSelection>(2, _omitFieldNames ? '' : 'selection',
        subBuilder: $1.EffectiveModelSelection.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectionChangedNotification clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SelectionChangedNotification copyWith(
          void Function(SelectionChangedNotification) updates) =>
      super.copyWith(
              (message) => updates(message as SelectionChangedNotification))
          as SelectionChangedNotification;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SelectionChangedNotification() / SelectionChangedNotification.new instead')
  static SelectionChangedNotification create() =>
      SelectionChangedNotification._();
  static $pb.GeneratedMessage $_createMessage() =>
      SelectionChangedNotification._();
  @$core.override
  SelectionChangedNotification createEmptyInstance() =>
      SelectionChangedNotification._();
  @$core.pragma('dart2js:noInline')
  static SelectionChangedNotification getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SelectionChangedNotification>(
          SelectionChangedNotification.$_createMessage);
  static SelectionChangedNotification? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  /// 变化后的有效选择整包;空 = 该线程选择被清回默认(同样是合法状态)。
  @$pb.TagNumber(2)
  $1.EffectiveModelSelection get selection => $_getN(1);
  @$pb.TagNumber(2)
  set selection($1.EffectiveModelSelection value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSelection() => $_has(1);
  @$pb.TagNumber(2)
  void clearSelection() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.EffectiveModelSelection ensureSelection() => $_ensure(1);
}

/// ---- P2:电脑 dev server 探测(手机一键预览,对齐原作 BrowserService)----
class ListDevServersRequest extends $pb.GeneratedMessage {
  factory ListDevServersRequest() => ListDevServersRequest._();

  ListDevServersRequest._();

  factory ListDevServersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListDevServersRequest()..mergeFromBuffer(data, registry);
  factory ListDevServersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListDevServersRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListDevServersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListDevServersRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListDevServersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListDevServersRequest copyWith(
          void Function(ListDevServersRequest) updates) =>
      super.copyWith((message) => updates(message as ListDevServersRequest))
          as ListDevServersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListDevServersRequest() / ListDevServersRequest.new instead')
  static ListDevServersRequest create() => ListDevServersRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListDevServersRequest._();
  @$core.override
  ListDevServersRequest createEmptyInstance() => ListDevServersRequest._();
  @$core.pragma('dart2js:noInline')
  static ListDevServersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListDevServersRequest>(
          ListDevServersRequest.$_createMessage);
  static ListDevServersRequest? _defaultInstance;
}

/// DevServer:电脑上探到的一个 HTTP 服务端口。
class DevServer extends $pb.GeneratedMessage {
  factory DevServer({
    $core.int? port,
    $core.String? url,
    $core.String? title,
    $core.String? serverHeader,
  }) {
    final result = DevServer._();
    if (port != null) result.port = port;
    if (url != null) result.url = url;
    if (title != null) result.title = title;
    if (serverHeader != null) result.serverHeader = serverHeader;
    return result;
  }

  DevServer._();

  factory DevServer.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DevServer()..mergeFromBuffer(data, registry);
  factory DevServer.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DevServer()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DevServer',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DevServer.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'port')
    ..aOS(2, _omitFieldNames ? '' : 'url')
    ..aOS(3, _omitFieldNames ? '' : 'title')
    ..aOS(4, _omitFieldNames ? '' : 'serverHeader')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DevServer clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DevServer copyWith(void Function(DevServer) updates) =>
      super.copyWith((message) => updates(message as DevServer)) as DevServer;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DevServer() / DevServer.new instead')
  static DevServer create() => DevServer._();
  static $pb.GeneratedMessage $_createMessage() => DevServer._();
  @$core.override
  DevServer createEmptyInstance() => DevServer._();
  @$core.pragma('dart2js:noInline')
  static DevServer getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DevServer>(DevServer.$_createMessage);
  static DevServer? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get port => $_getIZ(0);
  @$pb.TagNumber(1)
  set port($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPort() => $_has(0);
  @$pb.TagNumber(1)
  void clearPort() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get url => $_getSZ(1);
  @$pb.TagNumber(2)
  set url($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUrl() => $_has(1);
  @$pb.TagNumber(2)
  void clearUrl() => $_clearField(2);

  /// 手机端要自行把主机部分换成 daemon 的局域网/Tailscale 地址
  @$pb.TagNumber(3)
  $core.String get title => $_getSZ(2);
  @$pb.TagNumber(3)
  set title($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get serverHeader => $_getSZ(3);
  @$pb.TagNumber(4)
  set serverHeader($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasServerHeader() => $_has(3);
  @$pb.TagNumber(4)
  void clearServerHeader() => $_clearField(4);
}

class DevServerList extends $pb.GeneratedMessage {
  factory DevServerList({
    $core.Iterable<DevServer>? servers,
    $fixnum.Int64? scannedAtUnixMs,
  }) {
    final result = DevServerList._();
    if (servers != null) result.servers.addAll(servers);
    if (scannedAtUnixMs != null) result.scannedAtUnixMs = scannedAtUnixMs;
    return result;
  }

  DevServerList._();

  factory DevServerList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DevServerList()..mergeFromBuffer(data, registry);
  factory DevServerList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DevServerList()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DevServerList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DevServerList.$_createMessage)
    ..pPM<DevServer>(1, _omitFieldNames ? '' : 'servers',
        subBuilder: DevServer.$_createMessage)
    ..aInt64(2, _omitFieldNames ? '' : 'scannedAtUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DevServerList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DevServerList copyWith(void Function(DevServerList) updates) =>
      super.copyWith((message) => updates(message as DevServerList))
          as DevServerList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DevServerList() / DevServerList.new instead')
  static DevServerList create() => DevServerList._();
  static $pb.GeneratedMessage $_createMessage() => DevServerList._();
  @$core.override
  DevServerList createEmptyInstance() => DevServerList._();
  @$core.pragma('dart2js:noInline')
  static DevServerList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DevServerList>(
          DevServerList.$_createMessage);
  static DevServerList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<DevServer> get servers => $_getList(0);

  /// 结果生成时间(daemon 本地时钟,Unix 毫秒)。手机端可提示"多久前扫的";0 = 旧 daemon。
  @$pb.TagNumber(2)
  $fixnum.Int64 get scannedAtUnixMs => $_getI64(1);
  @$pb.TagNumber(2)
  set scannedAtUnixMs($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasScannedAtUnixMs() => $_has(1);
  @$pb.TagNumber(2)
  void clearScannedAtUnixMs() => $_clearField(2);
}

/// ---- W6b:启动恢复诊断(规格 §8.2 恢复三分支的进程级视图)----
///
/// 【为什么整块单列,而不是把关键字段摊平进 HostInfo】这张表是"本次启动的健康度
/// 快照",手机端作为一个单元整体呈现(顶部横幅 + 可展开明细);摊平后字段互相失去
/// 归属,HostInfo 的后续演进也更容易撞号。
class HostRecoveryReport extends $pb.GeneratedMessage {
  factory HostRecoveryReport({
    $core.String? status,
    $core.String? summary,
    HostStoreHealth? store,
    HostTurnJournalHealth? turnJournal,
    HostDeviceHealth? devices,
    HostIdentityHealth? identity,
  }) {
    final result = HostRecoveryReport._();
    if (status != null) result.status = status;
    if (summary != null) result.summary = summary;
    if (store != null) result.store = store;
    if (turnJournal != null) result.turnJournal = turnJournal;
    if (devices != null) result.devices = devices;
    if (identity != null) result.identity = identity;
    return result;
  }

  HostRecoveryReport._();

  factory HostRecoveryReport.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostRecoveryReport()..mergeFromBuffer(data, registry);
  factory HostRecoveryReport.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostRecoveryReport()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostRecoveryReport',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostRecoveryReport.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'status')
    ..aOS(2, _omitFieldNames ? '' : 'summary')
    ..aOM<HostStoreHealth>(3, _omitFieldNames ? '' : 'store',
        subBuilder: HostStoreHealth.$_createMessage)
    ..aOM<HostTurnJournalHealth>(4, _omitFieldNames ? '' : 'turnJournal',
        subBuilder: HostTurnJournalHealth.$_createMessage)
    ..aOM<HostDeviceHealth>(5, _omitFieldNames ? '' : 'devices',
        subBuilder: HostDeviceHealth.$_createMessage)
    ..aOM<HostIdentityHealth>(6, _omitFieldNames ? '' : 'identity',
        subBuilder: HostIdentityHealth.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostRecoveryReport clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostRecoveryReport copyWith(void Function(HostRecoveryReport) updates) =>
      super.copyWith((message) => updates(message as HostRecoveryReport))
          as HostRecoveryReport;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HostRecoveryReport() / HostRecoveryReport.new instead')
  static HostRecoveryReport create() => HostRecoveryReport._();
  static $pb.GeneratedMessage $_createMessage() => HostRecoveryReport._();
  @$core.override
  HostRecoveryReport createEmptyInstance() => HostRecoveryReport._();
  @$core.pragma('dart2js:noInline')
  static HostRecoveryReport getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HostRecoveryReport>(
          HostRecoveryReport.$_createMessage);
  static HostRecoveryReport? _defaultInstance;

  /// ok / degraded / corrupted —— 与 daemon 侧 RecoveryReport.Status 同值域。
  /// 【为什么用 string 不用 enum】手机端只做透传展示,值域由 daemon 单方面定义;
  /// 新增档位不该要求旧 APK 重新生成契约(旧枚举值会落进 UNRECOGNIZED)。
  @$pb.TagNumber(1)
  $core.String get status => $_getSZ(0);
  @$pb.TagNumber(1)
  set status($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatus() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get summary => $_getSZ(1);
  @$pb.TagNumber(2)
  set summary($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSummary() => $_has(1);
  @$pb.TagNumber(2)
  void clearSummary() => $_clearField(2);

  @$pb.TagNumber(3)
  HostStoreHealth get store => $_getN(2);
  @$pb.TagNumber(3)
  set store(HostStoreHealth value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasStore() => $_has(2);
  @$pb.TagNumber(3)
  void clearStore() => $_clearField(3);
  @$pb.TagNumber(3)
  HostStoreHealth ensureStore() => $_ensure(2);

  @$pb.TagNumber(4)
  HostTurnJournalHealth get turnJournal => $_getN(3);
  @$pb.TagNumber(4)
  set turnJournal(HostTurnJournalHealth value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasTurnJournal() => $_has(3);
  @$pb.TagNumber(4)
  void clearTurnJournal() => $_clearField(4);
  @$pb.TagNumber(4)
  HostTurnJournalHealth ensureTurnJournal() => $_ensure(3);

  @$pb.TagNumber(5)
  HostDeviceHealth get devices => $_getN(4);
  @$pb.TagNumber(5)
  set devices(HostDeviceHealth value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasDevices() => $_has(4);
  @$pb.TagNumber(5)
  void clearDevices() => $_clearField(5);
  @$pb.TagNumber(5)
  HostDeviceHealth ensureDevices() => $_ensure(4);

  @$pb.TagNumber(6)
  HostIdentityHealth get identity => $_getN(5);
  @$pb.TagNumber(6)
  set identity(HostIdentityHealth value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasIdentity() => $_has(5);
  @$pb.TagNumber(6)
  void clearIdentity() => $_clearField(6);
  @$pb.TagNumber(6)
  HostIdentityHealth ensureIdentity() => $_ensure(5);
}

class HostStoreHealth extends $pb.GeneratedMessage {
  factory HostStoreHealth({
    $core.String? outcome,
    $core.bool? hasBackup,
    $core.String? quarantinePath,
  }) {
    final result = HostStoreHealth._();
    if (outcome != null) result.outcome = outcome;
    if (hasBackup != null) result.hasBackup = hasBackup;
    if (quarantinePath != null) result.quarantinePath = quarantinePath;
    return result;
  }

  HostStoreHealth._();

  factory HostStoreHealth.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostStoreHealth()..mergeFromBuffer(data, registry);
  factory HostStoreHealth.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostStoreHealth()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostStoreHealth',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostStoreHealth.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'outcome')
    ..aOB(2, _omitFieldNames ? '' : 'hasBackup')
    ..aOS(3, _omitFieldNames ? '' : 'quarantinePath')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostStoreHealth clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostStoreHealth copyWith(void Function(HostStoreHealth) updates) =>
      super.copyWith((message) => updates(message as HostStoreHealth))
          as HostStoreHealth;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HostStoreHealth() / HostStoreHealth.new instead')
  static HostStoreHealth create() => HostStoreHealth._();
  static $pb.GeneratedMessage $_createMessage() => HostStoreHealth._();
  @$core.override
  HostStoreHealth createEmptyInstance() => HostStoreHealth._();
  @$core.pragma('dart2js:noInline')
  static HostStoreHealth getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<HostStoreHealth>(
          HostStoreHealth.$_createMessage);
  static HostStoreHealth? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get outcome => $_getSZ(0);
  @$pb.TagNumber(1)
  set outcome($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOutcome() => $_has(0);
  @$pb.TagNumber(1)
  void clearOutcome() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get hasBackup => $_getBF(1);
  @$pb.TagNumber(2)
  set hasBackup($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHasBackup() => $_has(1);
  @$pb.TagNumber(2)
  void clearHasBackup() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get quarantinePath => $_getSZ(2);
  @$pb.TagNumber(3)
  set quarantinePath($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasQuarantinePath() => $_has(2);
  @$pb.TagNumber(3)
  void clearQuarantinePath() => $_clearField(3);
}

/// HostTurnJournalHealth 是"排队输入丢失可见"的载体:条数、涉及会话、逐条明细。
/// 【为什么条数与明细都要】只有条数,用户仍然不知道"丢的是哪句话";只有明细,
/// 横幅文案又得在手机端重算总量。两者都给,各取所需。
class HostTurnJournalHealth extends $pb.GeneratedMessage {
  factory HostTurnJournalHealth({
    $core.int? pendingCount,
    $core.Iterable<$core.String>? threadIds,
    $core.Iterable<HostLostQueueInput>? lostInputs,
  }) {
    final result = HostTurnJournalHealth._();
    if (pendingCount != null) result.pendingCount = pendingCount;
    if (threadIds != null) result.threadIds.addAll(threadIds);
    if (lostInputs != null) result.lostInputs.addAll(lostInputs);
    return result;
  }

  HostTurnJournalHealth._();

  factory HostTurnJournalHealth.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostTurnJournalHealth()..mergeFromBuffer(data, registry);
  factory HostTurnJournalHealth.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostTurnJournalHealth()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostTurnJournalHealth',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostTurnJournalHealth.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'pendingCount')
    ..pPS(2, _omitFieldNames ? '' : 'threadIds')
    ..pPM<HostLostQueueInput>(3, _omitFieldNames ? '' : 'lostInputs',
        subBuilder: HostLostQueueInput.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostTurnJournalHealth clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostTurnJournalHealth copyWith(
          void Function(HostTurnJournalHealth) updates) =>
      super.copyWith((message) => updates(message as HostTurnJournalHealth))
          as HostTurnJournalHealth;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use HostTurnJournalHealth() / HostTurnJournalHealth.new instead')
  static HostTurnJournalHealth create() => HostTurnJournalHealth._();
  static $pb.GeneratedMessage $_createMessage() => HostTurnJournalHealth._();
  @$core.override
  HostTurnJournalHealth createEmptyInstance() => HostTurnJournalHealth._();
  @$core.pragma('dart2js:noInline')
  static HostTurnJournalHealth getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HostTurnJournalHealth>(
          HostTurnJournalHealth.$_createMessage);
  static HostTurnJournalHealth? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get pendingCount => $_getIZ(0);
  @$pb.TagNumber(1)
  set pendingCount($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPendingCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearPendingCount() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get threadIds => $_getList(1);

  @$pb.TagNumber(3)
  $pb.PbList<HostLostQueueInput> get lostInputs => $_getList(2);
}

class HostLostQueueInput extends $pb.GeneratedMessage {
  factory HostLostQueueInput({
    $core.String? threadId,
    $core.String? preview,
    $fixnum.Int64? createdAtUnixMs,
  }) {
    final result = HostLostQueueInput._();
    if (threadId != null) result.threadId = threadId;
    if (preview != null) result.preview = preview;
    if (createdAtUnixMs != null) result.createdAtUnixMs = createdAtUnixMs;
    return result;
  }

  HostLostQueueInput._();

  factory HostLostQueueInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostLostQueueInput()..mergeFromBuffer(data, registry);
  factory HostLostQueueInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostLostQueueInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostLostQueueInput',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostLostQueueInput.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'threadId')
    ..aOS(2, _omitFieldNames ? '' : 'preview')
    ..aInt64(3, _omitFieldNames ? '' : 'createdAtUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostLostQueueInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostLostQueueInput copyWith(void Function(HostLostQueueInput) updates) =>
      super.copyWith((message) => updates(message as HostLostQueueInput))
          as HostLostQueueInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HostLostQueueInput() / HostLostQueueInput.new instead')
  static HostLostQueueInput create() => HostLostQueueInput._();
  static $pb.GeneratedMessage $_createMessage() => HostLostQueueInput._();
  @$core.override
  HostLostQueueInput createEmptyInstance() => HostLostQueueInput._();
  @$core.pragma('dart2js:noInline')
  static HostLostQueueInput getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HostLostQueueInput>(
          HostLostQueueInput.$_createMessage);
  static HostLostQueueInput? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get threadId => $_getSZ(0);
  @$pb.TagNumber(1)
  set threadId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasThreadId() => $_has(0);
  @$pb.TagNumber(1)
  void clearThreadId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get preview => $_getSZ(1);
  @$pb.TagNumber(2)
  set preview($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPreview() => $_has(1);
  @$pb.TagNumber(2)
  void clearPreview() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get createdAtUnixMs => $_getI64(2);
  @$pb.TagNumber(3)
  set createdAtUnixMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCreatedAtUnixMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearCreatedAtUnixMs() => $_clearField(3);
}

class HostDeviceHealth extends $pb.GeneratedMessage {
  factory HostDeviceHealth({
    $core.bool? degraded,
    $core.String? reason,
  }) {
    final result = HostDeviceHealth._();
    if (degraded != null) result.degraded = degraded;
    if (reason != null) result.reason = reason;
    return result;
  }

  HostDeviceHealth._();

  factory HostDeviceHealth.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostDeviceHealth()..mergeFromBuffer(data, registry);
  factory HostDeviceHealth.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostDeviceHealth()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostDeviceHealth',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostDeviceHealth.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'degraded')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostDeviceHealth clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostDeviceHealth copyWith(void Function(HostDeviceHealth) updates) =>
      super.copyWith((message) => updates(message as HostDeviceHealth))
          as HostDeviceHealth;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HostDeviceHealth() / HostDeviceHealth.new instead')
  static HostDeviceHealth create() => HostDeviceHealth._();
  static $pb.GeneratedMessage $_createMessage() => HostDeviceHealth._();
  @$core.override
  HostDeviceHealth createEmptyInstance() => HostDeviceHealth._();
  @$core.pragma('dart2js:noInline')
  static HostDeviceHealth getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<HostDeviceHealth>(
          HostDeviceHealth.$_createMessage);
  static HostDeviceHealth? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get degraded => $_getBF(0);
  @$pb.TagNumber(1)
  set degraded($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDegraded() => $_has(0);
  @$pb.TagNumber(1)
  void clearDegraded() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

class HostIdentityHealth extends $pb.GeneratedMessage {
  factory HostIdentityHealth({
    $core.bool? degraded,
    $core.String? reason,
  }) {
    final result = HostIdentityHealth._();
    if (degraded != null) result.degraded = degraded;
    if (reason != null) result.reason = reason;
    return result;
  }

  HostIdentityHealth._();

  factory HostIdentityHealth.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostIdentityHealth()..mergeFromBuffer(data, registry);
  factory HostIdentityHealth.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      HostIdentityHealth()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HostIdentityHealth',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: HostIdentityHealth.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'degraded')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostIdentityHealth clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HostIdentityHealth copyWith(void Function(HostIdentityHealth) updates) =>
      super.copyWith((message) => updates(message as HostIdentityHealth))
          as HostIdentityHealth;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use HostIdentityHealth() / HostIdentityHealth.new instead')
  static HostIdentityHealth create() => HostIdentityHealth._();
  static $pb.GeneratedMessage $_createMessage() => HostIdentityHealth._();
  @$core.override
  HostIdentityHealth createEmptyInstance() => HostIdentityHealth._();
  @$core.pragma('dart2js:noInline')
  static HostIdentityHealth getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HostIdentityHealth>(
          HostIdentityHealth.$_createMessage);
  static HostIdentityHealth? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get degraded => $_getBF(0);
  @$pb.TagNumber(1)
  set degraded($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDegraded() => $_has(0);
  @$pb.TagNumber(1)
  void clearDegraded() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

/// 配对前置问询(自动配对)。【安全模型】handler 仅应答本机回环来源(USB reverse
/// 场景);daemon 关闭鉴权(-insecure-no-auth)时如实回答 pairing_enabled=false。
class GetPairingInfoRequest extends $pb.GeneratedMessage {
  factory GetPairingInfoRequest() => GetPairingInfoRequest._();

  GetPairingInfoRequest._();

  factory GetPairingInfoRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPairingInfoRequest()..mergeFromBuffer(data, registry);
  factory GetPairingInfoRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPairingInfoRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPairingInfoRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetPairingInfoRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPairingInfoRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPairingInfoRequest copyWith(
          void Function(GetPairingInfoRequest) updates) =>
      super.copyWith((message) => updates(message as GetPairingInfoRequest))
          as GetPairingInfoRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetPairingInfoRequest() / GetPairingInfoRequest.new instead')
  static GetPairingInfoRequest create() => GetPairingInfoRequest._();
  static $pb.GeneratedMessage $_createMessage() => GetPairingInfoRequest._();
  @$core.override
  GetPairingInfoRequest createEmptyInstance() => GetPairingInfoRequest._();
  @$core.pragma('dart2js:noInline')
  static GetPairingInfoRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPairingInfoRequest>(
          GetPairingInfoRequest.$_createMessage);
  static GetPairingInfoRequest? _defaultInstance;
}

class GetPairingInfoResponse extends $pb.GeneratedMessage {
  factory GetPairingInfoResponse({
    $core.bool? pairingEnabled,
    $core.String? pairCode,
    $core.String? daemonVersion,
  }) {
    final result = GetPairingInfoResponse._();
    if (pairingEnabled != null) result.pairingEnabled = pairingEnabled;
    if (pairCode != null) result.pairCode = pairCode;
    if (daemonVersion != null) result.daemonVersion = daemonVersion;
    return result;
  }

  GetPairingInfoResponse._();

  factory GetPairingInfoResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPairingInfoResponse()..mergeFromBuffer(data, registry);
  factory GetPairingInfoResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GetPairingInfoResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPairingInfoResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GetPairingInfoResponse.$_createMessage)
    ..aOB(1, _omitFieldNames ? '' : 'pairingEnabled')
    ..aOS(2, _omitFieldNames ? '' : 'pairCode')
    ..aOS(3, _omitFieldNames ? '' : 'daemonVersion')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPairingInfoResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPairingInfoResponse copyWith(
          void Function(GetPairingInfoResponse) updates) =>
      super.copyWith((message) => updates(message as GetPairingInfoResponse))
          as GetPairingInfoResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use GetPairingInfoResponse() / GetPairingInfoResponse.new instead')
  static GetPairingInfoResponse create() => GetPairingInfoResponse._();
  static $pb.GeneratedMessage $_createMessage() => GetPairingInfoResponse._();
  @$core.override
  GetPairingInfoResponse createEmptyInstance() => GetPairingInfoResponse._();
  @$core.pragma('dart2js:noInline')
  static GetPairingInfoResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPairingInfoResponse>(
          GetPairingInfoResponse.$_createMessage);
  static GetPairingInfoResponse? _defaultInstance;

  /// true = 配对码模式开启,且本次来源被允许读取配对码。
  @$pb.TagNumber(1)
  $core.bool get pairingEnabled => $_getBF(0);
  @$pb.TagNumber(1)
  set pairingEnabled($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPairingEnabled() => $_has(0);
  @$pb.TagNumber(1)
  void clearPairingEnabled() => $_clearField(1);

  /// 当前生效配对码(持久码或一次性码均指它;配对成功即走 Authorize 换 token)。
  @$pb.TagNumber(2)
  $core.String get pairCode => $_getSZ(1);
  @$pb.TagNumber(2)
  set pairCode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPairCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearPairCode() => $_clearField(2);

  /// daemon 版本(便于手机端展示"发现的主机"卡片)。
  @$pb.TagNumber(3)
  $core.String get daemonVersion => $_getSZ(2);
  @$pb.TagNumber(3)
  set daemonVersion($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDaemonVersion() => $_has(2);
  @$pb.TagNumber(3)
  void clearDaemonVersion() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
