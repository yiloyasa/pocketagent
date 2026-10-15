// Files 页测试：三态 + 行渲染 + 面包屑交互 + 预览路径 + 真通道默认接线。
//
// 【三态怎么测】数据源是注入点（FilesDirectorySource），测试用可控源把
// 「还在加载」「拉失败了」「空目录」三种时序摆到台面上——加载态用 Completer
// 卡住，错误态用一次性失败的源，重试后要真变绿。
//
// 【两条必须分家的路径】宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」；
// 宿主正常回空响应 → 空态「这个目录是空的」——各钉一条，互斥断言
// （红线：不留假绿，「通道没就绪」绝不许伪装成「空目录」）。
//
// 【预览怎么测】预览源同样是注入点（FilesPreviewSource）：文本/二进制/超限
// 三条路径各钉一条；默认接线（不注入）用 mock 宿主 wire 行为验 readFile 真链路。

import 'dart:async';
import 'dart:convert';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/fs.pb.dart';
import 'package:pocketagent_ui/pages/files.dart';
import 'package:pocketagent_ui/pages/files_bridge.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

/// 卡住不返回的目录源：把「加载中」钉在屏幕上。
class _PendingSource implements FilesDirectorySource {
  final Completer<List<FileEntry>> completer =
      Completer<List<FileEntry>>();

  @override
  Future<List<FileEntry>> load(String path) => completer.future;
}

/// 第一次必失败、之后成功：验证错误态的「重试」是真的重新拉。
class _RetrySource implements FilesDirectorySource {
  _RetrySource(this.entries);

  final List<FileEntry> entries;
  final List<String> seenPaths = <String>[];
  int calls = 0;

  @override
  Future<List<FileEntry>> load(String path) async {
    seenPaths.add(path);
    calls += 1;
    if (calls == 1) {
      throw const FilesLoadFailure('读目录失败: 系统找不到指定的路径');
    }
    return entries;
  }
}

class _StaticSource implements FilesDirectorySource {
  const _StaticSource(this.entries);

  final List<FileEntry> entries;

  @override
  Future<List<FileEntry>> load(String path) async => entries;
}

/// 记录每次 load 的路径，返回固定条目（静态源的带账本版）。
class _RecordingSource implements FilesDirectorySource {
  _RecordingSource(this.entries, this.seen);

  final List<FileEntry> entries;
  final List<String> seen;

  @override
  Future<List<FileEntry>> load(String path) async {
    seen.add(path);
    return entries;
  }
}

class _FakePreviewSource implements FilesPreviewSource {
  const _FakePreviewSource({this.text, this.failure});

  final String? text;
  final String? failure;

  @override
  Future<FilePreviewContent> readText(FileEntry entry) async {
    if (failure != null) {
      throw FilePreviewFailure(failure!);
    }
    return FilePreviewContent(text: text ?? '', truncated: false);
  }
}

const List<FileEntry> demoEntries = <FileEntry>[
  FileEntry(
    name: 'docs',
    path: r'G:\repo\docs',
    isDir: true,
    sizeBytes: 0,
    modifiedUnixMs: 0,
  ),
  FileEntry(
    name: 'README.md',
    path: r'G:\repo\README.md',
    isDir: false,
    sizeBytes: 12288,
    modifiedUnixMs: 0,
  ),
];

