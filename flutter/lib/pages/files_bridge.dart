// Files 文件树的模型 + 面包屑纯函数 + 真通道数据源（B3 第 3 波）。
//
// 【为什么单独一个文件】与 B3-3 的 threads_bridge.dart 同一分工：
// ①「列表行的字段各自从 wire 的哪个字段来 + 排序/体积/时间怎么格式化」是纯函数，
//    可被单测逐条钉死；②「真通道怎么包」= [BridgeDirectorySource]。
// files.dart 只留界面（三态 + 行渲染 + 面包屑）。三层各自可测，回归时分得清是
// 映射错、通道错还是渲染错。
//
// 【诚实红线（B4 前）】Kotlin 宿主桥 B4 波次才注册 `dev.pocketagent/bridge`
// （channel-contract.md §6）；在此之前 listDirectory/readFile 必然抛
// HOST_UNAVAILABLE。这里把它如实翻成 [FilesLoadFailure]（文案点名「桌面端文件
// 通道未就绪（B4 波次）」），页面进错误态。**绝不返回空列表冒充「空目录」**：
// 空态与错误态是两条路径，test/pages/files_bridge_test.dart 两条都钉死
// （红线：不留假绿）。
//
// 【写操作为什么不做】proto 有 createDirectory/deleteEntry/moveEntry/writeFile
// （重命名=move），Android FilesScreen 也全接了；但「手机端写电脑文件」是产品
// 红线，本波只读（列目录 + 读文本预览）。页面因此不摆新建/删除/重命名入口——
// 没有就是没有，不摆假按钮。

import 'dart:convert';

import 'package:fixnum/fixnum.dart';

import '../gen/pocketagent/v1/fs.pb.dart';
import '../transport/pa_bridge.dart';

// ── 行模型 ────────────────────────────────────────────────────────────────

/// 目录/文件一行 —— proto `DirectoryEntry` 的 Dart 投影（五字段一一对应）。
///
/// 【为什么不直接用 proto 消息当行模型】页面要的是「排序后、格式化好」的行，
/// 这些规则（目录在前、体积人话、时间档位）必须是纯函数才能单测；把 proto 消息
/// 直接拖进 widget，规则就只能靠肉眼在真机上对（Android 把同一批规则抽成
/// FilesListLogic.kt 正是这个原因）。
class FileEntry {
  const FileEntry({
    required this.name,
    required this.path,
    required this.isDir,
    required this.sizeBytes,
    required this.modifiedUnixMs,
  });

  final String name;
  final String path;
  final bool isDir;
  final int sizeBytes;
  final int modifiedUnixMs;

  @override
  bool operator ==(Object other) =>
      other is FileEntry &&
      other.name == name &&
      other.path == path &&
      other.isDir == isDir &&
      other.sizeBytes == sizeBytes &&
      other.modifiedUnixMs == modifiedUnixMs;

  @override
  int get hashCode =>
      Object.hash(name, path, isDir, sizeBytes, modifiedUnixMs);
}

/// proto `DirectoryEntry` → [FileEntry]（五字段逐字段对应，无加工）。
FileEntry fileEntryOf(DirectoryEntry e) => FileEntry(
      name: e.name,
      path: e.path,
      isDir: e.isDir,
      sizeBytes: e.sizeBytes.toInt(),
      modifiedUnixMs: e.modifiedUnixMs.toInt(),
    );

/// `ListDirectoryResponse` → 行列表（顺序保持 daemon 原序，不重排；
/// 排序是 [sortFileEntries] 的职责，两步分开才能各自单测）。
List<FileEntry> fileEntriesOfResponse(ListDirectoryResponse response) =>
    response.entries.map(fileEntryOf).toList(growable: false);

// ── 列表排序（口径 = Android sortFileRows，FilesListLogic.kt:39）────────────

