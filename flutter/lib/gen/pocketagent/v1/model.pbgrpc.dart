// This is a generated file - do not edit.
//
// Generated from v1/model.proto.

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
import 'model.pb.dart' as $0;

export 'model.pb.dart';

@$pb.GrpcServiceName('pocketagent.v1.ModelService')
class ModelServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  ModelServiceClient(super.channel, {super.options, super.interceptors});

  /// daemon 检测到的可用模型(来自配置的 API keys)
  $grpc.ResponseFuture<$0.ListModelsResponse> listModels(
    $0.ListModelsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listModels, request, options: options);
  }

  /// 设置默认模型 / 测试连通
  $grpc.ResponseFuture<$1.Empty> setDefaultModel(
    $0.SetDefaultModelRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setDefaultModel, request, options: options);
  }

  $grpc.ResponseFuture<$0.TestModelResponse> testModel(
    $0.TestModelRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$testModel, request, options: options);
  }

  /// ---- Codex 桌面端设置(桥接 codex app-server)----
  $grpc.ResponseFuture<$0.CodexSettings> getCodexSettings(
    $0.GetCodexSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getCodexSettings, request, options: options);
  }

  $grpc.ResponseFuture<$0.CodexSettings> setCodexSettings(
    $0.SetCodexSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setCodexSettings, request, options: options);
  }

  /// MCP 服务器状态(对应 codex 的 MCP 配置)
  $grpc.ResponseFuture<$0.ListMcpServersResponse> listMcpServers(
    $0.ListMcpServersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMcpServers, request, options: options);
  }

  /// ---- 用量 / 限流(account/rateLimits/read + account/usage/read)----
  /// 用户在手机外面发任务之前,得先知道"还有没有额度"。以前只把
  /// account/rateLimits/updated 当告警转发,没落成可查看的面板。
  $grpc.ResponseFuture<$0.CodexUsage> getCodexUsage(
    $0.GetCodexUsageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getCodexUsage, request, options: options);
  }

  /// ---- 能力可见性(只读):这台电脑上这个 agent 到底有哪些技能/hooks/插件 ----
  $grpc.ResponseFuture<$0.ListCodexCapabilitiesResponse> listCodexCapabilities(
    $0.ListCodexCapabilitiesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listCodexCapabilities, request, options: options);
  }

  /// ---- 原生设置目录(透传):各家 agent 自己报告的会话级配置项 ----
  /// 【为什么是动态透传而不是静态名单】目录来自 agent 原生应答(dsh:session/new|resume
  /// 的 configOptions;pi:get_state 的状态键),daemon 不维护"哪家有哪些设置"的名单 ——
  /// 上游加一项,手机端自动出现,daemon 不用改。候选值原样透传(dsh model 的 value 是
  /// 双层 JSON 编码串,任何加工都是污染);原生没报的候选绝不伪造。
  $grpc.ResponseFuture<$0.ListAgentConfigOptionsResponse>
      listAgentConfigOptions(
    $0.ListAgentConfigOptionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listAgentConfigOptions, request,
        options: options);
  }

  // method descriptors

  static final _$listModels =
      $grpc.ClientMethod<$0.ListModelsRequest, $0.ListModelsResponse>(
          '/pocketagent.v1.ModelService/ListModels',
          ($0.ListModelsRequest value) => value.writeToBuffer(),
          $0.ListModelsResponse.fromBuffer);
  static final _$setDefaultModel =
      $grpc.ClientMethod<$0.SetDefaultModelRequest, $1.Empty>(
          '/pocketagent.v1.ModelService/SetDefaultModel',
          ($0.SetDefaultModelRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$testModel =
      $grpc.ClientMethod<$0.TestModelRequest, $0.TestModelResponse>(
          '/pocketagent.v1.ModelService/TestModel',
          ($0.TestModelRequest value) => value.writeToBuffer(),
          $0.TestModelResponse.fromBuffer);
  static final _$getCodexSettings =
      $grpc.ClientMethod<$0.GetCodexSettingsRequest, $0.CodexSettings>(
          '/pocketagent.v1.ModelService/GetCodexSettings',
          ($0.GetCodexSettingsRequest value) => value.writeToBuffer(),
          $0.CodexSettings.fromBuffer);
  static final _$setCodexSettings =
      $grpc.ClientMethod<$0.SetCodexSettingsRequest, $0.CodexSettings>(
          '/pocketagent.v1.ModelService/SetCodexSettings',
          ($0.SetCodexSettingsRequest value) => value.writeToBuffer(),
          $0.CodexSettings.fromBuffer);
  static final _$listMcpServers =
      $grpc.ClientMethod<$0.ListMcpServersRequest, $0.ListMcpServersResponse>(
          '/pocketagent.v1.ModelService/ListMcpServers',
          ($0.ListMcpServersRequest value) => value.writeToBuffer(),
          $0.ListMcpServersResponse.fromBuffer);
  static final _$getCodexUsage =
      $grpc.ClientMethod<$0.GetCodexUsageRequest, $0.CodexUsage>(
          '/pocketagent.v1.ModelService/GetCodexUsage',
          ($0.GetCodexUsageRequest value) => value.writeToBuffer(),
          $0.CodexUsage.fromBuffer);
  static final _$listCodexCapabilities = $grpc.ClientMethod<
          $0.ListCodexCapabilitiesRequest, $0.ListCodexCapabilitiesResponse>(
      '/pocketagent.v1.ModelService/ListCodexCapabilities',
      ($0.ListCodexCapabilitiesRequest value) => value.writeToBuffer(),
      $0.ListCodexCapabilitiesResponse.fromBuffer);
  static final _$listAgentConfigOptions = $grpc.ClientMethod<
          $0.ListAgentConfigOptionsRequest, $0.ListAgentConfigOptionsResponse>(
      '/pocketagent.v1.ModelService/ListAgentConfigOptions',
      ($0.ListAgentConfigOptionsRequest value) => value.writeToBuffer(),
      $0.ListAgentConfigOptionsResponse.fromBuffer);
}

@$pb.GrpcServiceName('pocketagent.v1.ModelService')
abstract class ModelServiceBase extends $grpc.Service {
  $core.String get $name => 'pocketagent.v1.ModelService';

  ModelServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ListModelsRequest, $0.ListModelsResponse>(
        'ListModels',
        listModels_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListModelsRequest.fromBuffer(value),
        ($0.ListModelsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetDefaultModelRequest, $1.Empty>(
        'SetDefaultModel',
        setDefaultModel_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetDefaultModelRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TestModelRequest, $0.TestModelResponse>(
        'TestModel',
        testModel_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TestModelRequest.fromBuffer(value),
        ($0.TestModelResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetCodexSettingsRequest, $0.CodexSettings>(
            'GetCodexSettings',
            getCodexSettings_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetCodexSettingsRequest.fromBuffer(value),
            ($0.CodexSettings value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SetCodexSettingsRequest, $0.CodexSettings>(
            'SetCodexSettings',
            setCodexSettings_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SetCodexSettingsRequest.fromBuffer(value),
            ($0.CodexSettings value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListMcpServersRequest,
            $0.ListMcpServersResponse>(
        'ListMcpServers',
        listMcpServers_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListMcpServersRequest.fromBuffer(value),
        ($0.ListMcpServersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetCodexUsageRequest, $0.CodexUsage>(
        'GetCodexUsage',
        getCodexUsage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetCodexUsageRequest.fromBuffer(value),
        ($0.CodexUsage value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListCodexCapabilitiesRequest,
            $0.ListCodexCapabilitiesResponse>(
        'ListCodexCapabilities',
        listCodexCapabilities_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListCodexCapabilitiesRequest.fromBuffer(value),
        ($0.ListCodexCapabilitiesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListAgentConfigOptionsRequest,
            $0.ListAgentConfigOptionsResponse>(
        'ListAgentConfigOptions',
        listAgentConfigOptions_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListAgentConfigOptionsRequest.fromBuffer(value),
        ($0.ListAgentConfigOptionsResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ListModelsResponse> listModels_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListModelsRequest> $request) async {
    return listModels($call, await $request);
  }

  $async.Future<$0.ListModelsResponse> listModels(
      $grpc.ServiceCall call, $0.ListModelsRequest request);

  $async.Future<$1.Empty> setDefaultModel_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SetDefaultModelRequest> $request) async {
    return setDefaultModel($call, await $request);
  }

  $async.Future<$1.Empty> setDefaultModel(
      $grpc.ServiceCall call, $0.SetDefaultModelRequest request);

  $async.Future<$0.TestModelResponse> testModel_Pre($grpc.ServiceCall $call,
      $async.Future<$0.TestModelRequest> $request) async {
    return testModel($call, await $request);
  }

  $async.Future<$0.TestModelResponse> testModel(
      $grpc.ServiceCall call, $0.TestModelRequest request);

  $async.Future<$0.CodexSettings> getCodexSettings_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetCodexSettingsRequest> $request) async {
    return getCodexSettings($call, await $request);
  }

  $async.Future<$0.CodexSettings> getCodexSettings(
      $grpc.ServiceCall call, $0.GetCodexSettingsRequest request);

  $async.Future<$0.CodexSettings> setCodexSettings_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SetCodexSettingsRequest> $request) async {
    return setCodexSettings($call, await $request);
  }

  $async.Future<$0.CodexSettings> setCodexSettings(
      $grpc.ServiceCall call, $0.SetCodexSettingsRequest request);

  $async.Future<$0.ListMcpServersResponse> listMcpServers_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListMcpServersRequest> $request) async {
    return listMcpServers($call, await $request);
  }

  $async.Future<$0.ListMcpServersResponse> listMcpServers(
      $grpc.ServiceCall call, $0.ListMcpServersRequest request);

  $async.Future<$0.CodexUsage> getCodexUsage_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetCodexUsageRequest> $request) async {
    return getCodexUsage($call, await $request);
  }

  $async.Future<$0.CodexUsage> getCodexUsage(
      $grpc.ServiceCall call, $0.GetCodexUsageRequest request);

  $async.Future<$0.ListCodexCapabilitiesResponse> listCodexCapabilities_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListCodexCapabilitiesRequest> $request) async {
    return listCodexCapabilities($call, await $request);
  }

  $async.Future<$0.ListCodexCapabilitiesResponse> listCodexCapabilities(
      $grpc.ServiceCall call, $0.ListCodexCapabilitiesRequest request);

  $async.Future<$0.ListAgentConfigOptionsResponse> listAgentConfigOptions_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListAgentConfigOptionsRequest> $request) async {
    return listAgentConfigOptions($call, await $request);
  }

  $async.Future<$0.ListAgentConfigOptionsResponse> listAgentConfigOptions(
      $grpc.ServiceCall call, $0.ListAgentConfigOptionsRequest request);
}
