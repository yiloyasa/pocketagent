import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:fixnum/fixnum.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/host.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/common.pb.dart';

/// Go↔Dart wire 兼容对账（B2 核心）。
///
/// 【为什么有这组测试】Dart stub 是 protoc 从 proto/v1/*.proto 生成的，但与
/// 真正连着 Go daemon 跑之前，"能编译"不等于"线格式一致"：oneof 分支、proto3
/// optional 显式在场（hasXxx）、int64 的 varint、double 的 fixed64、enum 序号，
/// 任何一端理解偏差都会在真机联调时才炸。这里用 python protobuf（官方参考实现，
/// 与 Go/Dart 同源同描述符）当**第三方编码器**造 fixture，Dart 反序列化后逐项断言。
///
/// fixture 由 tools/protofixture/gen_fixtures.py 生成（protoc 现生成 + python
/// marshal），断言值在本文件**人工独立抄写**（与生成器同源但不同入口），防止
/// "生成器错了、测试跟着错"的假绿。fixture 是真实 wire 字节（base64 包裹防
/// git autocrlf 改坏），不是手写常量消息。
void main() {
  /// 读 fixture 原始字节。flutter test 的 cwd 是工程根；若有人从仓库根跑，
  /// 第二条候选兜底。两条都不中即 fail（绝不静默跳过对账）。
  Uint8List fixture(String name) {
    final candidates = <String>[
      '${Directory.current.path}/tools/protofixture/fixtures/$name.b64',
      '${Directory.current.path}/flutter/tools/protofixture/fixtures/$name.b64',
    ];
    for (final c in candidates) {
      final f = File(c);
      if (f.existsSync()) {
        return base64.decode(f.readAsStringSync().trim());
      }
    }
    fail('找不到 fixture $name（试过：${candidates.join(", ")}；'
        'cwd=${Directory.current.path}）');
  }

  group('wire fixture：Python protobuf 编码 → Dart 反序列化', () {
    test('turn_server_text：TurnServerMessage/text_delta + sequence', () {
      final msg = TurnServerMessage.fromBuffer(fixture('turn_server_text'));

      expect(msg.whichMsg(), TurnServerMessage_Msg.textDelta);
      expect(msg.hasTextDelta(), isTrue);
      expect(msg.textDelta.delta, '你好,世界 — δelta ✓',
          reason: 'UTF-8 字节数 ≠ 字符数，字符串字段按带长度字节读');
      expect(msg.sequence, Int64(424242), reason: 'int64 → fixnum Int64');
      // 同帧只有一个 oneof 分支在场，其它分支不得被误建
      expect(msg.hasApprovalRequest(), isFalse);
      expect(msg.hasCompleted(), isFalse);
    });

    test('turn_server_approval：嵌套 repeated ApprovalOption + oneof', () {
      final msg = TurnServerMessage.fromBuffer(fixture('turn_server_approval'));

      expect(msg.whichMsg(), TurnServerMessage_Msg.approvalRequest);
      expect(msg.sequence, Int64(777001));

      final req = msg.approvalRequest;
      expect(req.requestId, 'req-9f27c1');
      expect(req.action.id, 'call-001');
      expect(req.action.name, 'shell');
      expect(req.action.jsonArgs, '{"cmd":"ls -la","cwd":"."}');
      expect(req.reason, '即将在电脑上执行 shell 命令,请确认。');
      expect(req.details, '命令将在工作区根目录执行,可能读取文件列表。');

      // C1 三按钮审批：options 顺序 = 受理顺序，Dart 侧按同序渲染
      expect(req.options, hasLength(3));
      expect(req.options.map((o) => o.id).toList(),
          ['accept', 'acceptForSession', 'decline']);
      expect(req.options[1].label, '本次及本会话允许');
      expect(req.options[1].description, '本会话内同类请求不再询问');
    });

    test('turn_server_completed：ContentBlock 双 oneof 分支 + Usage + enum', () {
      final msg = TurnServerMessage.fromBuffer(fixture('turn_server_completed'));

      expect(msg.whichMsg(), TurnServerMessage_Msg.completed);
      expect(msg.sequence, Int64(8808));

      final done = msg.completed;
      expect(done.turnId, 'turn-0007');
      expect(done.elapsedMs, Int64(123456));
      expect(done.outputState, OutputState.OUTPUT_STATE_TEXT,
          reason: 'enum 按编号走线，不是按名字');

      expect(done.finalBlocks, hasLength(2));
      expect(done.finalBlocks[0].whichBlock(), ContentBlock_Block.text);
      expect(done.finalBlocks[0].text.text, '已完成');
      expect(done.finalBlocks[1].whichBlock(), ContentBlock_Block.toolCall);
      expect(done.finalBlocks[1].toolCall.name, 'write_file');

      final u = done.usage;
      expect(u.inputTokens, Int64.parseInt('9223372036854775807'),
          reason: 'int64 max = 10 字节 varint 上限，Dart Int64 不得溢出成负数');
      expect(u.outputTokens, Int64(6789));
      expect(u.cacheReadTokens, Int64(1000000));
      expect(u.cacheWriteTokens, Int64(512));
      expect(u.costUsd, closeTo(0.0425, 1e-12), reason: 'double = fixed64 位模式');
      expect(u.reasoningTokens, Int64(321));
    });

    test('host_info：能力位列表 + HostRecoveryReport 三层嵌套树', () {
      final msg = HostInfo.fromBuffer(fixture('host_info'));

      expect(msg.hostId, 'host-8f3a91c2');
      expect(msg.displayName, 'YANZI-PC');
      expect(msg.os, 'windows');
      expect(msg.arch, 'amd64');
      expect(msg.daemonVersion, '0.4.0');
      expect(msg.workspaces, [r'G:\proj\alpha', r'G:\proj\beta'],
          reason: 'Windows 反斜杠路径原样往返');
      expect(msg.capabilities, [
        'terminal', 'fs', 'git',
        'agent:codex', 'agent:pi',
        'collab:pi', 'runtime.v1',
      ], reason: '§3.1 能力位唯一真值的 wire 形态（顺序保持）');
      expect(msg.authRequired, isTrue);
      expect(msg.defaultShell, 'powershell');
      expect(msg.tailscaleIpv4, '100.64.0.7');
      expect(msg.lanIpv4, '192.168.1.20');
      expect(msg.minAppVersion, '0.3.0');

      final rec = msg.recovery;
      expect(rec.status, 'degraded');
      expect(rec.summary, '上次退出时有 2 条排队输入未执行。');
      expect(rec.store.outcome, 'clean');
      expect(rec.store.hasBackup, isTrue);
      expect(rec.turnJournal.pendingCount, 2);
      expect(rec.turnJournal.threadIds, ['thread-42', 'thread-7']);
      expect(rec.turnJournal.lostInputs, hasLength(1));
      expect(rec.turnJournal.lostInputs[0].preview, '把 README 翻译成中文');
      expect(rec.turnJournal.lostInputs[0].createdAtUnixMs, Int64(1728900500000));
      expect(rec.devices.degraded, isFalse);
      expect(rec.identity.degraded, isFalse);
    });

    test('thread：enum + proto3 optional 显式在场 + 嵌套选择真值', () {
      final msg = Thread.fromBuffer(fixture('thread'));

      expect(msg.id, 'thread-42');
      expect(msg.title, '修复登录页在低网速下的崩溃');
      expect(msg.workspace, r'G:\proj\alpha');
      expect(msg.goal, '保持向后兼容,旧版本照样能用');
      expect(msg.modelId, 'codex/gpt-5.4');
      expect(msg.status, ThreadStatus.THREAD_STATUS_WAITING_APPROVAL);
      expect(msg.createdAtUnixMs, Int64(1728900000000));
      expect(msg.updatedAtUnixMs, Int64(1728901200000));
      expect(msg.messageCount, 17);
      expect(msg.agentId, 'codex');
      expect(msg.effort, 'high');
      expect(msg.archived, isFalse);
      expect(msg.sectionId, 'sec-bugfix');

      // proto3 optional：设置了才在场（hasXxx），没设置不得被零值冒充
      expect(msg.hasTurnStartedUnixMs(), isTrue);
      expect(msg.turnStartedUnixMs, Int64(1728901000000));

      final sel = msg.effectiveSelection;
      expect(sel.agentId, 'codex');
      expect(sel.modelId, 'codex/gpt-5.4');
      expect(sel.effort, 'high');
      expect(sel.source, 'thread_override');
      expect(sel.appliesToNextTurn, isFalse);
      expect(sel.configurationConfirmed, isTrue);
      expect(sel.explanation, '本会话显式选择了 gpt-5.4 / high(本会话覆盖优先)。');
      expect(sel.availableEfforts, ['low', 'medium', 'high', 'xhigh']);
    });

    test('host_event：HostEvent/turn_finished + sequence（EventChannel 载体）', () {
      final msg = HostEvent.fromBuffer(fixture('host_event'));

      expect(msg.whichEvent(), HostEvent_Event.turnFinished);
      expect(msg.sequence, Int64(9001),
          reason: '与 TurnServerMessage.sequence 同一序列空间');
      expect(msg.timestampUnixMs, Int64(1728902400000));
      expect(msg.turnFinished.threadId, 'thread-42');
      expect(msg.turnFinished.summary, '已完成:修复登录页在低网速下的崩溃');
    });

    test('manifest 账本：protoc/python 版本未漂移', () {
      // 换 protoc 或 python protobuf 大版本时线格式理论兼容，但"环境与记录不符"
      // 必须显式失败一次，逼得维护者重新评估而不是默默换编码器。
      final f = File(
          '${Directory.current.path}/tools/protofixture/fixtures/manifest.json');
      expect(f.existsSync(), isTrue,
          reason: 'manifest 与 fixture 同目录生成，缺一份即是流程被绕过');
      final m = jsonDecode(f.readAsStringSync()) as Map<String, dynamic>;
      expect(m['protoc'], 'libprotoc 36.1');
      expect(m['python_protobuf'], '7.36.1');
      expect((m['proto_files'] as List).length, 8,
          reason: '8 个 proto 一个都不能少（common/fs/git/host/model/runtime/terminal/thread）');
    });
  });
}
