// 协同角色 store 测试（B4 第 4 波）：load/save/delete 三动作 + 三态 +
// 错误消息提取 + 错误态不冒充空列表。
//
// 【mock 的是宿主 wire 行为，PaBridge 全程真实】与 transport 层测试同一纪律
// （pa_bridge_threads_test / pa_bridge_files_git_test 的模式）：按通道名
// setMockMethodCallHandler，store 自己构造真 PaBridge 打真 invokeMethod。
// 这样测的是「store 怎么用通道」而不是「store 被 stub 成什么样」。
//
// 【两条必须分家的路径】宿主未注册（HOST_UNAVAILABLE）→ loadState=ERROR +
// loadError 点名 B4；宿主正常回空响应 → loadState=READY + roles 空。
// 各钉一条且互斥断言（红线：不留假绿，「通道没就绪」绝不许伪装成「没有角色」）。
//
// 【错误消息提取】daemon 的 InvalidArgument 人话原文经 PaBridgeException
// .message 透传（Kotlin 侧已剥掉 gRPC 状态码前缀）；宿主没给 message 时退
// toString（宁可难看也不泛化成「操作失败」）。

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/thread.pb.dart';
import 'package:pocketagent_ui/pages/collab_roles_state.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

CollabRole protoRole({
  String id = 'reviewer',
  String name = '安全审查者',
  String agentId = 'codex',
  String description = '只找安全问题',
  String instructions = '必须列证据',
  String model = 'codex/gpt-5',
  String effort = 'high',
  String workspace = '',
}) =>
    CollabRole(
      id: id,
      name: name,
      agentId: agentId,
      description: description,
      instructions: instructions,
      model: model,
      effort: effort,
      workspace: workspace,
    );

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  });

  group('load：三态与分家', () {
    test('成功 → READY + roles 逐字段落位（顺序保持 daemon 原序）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return ListCollabRolesResponse(
          roles: <CollabRole>[
            protoRole(id: 'reviewer', name: '安全审查者'),
            protoRole(id: 'tester', name: '', agentId: 'pi'),
          ],
        ).writeToBuffer();
      });

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      expect(store.loadState, CollabRolesLoadState.ready);
      expect(store.loadError, isNull);
      expect(store.roles, hasLength(2));
      expect(store.roles[0].id, 'reviewer');
      expect(store.roles[0].name, '安全审查者');
      expect(store.roles[1].id, 'tester');
      expect(store.roles[1].name, isEmpty, reason: '空 name 原样透传（显示名回落是页面层的事）');
      expect(store.roles[1].agentId, 'pi');
    });

    test('空响应（0 字节）→ READY + 真空列表（与错误路径分家的另一条）', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => ListCollabRolesResponse().writeToBuffer());

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      expect(store.loadState, CollabRolesLoadState.ready);
      expect(store.loadError, isNull);
      expect(store.roles, isEmpty);
    });

    test('宿主桥未实现（MissingPluginException）→ ERROR，不冒充空列表', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      expect(store.loadState, CollabRolesLoadState.error);
      expect(store.roles, isEmpty);
      // 文案点名 B4 波次（collabRolesErrorText 的定制分支）；裸
      // MissingPluginException 原文也保留在 store 的 loadError 里供定位。
      expect(collabRolesErrorText(store.loadError), contains('B4 波次'));
      expect(store.loadError, contains('No implementation found'));
    });

    test('宿主 PlatformException → ERROR + message 原文透传（不泛化）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(
          code: 'NOT_CONNECTED',
          message: '还没连上主机',
        );
      });

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      expect(store.loadState, CollabRolesLoadState.error);
      expect(store.loadError, '还没连上主机');
      expect(collabRolesErrorText(store.loadError), '还没连上主机');
    });

    test('坏字节 → ERROR + DECODE_FAILED 定位信息', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => Uint8List.fromList(<int>[0xFF, 0xFF, 0xFF]));

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      expect(store.loadState, CollabRolesLoadState.error);
      expect(store.loadError, contains('ListCollabRolesResponse'));
    });

    test('重试：宿主从「未注册」换成「正常应答」后 load 转 READY', () async {
      bool registered = false;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (!registered) {
          throw MissingPluginException('No implementation found');
        }
        return ListCollabRolesResponse(roles: <CollabRole>[protoRole()]).writeToBuffer();
      });

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;
      expect(store.loadState, CollabRolesLoadState.error);

      registered = true;
      store.load();
      await store.loadCompleted;
      expect(store.loadState, CollabRolesLoadState.ready);
      expect(store.roles, hasLength(1));
    });
  });

  group('save：upsert 往返', () {
    test('成功 → null + 本地列表用 daemon 回显真值刷新（trim 过的 id）', () async {
      SetCollabRoleRequest? seenRequest;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'setCollabRole') {
          seenRequest = SetCollabRoleRequest.fromBuffer(call.arguments as List<int>);
          // daemon 回显的是 trim 后的真值（用户填了 "  reviewer  "）
          return SetCollabRoleResponse(role: protoRole(id: 'reviewer')).writeToBuffer();
        }
        return ListCollabRolesResponse().writeToBuffer();
      });

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      final String? err = await store.save(CollabRole(id: '  reviewer  ', agentId: 'codex'));

      expect(err, isNull);
      expect(seenRequest, isNotNull);
      expect(seenRequest!.role.id, '  reviewer  ',
          reason: '方法面原样编码（trim 是 daemon 的职责， Dart 侧不预先改写）');
      expect(store.roles, hasLength(1));
      expect(store.roles.single.id, 'reviewer',
          reason: '回显落库真值，不保留用户输入的原样空白');
    });

    test('同 id 再保存 = 覆盖（不产生第二行）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'setCollabRole') {
          final SetCollabRoleRequest req =
              SetCollabRoleRequest.fromBuffer(call.arguments as List<int>);
          return SetCollabRoleResponse(role: req.role).writeToBuffer();
        }
        return ListCollabRolesResponse().writeToBuffer();
      });

      final CollabRolesStore store = CollabRolesStore();
      await store.save(CollabRole(id: 'reviewer', name: '第一版'));
      await store.save(CollabRole(id: 'reviewer', name: '第二版'));

      expect(store.roles, hasLength(1));
      expect(store.roles.single.name, '第二版');
    });

    test('daemon InvalidArgument → 原文透传（客户端前置校验之外的第二道）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'setCollabRole') {
          throw PlatformException(
            code: 'INVALID_ARGUMENT',
            message: '协同角色 id 不能含空格:请用连字符或下划线连接(如 code-reviewer)',
          );
        }
        return ListCollabRolesResponse().writeToBuffer();
      });

      final CollabRolesStore store = CollabRolesStore();
      final String? err = await store.save(CollabRole(id: 'my role'));

      expect(err, '协同角色 id 不能含空格:请用连字符或下划线连接(如 code-reviewer)');
      expect(store.roles, isEmpty, reason: '失败不动本地列表');
    });

    test('宿主未注册 → HOST_UNAVAILABLE 定制文案（不假装保存成功）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('No implementation found');
      });

      final CollabRolesStore store = CollabRolesStore();
      final String? err = await store.save(CollabRole(id: 'reviewer'));

      expect(err, isNotNull);
      expect(collabRolesErrorText(err), contains('B4 波次'));
      expect(store.roles, isEmpty);
    });
  });

  group('delete：幂等删除', () {
    test('成功 → null + 本地移除该行（其余不动）', () async {
      DeleteCollabRoleRequest? seenRequest;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'deleteCollabRole') {
          seenRequest = DeleteCollabRoleRequest.fromBuffer(call.arguments as List<int>);
          return DeleteCollabRoleResponse().writeToBuffer();
        }
        return ListCollabRolesResponse(
          roles: <CollabRole>[
            protoRole(id: 'reviewer'),
            protoRole(id: 'tester', agentId: 'pi'),
          ],
        ).writeToBuffer();
      });

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;
      expect(store.roles, hasLength(2));

      final String? err = await store.delete('reviewer');

      expect(err, isNull);
      expect(seenRequest!.id, 'reviewer');
      expect(store.roles, hasLength(1));
      expect(store.roles.single.id, 'tester');
    });

    test('失败 → daemon 原文返回，本地列表不动', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'deleteCollabRole') {
          throw PlatformException(
            code: 'INTERNAL',
            message: '删除失败: 角色文件被占用',
          );
        }
        return ListCollabRolesResponse(roles: <CollabRole>[protoRole()]).writeToBuffer();
      });

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      final String? err = await store.delete('reviewer');

      expect(err, '删除失败: 角色文件被占用');
      expect(store.roles, hasLength(1), reason: '删除失败行还在');
    });
  });

  group('extractRpcMessage：消息提取纪律（对位 Kotlin extractRpcMessage）', () {
    test('PaBridgeException → message 原文', () {
      expect(
        CollabRolesStore.extractRpcMessage(
            const PaBridgeException('INVALID_ARGUMENT', '协同角色 id 不能为空')),
        '协同角色 id 不能为空',
      );
    });

    test('message 空白 → 退 toString（不显空串）', () {
      expect(
        CollabRolesStore.extractRpcMessage(const PaBridgeException('INTERNAL', '   ')),
        isNotEmpty,
      );
    });

    test('非 PaBridgeException → 退 toString（含类型名，可核查）', () {
      expect(
        CollabRolesStore.extractRpcMessage(Exception('boom')),
        contains('boom'),
      );
    });
  });

  group('roles 只读视图', () {
    test('外部改不动 store 列表（写操作只能走 save/delete）', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => ListCollabRolesResponse().writeToBuffer());

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      expect(() => store.roles.add(protoRole()), throwsUnsupportedError);
    });
  });

  group('findById：编辑器进页预填', () {
    test('命中 → 该角色；未命中 → null（页面走「可能已被删除」降级）', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => ListCollabRolesResponse(
          roles: <CollabRole>[protoRole(id: 'reviewer')],
        ).writeToBuffer());

      final CollabRolesStore store = CollabRolesStore();
      store.load();
      await store.loadCompleted;

      expect(store.findById('reviewer')?.name, '安全审查者');
      expect(store.findById('ghost'), isNull);
    });
  });

  group('collabRolesErrorText：页面文案', () {
    test('null/空 → 兜底人话（不留空条）', () {
      expect(collabRolesErrorText(null), isNotEmpty);
      expect(collabRolesErrorText('   '), isNotEmpty);
    });

    test('HOST_UNAVAILABLE 原文 → 点名 B4 波次的定制文案', () {
      expect(
        collabRolesErrorText(
            '宿主桥未注册：dev.pocketagent/bridge（B4 波次实现）'),
        contains('B4 波次'),
      );
    });

    test('daemon 业务拒绝原文 → 原样透传（不改写、不吞）', () {
      const String raw = '协同角色 id 过长(65 字,上限 64):id 会被写进委派工具的参数里';
      expect(collabRolesErrorText(raw), raw);
    });
  });
}
