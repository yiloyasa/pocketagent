// This is a generated file - do not edit.
//
// Generated from v1/terminal.proto.

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

import 'common.pb.dart' as $1;
import 'terminal.pb.dart' as $0;

export 'terminal.pb.dart';

@$pb.GrpcServiceName('pocketagent.v1.TerminalService')
class TerminalServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  TerminalServiceClient(super.channel, {super.options, super.interceptors});

  /// 双向流:Open → Input*/Resize* → Output*/Exit,Close 任意方向
  $grpc.ResponseStream<$0.TerminalServerMessage> attach(
    $async.Stream<$0.TerminalClientMessage> request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(_$attach, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListTerminalsResponse> listTerminals(
    $0.ListTerminalsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listTerminals, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> closeTerminal(
    $0.CloseTerminalRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$closeTerminal, request, options: options);
  }

  // method descriptors

  static final _$attach =
      $grpc.ClientMethod<$0.TerminalClientMessage, $0.TerminalServerMessage>(
          '/pocketagent.v1.TerminalService/Attach',
          ($0.TerminalClientMessage value) => value.writeToBuffer(),
          $0.TerminalServerMessage.fromBuffer);
  static final _$listTerminals =
      $grpc.ClientMethod<$0.ListTerminalsRequest, $0.ListTerminalsResponse>(
          '/pocketagent.v1.TerminalService/ListTerminals',
          ($0.ListTerminalsRequest value) => value.writeToBuffer(),
          $0.ListTerminalsResponse.fromBuffer);
  static final _$closeTerminal =
      $grpc.ClientMethod<$0.CloseTerminalRequest, $1.Empty>(
          '/pocketagent.v1.TerminalService/CloseTerminal',
          ($0.CloseTerminalRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
}

@$pb.GrpcServiceName('pocketagent.v1.TerminalService')
abstract class TerminalServiceBase extends $grpc.Service {
  $core.String get $name => 'pocketagent.v1.TerminalService';

  TerminalServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.TerminalClientMessage, $0.TerminalServerMessage>(
            'Attach',
            attach,
            true,
            true,
            ($core.List<$core.int> value) =>
                $0.TerminalClientMessage.fromBuffer(value),
            ($0.TerminalServerMessage value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListTerminalsRequest, $0.ListTerminalsResponse>(
            'ListTerminals',
            listTerminals_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListTerminalsRequest.fromBuffer(value),
            ($0.ListTerminalsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CloseTerminalRequest, $1.Empty>(
        'CloseTerminal',
        closeTerminal_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CloseTerminalRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
  }

  $async.Stream<$0.TerminalServerMessage> attach(
      $grpc.ServiceCall call, $async.Stream<$0.TerminalClientMessage> request);

  $async.Future<$0.ListTerminalsResponse> listTerminals_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListTerminalsRequest> $request) async {
    return listTerminals($call, await $request);
  }

  $async.Future<$0.ListTerminalsResponse> listTerminals(
      $grpc.ServiceCall call, $0.ListTerminalsRequest request);

  $async.Future<$1.Empty> closeTerminal_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CloseTerminalRequest> $request) async {
    return closeTerminal($call, await $request);
  }

  $async.Future<$1.Empty> closeTerminal(
      $grpc.ServiceCall call, $0.CloseTerminalRequest request);
}