void main() {
  Future<void> pumpFiles(
    WidgetTester tester, {
    FilesDirectorySource? source,
    FilesPreviewSource? previewSource,
    String initialPath = '',
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: FilesPage(
            source: source,
            previewSource: previewSource,
            initialPath: initialPath,
          ),
        ),
      ),
    );
  }

  group('三态', () {
    testWidgets('加载中 → 「正在加载目录…」，数据到了切换成列表', (tester) async {
      final _PendingSource src = _PendingSource();
      await pumpFiles(tester, source: src);
      await tester.pump();

      expect(find.text('正在加载目录…'), findsOneWidget);
      expect(find.text('README.md'), findsNothing);

      src.completer.complete(demoEntries);
      await tester.pumpAndSettle();

      expect(find.text('正在加载目录…'), findsNothing);
      expect(find.text('README.md'), findsOneWidget);
    });

    testWidgets('加载失败 → 错误态带原因；重试后真拉到列表', (tester) async {
      final _RetrySource src = _RetrySource(demoEntries);
      await pumpFiles(tester, source: src);
      await tester.pumpAndSettle();

      expect(find.text('目录加载失败'), findsOneWidget);
      expect(find.textContaining('读目录失败'), findsOneWidget);
      expect(find.text('README.md'), findsNothing);

      await tester.tap(find.text('重试'));
      await tester.pumpAndSettle();

      expect(src.calls, 2, reason: '重试必须真的重新调用数据源');
      expect(find.text('目录加载失败'), findsNothing);
      expect(find.text('README.md'), findsOneWidget);
    });

    testWidgets('空目录 → 「这个目录是空的」（真空态，不是通道错误的伪装）', (tester) async {
      await pumpFiles(tester, source: const _StaticSource(<FileEntry>[]));
      await tester.pumpAndSettle();

      expect(find.text('这个目录是空的'), findsOneWidget);
      expect(find.textContaining('B4 波次'), findsNothing,
          reason: '空态不许出现「未就绪」字样——那是错误路径的词');
    });
  });

  group('行渲染', () {
    testWidgets('目录/文件两型行：图标 + 名 + 元信息（大小/时间）', (tester) async {
      await pumpFiles(tester, source: const _StaticSource(demoEntries));
      await tester.pumpAndSettle();

      // 目录行：只有名字（时间戳缺席 → 无元信息，不显示空段）
      expect(find.text('docs'), findsOneWidget);
      // 文件行：名字 + 体积
      expect(find.text('README.md'), findsOneWidget);
      expect(find.text('12 KB'), findsOneWidget);
      expect(find.byIcon(Icons.folder_rounded), findsOneWidget);
    });

    testWidgets('行带时间戳时显示「大小 · 时间」', (tester) async {
      await pumpFiles(
        tester,
        source: const _StaticSource(<FileEntry>[
          FileEntry(
            name: 'a.txt',
            path: r'G:\repo\a.txt',
            isDir: false,
            sizeBytes: 2048,
            modifiedUnixMs: 1770000000000,
          ),
        ]),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('2 KB'), findsOneWidget);
      expect(find.textContaining('·'), findsOneWidget,
          reason: '大小与时间两段用「 · 」连接');
    });

    testWidgets('搜索只过滤当前目录条目；筛空给「清除搜索」出口', (tester) async {
      await pumpFiles(tester, source: const _StaticSource(demoEntries));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.byKey(const ValueKey<String>('files-search')),
        'readme',
      );
      await tester.pumpAndSettle();
      expect(find.text('README.md'), findsOneWidget);
      expect(find.text('docs'), findsNothing);

      await tester.enterText(
        find.byKey(const ValueKey<String>('files-search')),
        'zzz',
      );
      await tester.pumpAndSettle();
      expect(find.text('当前目录没有匹配的文件'), findsOneWidget);
      expect(find.text('这个目录是空的'), findsNothing);

      await tester.tap(find.text('清除搜索'));
      await tester.pumpAndSettle();
      expect(find.text('README.md'), findsOneWidget);
    });
  });

  group('面包屑', () {
    testWidgets('初始路径 → 分段渲染；点父级段真回跳那一层', (tester) async {
      final _RetrySource src = _RetrySource(demoEntries);
      await pumpFiles(tester, source: src, initialPath: r'G:\repo\docs');
      await tester.pumpAndSettle();

      // 根段 + 盘符 + repo + docs 四段（末段是当前层）
      expect(find.text('默认目录'), findsOneWidget);
      expect(find.text(r'G:\'), findsOneWidget);
      expect(find.text('repo'), findsOneWidget);
      expect(find.text('docs'), findsOneWidget);

      // 点「repo」段 → 数据源被以 G:\repo\ 重新调用
      await tester.tap(find.text('repo'));
      await tester.pumpAndSettle();
      expect(src.seenPaths, contains(r'G:\repo\'));
    });

    testWidgets('点根段（空 target）→ 以空路径重新加载（daemon 回落 workspace）',
        (tester) async {
      final _RetrySource src = _RetrySource(demoEntries);
      await pumpFiles(tester, source: src, initialPath: r'G:\repo');
      await tester.pumpAndSettle();

      await tester.tap(find.text('默认目录'));
      await tester.pumpAndSettle();
      expect(src.seenPaths, contains(''));
    });
  });

  group('预览路径', () {
    testWidgets('文本文件点开 → 显示 readFile 内容', (tester) async {
      await pumpFiles(
        tester,
        source: const _StaticSource(demoEntries),
        previewSource: const _FakePreviewSource(text: '# 标题\n正文'),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('README.md'));
      await tester.pumpAndSettle();

      expect(find.text('# 标题\n正文'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('files-preview-back')), findsOneWidget);
    });

    testWidgets('二进制文件 → 诚实提示「不能按文本预览」，不摆乱码', (tester) async {
      await pumpFiles(
        tester,
        source: const _StaticSource(demoEntries),
        previewSource: const _FakePreviewSource(failure: '这是二进制文件，不能按文本预览。'),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('README.md'));
      await tester.pumpAndSettle();

      expect(find.text('无法预览这个文件'), findsOneWidget);
      expect(find.textContaining('二进制'), findsOneWidget);
    });

    testWidgets('超大文件 → 进预览层前就被大小闸拒（不发起 readFile）', (tester) async {
      await pumpFiles(
        tester,
        source: const _StaticSource(<FileEntry>[
          FileEntry(
            name: 'big.log',
            path: r'G:\repo\big.log',
            isDir: false,
            sizeBytes: kFilePreviewMaxBytes + 1,
            modifiedUnixMs: 0,
          ),
        ]),
        previewSource: const _FakePreviewSource(text: '不该被读到'),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('big.log'));
      await tester.pumpAndSettle();

      expect(find.text('无法预览这个文件'), findsOneWidget);
      expect(find.textContaining('超出文本预览上限'), findsOneWidget);
      expect(find.text('不该被读到'), findsNothing,
          reason: '超限文件根本不发起 readFile——大小闸在调用前');
    });

    testWidgets('目录点开 → 切到那个目录（不是预览）', (tester) async {
      // 静态源：任何路径都回同一份 demoEntries，验的是「路径被切过去」。
      final List<String> seenPaths = <String>[];
      final FilesDirectorySource src = _RecordingSource(demoEntries, seenPaths);
      await pumpFiles(tester, source: src);
      await tester.pumpAndSettle();

      await tester.tap(find.text('docs'));
      await tester.pumpAndSettle();

      expect(seenPaths, contains(r'G:\repo\docs'));
      expect(find.byKey(const ValueKey<String>('files-preview-back')), findsNothing);
    });
  });

  group('默认接线：真通道（不注入 source）', () {
    const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
    late TestDefaultBinaryMessenger messenger;

    setUp(() {
      messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    });

    DirectoryEntry protoEntry(String name, String path,
            {bool isDir = false, int size = 0}) =>
        DirectoryEntry(
          name: name,
          path: path,
          isDir: isDir,
          sizeBytes: Int64(size),
        );

    testWidgets('宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」，绝不是空态',
        (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });

      await pumpFiles(tester);
      await tester.pumpAndSettle();

      expect(find.text('目录加载失败'), findsOneWidget);
      expect(find.textContaining('未就绪（B4 波次）'), findsOneWidget);
      expect(find.text('这个目录是空的'), findsNothing,
          reason: '通道没就绪不许走空态——那是「真空目录」的伪装版（红线：不留假绿）');
      expect(find.text('重试'), findsOneWidget, reason: '错误态给重试出口');

      // 重试必须真的重新发真调用：把宿主「装上」后重试应转绿。
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return ListDirectoryResponse(
          entries: <DirectoryEntry>[protoEntry('README.md', r'G:\repo\README.md', size: 12)],
        ).writeToBuffer();
      });
      await tester.tap(find.text('重试'));
      await tester.pumpAndSettle();

      expect(find.text('目录加载失败'), findsNothing);
      expect(find.text('README.md'), findsOneWidget);
    });

    testWidgets('宿主正常回空响应 → 空态「这个目录是空的」（与错误态分家的另一条）',
        (tester) async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async =>
              ListDirectoryResponse().writeToBuffer());

      await pumpFiles(tester);
      await tester.pumpAndSettle();

      expect(find.text('这个目录是空的'), findsOneWidget);
      expect(find.text('目录加载失败'), findsNothing);
      expect(find.textContaining('未就绪'), findsNothing,
          reason: '空态不许出现错误路径的词——两条路径断言必须互相排斥');
    });

    testWidgets('宿主正常回目录 → 行渲染（目录在前 + 体积）', (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return ListDirectoryResponse(
          entries: <DirectoryEntry>[
            protoEntry('z.txt', r'G:\repo\z.txt', size: 2048),
            protoEntry('docs', r'G:\repo\docs', isDir: true),
          ],
        ).writeToBuffer();
      });

      await pumpFiles(tester);
      await tester.pumpAndSettle();

      // 目录恒在前（sortFileEntries）：docs 行的 element 在 z.txt 行之前
      expect(find.text('docs'), findsOneWidget);
      expect(find.text('z.txt'), findsOneWidget);
      expect(find.text('2 KB'), findsOneWidget);
    });

    testWidgets('预览走真通道：宿主回文本 FileChunk → 显示正文', (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'listDirectory') {
          return ListDirectoryResponse(
            entries: <DirectoryEntry>[
              protoEntry('hello.txt', r'G:\repo\hello.txt', size: 12),
            ],
          ).writeToBuffer();
        }
        // readFile：首块 FileChunk（daemon fs.go:106-109 形态）
        return FileChunk(
          data: Uint8List.fromList(utf8.encode('你好')),
          totalSize: Int64(6),
          text: true,
        ).writeToBuffer();
      });

      await pumpFiles(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.text('hello.txt'));
      await tester.pumpAndSettle();

      expect(find.text('你好'), findsOneWidget);
    });
  });
}
