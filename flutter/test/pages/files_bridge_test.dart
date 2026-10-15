// Files 纯函数 + BridgeDirectorySource/BridgeFilePreviewSource 测试（B3 第 3 波）。
//
// 【两层各自的证词】
//   ① 纯函数（排序 / 体积 / 时间 / 面包屑 / 上级目录 / 预览闸门）不拖通道，
//      逐条钉死「行上的字从 wire 哪个字段来 + 什么规则」；
//   ② 数据源走真 PaBridge + mock 宿主 wire 行为：两条**必须分家**的路径——
//      HOST_UNAVAILABLE → 人话错误（点名 B4 波次）vs 空响应 → 真空列表——
//      各钉一条（红线：不留假绿，「通道没就绪」绝不许伪装成「空目录」）；
//      预览路径再钉二进制拒绝与超限拒绝（诚实提示，不摆假预览）。
//
// 【口径来源】Android FilesListLogic.kt（排序/时间/体积）与 component/PathNav.kt
// （面包屑/上级目录）逐条对齐，双端同一目录长同一样；体积格式对齐全工程统一
// 口径 PaFormat.formatBytesLabel（不是 FilesListLogic 的旧兼容壳，见
// humanFileSize 注释）。

import 'dart:convert';

import 'package:fixnum/fixnum.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/fs.pb.dart';
import 'package:pocketagent_ui/pages/files_bridge.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  late TestDefaultBinaryMessenger messenger;

  setUp(() {
    messenger =
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  });

  FileEntry entry({
    required String name,
    String path = '',
    bool isDir = false,
    int sizeBytes = 0,
    int modifiedUnixMs = 0,
  }) =>
      FileEntry(
        name: name,
        path: path.isEmpty ? name : path,
        isDir: isDir,
        sizeBytes: sizeBytes,
        modifiedUnixMs: modifiedUnixMs,
      );

  DirectoryEntry protoEntry({
    required String name,
    required String path,
    bool isDir = false,
    int sizeBytes = 0,
    int modifiedUnixMs = 0,
  }) =>
      DirectoryEntry(
        name: name,
        path: path,
        isDir: isDir,
        sizeBytes: Int64(sizeBytes),
        modifiedUnixMs: Int64(modifiedUnixMs),
      );

  group('fileEntryOf / fileEntriesOfResponse：wire → 行模型', () {
    test('五字段逐字段对应（int64 经 toInt 收口）', () {
      final FileEntry e = fileEntryOf(protoEntry(
        name: 'README.md',
        path: r'G:\repo\README.md',
        sizeBytes: 12288,
        modifiedUnixMs: 1728901200000,
      ));

      expect(e.name, 'README.md');
      expect(e.path, r'G:\repo\README.md');
      expect(e.isDir, isFalse);
      expect(e.sizeBytes, 12288);
      expect(e.modifiedUnixMs, 1728901200000);
    });

    test('响应条目序保持 daemon 原序（排序是 sortFileEntries 的职责）', () {
      final List<FileEntry> rows = fileEntriesOfResponse(
        ListDirectoryResponse(entries: <DirectoryEntry>[
          protoEntry(name: 'zeta', path: 'zeta'),
          protoEntry(name: 'alpha', path: 'alpha'),
        ]),
      );

      expect(rows.map((FileEntry e) => e.name).toList(), <String>['zeta', 'alpha']);
    });
  });

  group('sortFileEntries：目录在前 + 名字不分大小写', () {
    test('目录恒在前；同组内按名字升序', () {
      final List<FileEntry> sorted = sortFileEntries(<FileEntry>[
        entry(name: 'b.txt'),
        entry(name: 'docs', isDir: true),
        entry(name: 'A.txt'),
        entry(name: 'android', isDir: true),
      ]);

      expect(
        sorted.map((FileEntry e) => e.name).toList(),
        <String>['android', 'docs', 'A.txt', 'b.txt'],
        reason: '目录组在前，随后文件组按名字不分大小写',
      );
    });

    test('同名再按完整路径稳定排序（不修改入参）', () {
      final List<FileEntry> input = <FileEntry>[
        entry(name: 'same', path: 'z/same'),
        entry(name: 'same', path: 'a/same'),
      ];
      final List<FileEntry> sorted = sortFileEntries(input);

      expect(sorted.map((FileEntry e) => e.path).toList(),
          <String>['a/same', 'z/same']);
      expect(input.first.path, 'z/same', reason: '纯函数，入参不被修改');
    });
  });

  group('humanFileSize：体积四档（口径 = Android formatBytesLabel）', () {
    test('B / KB / MB / GB 档位与小数规则', () {
      expect(humanFileSize(0), '0 B');
      expect(humanFileSize(512), '512 B');
      expect(humanFileSize(1024), '1 KB', reason: '整数档不补 ".0"');
      expect(humanFileSize(1536), '1.5 KB');
      expect(humanFileSize(1048576), '1 MB');
      expect(humanFileSize(1572864), '1.5 MB');
      expect(humanFileSize(1073741824), '1 GB');
      expect(humanFileSize(1610612736), '1.5 GB');
    });
  });

  group('fileEntryTimeLabel：时间档位（口径 = Android formatEntryTime）', () {
    test('同年 → MM-dd HH:mm；跨年 → yyyy-MM-dd；缺席 → 空串', () {
      const int nowYear = 2026;
      final int sameYear =
          DateTime(2026, 10, 14, 9, 5).millisecondsSinceEpoch;
      final int otherYear =
          DateTime(2025, 1, 2, 3, 4).millisecondsSinceEpoch;

      expect(fileEntryTimeLabel(sameYear, nowYear: nowYear), '10-14 09:05');
      expect(fileEntryTimeLabel(otherYear, nowYear: nowYear), '2025-01-02');
      expect(fileEntryTimeLabel(0, nowYear: nowYear), '',
          reason: '时间戳缺席不显示假时间');
      expect(fileEntryTimeLabel(-1, nowYear: nowYear), '');
    });
  });

  group('fileEntryMetaLine：行副文案', () {
    test('文件 = 大小 · 时间；目录只有时间（不编目录大小）', () {
      const int nowYear = 2026;
      final int ts = DateTime(2026, 3, 8, 12, 0).millisecondsSinceEpoch;

      expect(
        fileEntryMetaLine(entry(name: 'a.txt', sizeBytes: 2048, modifiedUnixMs: ts),
            nowYear: nowYear),
        '2 KB · 03-08 12:00',
      );
      expect(
        fileEntryMetaLine(entry(name: 'docs', isDir: true, modifiedUnixMs: ts),
            nowYear: nowYear),
        '03-08 12:00',
        reason: 'daemon 没给目录大小（size_bytes 恒 0），不显示 "0 B"',
      );
    });

    test('时间缺席 → 文件只剩大小，不出现空段', () {
      expect(
        fileEntryMetaLine(entry(name: 'a.txt', sizeBytes: 10), nowYear: 2026),
        '10 B',
      );
    });
  });

  group('breadcrumbSegmentsOf：面包屑分段（口径 = Android pathBreadcrumbs）', () {
    test('空路径 = 只有根段「默认目录」', () {
      final List<PathCrumb> crumbs = breadcrumbSegmentsOf('');

      expect(crumbs, hasLength(1));
      expect(crumbs.single.label, '默认目录');
      expect(crumbs.single.target, isEmpty);
    });

    test('Windows 深路径：根段 + 盘符 + 各级目录，末段 target = 原样路径', () {
      final List<PathCrumb> crumbs = breadcrumbSegmentsOf(r'G:\a\b');

      expect(crumbs.map((PathCrumb c) => c.label).toList(),
          <String>['默认目录', r'G:\', 'a', 'b']);
      expect(crumbs.map((PathCrumb c) => c.target).toList(),
          <String>['', r'G:\', r'G:\a\', r'G:\a\b']);
    });

    test('Windows 盘符根（G:\\）= 根段 + 盘符自身', () {
      final List<PathCrumb> crumbs = breadcrumbSegmentsOf(r'G:\');

      expect(crumbs.map((PathCrumb c) => c.label).toList(),
          <String>['默认目录', r'G:\']);
      expect(crumbs.last.target, r'G:\');
    });

    test('Unix 深路径与根', () {
      expect(
        breadcrumbSegmentsOf('/home/u')
            .map((PathCrumb c) => (c.label, c.target))
            .toList(),
        <(String, String)>[
          ('默认目录', ''),
          ('/', '/'),
          ('home', '/home/'),
          ('u', '/home/u'),
        ],
      );
      expect(
        breadcrumbSegmentsOf('/')
            .map((PathCrumb c) => (c.label, c.target))
            .toList(),
        <(String, String)>[
          ('默认目录', ''),
          ('/', '/'),
        ],
        reason: '路径只剩分隔符本身：当前层就是它自己，不被根段吞掉',
      );
    });
  });

  group('parentDirectoryOf：上级目录（口径 = Android parentDirectoryOf）', () {
    test('Windows：逐级上退，盘符根返回 null（到顶）', () {
      expect(parentDirectoryOf(r'G:\a\b'), r'G:\a\');
      expect(parentDirectoryOf(r'G:\a'), r'G:\');
      expect(parentDirectoryOf(r'G:\'), isNull,
          reason: '已在最顶层：调用方应回根段（默认目录），不原地重载');
    });

    test('Unix：逐级上退，根返回 null', () {
      expect(parentDirectoryOf('/home/u'), '/home/');
      expect(parentDirectoryOf('/home'), '/');
      expect(parentDirectoryOf('/'), isNull);
    });
  });

  group('directoryTitleOf：目录显示名', () {
    test('取最后一段；空路径 = 默认目录', () {
      expect(directoryTitleOf(r'G:\a\b'), 'b');
      expect(directoryTitleOf('/home/u'), 'u');
      expect(directoryTitleOf(''), '默认目录');
    });
  });

  group('预览闸门（filePreviewable / filePreviewRejection）', () {
    test('目录不可预览、小文本可预览', () {
      expect(filePreviewable(entry(name: 'docs', isDir: true)), isFalse);
      expect(filePreviewRejection(entry(name: 'docs', isDir: true)), isEmpty,
          reason: '目录的点按是进目录，没有「拒绝预览」这回事');
      expect(filePreviewable(entry(name: 'a.txt', sizeBytes: 1024)), isTrue);
      expect(filePreviewRejection(entry(name: 'a.txt', sizeBytes: 1024)), isEmpty);
    });

    test('超出上限 → 如实拒绝并说明上限（不摆假预览）', () {
      final FileEntry big =
          entry(name: 'big.log', sizeBytes: kFilePreviewMaxBytes + 1);

      expect(filePreviewable(big), isFalse);
      expect(
        filePreviewRejection(big),
        contains('超出文本预览上限'),
        reason: '拒绝原因要点名上限，用户才知道是「太大」不是「坏了」',
      );
    });

    test('恰好等于上限 → 仍可预览（<= 语义）', () {
      expect(
        filePreviewable(entry(name: 'a.txt', sizeBytes: kFilePreviewMaxBytes)),
        isTrue,
      );
    });
  });

  group('BridgeDirectorySource：真通道 + mock 宿主 wire', () {
    Uint8List dirBytes(List<DirectoryEntry> entries) =>
        ListDirectoryResponse(entries: entries).writeToBuffer();

    test('成功 → 行模型（已按目录在前排序）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return dirBytes(<DirectoryEntry>[
          protoEntry(name: 'b.txt', path: r'G:\repo\b.txt', sizeBytes: 10),
          protoEntry(name: 'docs', path: r'G:\repo\docs', isDir: true),
          protoEntry(name: 'a.txt', path: r'G:\repo\a.txt', sizeBytes: 20),
        ]);
      });

      final BridgeDirectorySource source = BridgeDirectorySource();
      final List<FileEntry> rows = await source.load(r'G:\repo');

      expect(rows.map((FileEntry e) => e.name).toList(),
          <String>['docs', 'a.txt', 'b.txt']);
    });

    test('宿主桥未实现 → HOST_UNAVAILABLE → FilesLoadFailure 点名 B4 波次', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('No implementation found for method listDirectory');
      });

      final BridgeDirectorySource source = BridgeDirectorySource();

      await expectLater(
        source.load(r'G:\repo'),
        throwsA(isA<FilesLoadFailure>().having(
          (FilesLoadFailure e) => e.message,
          'message',
          allOf(contains('桌面端文件通道未就绪'), contains('B4 波次')),
        )),
      );
    });

    test('宿主正常回空响应 → 真空列表（与错误路径分家的另一条）', () async {
      messenger.setMockMethodCallHandler(
          methodChannel, (MethodCall call) async => dirBytes(<DirectoryEntry>[]));

      final BridgeDirectorySource source = BridgeDirectorySource();
      final List<FileEntry> rows = await source.load(r'G:\repo');

      expect(rows, isEmpty,
          reason: '空目录是合法事实，但必须走返回值而不是异常——两条路径分家');
    });

    test('宿主 PlatformException → message 原样透传（不改写不吞）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw PlatformException(
          code: 'INVALID_ARGUMENT',
          message: '读目录失败: 系统找不到指定的路径',
        );
      });

      final BridgeDirectorySource source = BridgeDirectorySource();

      await expectLater(
        source.load(r'G:\nope'),
        throwsA(isA<FilesLoadFailure>().having(
          (FilesLoadFailure e) => e.message,
          'message',
          contains('读目录失败'),
        )),
      );
    });
  });

  group('BridgeFilePreviewSource：readFile 首块 → 文本 / 拒绝', () {
    test('文本文件 → UTF-8 正文 + 未截断', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return FileChunk(
          data: Uint8List.fromList(utf8.encode('你好，PocketAgent')),
          totalSize: Int64(utf8.encode('你好，PocketAgent').length),
          text: true,
        ).writeToBuffer();
      });

      final BridgeFilePreviewSource source = BridgeFilePreviewSource();
      final FilePreviewContent content =
          await source.readText(entry(name: 'a.txt', sizeBytes: 1024));

      expect(content.text, '你好，PocketAgent');
      expect(content.truncated, isFalse);
    });

    test('二进制文件（text=false）→ 诚实拒绝，不把乱码当正文', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return FileChunk(
          data: Uint8List.fromList(<int>[0x00, 0xFF, 0xFE]),
          totalSize: Int64(3),
          text: false,
        ).writeToBuffer();
      });

      final BridgeFilePreviewSource source = BridgeFilePreviewSource();

      await expectLater(
        source.readText(entry(name: 'a.bin', sizeBytes: 3)),
        throwsA(isA<FilePreviewFailure>().having(
          (FilePreviewFailure e) => e.message,
          'message',
          contains('二进制'),
        )),
      );
    });

    test('首块短于 total_size → truncated=true（不把半份文件画成完整文件）', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        return FileChunk(
          data: Uint8List.fromList(utf8.encode('半份')),
          totalSize: Int64(4096),
          text: true,
        ).writeToBuffer();
      });

      final BridgeFilePreviewSource source = BridgeFilePreviewSource();
      final FilePreviewContent content =
          await source.readText(entry(name: 'a.txt', sizeBytes: 4096));

      expect(content.text, '半份');
      expect(content.truncated, isTrue);
    });

    test('宿主桥未实现 → HOST_UNAVAILABLE → FilePreviewFailure 点名 B4 波次', () async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException('No implementation found for method readFile');
      });

      final BridgeFilePreviewSource source = BridgeFilePreviewSource();

      await expectLater(
        source.readText(entry(name: 'a.txt', sizeBytes: 1024)),
        throwsA(isA<FilePreviewFailure>().having(
          (FilePreviewFailure e) => e.message,
          'message',
          allOf(contains('桌面端文件通道未就绪'), contains('B4 波次')),
        )),
      );
    });

    test('请求带 max_bytes = 预览上限（不读超大文件）', () async {
      MethodCall? seen;
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        seen = call;
        return FileChunk(text: true).writeToBuffer();
      });

      final BridgeFilePreviewSource source = BridgeFilePreviewSource();
      await source.readText(entry(name: 'a.txt', path: r'G:\repo\a.txt', sizeBytes: 1024));

      final ReadFileRequest wire =
          ReadFileRequest.fromBuffer(seen!.arguments as Uint8List);
      expect(wire.path, r'G:\repo\a.txt');
      expect(wire.maxBytes.toInt(), kFilePreviewMaxBytes);
    });
  });
}