/// 排序：目录恒在前；同组内按名字不分大小写升序，同名再按完整路径稳定排序。
///
/// 【为什么是稳定排序】daemon 的 os.ReadDir 本身按名字排序，这里再排一次是
/// 为了「目录在前」这条 Android 双端一致的阅读次序；同键时保持原相对顺序，
/// 不让 UI 自己发明第三套次序。纯函数，不修改入参。
List<FileEntry> sortFileEntries(List<FileEntry> entries) {
  final List<FileEntry> sorted = List<FileEntry>.of(entries);
  sorted.sort((FileEntry a, FileEntry b) {
    if (a.isDir != b.isDir) return a.isDir ? -1 : 1;
    final int byName = a.name.toLowerCase().compareTo(b.name.toLowerCase());
    if (byName != 0) return byName;
    return a.path.compareTo(b.path);
  });
  return sorted;
}

// ── 行元信息格式化 ────────────────────────────────────────────────────────

/// 人类可读体积：B / KB / MB / GB 四档（口径 = Android formatBytesLabel，
/// PaFormat.kt:47——**不用** FilesListLogic.formatFileSizeHuman 的旧格式，
/// 那是 Android 自己标注了「历史差异待主模型决策」的兼容壳；Flutter 侧没有历史
/// 包袱，直接对齐全工程统一口径）。整数档不补 ".0"（"1 KB" 而非 "1.0 KB"）。
String humanFileSize(int bytes) {
  final int abs = bytes < 0 ? -bytes : bytes;
  if (abs >= 1 << 30) return '${_oneDecimal(bytes / (1 << 30))} GB';
  if (abs >= 1 << 20) return '${_oneDecimal(bytes / (1 << 20))} MB';
  if (abs >= 1 << 10) return '${_oneDecimal(bytes / (1 << 10))} KB';
  return '$bytes B';
}

String _oneDecimal(double v) {
  if (v == v.roundToDouble()) return v.round().toString();
  return v.toStringAsFixed(1);
}

/// 条目时间标签：同一年 → "MM-dd HH:mm"，跨年 → "yyyy-MM-dd"；无时间戳返回
/// 空串（不显示假时间）。口径 = Android formatEntryTime（FilesListLogic.kt:95）。
///
/// [nowYear] 显式注入（生产传当前年）：否则「跨年」判定不可单测，测试跑在不同
/// 时钟上必飘。
String fileEntryTimeLabel(int unixMs, {required int nowYear}) {
  if (unixMs <= 0) return '';
  final DateTime t = DateTime.fromMillisecondsSinceEpoch(unixMs);
  final String mm = t.month.toString().padLeft(2, '0');
  final String dd = t.day.toString().padLeft(2, '0');
  final String hh = t.hour.toString().padLeft(2, '0');
  final String mi = t.minute.toString().padLeft(2, '0');
  if (t.year == nowYear) return '$mm-$dd $hh:$mi';
  return '${t.year}-$mm-$dd';
}

/// 行副文案（纯函数）：
///   - 目录：只有时间（daemon 没给目录大小，不编数字——Android entryMetaLine
///     同款纪律，`size_bytes` 对目录恒 0 也不显示 "0 B"）；
///   - 文件：`大小 · 时间`，缺项自动省略（不出现空段/占位）。
String fileEntryMetaLine(FileEntry e, {required int nowYear}) {
  final List<String> parts = <String>[];
  if (!e.isDir) parts.add(humanFileSize(e.sizeBytes));
  final String time = fileEntryTimeLabel(e.modifiedUnixMs, nowYear: nowYear);
  if (time.isNotEmpty) parts.add(time);
  return parts.join(' · ');
}

// ── 面包屑（口径 = Android pathBreadcrumbs，PathNav.kt:58）────────────────

/// 面包屑的一段：[label] 是显示文本，[target] 是点击后要加载的目录路径。
///
/// [target] 为空串 = 回「默认目录」（daemon 回落 workspace 的那一层，见
/// [kDefaultDirTarget]）。父级段一律带尾分隔符（可直接喂给 ListDirectory）；
/// 末段（当前目录）的 target 就是传入的原样路径，页面不响应它的点击
/// （点了等于原地重载）。
class PathCrumb {
  const PathCrumb({required this.label, required this.target});

  final String label;
  final String target;
}

