// This is a generated file - do not edit.
//
// Generated from v1/runtime.proto.

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

import 'runtime.pb.dart' as $0;

export 'runtime.pb.dart';

/// ============ 本机运行时服务(规格 §13 P1/P2、§14)============
///
/// 【为什么预算策略要有一个 RPC 出口】在此之前,资源预算只能靠用户手编
/// <数据目录>/runtime-policy.json,改完还得重启 daemon —— 那等于"有配置、没有控制面"。
/// 手机端要能看、能改,才谈得上"策略"。
///
/// 【错误语义】与既有 6 个服务一致,一律走 gRPC status(codes.InvalidArgument /
/// FailedPrecondition / Internal ...),不在响应体里再放一份 Error —— 规格 §21.2
/// 的草案允许响应内 Error,但两套错误语义并存会逼客户端判两处,且本项目既有实现
/// 全部走 status。此处从既有实现,不从草案。
@$pb.GrpcServiceName('pocketagent.v1.RuntimeService')
class RuntimeServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  RuntimeServiceClient(super.channel, {super.options, super.interceptors});

  /// 读回当前生效的预算策略(revision + 落盘位置)。
  /// 【为什么 Set 之外还要 Get】手机换机/重装后没有本地副本;没有 Get,"能改"就等于
  /// "只能盲改" —— 用户看不到当前值,也就无从判断改没改对。
  $grpc.ResponseFuture<$0.RuntimePolicyResponse> getRuntimePolicy(
    $0.GetRuntimePolicyRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getRuntimePolicy, request, options: options);
  }

  /// 提交新策略:校验 → 原子落盘 → 立即生效(不需要重启 daemon)。
  $grpc.ResponseFuture<$0.RuntimePolicyResponse> setRuntimePolicy(
    $0.SetRuntimePolicyRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setRuntimePolicy, request, options: options);
  }

  /// 查某作用域下的资源用量与限额(每个租约的 soft/hard 与压力档)。
  $grpc.ResponseFuture<$0.ResourceUsageResponse> getResourceUsage(
    $0.GetResourceUsageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getResourceUsage, request, options: options);
  }

  /// 未知事件的诊断索引(规格 §9:未知事件要"可保存、可查询、可展示")。
  /// 【为什么这三条必须存在】原生 agent 升级后会发来我们没见过的通知与反向请求;
  /// 只把它们写进 daemon 日志,用户与手机端就永远看不到"最近到底来了什么" ——
  /// 而那正是判断"要不要等上游支持/要不要关掉某个实验特性"的唯一依据。
  $grpc.ResponseFuture<$0.ListUnknownEventsResponse> listUnknownEvents(
    $0.ListUnknownEventsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listUnknownEvents, request, options: options);
  }

  $grpc.ResponseFuture<$0.UnknownEventResponse> getUnknownEvent(
    $0.GetUnknownEventRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getUnknownEvent, request, options: options);
  }

  $grpc.ResponseFuture<$0.AcknowledgeUnknownEventResponse>
      acknowledgeUnknownEvent(
    $0.AcknowledgeUnknownEventRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$acknowledgeUnknownEvent, request,
        options: options);
  }

  /// 协同策略(规格 §11):协同/委派/扇出的开关与扇出成员上限。
  /// 【为什么放在 daemon】手机上的本地开关只是"这台设备当前的显示偏好",换台手机、
  /// 重装 App 就没了;而"允不允许 agent 互相派活"是这台电脑的运行策略,必须由
  /// daemon 持久化并作为唯一事实来源。
  $grpc.ResponseFuture<$0.CollabPolicyResponse> getCollabPolicy(
    $0.GetCollabPolicyRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getCollabPolicy, request, options: options);
  }

  $grpc.ResponseFuture<$0.CollabPolicyResponse> setCollabPolicy(
    $0.SetCollabPolicyRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setCollabPolicy, request, options: options);
  }

  /// 本机运行时总览 + 每个 agent 的**能力声明核验**结果(规格 §10)。
  /// 【为什么核验结果要经 RPC 出去】契约的价值在于"缺失可见":某家 agent 没申报
  /// 运行时形态、或申报了却没交代必需入口,这些以前只写进 daemon 日志 —— 用户在
  /// 手机上只会看到"这台 agent 好像少点什么",却指不出是哪一项。
  $grpc.ResponseFuture<$0.RuntimeStateResponse> getRuntimeState(
    $0.GetRuntimeStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getRuntimeState, request, options: options);
  }

  /// 单个 agent 的运行时形态与用量。
  $grpc.ResponseFuture<$0.AgentRuntimeResponse> getAgentRuntime(
    $0.GetAgentRuntimeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getAgentRuntime, request, options: options);
  }

  /// 按会话列出某 agent 名下的语义记忆(内存态 + 盘上 checkpoint 载荷合并视图)。
  /// 【为什么按会话而不是整家一团】"哪几个会话占了记忆"回答不了,管理就无从谈起:
  /// 用户需要的粒度是"把这个走歪了的会话的记忆清掉重新开始",不是"整家清空伤及无辜"。
  /// loaded=false 的行 = 重启后从未触碰、值来自盘上落盘那一刻的快照 —— 两个口径
  /// 分开报,客户端不得把"盘上快照"当成实时值展示。
  $grpc.ResponseFuture<$0.ListThreadMemoriesResponse> listThreadMemories(
    $0.ListThreadMemoriesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listThreadMemories, request, options: options);
  }

  /// 清掉一个会话名下全部 PocketAgent 侧记忆(内存态 + 盘上 checkpoint),返回剩余合计。
  /// 【红线】只动 PocketAgent 自己的数据目录;~/.codex、~/.pi 等原生侧的会话与用户
  /// 数据一个字节都不碰。会话不存在时回 NotFound —— "没清任何东西"必须让客户端
  /// 知道,不能谎报成功。
  $grpc.ResponseFuture<$0.ClearThreadMemoryResponse> clearThreadMemory(
    $0.ClearThreadMemoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$clearThreadMemory, request, options: options);
  }

  /// 控制面总览(聚合读):一个 RPC 回来 daemon 全部可控域的当前状态与能力,
  /// 手机一个页面看全、就地改。
  /// 【它只是呈现,不是新的写通道】这里不落任何新配置文件:切换动作仍走各自
  /// 既有的 SetRuntimePolicy / SetCollabPolicy / SetCodexSettings /
  /// AcknowledgeUnknownEvent / ClearThreadMemory —— 各控制通道早已存在,本 RPC
  /// 补的是"一盘棋的总览",重写任何一条通道都只会制造第二份真值。
  /// 【单域降级】某个域的状态取不到时,该域单独降级为 status_code=2 并说明原因,
  /// 其余域照常返回 —— 总览页不该被一个域的故障掀翻。
  $grpc.ResponseFuture<$0.ControlPlane> getControlPlane(
    $0.GetControlPlaneRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getControlPlane, request, options: options);
  }

  /// ---- 未知事件用户动作执行(2026-10-04 用户处理中心计划,只增)----
  /// 执行一个由 UnknownEventSummary.actions 声明过的安全动作。动作白名单在
  /// daemon 侧硬编码:inspect / acknowledge / retry_later / open_native;
  /// **绝不**提供 approve/execute 类动作 —— 未知请求永远不能被"一键放行"
  /// (与 UserActionDescriptor.safe 的红线同一纪律)。
  /// 【幂等】同一动作重复提交不产生副作用:返回当前状态(ok=true + 最新
  /// summary/new_revision),客户端重复点击、多设备同时处理都安全。
  /// 【冲突是结构化结果不是 error】expected_revision 与当前 revision 不一致时
  /// 回 conflict=true + 人话 detail,gRPC status 不承载业务冲突 —— 客户端据此
  /// 重新拉取详情(GetUnknownEvent)再决定,不炸 UI。
  $grpc.ResponseFuture<$0.UnknownEventActionResult> executeUnknownEventAction(
    $0.UnknownEventActionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$executeUnknownEventAction, request,
        options: options);
  }

  // method descriptors

  static final _$getRuntimePolicy =
      $grpc.ClientMethod<$0.GetRuntimePolicyRequest, $0.RuntimePolicyResponse>(
          '/pocketagent.v1.RuntimeService/GetRuntimePolicy',
          ($0.GetRuntimePolicyRequest value) => value.writeToBuffer(),
          $0.RuntimePolicyResponse.fromBuffer);
  static final _$setRuntimePolicy =
      $grpc.ClientMethod<$0.SetRuntimePolicyRequest, $0.RuntimePolicyResponse>(
          '/pocketagent.v1.RuntimeService/SetRuntimePolicy',
          ($0.SetRuntimePolicyRequest value) => value.writeToBuffer(),
          $0.RuntimePolicyResponse.fromBuffer);
  static final _$getResourceUsage =
      $grpc.ClientMethod<$0.GetResourceUsageRequest, $0.ResourceUsageResponse>(
          '/pocketagent.v1.RuntimeService/GetResourceUsage',
          ($0.GetResourceUsageRequest value) => value.writeToBuffer(),
          $0.ResourceUsageResponse.fromBuffer);
  static final _$listUnknownEvents = $grpc.ClientMethod<
          $0.ListUnknownEventsRequest, $0.ListUnknownEventsResponse>(
      '/pocketagent.v1.RuntimeService/ListUnknownEvents',
      ($0.ListUnknownEventsRequest value) => value.writeToBuffer(),
      $0.ListUnknownEventsResponse.fromBuffer);
  static final _$getUnknownEvent =
      $grpc.ClientMethod<$0.GetUnknownEventRequest, $0.UnknownEventResponse>(
          '/pocketagent.v1.RuntimeService/GetUnknownEvent',
          ($0.GetUnknownEventRequest value) => value.writeToBuffer(),
          $0.UnknownEventResponse.fromBuffer);
  static final _$acknowledgeUnknownEvent = $grpc.ClientMethod<
          $0.AcknowledgeUnknownEventRequest,
          $0.AcknowledgeUnknownEventResponse>(
      '/pocketagent.v1.RuntimeService/AcknowledgeUnknownEvent',
      ($0.AcknowledgeUnknownEventRequest value) => value.writeToBuffer(),
      $0.AcknowledgeUnknownEventResponse.fromBuffer);
  static final _$getCollabPolicy =
      $grpc.ClientMethod<$0.GetCollabPolicyRequest, $0.CollabPolicyResponse>(
          '/pocketagent.v1.RuntimeService/GetCollabPolicy',
          ($0.GetCollabPolicyRequest value) => value.writeToBuffer(),
          $0.CollabPolicyResponse.fromBuffer);
  static final _$setCollabPolicy =
      $grpc.ClientMethod<$0.SetCollabPolicyRequest, $0.CollabPolicyResponse>(
          '/pocketagent.v1.RuntimeService/SetCollabPolicy',
          ($0.SetCollabPolicyRequest value) => value.writeToBuffer(),
          $0.CollabPolicyResponse.fromBuffer);
  static final _$getRuntimeState =
      $grpc.ClientMethod<$0.GetRuntimeStateRequest, $0.RuntimeStateResponse>(
          '/pocketagent.v1.RuntimeService/GetRuntimeState',
          ($0.GetRuntimeStateRequest value) => value.writeToBuffer(),
          $0.RuntimeStateResponse.fromBuffer);
  static final _$getAgentRuntime =
      $grpc.ClientMethod<$0.GetAgentRuntimeRequest, $0.AgentRuntimeResponse>(
          '/pocketagent.v1.RuntimeService/GetAgentRuntime',
          ($0.GetAgentRuntimeRequest value) => value.writeToBuffer(),
          $0.AgentRuntimeResponse.fromBuffer);
  static final _$listThreadMemories = $grpc.ClientMethod<
          $0.ListThreadMemoriesRequest, $0.ListThreadMemoriesResponse>(
      '/pocketagent.v1.RuntimeService/ListThreadMemories',
      ($0.ListThreadMemoriesRequest value) => value.writeToBuffer(),
      $0.ListThreadMemoriesResponse.fromBuffer);
  static final _$clearThreadMemory = $grpc.ClientMethod<
          $0.ClearThreadMemoryRequest, $0.ClearThreadMemoryResponse>(
      '/pocketagent.v1.RuntimeService/ClearThreadMemory',
      ($0.ClearThreadMemoryRequest value) => value.writeToBuffer(),
      $0.ClearThreadMemoryResponse.fromBuffer);
  static final _$getControlPlane =
      $grpc.ClientMethod<$0.GetControlPlaneRequest, $0.ControlPlane>(
          '/pocketagent.v1.RuntimeService/GetControlPlane',
          ($0.GetControlPlaneRequest value) => value.writeToBuffer(),
          $0.ControlPlane.fromBuffer);
  static final _$executeUnknownEventAction = $grpc.ClientMethod<
          $0.UnknownEventActionRequest, $0.UnknownEventActionResult>(
      '/pocketagent.v1.RuntimeService/ExecuteUnknownEventAction',
      ($0.UnknownEventActionRequest value) => value.writeToBuffer(),
      $0.UnknownEventActionResult.fromBuffer);
}

