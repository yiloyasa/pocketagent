// This is a generated file - do not edit.
//
// Generated from v1/host.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'host.pb.dart' as $0;

export 'host.pb.dart';

@$pb.GrpcServiceName('pocketagent.v1.HostService')
class HostServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  HostServiceClient(super.channel, {super.options, super.interceptors});

  /// 局域网发现后的第一问:你是谁、你能干什么
  $grpc.ResponseFuture<$0.HostInfo> getHostInfo(
    $0.GetHostInfoRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getHostInfo, request, options: options);
  }

  /// 配对前置问询(2026-09-30 自动配对):返回当前配对码,供 USB/回环来源的
  /// 手机自动完成配对(免手输)。handler 内自检来源必须为本机回环(USB reverse
  /// 后手机请求即回环);局域网/Tailscale 来源一律拒绝 —— 物理接触才拿得到码。
  $grpc.ResponseFuture<$0.GetPairingInfoResponse> getPairingInfo(
    $0.GetPairingInfoRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPairingInfo, request, options: options);
  }

  /// 配对码换长期 token
  $grpc.ResponseFuture<$0.AuthorizeResponse> authorize(
    $0.AuthorizeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$authorize, request, options: options);
  }

  $grpc.ResponseFuture<$0.PingResponse> ping(
    $0.PingRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$ping, request, options: options);
  }

  /// 主机状态推送。当前唯一生产事件是 turn_finished(后台轮次完成,推送场景);
  /// 早期规划的 workspace_changed/model_ready/update_available 三分支自基线起
  /// 就无 daemon 生产点、app 零消费(2026-09-26 审计 R6/S2 双端 grep 实证),已删。
  /// 重引入主机态事件时另起新字段号 —— 旧号已 reserved,复用会让持有旧契约的
  /// 对端把新事件错解成旧消息类型。
  $grpc.ResponseStream<$0.HostEvent> watchEvents(
    $0.WatchEventsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$watchEvents, $async.Stream.fromIterable([request]),
        options: options);
  }

  /// P2:列出本机正在运行的 dev server(daemon 拨常见端口,1s/端口,整扫 ≤8s,
  /// 结果缓存 60s —— 手机手动刷新才重扫)。对齐原作 BrowserService.ListDevServers。
  $grpc.ResponseFuture<$0.DevServerList> listDevServers(
    $0.ListDevServersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listDevServers, request, options: options);
  }

  // method descriptors

  static final _$getHostInfo =
      $grpc.ClientMethod<$0.GetHostInfoRequest, $0.HostInfo>(
          '/pocketagent.v1.HostService/GetHostInfo',
          ($0.GetHostInfoRequest value) => value.writeToBuffer(),
          $0.HostInfo.fromBuffer);
  static final _$getPairingInfo =
      $grpc.ClientMethod<$0.GetPairingInfoRequest, $0.GetPairingInfoResponse>(
          '/pocketagent.v1.HostService/GetPairingInfo',
          ($0.GetPairingInfoRequest value) => value.writeToBuffer(),
          $0.GetPairingInfoResponse.fromBuffer);
  static final _$authorize =
      $grpc.ClientMethod<$0.AuthorizeRequest, $0.AuthorizeResponse>(
          '/pocketagent.v1.HostService/Authorize',
          ($0.AuthorizeRequest value) => value.writeToBuffer(),
          $0.AuthorizeResponse.fromBuffer);
  static final _$ping = $grpc.ClientMethod<$0.PingRequest, $0.PingResponse>(
      '/pocketagent.v1.HostService/Ping',
      ($0.PingRequest value) => value.writeToBuffer(),
      $0.PingResponse.fromBuffer);
  static final _$watchEvents =
      $grpc.ClientMethod<$0.WatchEventsRequest, $0.HostEvent>(
          '/pocketagent.v1.HostService/WatchEvents',
          ($0.WatchEventsRequest value) => value.writeToBuffer(),
          $0.HostEvent.fromBuffer);
  static final _$listDevServers =
      $grpc.ClientMethod<$0.ListDevServersRequest, $0.DevServerList>(
          '/pocketagent.v1.HostService/ListDevServers',
          ($0.ListDevServersRequest value) => value.writeToBuffer(),
          $0.DevServerList.fromBuffer);
}

@$pb.GrpcServiceName('pocketagent.v1.HostService')
abstract class HostServiceBase extends $grpc.Service {
  $core.String get $name => 'pocketagent.v1.HostService';

  HostServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GetHostInfoRequest, $0.HostInfo>(
        'GetHostInfo',
        getHostInfo_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetHostInfoRequest.fromBuffer(value),
        ($0.HostInfo value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetPairingInfoRequest,
            $0.GetPairingInfoResponse>(
        'GetPairingInfo',
        getPairingInfo_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetPairingInfoRequest.fromBuffer(value),
        ($0.GetPairingInfoResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AuthorizeRequest, $0.AuthorizeResponse>(
        'Authorize',
        authorize_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.AuthorizeRequest.fromBuffer(value),
        ($0.AuthorizeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.PingRequest, $0.PingResponse>(
        'Ping',
        ping_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.PingRequest.fromBuffer(value),
        ($0.PingResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.WatchEventsRequest, $0.HostEvent>(
        'WatchEvents',
        watchEvents_Pre,
        false,
        true,
        ($core.List<$core.int> value) =>
            $0.WatchEventsRequest.fromBuffer(value),
        ($0.HostEvent value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListDevServersRequest, $0.DevServerList>(
        'ListDevServers',
        listDevServers_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListDevServersRequest.fromBuffer(value),
        ($0.DevServerList value) => value.writeToBuffer()));
  }

  $async.Future<$0.HostInfo> getHostInfo_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetHostInfoRequest> $request) async {
    return getHostInfo($call, await $request);
  }

  $async.Future<$0.HostInfo> getHostInfo(
      $grpc.ServiceCall call, $0.GetHostInfoRequest request);

  $async.Future<$0.GetPairingInfoResponse> getPairingInfo_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetPairingInfoRequest> $request) async {
    return getPairingInfo($call, await $request);
  }

  $async.Future<$0.GetPairingInfoResponse> getPairingInfo(
      $grpc.ServiceCall call, $0.GetPairingInfoRequest request);

  $async.Future<$0.AuthorizeResponse> authorize_Pre($grpc.ServiceCall $call,
      $async.Future<$0.AuthorizeRequest> $request) async {
    return authorize($call, await $request);
  }

  $async.Future<$0.AuthorizeResponse> authorize(
      $grpc.ServiceCall call, $0.AuthorizeRequest request);

  $async.Future<$0.PingResponse> ping_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.PingRequest> $request) async {
    return ping($call, await $request);
  }

  $async.Future<$0.PingResponse> ping(
      $grpc.ServiceCall call, $0.PingRequest request);

  $async.Stream<$0.HostEvent> watchEvents_Pre($grpc.ServiceCall $call,
      $async.Future<$0.WatchEventsRequest> $request) async* {
    yield* watchEvents($call, await $request);
  }

  $async.Stream<$0.HostEvent> watchEvents(
      $grpc.ServiceCall call, $0.WatchEventsRequest request);

  $async.Future<$0.DevServerList> listDevServers_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListDevServersRequest> $request) async {
    return listDevServers($call, await $request);
  }

  $async.Future<$0.DevServerList> listDevServers(
      $grpc.ServiceCall call, $0.ListDevServersRequest request);
}
