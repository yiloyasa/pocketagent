// This is a generated file - do not edit.
//
// Generated from v1/thread.proto.

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
import 'thread.pb.dart' as $0;

export 'thread.pb.dart';

@$pb.GrpcServiceName('pocketagent.v1.ThreadService')
class ThreadServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  ThreadServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.Thread> createThread(
    $0.CreateThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createThread, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListThreadsResponse> listThreads(
    $0.ListThreadsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listThreads, request, options: options);
  }

  $grpc.ResponseFuture<$0.Thread> getThread(
    $0.GetThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getThread, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> deleteThread(
    $0.DeleteThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteThread, request, options: options);
  }

  $grpc.ResponseFuture<$0.Thread> archiveThread(
    $0.ArchiveThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$archiveThread, request, options: options);
  }

  $grpc.ResponseFuture<$0.Thread> renameThread(
    $0.RenameThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$renameThread, request, options: options);
  }

  $grpc.ResponseFuture<$0.Thread> setThreadGoal(
    $0.SetThreadGoalRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setThreadGoal, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> compactThread(
    $0.CompactThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$compactThread, request, options: options);
  }

  /// 核心:执行一轮 agent。客户端 → 服务端:用户输入/中途补充/审批应答;
  /// 服务端 → 客户端:文本增量/工具调用/审批请求/turn 结束。
  $grpc.ResponseStream<$0.TurnServerMessage> startTurn(
    $async.Stream<$0.TurnClientMessage> request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(_$startTurn, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> cancelTurn(
    $0.CancelTurnRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$cancelTurn, request, options: options);
  }

  /// 撤回该会话在上游排队中的全部输入(0.87.1 pi clear_queue 接线,只增)。
  /// 等价 pi TUI 里按 Esc:清空的是 pi 子进程内存里的原生队列(steer/followUp),
  /// 不是 daemon 侧的续跑队列(那归 CancelTurn/轮次收尾管)。仅 pi 会话支持。
  $grpc.ResponseFuture<$0.ClearQueuedInputsResponse> clearQueuedInputs(
    $0.ClearQueuedInputsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$clearQueuedInputs, request, options: options);
  }

  /// Reconnect-safe approval delivery; request_id must still be pending.
  $grpc.ResponseFuture<$1.Empty> submitApproval(
    $0.SubmitApprovalRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$submitApproval, request, options: options);
  }

  /// 历史消息(打开旧会话时拉取)
  $grpc.ResponseFuture<$0.GetThreadHistoryResponse> getThreadHistory(
    $0.GetThreadHistoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getThreadHistory, request, options: options);
  }

  /// ---- 会话内消息内容搜索(只增)----
  /// 在一个会话的历史消息正文里搜一段文字(大小写不敏感),命中项带前后文
  /// 摘要(snippet)。手机端历史导航浮层目前只能按标题/摘要过滤本地已加载的
  /// 列表;daemon 内存 store 里有**全量**消息(含未随 limit 下发的更早内容),
  /// 这个 RPC 让"搜历史聊过什么"不需要把整条会话拖到手机上再翻。
  /// 鉴权走既有 UnaryAuth 通道(与 GetThreadHistory 同级),无新增配置。
  $grpc.ResponseFuture<$0.SearchThreadMessagesResponse> searchThreadMessages(
    $0.SearchThreadMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchThreadMessages, request, options: options);
  }

  /// ---- P1-5 事件断点续传(只增)----
  /// 按 seq 增量回放某线程的事件帧(seq > after_sequence,升序),回放完即关流,
  /// 不做 live 跟随。手机断线重连后用它把错过的帧无缝补上,替代"整屏历史回填"。
  /// 若 after_sequence 已超出 daemon 的保留窗口(events.jsonl 轮转丢弃),
  /// 流里只会有一帧 ResyncRequired —— 手机端收到后必须退回 GetThreadHistory 全量回填。
  $grpc.ResponseStream<$0.TurnServerMessage> getThreadEventsSince(
    $0.GetThreadEventsSinceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$getThreadEventsSince, $async.Stream.fromIterable([request]),
        options: options);
  }

  /// 电脑上 codex 已有的会话:列出 + 导入(绑定为本地线程,可继续聊)
  $grpc.ResponseFuture<$0.ListCodexThreadsResponse> listCodexThreads(
    $0.ListCodexThreadsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listCodexThreads, request, options: options);
  }

  $grpc.ResponseFuture<$0.ImportCodexThreadResponse> importCodexThread(
    $0.ImportCodexThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$importCodexThread, request, options: options);
  }

  /// 电脑上其它桌面 agent(pi / zcode)已有的会话:列出 + 导入(绑定为本地线程,可继续聊)。
  /// 【为什么是一对通用 RPC 而不是两对】pi / zcode 的差异只在"列表来源"与
  /// "外部会话引用怎么填",协议面完全同构 —— agent_id 一个参数就能区分,
  /// 开两套 RPC 只会让手机端多写一遍一模一样的接线。codex 不并入:它有
  /// app-server 标题同步 / rollout 删除等专属逻辑,保持原样不动。
  $grpc.ResponseFuture<$0.ListAgentThreadsResponse> listAgentThreads(
    $0.ListAgentThreadsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listAgentThreads, request, options: options);
  }

  $grpc.ResponseFuture<$0.ImportAgentThreadResponse> importAgentThread(
    $0.ImportAgentThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$importAgentThread, request, options: options);
  }

  /// ---- 会话组织:分组 / 分支 / 回滚 ----
  /// 【为什么 fork 和 rollback 必须二次确认】两者都会改变"这个会话之后是什么" ——
  /// fork 会分出一条新会话,rollback 会直接丢掉最近 N 个回合。手机上误触的代价比
  /// 电脑上高得多(屏幕小、手指滑一下就点到),所以 App 侧必须弹确认再调。
  $grpc.ResponseFuture<$0.Thread> forkThread(
    $0.ForkThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$forkThread, request, options: options);
  }

  $grpc.ResponseFuture<$0.Thread> rollbackThread(
    $0.RollbackThreadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$rollbackThread, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListThreadSectionsResponse> listThreadSections(
    $0.ListThreadSectionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listThreadSections, request, options: options);
  }

  $grpc.ResponseFuture<$0.ThreadSection> createThreadSection(
    $0.CreateThreadSectionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createThreadSection, request, options: options);
  }

  $grpc.ResponseFuture<$0.ThreadSection> renameThreadSection(
    $0.RenameThreadSectionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$renameThreadSection, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> deleteThreadSection(
    $0.DeleteThreadSectionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteThreadSection, request, options: options);
  }

  $grpc.ResponseFuture<$0.Thread> moveThreadToSection(
    $0.MoveThreadToSectionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$moveThreadToSection, request, options: options);
  }

  /// 审查者(guardian/自动审查)拒绝之后,用户推翻它的结论并要求继续执行。
  /// 手机端不提供这个按钮的话,用户看到的就是"任务莫名其妙失败了"。
  $grpc.ResponseFuture<$1.Empty> approveGuardianDeniedAction(
    $0.ApproveGuardianDeniedActionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$approveGuardianDeniedAction, request,
        options: options);
  }

  /// ---- 会话导出:离线 HTML 存档(只增)----
  /// 把整棵会话渲染成离线 HTML 落盘在电脑上(目前由 pi 侧 export_html 能力提供,
  /// 文件固定写在会话工作区的 .pocketagent-exports/<threadId>.html)。返回电脑上的
  /// 绝对路径,手机端用现有 ReadFile(FileChunk 流)下载链路取文件、浏览器离线打开。
  /// 与"导出对话(Markdown)"互补:Markdown 只导出手机选中的正文,HTML 是电脑端
  /// 的完整会话存档(含思考/工具过程)。非 pi 会话 / 未落地 / 导出失败一律走
  /// gRPC 错误(人话),手机端 Toast 降级,不炸 UI。
  $grpc.ResponseFuture<$0.ExportThreadHtmlResponse> exportThreadHtml(
    $0.ExportThreadHtmlRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$exportThreadHtml, request, options: options);
  }

  /// ---- 对话栏提示词优化(只增)----
  /// 手机输入栏的一键改写:把用户的原始输入交给电脑端 ZCode 的文本生成能力
  /// (app-server 的 workspace/generateText,复用桌面端已登录凭据,零新增配置)
  /// 改写成"给 AI 编程代理的清晰指令"。纯文本进、纯文本出:不建会话、不落库、
  /// 不影响任何线程状态;失败走 gRPC 错误(人话),手机端 Toast 降级,不炸 UI。
  $grpc.ResponseFuture<$0.OptimizePromptResponse> optimizePrompt(
    $0.OptimizePromptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$optimizePrompt, request, options: options);
  }

  /// ---- zcode 会话的斜杠命令面板(只增)----
  /// 手机输入栏以 "/" 开头且会话是 zcode 时,输入框上方弹命令候选条;候选数据
  /// 来自 daemon 侧该线程归属 zcode 会话的快照(slashCommands[],来源 ZCode
  /// app-server 的 session/create|resume 应答)。非 zcode 会话 / 拉取失败一律回
  /// **空列表**(这不是错误:手机端按"没有命令面板"处理,绝不 Toast 报错)。
  $grpc.ResponseFuture<$0.ListSlashCommandsResponse> listSlashCommands(
    $0.ListSlashCommandsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSlashCommands, request, options: options);
  }

  /// ---- /计划:发起计划审查(只增)----
  /// 手机端斜杠面板的内置伪命令"/计划"的落点:让电脑端 codex 对当前会话拟定的
  /// 计划发起一轮审查(codex review/start,实测 fire-and-return —— 应答里
  /// turn.status=inProgress 即回,审查产出以 item/* 事件流回**原会话**,手机端
  /// 现有的事件时间线直接可见)。仅 codex 会话支持;其他 agent / 未落地会话
  /// 回 gRPC 错误(人话),手机端 Toast 降级,不炸 UI。
  $grpc.ResponseFuture<$0.StartReviewResponse> startReview(
    $0.StartReviewRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startReview, request, options: options);
  }

  /// ---- 跨 agent 委派:T-L2fix 单写者重构(只增)----
  /// -collab-mcp 子模式(本机 MCP 代理进程,经 pi-mcp-adapter 等拉起)的薄代理
  /// 落点:代理进程收到 agent_delegate 工具调用后,经本机回环 gRPC
  /// (127.0.0.1:7246,TLS + Bearer token)调本 RPC,把委派汇入主 daemon 的
  /// 单一委派引擎实例(collab_delegate.go)。
  /// 【为什么收归主 daemon 单实例】子模式不再自持 FileStore/providers/引擎:
  ///   1. 消灭写冲突 —— 子代理进程不再触碰 threads.json/events.jsonl,主
  ///      daemon 是唯一写者(子线程、事件 seq 不再被双进程互相抹掉/污染);
  ///   2. 深度锁跨进程成立 —— 所有代理进程的委派汇入同一引擎,childThreads
  ///      登记簿与在飞深度计数成为全局真值,"子不生子"不再被跨进程击穿
  ///      (子 pi 再拉起的代理进程同样汇入)。
  /// 鉴权:走既有鉴权拦截器(UnaryAuth → authorized),已配对设备的
  /// Bearer token(含 legacy 单 token)才可委派。
  /// 错误语义:业务失败(agent 不存在/深度拒绝/轮次失败等)走应答的 error
  /// 字段 + 人话,gRPC status 不承载业务失败(与既有风格对齐);gRPC status
  /// 仅留给鉴权/传输层错误。
  $grpc.ResponseFuture<$0.DelegateSubTaskResponse> delegateSubTask(
    $0.DelegateSubTaskRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$delegateSubTask, request, options: options);
  }

  /// ---- 协同角色(分工编排):可复用的"子代理岗位"(只增)----
  /// 为什么需要这一层:
  /// 委派原本只能在**每次调用**时传 model/effort/task —— 父模型要让某个子代理
  /// 反复扮演同一角色(例如"审查者":只找安全问题、不要改代码、用便宜模型),
  /// 就得把整段人设在每个 task 里复述一遍,改口径更是要改 N 处。角色表把这部分
  /// 抽出来:一次配置,后续所有委派按 id 引用即生效。
  /// 边界(刻意不做):daemon **不自动编排分工** —— 谁在什么时候派给谁,由父
  /// 模型(经 MCP 工具入参)或手机端决定。让 daemon 替用户猜分工,既不可预测
  /// 也不可控,那是假智能。
  $grpc.ResponseFuture<$0.ListCollabRolesResponse> listCollabRoles(
    $0.ListCollabRolesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listCollabRoles, request, options: options);
  }

  /// 按 role.id 幂等 upsert;id 为空 = 新建(daemon 生成),同名/同 id 覆盖。
  $grpc.ResponseFuture<$0.SetCollabRoleResponse> setCollabRole(
    $0.SetCollabRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setCollabRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteCollabRoleResponse> deleteCollabRole(
    $0.DeleteCollabRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteCollabRole, request, options: options);
  }

  /// ---- 协同扇出:一条指令交给**多个 agent 并行执行**(只增)----
  /// 语义与委派(DelegateSubTask)根本不同:委派是**父子**(父 agent 自己决定
  /// 派活给谁);本 RPC 是**并行协作** —— 用户在协同页选好若干 agent、发一条
  /// 指令,daemon 给每个 agent 各开一个子线程同时跑,它们之间对等、无父子。
  /// 【为什么不开"谁先谁后 / 谁汇总"的编排】那是让 daemon 替用户猜分工,既不
  /// 可预测也不可控;协同页的选人 + 角色配置已经把决策权交回用户手里。
  $grpc.ResponseFuture<$0.DispatchCollabResponse> dispatchCollab(
    $0.DispatchCollabRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$dispatchCollab, request, options: options);
  }

  /// ---- 协同会话闭环:快照 / 观察 / 取消 / 重试(只增,2026-10-04 协同闭环计划)----
  /// 【为什么从"派发即完"升级成会话聚合】DispatchCollab 只回答"受理了没有",
  /// 不回答"后来怎么样了" —— 手机端离开协同页再回来,看到的是一组散落的子线程
  /// 状态,凑不出"2/3 已完成 · 1 个等待确认"。引入持久化 CollabSession 后,daemon
  /// 是协同状态的唯一事实来源:快照(Get/List)负责恢复,事件流(Watch)负责跟随,
  /// 取消/重试负责干预。全部 additive:旧客户端继续只用 DispatchCollab 不受影响。
  /// 【为什么状态是 string 不是 enum】会话/成员状态(ACCEPTED/RUNNING/COMPLETED/
  /// FAILED/CANCELLED/WAITING_APPROVAL)由 daemon 状态机定义,协议先按字符串透传;
  /// 状态集稳定后再冻结枚举,避免半成品枚举把客户端锁死在错误词表上。
  /// 单个协同会话的完整快照(会话级状态 + 全体成员卡片)。
  $grpc.ResponseFuture<$0.CollabSessionSnapshot> getCollabSession(
    $0.GetCollabSessionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getCollabSession, request, options: options);
  }

  /// 列出协同会话(最简全量形态;筛选交给手机端,daemon 不猜口径)。
  $grpc.ResponseFuture<$0.ListCollabSessionsResponse> listCollabSessions(
    $0.ListCollabSessionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listCollabSessions, request, options: options);
  }

  /// 订阅单个协同会话的事件流:先回放 seq > after_sequence 的既有事件,再跟随
  /// 实时事件,直至会话终局或客户端取消。断线重连后带 lastSequence 续传;
  /// 超出保留窗口时由服务端改发一帧全量 snapshot 兜底。
  $grpc.ResponseStream<$0.CollabSessionEvent> watchCollabSession(
    $0.WatchCollabSessionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$watchCollabSession, $async.Stream.fromIterable([request]),
        options: options);
  }

  /// 取消整个协同会话:逐成员发起取消并等待状态收敛,返回收敛后的会话快照。
  $grpc.ResponseFuture<$0.CollabSessionSnapshot> cancelCollabSession(
    $0.CancelCollabSessionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$cancelCollabSession, request, options: options);
  }

  /// 只取消一个成员(对应子线程),不动其它成员。
  $grpc.ResponseFuture<$0.CollabMemberSnapshot> cancelCollabMember(
    $0.CancelCollabMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$cancelCollabMember, request, options: options);
  }

  /// 重试一个成员:创建新的成员轮次(不覆盖旧结果),返回新成员快照。
  $grpc.ResponseFuture<$0.CollabMemberSnapshot> retryCollabMember(
    $0.RetryCollabMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$retryCollabMember, request, options: options);
  }

  // method descriptors

  static final _$createThread =
      $grpc.ClientMethod<$0.CreateThreadRequest, $0.Thread>(
          '/pocketagent.v1.ThreadService/CreateThread',
          ($0.CreateThreadRequest value) => value.writeToBuffer(),
          $0.Thread.fromBuffer);
  static final _$listThreads =
      $grpc.ClientMethod<$0.ListThreadsRequest, $0.ListThreadsResponse>(
          '/pocketagent.v1.ThreadService/ListThreads',
          ($0.ListThreadsRequest value) => value.writeToBuffer(),
          $0.ListThreadsResponse.fromBuffer);
  static final _$getThread = $grpc.ClientMethod<$0.GetThreadRequest, $0.Thread>(
      '/pocketagent.v1.ThreadService/GetThread',
      ($0.GetThreadRequest value) => value.writeToBuffer(),
      $0.Thread.fromBuffer);
  static final _$deleteThread =
      $grpc.ClientMethod<$0.DeleteThreadRequest, $1.Empty>(
          '/pocketagent.v1.ThreadService/DeleteThread',
          ($0.DeleteThreadRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$archiveThread =
      $grpc.ClientMethod<$0.ArchiveThreadRequest, $0.Thread>(
          '/pocketagent.v1.ThreadService/ArchiveThread',
          ($0.ArchiveThreadRequest value) => value.writeToBuffer(),
          $0.Thread.fromBuffer);
  static final _$renameThread =
      $grpc.ClientMethod<$0.RenameThreadRequest, $0.Thread>(
          '/pocketagent.v1.ThreadService/RenameThread',
          ($0.RenameThreadRequest value) => value.writeToBuffer(),
          $0.Thread.fromBuffer);
  static final _$setThreadGoal =
      $grpc.ClientMethod<$0.SetThreadGoalRequest, $0.Thread>(
          '/pocketagent.v1.ThreadService/SetThreadGoal',
          ($0.SetThreadGoalRequest value) => value.writeToBuffer(),
          $0.Thread.fromBuffer);
  static final _$compactThread =
      $grpc.ClientMethod<$0.CompactThreadRequest, $1.Empty>(
          '/pocketagent.v1.ThreadService/CompactThread',
          ($0.CompactThreadRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$startTurn =
      $grpc.ClientMethod<$0.TurnClientMessage, $0.TurnServerMessage>(
          '/pocketagent.v1.ThreadService/StartTurn',
          ($0.TurnClientMessage value) => value.writeToBuffer(),
          $0.TurnServerMessage.fromBuffer);
  static final _$cancelTurn =
      $grpc.ClientMethod<$0.CancelTurnRequest, $1.Empty>(
          '/pocketagent.v1.ThreadService/CancelTurn',
          ($0.CancelTurnRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$clearQueuedInputs = $grpc.ClientMethod<
          $0.ClearQueuedInputsRequest, $0.ClearQueuedInputsResponse>(
      '/pocketagent.v1.ThreadService/ClearQueuedInputs',
      ($0.ClearQueuedInputsRequest value) => value.writeToBuffer(),
      $0.ClearQueuedInputsResponse.fromBuffer);
  static final _$submitApproval =
      $grpc.ClientMethod<$0.SubmitApprovalRequest, $1.Empty>(
          '/pocketagent.v1.ThreadService/SubmitApproval',
          ($0.SubmitApprovalRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$getThreadHistory = $grpc.ClientMethod<
          $0.GetThreadHistoryRequest, $0.GetThreadHistoryResponse>(
      '/pocketagent.v1.ThreadService/GetThreadHistory',
      ($0.GetThreadHistoryRequest value) => value.writeToBuffer(),
      $0.GetThreadHistoryResponse.fromBuffer);
  static final _$searchThreadMessages = $grpc.ClientMethod<
          $0.SearchThreadMessagesRequest, $0.SearchThreadMessagesResponse>(
      '/pocketagent.v1.ThreadService/SearchThreadMessages',
      ($0.SearchThreadMessagesRequest value) => value.writeToBuffer(),
      $0.SearchThreadMessagesResponse.fromBuffer);
  static final _$getThreadEventsSince =
      $grpc.ClientMethod<$0.GetThreadEventsSinceRequest, $0.TurnServerMessage>(
          '/pocketagent.v1.ThreadService/GetThreadEventsSince',
          ($0.GetThreadEventsSinceRequest value) => value.writeToBuffer(),
          $0.TurnServerMessage.fromBuffer);
  static final _$listCodexThreads = $grpc.ClientMethod<
          $0.ListCodexThreadsRequest, $0.ListCodexThreadsResponse>(
      '/pocketagent.v1.ThreadService/ListCodexThreads',
      ($0.ListCodexThreadsRequest value) => value.writeToBuffer(),
      $0.ListCodexThreadsResponse.fromBuffer);
  static final _$importCodexThread = $grpc.ClientMethod<
          $0.ImportCodexThreadRequest, $0.ImportCodexThreadResponse>(
      '/pocketagent.v1.ThreadService/ImportCodexThread',
      ($0.ImportCodexThreadRequest value) => value.writeToBuffer(),
      $0.ImportCodexThreadResponse.fromBuffer);
  static final _$listAgentThreads = $grpc.ClientMethod<
          $0.ListAgentThreadsRequest, $0.ListAgentThreadsResponse>(
      '/pocketagent.v1.ThreadService/ListAgentThreads',
      ($0.ListAgentThreadsRequest value) => value.writeToBuffer(),
      $0.ListAgentThreadsResponse.fromBuffer);
  static final _$importAgentThread = $grpc.ClientMethod<
          $0.ImportAgentThreadRequest, $0.ImportAgentThreadResponse>(
      '/pocketagent.v1.ThreadService/ImportAgentThread',
      ($0.ImportAgentThreadRequest value) => value.writeToBuffer(),
      $0.ImportAgentThreadResponse.fromBuffer);
  static final _$forkThread =
      $grpc.ClientMethod<$0.ForkThreadRequest, $0.Thread>(
          '/pocketagent.v1.ThreadService/ForkThread',
          ($0.ForkThreadRequest value) => value.writeToBuffer(),
          $0.Thread.fromBuffer);
  static final _$rollbackThread =
      $grpc.ClientMethod<$0.RollbackThreadRequest, $0.Thread>(
          '/pocketagent.v1.ThreadService/RollbackThread',
          ($0.RollbackThreadRequest value) => value.writeToBuffer(),
          $0.Thread.fromBuffer);
  static final _$listThreadSections = $grpc.ClientMethod<
          $0.ListThreadSectionsRequest, $0.ListThreadSectionsResponse>(
      '/pocketagent.v1.ThreadService/ListThreadSections',
      ($0.ListThreadSectionsRequest value) => value.writeToBuffer(),
      $0.ListThreadSectionsResponse.fromBuffer);
  static final _$createThreadSection =
      $grpc.ClientMethod<$0.CreateThreadSectionRequest, $0.ThreadSection>(
          '/pocketagent.v1.ThreadService/CreateThreadSection',
          ($0.CreateThreadSectionRequest value) => value.writeToBuffer(),
          $0.ThreadSection.fromBuffer);
  static final _$renameThreadSection =
      $grpc.ClientMethod<$0.RenameThreadSectionRequest, $0.ThreadSection>(
          '/pocketagent.v1.ThreadService/RenameThreadSection',
          ($0.RenameThreadSectionRequest value) => value.writeToBuffer(),
          $0.ThreadSection.fromBuffer);
  static final _$deleteThreadSection =
      $grpc.ClientMethod<$0.DeleteThreadSectionRequest, $1.Empty>(
          '/pocketagent.v1.ThreadService/DeleteThreadSection',
          ($0.DeleteThreadSectionRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$moveThreadToSection =
      $grpc.ClientMethod<$0.MoveThreadToSectionRequest, $0.Thread>(
          '/pocketagent.v1.ThreadService/MoveThreadToSection',
          ($0.MoveThreadToSectionRequest value) => value.writeToBuffer(),
          $0.Thread.fromBuffer);
  static final _$approveGuardianDeniedAction =
      $grpc.ClientMethod<$0.ApproveGuardianDeniedActionRequest, $1.Empty>(
          '/pocketagent.v1.ThreadService/ApproveGuardianDeniedAction',
          ($0.ApproveGuardianDeniedActionRequest value) =>
              value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$exportThreadHtml = $grpc.ClientMethod<
          $0.ExportThreadHtmlRequest, $0.ExportThreadHtmlResponse>(
      '/pocketagent.v1.ThreadService/ExportThreadHtml',
      ($0.ExportThreadHtmlRequest value) => value.writeToBuffer(),
      $0.ExportThreadHtmlResponse.fromBuffer);
  static final _$optimizePrompt =
      $grpc.ClientMethod<$0.OptimizePromptRequest, $0.OptimizePromptResponse>(
          '/pocketagent.v1.ThreadService/OptimizePrompt',
          ($0.OptimizePromptRequest value) => value.writeToBuffer(),
          $0.OptimizePromptResponse.fromBuffer);
  static final _$listSlashCommands = $grpc.ClientMethod<
          $0.ListSlashCommandsRequest, $0.ListSlashCommandsResponse>(
      '/pocketagent.v1.ThreadService/ListSlashCommands',
      ($0.ListSlashCommandsRequest value) => value.writeToBuffer(),
      $0.ListSlashCommandsResponse.fromBuffer);
  static final _$startReview =
      $grpc.ClientMethod<$0.StartReviewRequest, $0.StartReviewResponse>(
          '/pocketagent.v1.ThreadService/StartReview',
          ($0.StartReviewRequest value) => value.writeToBuffer(),
          $0.StartReviewResponse.fromBuffer);
  static final _$delegateSubTask =
      $grpc.ClientMethod<$0.DelegateSubTaskRequest, $0.DelegateSubTaskResponse>(
          '/pocketagent.v1.ThreadService/DelegateSubTask',
          ($0.DelegateSubTaskRequest value) => value.writeToBuffer(),
          $0.DelegateSubTaskResponse.fromBuffer);
  static final _$listCollabRoles =
      $grpc.ClientMethod<$0.ListCollabRolesRequest, $0.ListCollabRolesResponse>(
          '/pocketagent.v1.ThreadService/ListCollabRoles',
          ($0.ListCollabRolesRequest value) => value.writeToBuffer(),
          $0.ListCollabRolesResponse.fromBuffer);
  static final _$setCollabRole =
      $grpc.ClientMethod<$0.SetCollabRoleRequest, $0.SetCollabRoleResponse>(
          '/pocketagent.v1.ThreadService/SetCollabRole',
          ($0.SetCollabRoleRequest value) => value.writeToBuffer(),
          $0.SetCollabRoleResponse.fromBuffer);
  static final _$deleteCollabRole = $grpc.ClientMethod<
          $0.DeleteCollabRoleRequest, $0.DeleteCollabRoleResponse>(
      '/pocketagent.v1.ThreadService/DeleteCollabRole',
      ($0.DeleteCollabRoleRequest value) => value.writeToBuffer(),
      $0.DeleteCollabRoleResponse.fromBuffer);
  static final _$dispatchCollab =
      $grpc.ClientMethod<$0.DispatchCollabRequest, $0.DispatchCollabResponse>(
          '/pocketagent.v1.ThreadService/DispatchCollab',
          ($0.DispatchCollabRequest value) => value.writeToBuffer(),
          $0.DispatchCollabResponse.fromBuffer);
  static final _$getCollabSession =
      $grpc.ClientMethod<$0.GetCollabSessionRequest, $0.CollabSessionSnapshot>(
          '/pocketagent.v1.ThreadService/GetCollabSession',
          ($0.GetCollabSessionRequest value) => value.writeToBuffer(),
          $0.CollabSessionSnapshot.fromBuffer);
  static final _$listCollabSessions = $grpc.ClientMethod<
          $0.ListCollabSessionsRequest, $0.ListCollabSessionsResponse>(
      '/pocketagent.v1.ThreadService/ListCollabSessions',
      ($0.ListCollabSessionsRequest value) => value.writeToBuffer(),
      $0.ListCollabSessionsResponse.fromBuffer);
  static final _$watchCollabSession =
      $grpc.ClientMethod<$0.WatchCollabSessionRequest, $0.CollabSessionEvent>(
          '/pocketagent.v1.ThreadService/WatchCollabSession',
          ($0.WatchCollabSessionRequest value) => value.writeToBuffer(),
          $0.CollabSessionEvent.fromBuffer);
  static final _$cancelCollabSession = $grpc.ClientMethod<
          $0.CancelCollabSessionRequest, $0.CollabSessionSnapshot>(
      '/pocketagent.v1.ThreadService/CancelCollabSession',
      ($0.CancelCollabSessionRequest value) => value.writeToBuffer(),
      $0.CollabSessionSnapshot.fromBuffer);
  static final _$cancelCollabMember =
      $grpc.ClientMethod<$0.CancelCollabMemberRequest, $0.CollabMemberSnapshot>(
          '/pocketagent.v1.ThreadService/CancelCollabMember',
          ($0.CancelCollabMemberRequest value) => value.writeToBuffer(),
          $0.CollabMemberSnapshot.fromBuffer);
  static final _$retryCollabMember =
      $grpc.ClientMethod<$0.RetryCollabMemberRequest, $0.CollabMemberSnapshot>(
          '/pocketagent.v1.ThreadService/RetryCollabMember',
          ($0.RetryCollabMemberRequest value) => value.writeToBuffer(),
          $0.CollabMemberSnapshot.fromBuffer);
}

@$pb.GrpcServiceName('pocketagent.v1.ThreadService')
abstract class ThreadServiceBase extends $grpc.Service {
  $core.String get $name => 'pocketagent.v1.ThreadService';

  ThreadServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.CreateThreadRequest, $0.Thread>(
        'CreateThread',
        createThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateThreadRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListThreadsRequest, $0.ListThreadsResponse>(
            'ListThreads',
            listThreads_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListThreadsRequest.fromBuffer(value),
            ($0.ListThreadsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetThreadRequest, $0.Thread>(
        'GetThread',
        getThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetThreadRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteThreadRequest, $1.Empty>(
        'DeleteThread',
        deleteThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeleteThreadRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ArchiveThreadRequest, $0.Thread>(
        'ArchiveThread',
        archiveThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ArchiveThreadRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RenameThreadRequest, $0.Thread>(
        'RenameThread',
        renameThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RenameThreadRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetThreadGoalRequest, $0.Thread>(
        'SetThreadGoal',
        setThreadGoal_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetThreadGoalRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CompactThreadRequest, $1.Empty>(
        'CompactThread',
        compactThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CompactThreadRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TurnClientMessage, $0.TurnServerMessage>(
        'StartTurn',
        startTurn,
        true,
        true,
        ($core.List<$core.int> value) => $0.TurnClientMessage.fromBuffer(value),
        ($0.TurnServerMessage value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CancelTurnRequest, $1.Empty>(
        'CancelTurn',
        cancelTurn_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CancelTurnRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ClearQueuedInputsRequest,
            $0.ClearQueuedInputsResponse>(
        'ClearQueuedInputs',
        clearQueuedInputs_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ClearQueuedInputsRequest.fromBuffer(value),
        ($0.ClearQueuedInputsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SubmitApprovalRequest, $1.Empty>(
        'SubmitApproval',
        submitApproval_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SubmitApprovalRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetThreadHistoryRequest,
            $0.GetThreadHistoryResponse>(
        'GetThreadHistory',
        getThreadHistory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetThreadHistoryRequest.fromBuffer(value),
        ($0.GetThreadHistoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SearchThreadMessagesRequest,
            $0.SearchThreadMessagesResponse>(
        'SearchThreadMessages',
        searchThreadMessages_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SearchThreadMessagesRequest.fromBuffer(value),
        ($0.SearchThreadMessagesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetThreadEventsSinceRequest,
            $0.TurnServerMessage>(
        'GetThreadEventsSince',
        getThreadEventsSince_Pre,
        false,
        true,
        ($core.List<$core.int> value) =>
            $0.GetThreadEventsSinceRequest.fromBuffer(value),
        ($0.TurnServerMessage value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListCodexThreadsRequest,
            $0.ListCodexThreadsResponse>(
        'ListCodexThreads',
        listCodexThreads_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListCodexThreadsRequest.fromBuffer(value),
        ($0.ListCodexThreadsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ImportCodexThreadRequest,
            $0.ImportCodexThreadResponse>(
        'ImportCodexThread',
        importCodexThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ImportCodexThreadRequest.fromBuffer(value),
        ($0.ImportCodexThreadResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListAgentThreadsRequest,
            $0.ListAgentThreadsResponse>(
        'ListAgentThreads',
        listAgentThreads_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListAgentThreadsRequest.fromBuffer(value),
        ($0.ListAgentThreadsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ImportAgentThreadRequest,
            $0.ImportAgentThreadResponse>(
        'ImportAgentThread',
        importAgentThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ImportAgentThreadRequest.fromBuffer(value),
        ($0.ImportAgentThreadResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ForkThreadRequest, $0.Thread>(
        'ForkThread',
        forkThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ForkThreadRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RollbackThreadRequest, $0.Thread>(
        'RollbackThread',
        rollbackThread_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RollbackThreadRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListThreadSectionsRequest,
            $0.ListThreadSectionsResponse>(
        'ListThreadSections',
        listThreadSections_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListThreadSectionsRequest.fromBuffer(value),
        ($0.ListThreadSectionsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CreateThreadSectionRequest, $0.ThreadSection>(
            'CreateThreadSection',
            createThreadSection_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateThreadSectionRequest.fromBuffer(value),
            ($0.ThreadSection value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RenameThreadSectionRequest, $0.ThreadSection>(
            'RenameThreadSection',
            renameThreadSection_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RenameThreadSectionRequest.fromBuffer(value),
            ($0.ThreadSection value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteThreadSectionRequest, $1.Empty>(
        'DeleteThreadSection',
        deleteThreadSection_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeleteThreadSectionRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MoveThreadToSectionRequest, $0.Thread>(
        'MoveThreadToSection',
        moveThreadToSection_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.MoveThreadToSectionRequest.fromBuffer(value),
        ($0.Thread value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ApproveGuardianDeniedActionRequest, $1.Empty>(
            'ApproveGuardianDeniedAction',
            approveGuardianDeniedAction_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ApproveGuardianDeniedActionRequest.fromBuffer(value),
            ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ExportThreadHtmlRequest,
            $0.ExportThreadHtmlResponse>(
        'ExportThreadHtml',
        exportThreadHtml_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ExportThreadHtmlRequest.fromBuffer(value),
        ($0.ExportThreadHtmlResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.OptimizePromptRequest,
            $0.OptimizePromptResponse>(
        'OptimizePrompt',
        optimizePrompt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.OptimizePromptRequest.fromBuffer(value),
        ($0.OptimizePromptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListSlashCommandsRequest,
            $0.ListSlashCommandsResponse>(
        'ListSlashCommands',
        listSlashCommands_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListSlashCommandsRequest.fromBuffer(value),
        ($0.ListSlashCommandsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.StartReviewRequest, $0.StartReviewResponse>(
            'StartReview',
            startReview_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.StartReviewRequest.fromBuffer(value),
            ($0.StartReviewResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DelegateSubTaskRequest,
            $0.DelegateSubTaskResponse>(
        'DelegateSubTask',
        delegateSubTask_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DelegateSubTaskRequest.fromBuffer(value),
        ($0.DelegateSubTaskResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListCollabRolesRequest,
            $0.ListCollabRolesResponse>(
        'ListCollabRoles',
        listCollabRoles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListCollabRolesRequest.fromBuffer(value),
        ($0.ListCollabRolesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SetCollabRoleRequest, $0.SetCollabRoleResponse>(
            'SetCollabRole',
            setCollabRole_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SetCollabRoleRequest.fromBuffer(value),
            ($0.SetCollabRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteCollabRoleRequest,
            $0.DeleteCollabRoleResponse>(
        'DeleteCollabRole',
        deleteCollabRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeleteCollabRoleRequest.fromBuffer(value),
        ($0.DeleteCollabRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DispatchCollabRequest,
            $0.DispatchCollabResponse>(
        'DispatchCollab',
        dispatchCollab_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DispatchCollabRequest.fromBuffer(value),
        ($0.DispatchCollabResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetCollabSessionRequest,
            $0.CollabSessionSnapshot>(
        'GetCollabSession',
        getCollabSession_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetCollabSessionRequest.fromBuffer(value),
        ($0.CollabSessionSnapshot value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListCollabSessionsRequest,
            $0.ListCollabSessionsResponse>(
        'ListCollabSessions',
        listCollabSessions_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListCollabSessionsRequest.fromBuffer(value),
        ($0.ListCollabSessionsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.WatchCollabSessionRequest,
            $0.CollabSessionEvent>(
        'WatchCollabSession',
        watchCollabSession_Pre,
        false,
        true,
        ($core.List<$core.int> value) =>
            $0.WatchCollabSessionRequest.fromBuffer(value),
        ($0.CollabSessionEvent value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CancelCollabSessionRequest,
            $0.CollabSessionSnapshot>(
        'CancelCollabSession',
        cancelCollabSession_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CancelCollabSessionRequest.fromBuffer(value),
        ($0.CollabSessionSnapshot value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CancelCollabMemberRequest,
            $0.CollabMemberSnapshot>(
        'CancelCollabMember',
        cancelCollabMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CancelCollabMemberRequest.fromBuffer(value),
        ($0.CollabMemberSnapshot value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RetryCollabMemberRequest,
            $0.CollabMemberSnapshot>(
        'RetryCollabMember',
        retryCollabMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RetryCollabMemberRequest.fromBuffer(value),
        ($0.CollabMemberSnapshot value) => value.writeToBuffer()));
  }

  $async.Future<$0.Thread> createThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateThreadRequest> $request) async {
    return createThread($call, await $request);
  }

  $async.Future<$0.Thread> createThread(
      $grpc.ServiceCall call, $0.CreateThreadRequest request);

  $async.Future<$0.ListThreadsResponse> listThreads_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListThreadsRequest> $request) async {
    return listThreads($call, await $request);
  }

  $async.Future<$0.ListThreadsResponse> listThreads(
      $grpc.ServiceCall call, $0.ListThreadsRequest request);

  $async.Future<$0.Thread> getThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetThreadRequest> $request) async {
    return getThread($call, await $request);
  }

  $async.Future<$0.Thread> getThread(
      $grpc.ServiceCall call, $0.GetThreadRequest request);

  $async.Future<$1.Empty> deleteThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteThreadRequest> $request) async {
    return deleteThread($call, await $request);
  }

  $async.Future<$1.Empty> deleteThread(
      $grpc.ServiceCall call, $0.DeleteThreadRequest request);

  $async.Future<$0.Thread> archiveThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ArchiveThreadRequest> $request) async {
    return archiveThread($call, await $request);
  }

  $async.Future<$0.Thread> archiveThread(
      $grpc.ServiceCall call, $0.ArchiveThreadRequest request);

  $async.Future<$0.Thread> renameThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.RenameThreadRequest> $request) async {
    return renameThread($call, await $request);
  }

  $async.Future<$0.Thread> renameThread(
      $grpc.ServiceCall call, $0.RenameThreadRequest request);

  $async.Future<$0.Thread> setThreadGoal_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SetThreadGoalRequest> $request) async {
    return setThreadGoal($call, await $request);
  }

  $async.Future<$0.Thread> setThreadGoal(
      $grpc.ServiceCall call, $0.SetThreadGoalRequest request);

  $async.Future<$1.Empty> compactThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CompactThreadRequest> $request) async {
    return compactThread($call, await $request);
  }

  $async.Future<$1.Empty> compactThread(
      $grpc.ServiceCall call, $0.CompactThreadRequest request);

  $async.Stream<$0.TurnServerMessage> startTurn(
      $grpc.ServiceCall call, $async.Stream<$0.TurnClientMessage> request);

  $async.Future<$1.Empty> cancelTurn_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CancelTurnRequest> $request) async {
    return cancelTurn($call, await $request);
  }

  $async.Future<$1.Empty> cancelTurn(
      $grpc.ServiceCall call, $0.CancelTurnRequest request);

  $async.Future<$0.ClearQueuedInputsResponse> clearQueuedInputs_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ClearQueuedInputsRequest> $request) async {
    return clearQueuedInputs($call, await $request);
  }

  $async.Future<$0.ClearQueuedInputsResponse> clearQueuedInputs(
      $grpc.ServiceCall call, $0.ClearQueuedInputsRequest request);

  $async.Future<$1.Empty> submitApproval_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SubmitApprovalRequest> $request) async {
    return submitApproval($call, await $request);
  }

  $async.Future<$1.Empty> submitApproval(
      $grpc.ServiceCall call, $0.SubmitApprovalRequest request);

  $async.Future<$0.GetThreadHistoryResponse> getThreadHistory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetThreadHistoryRequest> $request) async {
    return getThreadHistory($call, await $request);
  }

  $async.Future<$0.GetThreadHistoryResponse> getThreadHistory(
      $grpc.ServiceCall call, $0.GetThreadHistoryRequest request);

  $async.Future<$0.SearchThreadMessagesResponse> searchThreadMessages_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SearchThreadMessagesRequest> $request) async {
    return searchThreadMessages($call, await $request);
  }

  $async.Future<$0.SearchThreadMessagesResponse> searchThreadMessages(
      $grpc.ServiceCall call, $0.SearchThreadMessagesRequest request);

  $async.Stream<$0.TurnServerMessage> getThreadEventsSince_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetThreadEventsSinceRequest> $request) async* {
    yield* getThreadEventsSince($call, await $request);
  }

  $async.Stream<$0.TurnServerMessage> getThreadEventsSince(
      $grpc.ServiceCall call, $0.GetThreadEventsSinceRequest request);

  $async.Future<$0.ListCodexThreadsResponse> listCodexThreads_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListCodexThreadsRequest> $request) async {
    return listCodexThreads($call, await $request);
  }

  $async.Future<$0.ListCodexThreadsResponse> listCodexThreads(
      $grpc.ServiceCall call, $0.ListCodexThreadsRequest request);

  $async.Future<$0.ImportCodexThreadResponse> importCodexThread_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ImportCodexThreadRequest> $request) async {
    return importCodexThread($call, await $request);
  }

  $async.Future<$0.ImportCodexThreadResponse> importCodexThread(
      $grpc.ServiceCall call, $0.ImportCodexThreadRequest request);

  $async.Future<$0.ListAgentThreadsResponse> listAgentThreads_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListAgentThreadsRequest> $request) async {
    return listAgentThreads($call, await $request);
  }

  $async.Future<$0.ListAgentThreadsResponse> listAgentThreads(
      $grpc.ServiceCall call, $0.ListAgentThreadsRequest request);

  $async.Future<$0.ImportAgentThreadResponse> importAgentThread_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ImportAgentThreadRequest> $request) async {
    return importAgentThread($call, await $request);
  }

  $async.Future<$0.ImportAgentThreadResponse> importAgentThread(
      $grpc.ServiceCall call, $0.ImportAgentThreadRequest request);

  $async.Future<$0.Thread> forkThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ForkThreadRequest> $request) async {
    return forkThread($call, await $request);
  }

  $async.Future<$0.Thread> forkThread(
      $grpc.ServiceCall call, $0.ForkThreadRequest request);

  $async.Future<$0.Thread> rollbackThread_Pre($grpc.ServiceCall $call,
      $async.Future<$0.RollbackThreadRequest> $request) async {
    return rollbackThread($call, await $request);
  }

  $async.Future<$0.Thread> rollbackThread(
      $grpc.ServiceCall call, $0.RollbackThreadRequest request);

  $async.Future<$0.ListThreadSectionsResponse> listThreadSections_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListThreadSectionsRequest> $request) async {
    return listThreadSections($call, await $request);
  }

  $async.Future<$0.ListThreadSectionsResponse> listThreadSections(
      $grpc.ServiceCall call, $0.ListThreadSectionsRequest request);

  $async.Future<$0.ThreadSection> createThreadSection_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateThreadSectionRequest> $request) async {
    return createThreadSection($call, await $request);
  }

  $async.Future<$0.ThreadSection> createThreadSection(
      $grpc.ServiceCall call, $0.CreateThreadSectionRequest request);

  $async.Future<$0.ThreadSection> renameThreadSection_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RenameThreadSectionRequest> $request) async {
    return renameThreadSection($call, await $request);
  }

  $async.Future<$0.ThreadSection> renameThreadSection(
      $grpc.ServiceCall call, $0.RenameThreadSectionRequest request);

  $async.Future<$1.Empty> deleteThreadSection_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteThreadSectionRequest> $request) async {
    return deleteThreadSection($call, await $request);
  }

  $async.Future<$1.Empty> deleteThreadSection(
      $grpc.ServiceCall call, $0.DeleteThreadSectionRequest request);

  $async.Future<$0.Thread> moveThreadToSection_Pre($grpc.ServiceCall $call,
      $async.Future<$0.MoveThreadToSectionRequest> $request) async {
    return moveThreadToSection($call, await $request);
  }

  $async.Future<$0.Thread> moveThreadToSection(
      $grpc.ServiceCall call, $0.MoveThreadToSectionRequest request);

  $async.Future<$1.Empty> approveGuardianDeniedAction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ApproveGuardianDeniedActionRequest> $request) async {
    return approveGuardianDeniedAction($call, await $request);
  }

  $async.Future<$1.Empty> approveGuardianDeniedAction(
      $grpc.ServiceCall call, $0.ApproveGuardianDeniedActionRequest request);

  $async.Future<$0.ExportThreadHtmlResponse> exportThreadHtml_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ExportThreadHtmlRequest> $request) async {
    return exportThreadHtml($call, await $request);
  }

  $async.Future<$0.ExportThreadHtmlResponse> exportThreadHtml(
      $grpc.ServiceCall call, $0.ExportThreadHtmlRequest request);

  $async.Future<$0.OptimizePromptResponse> optimizePrompt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.OptimizePromptRequest> $request) async {
    return optimizePrompt($call, await $request);
  }

  $async.Future<$0.OptimizePromptResponse> optimizePrompt(
      $grpc.ServiceCall call, $0.OptimizePromptRequest request);

  $async.Future<$0.ListSlashCommandsResponse> listSlashCommands_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListSlashCommandsRequest> $request) async {
    return listSlashCommands($call, await $request);
  }

  $async.Future<$0.ListSlashCommandsResponse> listSlashCommands(
      $grpc.ServiceCall call, $0.ListSlashCommandsRequest request);

  $async.Future<$0.StartReviewResponse> startReview_Pre($grpc.ServiceCall $call,
      $async.Future<$0.StartReviewRequest> $request) async {
    return startReview($call, await $request);
  }

  $async.Future<$0.StartReviewResponse> startReview(
      $grpc.ServiceCall call, $0.StartReviewRequest request);

  $async.Future<$0.DelegateSubTaskResponse> delegateSubTask_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DelegateSubTaskRequest> $request) async {
    return delegateSubTask($call, await $request);
  }

  $async.Future<$0.DelegateSubTaskResponse> delegateSubTask(
      $grpc.ServiceCall call, $0.DelegateSubTaskRequest request);

  $async.Future<$0.ListCollabRolesResponse> listCollabRoles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListCollabRolesRequest> $request) async {
    return listCollabRoles($call, await $request);
  }

  $async.Future<$0.ListCollabRolesResponse> listCollabRoles(
      $grpc.ServiceCall call, $0.ListCollabRolesRequest request);

  $async.Future<$0.SetCollabRoleResponse> setCollabRole_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetCollabRoleRequest> $request) async {
    return setCollabRole($call, await $request);
  }

  $async.Future<$0.SetCollabRoleResponse> setCollabRole(
      $grpc.ServiceCall call, $0.SetCollabRoleRequest request);

  $async.Future<$0.DeleteCollabRoleResponse> deleteCollabRole_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteCollabRoleRequest> $request) async {
    return deleteCollabRole($call, await $request);
  }

  $async.Future<$0.DeleteCollabRoleResponse> deleteCollabRole(
      $grpc.ServiceCall call, $0.DeleteCollabRoleRequest request);

  $async.Future<$0.DispatchCollabResponse> dispatchCollab_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DispatchCollabRequest> $request) async {
    return dispatchCollab($call, await $request);
  }

  $async.Future<$0.DispatchCollabResponse> dispatchCollab(
      $grpc.ServiceCall call, $0.DispatchCollabRequest request);

  $async.Future<$0.CollabSessionSnapshot> getCollabSession_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetCollabSessionRequest> $request) async {
    return getCollabSession($call, await $request);
  }

  $async.Future<$0.CollabSessionSnapshot> getCollabSession(
      $grpc.ServiceCall call, $0.GetCollabSessionRequest request);

  $async.Future<$0.ListCollabSessionsResponse> listCollabSessions_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListCollabSessionsRequest> $request) async {
    return listCollabSessions($call, await $request);
  }

  $async.Future<$0.ListCollabSessionsResponse> listCollabSessions(
      $grpc.ServiceCall call, $0.ListCollabSessionsRequest request);

  $async.Stream<$0.CollabSessionEvent> watchCollabSession_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.WatchCollabSessionRequest> $request) async* {
    yield* watchCollabSession($call, await $request);
  }

  $async.Stream<$0.CollabSessionEvent> watchCollabSession(
      $grpc.ServiceCall call, $0.WatchCollabSessionRequest request);

  $async.Future<$0.CollabSessionSnapshot> cancelCollabSession_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CancelCollabSessionRequest> $request) async {
    return cancelCollabSession($call, await $request);
  }

  $async.Future<$0.CollabSessionSnapshot> cancelCollabSession(
      $grpc.ServiceCall call, $0.CancelCollabSessionRequest request);

  $async.Future<$0.CollabMemberSnapshot> cancelCollabMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CancelCollabMemberRequest> $request) async {
    return cancelCollabMember($call, await $request);
  }

  $async.Future<$0.CollabMemberSnapshot> cancelCollabMember(
      $grpc.ServiceCall call, $0.CancelCollabMemberRequest request);

  $async.Future<$0.CollabMemberSnapshot> retryCollabMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RetryCollabMemberRequest> $request) async {
    return retryCollabMember($call, await $request);
  }

  $async.Future<$0.CollabMemberSnapshot> retryCollabMember(
      $grpc.ServiceCall call, $0.RetryCollabMemberRequest request);
}