@$pb.GrpcServiceName('pocketagent.v1.RuntimeService')
abstract class RuntimeServiceBase extends $grpc.Service {
  $core.String get $name => 'pocketagent.v1.RuntimeService';

  RuntimeServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GetRuntimePolicyRequest,
            $0.RuntimePolicyResponse>(
        'GetRuntimePolicy',
        getRuntimePolicy_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetRuntimePolicyRequest.fromBuffer(value),
        ($0.RuntimePolicyResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetRuntimePolicyRequest,
            $0.RuntimePolicyResponse>(
        'SetRuntimePolicy',
        setRuntimePolicy_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetRuntimePolicyRequest.fromBuffer(value),
        ($0.RuntimePolicyResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetResourceUsageRequest,
            $0.ResourceUsageResponse>(
        'GetResourceUsage',
        getResourceUsage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetResourceUsageRequest.fromBuffer(value),
        ($0.ResourceUsageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListUnknownEventsRequest,
            $0.ListUnknownEventsResponse>(
        'ListUnknownEvents',
        listUnknownEvents_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListUnknownEventsRequest.fromBuffer(value),
        ($0.ListUnknownEventsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetUnknownEventRequest, $0.UnknownEventResponse>(
            'GetUnknownEvent',
            getUnknownEvent_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetUnknownEventRequest.fromBuffer(value),
            ($0.UnknownEventResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AcknowledgeUnknownEventRequest,
            $0.AcknowledgeUnknownEventResponse>(
        'AcknowledgeUnknownEvent',
        acknowledgeUnknownEvent_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AcknowledgeUnknownEventRequest.fromBuffer(value),
        ($0.AcknowledgeUnknownEventResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetCollabPolicyRequest, $0.CollabPolicyResponse>(
            'GetCollabPolicy',
            getCollabPolicy_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetCollabPolicyRequest.fromBuffer(value),
            ($0.CollabPolicyResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SetCollabPolicyRequest, $0.CollabPolicyResponse>(
            'SetCollabPolicy',
            setCollabPolicy_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SetCollabPolicyRequest.fromBuffer(value),
            ($0.CollabPolicyResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetRuntimeStateRequest, $0.RuntimeStateResponse>(
            'GetRuntimeState',
            getRuntimeState_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetRuntimeStateRequest.fromBuffer(value),
            ($0.RuntimeStateResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetAgentRuntimeRequest, $0.AgentRuntimeResponse>(
            'GetAgentRuntime',
            getAgentRuntime_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetAgentRuntimeRequest.fromBuffer(value),
            ($0.AgentRuntimeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListThreadMemoriesRequest,
            $0.ListThreadMemoriesResponse>(
        'ListThreadMemories',
        listThreadMemories_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListThreadMemoriesRequest.fromBuffer(value),
        ($0.ListThreadMemoriesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ClearThreadMemoryRequest,
            $0.ClearThreadMemoryResponse>(
        'ClearThreadMemory',
        clearThreadMemory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ClearThreadMemoryRequest.fromBuffer(value),
        ($0.ClearThreadMemoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetControlPlaneRequest, $0.ControlPlane>(
        'GetControlPlane',
        getControlPlane_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetControlPlaneRequest.fromBuffer(value),
        ($0.ControlPlane value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UnknownEventActionRequest,
            $0.UnknownEventActionResult>(
        'ExecuteUnknownEventAction',
        executeUnknownEventAction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UnknownEventActionRequest.fromBuffer(value),
        ($0.UnknownEventActionResult value) => value.writeToBuffer()));
  }

  $async.Future<$0.RuntimePolicyResponse> getRuntimePolicy_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetRuntimePolicyRequest> $request) async {
    return getRuntimePolicy($call, await $request);
  }

  $async.Future<$0.RuntimePolicyResponse> getRuntimePolicy(
      $grpc.ServiceCall call, $0.GetRuntimePolicyRequest request);

  $async.Future<$0.RuntimePolicyResponse> setRuntimePolicy_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetRuntimePolicyRequest> $request) async {
    return setRuntimePolicy($call, await $request);
  }

  $async.Future<$0.RuntimePolicyResponse> setRuntimePolicy(
      $grpc.ServiceCall call, $0.SetRuntimePolicyRequest request);

  $async.Future<$0.ResourceUsageResponse> getResourceUsage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetResourceUsageRequest> $request) async {
    return getResourceUsage($call, await $request);
  }

  $async.Future<$0.ResourceUsageResponse> getResourceUsage(
      $grpc.ServiceCall call, $0.GetResourceUsageRequest request);

  $async.Future<$0.ListUnknownEventsResponse> listUnknownEvents_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListUnknownEventsRequest> $request) async {
    return listUnknownEvents($call, await $request);
  }

  $async.Future<$0.ListUnknownEventsResponse> listUnknownEvents(
      $grpc.ServiceCall call, $0.ListUnknownEventsRequest request);

  $async.Future<$0.UnknownEventResponse> getUnknownEvent_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetUnknownEventRequest> $request) async {
    return getUnknownEvent($call, await $request);
  }

  $async.Future<$0.UnknownEventResponse> getUnknownEvent(
      $grpc.ServiceCall call, $0.GetUnknownEventRequest request);

  $async.Future<$0.AcknowledgeUnknownEventResponse> acknowledgeUnknownEvent_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AcknowledgeUnknownEventRequest> $request) async {
    return acknowledgeUnknownEvent($call, await $request);
  }

  $async.Future<$0.AcknowledgeUnknownEventResponse> acknowledgeUnknownEvent(
      $grpc.ServiceCall call, $0.AcknowledgeUnknownEventRequest request);

  $async.Future<$0.CollabPolicyResponse> getCollabPolicy_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetCollabPolicyRequest> $request) async {
    return getCollabPolicy($call, await $request);
  }

  $async.Future<$0.CollabPolicyResponse> getCollabPolicy(
      $grpc.ServiceCall call, $0.GetCollabPolicyRequest request);

  $async.Future<$0.CollabPolicyResponse> setCollabPolicy_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetCollabPolicyRequest> $request) async {
    return setCollabPolicy($call, await $request);
  }

  $async.Future<$0.CollabPolicyResponse> setCollabPolicy(
      $grpc.ServiceCall call, $0.SetCollabPolicyRequest request);

  $async.Future<$0.RuntimeStateResponse> getRuntimeState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetRuntimeStateRequest> $request) async {
    return getRuntimeState($call, await $request);
  }

  $async.Future<$0.RuntimeStateResponse> getRuntimeState(
      $grpc.ServiceCall call, $0.GetRuntimeStateRequest request);

  $async.Future<$0.AgentRuntimeResponse> getAgentRuntime_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetAgentRuntimeRequest> $request) async {
    return getAgentRuntime($call, await $request);
  }

  $async.Future<$0.AgentRuntimeResponse> getAgentRuntime(
      $grpc.ServiceCall call, $0.GetAgentRuntimeRequest request);

  $async.Future<$0.ListThreadMemoriesResponse> listThreadMemories_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListThreadMemoriesRequest> $request) async {
    return listThreadMemories($call, await $request);
  }

  $async.Future<$0.ListThreadMemoriesResponse> listThreadMemories(
      $grpc.ServiceCall call, $0.ListThreadMemoriesRequest request);

  $async.Future<$0.ClearThreadMemoryResponse> clearThreadMemory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ClearThreadMemoryRequest> $request) async {
    return clearThreadMemory($call, await $request);
  }

  $async.Future<$0.ClearThreadMemoryResponse> clearThreadMemory(
      $grpc.ServiceCall call, $0.ClearThreadMemoryRequest request);

  $async.Future<$0.ControlPlane> getControlPlane_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetControlPlaneRequest> $request) async {
    return getControlPlane($call, await $request);
  }

  $async.Future<$0.ControlPlane> getControlPlane(
      $grpc.ServiceCall call, $0.GetControlPlaneRequest request);

  $async.Future<$0.UnknownEventActionResult> executeUnknownEventAction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UnknownEventActionRequest> $request) async {
    return executeUnknownEventAction($call, await $request);
  }

  $async.Future<$0.UnknownEventActionResult> executeUnknownEventAction(
      $grpc.ServiceCall call, $0.UnknownEventActionRequest request);
}