/// 面包屑根段文案。
///
/// 【为什么不叫「此电脑」】Android 的根段是盘符列表（ListRoots），本波 transport
/// 只增 listDirectory/readFile/listBranches 三个方法面，**没有 listRoots**——
/// 叫「此电脑」却点不出盘符列表就是假承诺。daemon 对空 path 的语义是回落
/// workspace（fs.go:41-43），所以根段如实叫「默认目录」。
const String kDefaultDirLabel = '默认目录';

/// 根段跳转目标：空串 = daemon 回落 workspace（fs.proto/daemon fs.go:41-43）。
const String kDefaultDirTarget = '';

/// 面包屑分段（纯函数）：把完整路径切成可点级跳转的段列表。
///
/// ```
/// G:\a\b   -> [默认目录(""), G:\("G:\"), a("G:\"), b("G:\a\b")]
/// /home/u  -> [默认目录(""), /("/"), home("/home/"), u("/home/u")]
/// G:\      -> [默认目录(""), G:\("G:\")]
/// ""       -> [默认目录("")]
/// ```
/// 分隔符按路径里实际出现的取（Windows `\` / Unix `/`），不把 Unix 路径的 "/"
/// 错换成 "\"；路径只剩分隔符本身（Unix 根 "/"）时当前层就是它自己，不能被
/// 根段吞掉。产出顺序恒为根在前、当前层在后，UI 顺序渲染即可。
List<PathCrumb> breadcrumbSegmentsOf(String path) {
  final List<PathCrumb> out = <PathCrumb>[
    const PathCrumb(label: kDefaultDirLabel, target: kDefaultDirTarget),
  ];
  if (path.isEmpty) return out;
  final String sep = path.contains(r'\') ? r'\' : '/';
  final List<String> parts =
      path.split(sep).where((String p) => p.isNotEmpty).toList();
  if (parts.isEmpty) {
    // 路径只剩分隔符本身（Unix 根 "/"）：当前层就是它自己
    out.add(PathCrumb(label: sep, target: path));
    return out;
  }
  if (sep == '\\') {
    // Windows：第一段是盘符（"G:"），展示成 "G:\"
    out.add(PathCrumb(label: '${parts[0]}$sep', target: '${parts[0]}$sep'));
    String acc = '${parts[0]}$sep';
    for (int i = 1; i < parts.length; i++) {
      acc += '${parts[i]}$sep';
      out.add(PathCrumb(label: parts[i], target: acc));
    }
  } else {
    out.add(const PathCrumb(label: '/', target: '/'));
    String acc = '/';
    for (final String part in parts) {
      acc += '$part$sep';
      out.add(PathCrumb(label: part, target: acc));
    }
  }
  // 末段 = 当前目录：target 保持传入原样，点它不走重载
  out[out.length - 1] = PathCrumb(label: out[out.length - 1].label, target: path);
  return out;
}

/// 上级目录（纯函数）：`G:\a\b -> G:\a\`、`G:\a -> G:\`、`G:\ -> null`（已到顶，
/// 调用方应改为显示盘符列表）；Unix 同规则。口径 = Android parentDirectoryOf
/// （PathNav.kt:20——历史 bug：盘符直下一级的 lastIndexOf 恰好是 2 被特判成
/// "拼回自己"，返回键原地重载；这里 idx>=2 才切，与修复后同规则）。
String? parentDirectoryOf(String path) {
  if (path.isEmpty) return null;
  final String sep = path.contains(r'\') ? r'\' : '/';
  final List<String> parts =
      path.split(sep).where((String p) => p.isNotEmpty).toList();
  // Windows 的 "G:" 本身就是两级，所以 parts>=2 才能切；Unix 的 "/" 切出空
  // parts（已在上面 return），"home" 一级即可切出 "/"。
  final int minLead = sep == r'\' ? 2 : 1;
  if (parts.length < minLead) return null;
  final String parent = parts.sublist(0, parts.length - 1).join(sep);
  // Windows 盘符父级要带尾分隔符（"G:" -> "G:\"）；Unix 根父级就是 "/"
  if (sep == r'\') return parent + sep;
  return parent.isEmpty ? '/' : '/$parent/';
}

/// 目录显示名（面包屑末段/顶栏标题用）：取最后一段；盘符根（`G:\`）显示
/// "G:\" 自身，Unix 根显示 "/"。空路径 = daemon 回落 workspace 层，显示
/// [kDefaultDirLabel]。
String directoryTitleOf(String path) {
  if (path.isEmpty) return kDefaultDirLabel;
  final String sep = path.contains(r'\') ? r'\' : '/';
  final List<String> parts =
      path.split(sep).where((String p) => p.isNotEmpty).toList();
  if (parts.isEmpty) return sep;
  return parts.last;
}

// ── 文件预览（readFile 的判定与拒绝）─────────────────────────────────────

/// 预览上限（字节）：超出即拒绝预览并如实说明。
///
/// 【为什么是这个数】Android FilesScreen 的纯文本预览同样 ≤256KB（文件头注释
/// "文件点开为可滚动文本预览(≤256KB)"）；再大屏幕也渲染不动，且 Proto 的
/// ReadFile 是分块流，首块 64KB——只预览首块会把"大文件"画成"截断的文件"，
/// 那是假象，不如明说。
const int kFilePreviewMaxBytes = 256 * 1024;

/// 能否走 readFile 文本预览（纯函数）：三条件全真才预览——
///   ① 不是目录（目录没有 readFile 语义）；
///   ② 文件大小 ≤ [kFilePreviewMaxBytes]（超出 → 诚实提示不支持）；
///   ③ daemon 在首块 FileChunk 里回的 `text` 标记为真（UTF-8 可直显，
///      fs.go:107 的 `first && utf8.Valid(chunk)`；二进制 → 诚实提示不支持）。
///
/// ③ 的判定发生在读到首块之后，函数只管前两条（调用方拿到 FileChunk 后按
/// `chunk.text` 再判一次）；两处都拒绝才显示提示，任何一处放行都不显示。
bool filePreviewable(FileEntry e) => !e.isDir && e.sizeBytes <= kFilePreviewMaxBytes;

/// 预览被拒的原因（纯函数，页面据此显示诚实提示）；可预览返回空串。
///
/// 【为什么大小用「文件真实大小」而不是 chunk.total_size】DirectoryEntry 的
/// size_bytes 就是 daemon 报的文件大小（fs.go:60），预览前就有，不必先读一次
/// 文件才知道能不能预览。
String filePreviewRejection(FileEntry e) {
  if (e.isDir) return '';
  if (e.sizeBytes > kFilePreviewMaxBytes) {
    return '${humanFileSize(e.sizeBytes)} 超出文本预览上限'
        '（${humanFileSize(kFilePreviewMaxBytes)}）：本波只支持小文本文件预览。';
  }
  return '';
}

// ── 错误文案 ──────────────────────────────────────────────────────────────

/// 错误码 → 页面人话文案（页面错误态直接展示这一条）。
///
/// 【只在 HOST_UNAVAILABLE 定制】B4 前这是常态：点名「文件通道未就绪（B4 波次）」
/// 让用户知道不是自己操作错了，也不是「空目录」。其余码原样透传宿主 message
/// （不改写、不吞——与 settings/threads 页同一套透传纪律）；宿主没给 message 时
/// 退 code，宁可难看也不吞。
String filesLoadErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端文件通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '文件列表暂不可用。';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}

/// 预览失败文案（readFile 路径）：与列表同款的 HOST_UNAVAILABLE 定制 + 透传。
String filePreviewErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端文件通道未就绪（B4 波次）：文件预览暂不可用。';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}

// ── 真通道数据源 ──────────────────────────────────────────────────────────

/// 目录加载失败：只带人话原因，不吞原始错误类型（页面据此进错误态）。
class FilesLoadFailure implements Exception {
  const FilesLoadFailure(this.message);

  final String message;

  @override
  String toString() => 'FilesLoadFailure: $message';
}

/// 预览加载失败：同上，独立类型让页面能区分「列表失败」与「预览失败」两条态。
class FilePreviewFailure implements Exception {
  const FilePreviewFailure(this.message);

  final String message;

  @override
  String toString() => 'FilePreviewFailure: $message';
}

/// 目录数据源抽象 —— 页面只认这一个口子（测试注入可控源，生产走真通道）。
abstract class FilesDirectorySource {
  /// 拉一次目录；失败以 [FilesLoadFailure] 上抛（页面据此进错误态）。
  Future<List<FileEntry>> load(String path);
}

/// 预览数据源抽象 —— 返回 [FilePreviewContent]（正文 + 截断标注）；二进制/
/// 失败以 [FilePreviewFailure] 上抛。
abstract class FilesPreviewSource {
  Future<FilePreviewContent> readText(FileEntry entry);
}

/// 真通道目录数据源：`PaBridge.listDirectory()` → `ListDirectoryResponse`
/// 字节已在 transport 层验形（Uint8List + proto 反序列化，坏字节抛
/// DECODE_FAILED）——这里只做「响应字节已成型之后」的两件事：错误 →
/// [FilesLoadFailure] 人话；成功 → 行模型 + 排序。
///
/// [bridge] 是注入点：测试按通道名 mock 宿主 wire 行为（PaBridge 全程真实）。
class BridgeDirectorySource implements FilesDirectorySource {
  BridgeDirectorySource({PaBridge? bridge}) : _bridge = bridge ?? PaBridge();

  final PaBridge _bridge;

  @override
  Future<List<FileEntry>> load(String path) async {
    final ListDirectoryResponse response;
    try {
      response = await _bridge.listDirectory(ListDirectoryRequest(path: path));
    } on PaBridgeException catch (e) {
      // 页面错误态只认一条人话原因：「通道未就绪」与「宿主/解码出错」都在错误态
      // 这一条路径上，具体是哪种由文案说清（HOST_UNAVAILABLE 点名 B4）。
      // **不在这里回退成空列表**：那会把错误粉饰成「空目录」（假空态），
      // 两条路径从此再也分不开。
      throw FilesLoadFailure(filesLoadErrorText(e));
    }
    return sortFileEntries(fileEntriesOfResponse(response));
  }
}

/// 预览内容：文本正文 + 来源文件的大小/mtime（诚实提示用）。
class FilePreviewContent {
  const FilePreviewContent({required this.text, required this.truncated});

  /// UTF-8 解码后的正文（首块，可能被 max_bytes 截断）。
  final String text;

  /// 是否被截断（读到的字节数 < 文件真实大小）。
  final bool truncated;
}

/// 真通道预览数据源：`PaBridge.readFile()` → 首块 `FileChunk` 字节 →
/// UTF-8 解码。
///
/// 【为什么只取首块】宿主侧（B4 约定）把 ReadFile 流汇聚成首块回传
/// （channel-contract.md §3.8）；预览上限 [kFilePreviewMaxBytes] 由 Dart 侧
/// 在调用前按 DirectoryEntry.size_bytes 判掉，超限的文件根本不发起 readFile
/// （见 [filePreviewRejection]），所以首块即全文。
/// 【截断判定】chunk.total_size > data.length = 还有后续块没读 → 如实标注
/// truncated，不把半份文件画成完整文件。
class BridgeFilePreviewSource implements FilesPreviewSource {
  BridgeFilePreviewSource({PaBridge? bridge}) : _bridge = bridge ?? PaBridge();

  final PaBridge _bridge;

  @override
  Future<FilePreviewContent> readText(FileEntry entry) async {
    final FileChunk chunk;
    try {
      chunk = await _bridge.readFile(ReadFileRequest(
        path: entry.path,
        maxBytes: Int64(kFilePreviewMaxBytes),
      ));    } on PaBridgeException catch (e) {
      throw FilePreviewFailure(filePreviewErrorText(e));
    }
    if (!chunk.text) {
      // daemon 在首块判 UTF-8（fs.go:107）：非文本文件不含糊，如实拒预览。
      throw const FilePreviewFailure('这是二进制文件，不能按文本预览。');
    }
    final String text = utf8.decode(chunk.data, allowMalformed: true);
    final bool truncated = chunk.totalSize.toInt() > chunk.data.length;
    return FilePreviewContent(text: text, truncated: truncated);
  }
}
