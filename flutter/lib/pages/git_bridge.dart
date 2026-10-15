// Git 页的分支行模型 + 真通道数据源（B3 第 3 波）。
//
// 【为什么单独一个文件】与 threads_bridge.dart / files_bridge.dart 同一分工：
// ① 行字段从 wire 哪个字段来（纯函数，可单测）；② 真通道怎么包
// （[BridgeGitBranchesSource]）。git.dart 只留界面（三态 + 仓库选择 + 分支渲染）。
//
// 【诚实红线（B4 前）】Kotlin 宿主桥 B4 波次才注册 `dev.pocketagent/bridge`；
// 在此之前 listBranches 必然抛 HOST_UNAVAILABLE，如实翻成
// [GitLoadFailure]（文案点名「桌面端 Git 通道未就绪（B4 波次）」），页面进错误态。
// **绝不返回空分支列表冒充「没有分支」**：空态与错误态两条路径，测试各钉一条。
//
// 【本波只读】proto 的 checkoutBranch / createBranch / stage / commit /
// discardChanges 全是写操作；红线「手机端写电脑文件/仓库」待拍板，本波不接
// 这些方法面，页面因此不摆切换/新建分支/提交入口——没有就是没有。
// status / log / diff 同样本波不接（page-contracts §1.5 的完整 Git 页是后续
// 波次），本页范围 = 仓库选择 + 分支列表 + 当前分支高亮。

import '../gen/pocketagent/v1/git.pb.dart';
import '../transport/pa_bridge.dart';

// ── 行模型 ────────────────────────────────────────────────────────────────

/// 分支行 —— proto `GitBranch` 的 Dart 投影（三字段一一对应）。
class GitBranchRow {
  const GitBranchRow({
    required this.name,
    required this.isCurrent,
    required this.isRemote,
  });

  final String name;

  /// 当前 HEAD 分支（daemon git.go:214 由 `git branch -a --format=%(HEAD)`
  /// 判定，`*` 号行）。列表里高亮它。
  final bool isCurrent;

  /// 远端分支（daemon 按名字含 "/" 判定，如 origin/main）。行尾给「远端」标注，
  /// 与本地分支区分——点了也不会切（本波无写操作）。
  final bool isRemote;

  @override
  bool operator ==(Object other) =>
      other is GitBranchRow &&
      other.name == name &&
      other.isCurrent == isCurrent &&
      other.isRemote == isRemote;

  @override
  int get hashCode => Object.hash(name, isCurrent, isRemote);
}

/// proto `GitBranch` → [GitBranchRow]（三字段逐字段对应，无加工）。
GitBranchRow gitBranchRowOf(GitBranch b) =>
    GitBranchRow(name: b.name, isCurrent: b.isCurrent, isRemote: b.isRemote);

/// `ListBranchesResponse` → 行列表（顺序保持 daemon 原序，不重排——
/// `git branch -a` 的顺序即 git 自己的排序，UI 不发明第二套）。
List<GitBranchRow> gitBranchRowsOfResponse(ListBranchesResponse response) =>
    response.branches.map(gitBranchRowOf).toList(growable: false);

/// 当前分支名（纯函数）：取 [isCurrent] 行的名字；没有则空串。
///
/// 【为什么不再判 isRemote】daemon 的 is_current 只可能落在本地分支上；
/// 空串时页面显示「未读到当前分支」而不是编一个名字。
String currentBranchNameOf(List<GitBranchRow> branches) {
  for (final GitBranchRow b in branches) {
    if (b.isCurrent) return b.name;
  }
  return '';
}

// ── 错误文案 ──────────────────────────────────────────────────────────────

/// 错误码 → 页面人话文案（页面错误态直接展示这一条）。
///
/// 【只在 HOST_UNAVAILABLE 定制】B4 前这是常态：点名「Git 通道未就绪（B4 波次）」。
/// 其余码原样透传宿主 message（不改写、不吞——与 settings/threads/files 页
/// 同一套透传纪律）；宿主没给 message 时退 code。
String gitLoadErrorText(PaBridgeException e) {
  if (e.code == PaBridgeErrorCodes.hostUnavailable) {
    return '桌面端 Git 通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '分支列表暂不可用。';
  }
  return e.message.isNotEmpty ? e.message : e.code;
}

/// 分支加载失败：只带人话原因，不吞原始错误类型。
class GitLoadFailure implements Exception {
  const GitLoadFailure(this.message);

  final String message;

  @override
  String toString() => 'GitLoadFailure: $message';
}

// ── 数据源 ────────────────────────────────────────────────────────────────

/// 分支数据源抽象 —— 页面只认这一个口子（测试注入可控源，生产走真通道）。
abstract class GitBranchesSource {
  /// 拉一次分支列表；失败以 [GitLoadFailure] 上抛（页面据此进错误态）。
  Future<List<GitBranchRow>> load(String repo);
}

/// 真通道分支数据源：`PaBridge.listBranches()` → `ListBranchesResponse`
/// 字节已在 transport 层验形——这里只做：错误 → [GitLoadFailure] 人话；
/// 成功 → 行模型。
///
/// [bridge] 是注入点：测试按通道名 mock 宿主 wire 行为（PaBridge 全程真实）。
class BridgeGitBranchesSource implements GitBranchesSource {
  BridgeGitBranchesSource({PaBridge? bridge}) : _bridge = bridge ?? PaBridge();

  final PaBridge _bridge;

  @override
  Future<List<GitBranchRow>> load(String repo) async {
    final ListBranchesResponse response;
    try {
      response = await _bridge.listBranches(ListBranchesRequest(repo: repo));
    } on PaBridgeException catch (e) {
      // **不在这里回退成空列表**：那会把错误粉饰成「没有分支」（假空态），
      // 空态与错误态两条路径从此再也分不开（红线：不留假绿）。
      throw GitLoadFailure(gitLoadErrorText(e));
    }
    return gitBranchRowsOfResponse(response);
  }
}
