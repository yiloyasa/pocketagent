// `listDirectory` / `readFile` / `listBranches` 方法面测试（B3 第 3 波追补的
// 第 8/9/10 条方法）——mock 的仍是「宿主桥的 wire 行为」，PaBridge 全程真实。
// 模式与 pa_bridge_test.dart / pa_bridge_threads_test.dart 完全一致（同一套
// mock handler + 同一套错误映射断言），只是文件按方法面拆分。
//
// 【字节来源】响应字节一律由 Dart gen stub 现组（`XxxResponse(...)
// .writeToBuffer()`），等价 Kotlin 宿主 `XxxResponse.newBuilder().addXxx(...)`
// 的写法；readFile 的 FileChunk 带 data/total_size/text/modified_unix_ms 四字段，
// 对齐 daemon fs.go:106-109 首块形态。空响应用空消息 writeToBuffer()——0 字节
// 也是合法 proto。
//
// 【流式怎么测】proto 的 ReadFile 是服务器流；Dart 方法面按契约收成「首块」
// 单份 FileChunk（channel-contract.md §3.8），所以这里按单份字节断言，不假装
// 测多块汇聚（那是 B4 宿主的实现，Dart 侧只验首块的字段解码）。

import 'dart:convert';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/fs.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/git.pb.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;
  late PaBridge bridge;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    bridge = PaBridge();
    // mock handler 由 TestDefaultBinaryMessenger 在每个测试后自动清空。
  });

  group('listDirectory：参数编码与返回解码', () {
    test('默认（无参）= 空 ListDirectoryRequest 字节；响应 → 条目逐字段对账',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListDirectoryResponse(
          entries: <DirectoryEntry>[
            DirectoryEntry(
              name: 'docs',
              path: r'G:\repo\docs',
              isDir: true,
              sizeBytes: Int64(0),
              modifiedUnixMs: Int64(1728901200000),
            ),
            DirectoryEntry(
              name: 'README.md',
              path: r'G:\repo\README.md',
              sizeBytes: Int64(12288),
              modifiedUnixMs: Int64(1728901200000),
            ),
          ],
        ).writeToBuffer();
      });

      final ListDirectoryResponse response = await bridge.listDirectory();

      expect(seen!.method, 'listDirectory');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空请求 = 空字节数组（编码总则 §2.1，同 listThreads）');
      expect(response.entries, hasLength(2));
      expect(response.entries[0].name, 'docs');
      expect(response.entries[0].isDir, isTrue);
      expect(response.entries[1].sizeBytes.toInt(), 12288,
          reason: 'int64 → fixnum Int64，toInt 后对账');
    });

    test('带 path/include_hidden：request 字段原样过桥，宿主可按 proto 回读',
        () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListDirectoryResponse().writeToBuffer();
      });

      await bridge.listDirectory(
        ListDirectoryRequest(path: r'G:\repo', includeHidden: true),
      );

      final ListDirectoryRequest wire =
          ListDirectoryRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.path, r'G:\repo');
      expect(wire.includeHidden, isTrue);
    });

    test('宿主回空响应字节（0 字节）= 合法空列表，不是错误', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async =>
              ListDirectoryResponse().writeToBuffer());

      final ListDirectoryResponse response = await bridge.listDirectory();

      expect(response.entries, isEmpty);
    });
  });

  group('listDirectory：错误映射', () {
    test('宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method listDirectory');
      });

      await expectLater(
        bridge.listDirectory(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });

    test('宿主 PlatformException → code/message/details 原样透传', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(
          code: 'NOT_CONNECTED',
          message: '还没连上主机',
          details: <String, Object?>{'retryable': true},
        );
      });

      await expectLater(
        bridge.listDirectory(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'NOT_CONNECTED')
            .having((PaBridgeException e) => e.message, 'message', '还没连上主机')),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        bridge.listDirectory(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('Uint8List'))),
      );
    });

    test('坏字节（非法 wire type）→ DECODE_FAILED，不裸抛 protobuf 异常', () async {
      final Uint8List corrupt = Uint8List.fromList([0xFF, 0xFF, 0xFF]);
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => corrupt);

      await expectLater(
        bridge.listDirectory(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('ListDirectoryResponse'))),
      );
    });
  });

  group('readFile：首块 FileChunk 解码', () {
    test('默认（无参）= 空 ReadFileRequest 字节；首块四字段逐项对账', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        // 首块形态对齐 daemon fs.go:106-109：data + total_size + text + mtime
        return FileChunk(
          data: Uint8List.fromList(utf8.encode('hello 世界')),
          totalSize: Int64(20),
          text: true,
          modifiedUnixMs: Int64(1728901200000),
        ).writeToBuffer();
      });

      final FileChunk chunk = await bridge.readFile();

      expect(seen!.method, 'readFile');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空请求 = 空字节数组（编码总则 §2.1）');
      expect(chunk.text, isTrue);
      expect(chunk.totalSize.toInt(), 20);
      expect(chunk.modifiedUnixMs.toInt(), 1728901200000);
      expect(utf8.decode(chunk.data), 'hello 世界',
          reason: 'bytes 字段按 UTF-8 字节往返（UTF-8 字节数 ≠ 字符数）');
    });

    test('带 path/max_bytes：request 字段原样过桥', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return FileChunk().writeToBuffer();
      });

      await bridge.readFile(
        ReadFileRequest(path: r'G:\repo\README.md', maxBytes: Int64(1024)),
      );

      final ReadFileRequest wire =
          ReadFileRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.path, r'G:\repo\README.md');
      expect(wire.maxBytes.toInt(), 1024);
    });

    test('二进制文件（text=false）照样解码：text 标记透传给页面判定', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return FileChunk(
          data: Uint8List.fromList(<int>[0x00, 0x01, 0x02, 0xFF]),
          totalSize: Int64(4),
          text: false,
        ).writeToBuffer();
      });

      final FileChunk chunk = await bridge.readFile();

      expect(chunk.text, isFalse,
          reason: '二进制判定在 daemon 首块（fs.go:107 utf8.Valid），Dart 不改写');
    });

    test('宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method readFile');
      });

      await expectLater(
        bridge.readFile(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });

    test('坏字节 → DECODE_FAILED，消息定位到 FileChunk', () async {
      final Uint8List corrupt = Uint8List.fromList([0xFF, 0xFF, 0xFF]);
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => corrupt);

      await expectLater(
        bridge.readFile(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('FileChunk'))),
      );
    });
  });

  group('listBranches：参数编码与返回解码', () {
    test('默认（无参）= 空 ListBranchesRequest 字节；分支三字段对账', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListBranchesResponse(
          branches: <GitBranch>[
            GitBranch(name: 'main', isCurrent: true, isRemote: false),
            GitBranch(name: 'origin/main', isRemote: true),
            GitBranch(name: 'feature/login', isRemote: false),
          ],
        ).writeToBuffer();
      });

      final ListBranchesResponse response = await bridge.listBranches();

      expect(seen!.method, 'listBranches');
      expect(seen!.arguments, isA<Uint8List>());
      expect(seen!.arguments as Uint8List, isEmpty,
          reason: '空请求 = 空字节数组（编码总则 §2.1）');
      expect(response.branches, hasLength(3));
      expect(response.branches[0].isCurrent, isTrue);
      expect(response.branches[1].isRemote, isTrue,
          reason: 'is_remote 由 daemon 判（git.go:225），Dart 透传');
    });

    test('带 repo：request 字段原样过桥', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return ListBranchesResponse().writeToBuffer();
      });

      await bridge.listBranches(ListBranchesRequest(repo: r'G:\repo'));

      final ListBranchesRequest wire =
          ListBranchesRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.repo, r'G:\repo');
    });

    test('宿主回空响应字节（0 字节）= 合法空分支列表，不是错误', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async =>
              ListBranchesResponse().writeToBuffer());

      final ListBranchesResponse response = await bridge.listBranches();

      expect(response.branches, isEmpty);
    });
  });

  group('listBranches：错误映射', () {
    test('宿主桥未实现（MissingPluginException）→ HOST_UNAVAILABLE 如实上抛',
        () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method listBranches');
      });

      await expectLater(
        bridge.listBranches(),
        throwsA(isA<PaBridgeException>().having(
            (PaBridgeException e) => e.code,
            'code',
            PaBridgeErrorCodes.hostUnavailable)),
      );
    });

    test('宿主 PlatformException（git 失败信息）→ code/message 原样透传', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(
          code: 'INVALID_ARGUMENT',
          message: 'git branch 失败: not a git repository',
        );
      });

      await expectLater(
        bridge.listBranches(ListBranchesRequest(repo: r'G:\nope')),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code', 'INVALID_ARGUMENT')
            .having((PaBridgeException e) => e.message, 'message',
                contains('not a git repository'))),
      );
    });

    test('返回类型不符（String 而非 wire 字节）→ DECODE_FAILED', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => 'not bytes');

      await expectLater(
        bridge.listBranches(),
        throwsA(isA<PaBridgeException>()
            .having((PaBridgeException e) => e.code, 'code',
                PaBridgeErrorCodes.decodeFailed)
            .having((PaBridgeException e) => e.message, 'message',
                contains('Uint8List'))),
      );
    });
  });
}
