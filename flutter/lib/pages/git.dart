// Git 页（B3 第 3 波）：仓库选择 + 分支列表（只读），全走真通道。
//
// 【页面职责】page-contracts §1.5 Git 的**只读子集**：仓库路径选择（本波没有
// FolderPicker 的 Dart 对位物——那是另一个 249 行的目录选择器组件，属后续波次；
// 这里用 getHostInfo 的 workspaces 候选 + 手输路径二选一）、分支列表
// （listBranches）+ 当前分支高亮、三态纪律。
//
// 【诚实红线（B4 前）】宿主桥未注册 → listBranches/getHostInfo 必然抛
// HOST_UNAVAILABLE，如实进错误态「桌面端 Git 通道未就绪（B4 波次）」，
// **不伪装成空分支列表**（两条路径分别有测试钉死）。
//
// 【本波不做的】status/log/diff 三段（契约 §1.5 的变更/历史页签）与全部写操作
// （checkout/createBranch/commit/stage/discard）——手机端写电脑仓库待拍板。
// 页面因此只有「分支」一个内容区，不摆切换/新建入口；工作区候选来自
// getHostInfo 的 workspaces（proto/v1/host.proto:38 HostInfo.workspaces），
// 与 Android resolveGitRepo 的候选来源同源（它另外还合并 recentWorkspaces——
// 那是宿主进程内存态，Dart 侧拿不到，不造假来源）。
import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';
import '../transport/pa_bridge.dart';
import '../gen/pocketagent/v1/host.pb.dart';
import 'git_bridge.dart';

/// Git 页：真通道分支列表（只读）。
///
/// 数据源是注入点：测试传可控源或带 mock 通道的桥源；生产不传即
/// [BridgeGitBranchesSource]。[bridge] 单独注入给仓库候选（getHostInfo 的
/// workspaces）——它和方法面共用同一个 PaBridge，测试按通道名 mock 即可。
class GitPage extends StatefulWidget {
  const GitPage({
    super.key,
    this.source,
    this.bridge,
    this.initialRepo = '',
  });

  final GitBranchesSource? source;
  final PaBridge? bridge;

  /// 初始仓库路径（page-contracts §1.5：入 = `home` tab3「或带 workspacePath」）。
  /// 空 = 没有预设仓库：页面先读 getHostInfo 候选，取第一个当默认（与 Android
  /// resolveGitRepo 同策略）；候选也读不到时停在「还没有选仓库」的诚实空态，
  /// 用户可以手输路径——不猜一个仓库路径出来。
  final String initialRepo;

  @override
  State<GitPage> createState() => _GitPageState();
}

/// 仓库选择层的三态（与分支列表三态分开：候选没读到 ≠ 分支没读到）。
enum _RepoPhase { idle, loading, ready }

/// 分支列表三态：加载中 / 加载失败 / 已就绪（就绪后再分「空列表」与「未选仓库」）。
enum _GitPhase { loading, error, ready }

class _GitPageState extends State<GitPage> {
  late GitBranchesSource _source;
  late PaBridge _bridge;
  _RepoPhase _repoPhase = _RepoPhase.idle;
  _GitPhase _phase = _GitPhase.loading;
  String _errorMessage = '';
  String _repoErrorText = '';
  List<GitBranchRow> _branches = const <GitBranchRow>[];

  /// 当前仓库路径（空 = 还没选）。
  String _repo = '';

  /// 工作区候选（getHostInfo.workspaces，去空保序）。
  List<String> _candidates = const <String>[];

  final TextEditingController _repoCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    _source = widget.source ?? BridgeGitBranchesSource();
    _bridge = widget.bridge ?? PaBridge();
    // 预设仓库（Kotlin workspacePath 同语义）优先于候选默认值：调用方明确指定
    // 时不去猜，也不被候选覆盖。
    if (widget.initialRepo.trim().isNotEmpty) {
      _repo = widget.initialRepo.trim();
      _repoCtrl.text = _repo;
    }
  }

  bool _firstFrame = true;

  /// 首帧做两件事：读候选 + 拉分支。
  ///
  /// 【为什么串行（先候选后分支）而不是并行】两个请求都写 [_phase]，并行时
  /// 「空 repo 的分支失败」与「候选成功的默认仓库」互相覆盖，谁后到谁说话——
  /// 页面会闪一个假的错误态/空态。串行多一跳候选延迟，但状态机只有一条写入
  /// 路径：候选定了 _repo 之后才拉分支，拉的就是最终要展示的那个仓库
  /// （Android GitScreen 的 resolveDefaultRepo → refresh 也是这个次序）。
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_firstFrame) return;
    _firstFrame = false;
    _loadCandidates();
  }

  @override
  void dispose() {
    _repoCtrl.dispose();
    super.dispose();
  }

  /// 读工作区候选：getHostInfo 的 workspaces（proto HostInfo 字段 6）。
  ///
  /// 【失败不刷红】候选只是快捷入口，读不到 → 用户还能手输路径（Android 的
  /// candidates 同样可空）。只有分支列表本身的失败才进错误态。
  Future<void> _loadCandidates() async {
    setState(() => _repoPhase = _RepoPhase.loading);
    try {
      final HostInfo info = await _bridge.getHostInfo();
      if (!mounted) return;
      final List<String> cands = <String>[];
      for (final String w in info.workspaces) {
        final String t = w.trim();
        if (t.isNotEmpty && !cands.contains(t)) cands.add(t);
      }
      setState(() {
        _candidates = cands;
        _repoPhase = _RepoPhase.ready;
        // 与 Android resolveGitRepo 同策略：没选过仓库时，拿第一个候选当默认，
        // 避免一进页面就是空态。用户显式选过（_repo 非空）则不动。
        if (_repo.isEmpty && cands.isNotEmpty) {
          _repo = cands.first;
          _repoCtrl.text = cands.first;
        }
      });
      if (_repo.isNotEmpty) {
        await _load();
      } else {
        setState(() => _phase = _GitPhase.ready);
      }
    } on PaBridgeException catch (e) {
      if (!mounted) return;
      setState(() {
        _repoPhase = _RepoPhase.ready;
        _repoErrorText = gitLoadErrorText(e);
        // 候选读不到（含 HOST_UNAVAILABLE）时：有预设仓库 → 照样拉分支（让
        // 分支自己的错误说话）；没有预设仓库 → 分支也必然读不到，把同一句人话
        // 摆进错误态，**不摆空分支列表**——诚实占位路径。
        if (_repo.isEmpty) {
          _phase = _GitPhase.error;
          _errorMessage = _repoErrorText;
        }
      });
      if (_repo.isNotEmpty) {
        await _load();
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _repoPhase = _RepoPhase.ready;
        _repoErrorText = '$e';
        if (_repo.isEmpty) {
          _phase = _GitPhase.error;
          _errorMessage = _repoErrorText;
        }
      });
      if (_repo.isNotEmpty) {
        await _load();
      }
    }
  }

  Future<void> _load() async {
    final String target = _repo;
    if (_phase != _GitPhase.loading || _errorMessage.isNotEmpty) {
      setState(() {
        _phase = _GitPhase.loading;
        _errorMessage = '';
      });
    }
    try {
      final List<GitBranchRow> branches = await _source.load(target);
      if (!mounted) return;
      setState(() {
        _branches = branches;
        _repo = target;
        _phase = _GitPhase.ready;
      });
    } on GitLoadFailure catch (e) {
      if (!mounted) return;
      setState(() {
        _phase = _GitPhase.error;
        _errorMessage = e.message;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _phase = _GitPhase.error;
        _errorMessage = '$e';
      });
    }
  }

  void _applyRepo(String repo) {
    final String t = repo.trim();
    if (t.isEmpty || t == _repo) return;
    setState(() => _repo = t);
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final String current = currentBranchNameOf(_branches);
    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 24),
      children: <Widget>[
        _buildRepoSection(pa),
        const SizedBox(height: 12),
        if (current.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(left: 4, bottom: 8),
            child: Text(
              '当前分支：$current',
              key: const ValueKey<String>('git-current-branch'),
              style: PATypography.caption.copyWith(color: pa.fgSecondary),
            ),
          ),
        ..._buildPhaseBody(context, pa),
      ],
    );
  }

  // ── 仓库选择 ───────────────────────────────────────────────────────────

  Widget _buildRepoSection(PAColors pa) {
    return PACard(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            '仓库',
            style: PATypography.body.copyWith(
              color: pa.fgPrimary,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            _repo.isEmpty ? '未选择仓库' : _repo,
            key: const ValueKey<String>('git-repo-path'),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: PATypography.code.copyWith(color: pa.fgSecondary),
          ),
          const SizedBox(height: 10),
          if (_repoPhase == _RepoPhase.loading)
            Text(
              '正在读取工作区候选…',
              style: PATypography.caption.copyWith(color: pa.fgTertiary),
            )
          else ...<Widget>[
            for (final String c in _candidates)
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: InkWell(
                  key: ValueKey<String>('git-candidate-$c'),
                  borderRadius: BorderRadius.circular(8),
                  onTap: () => _applyRepo(c),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 8,
                    ),
                    child: Row(
                      children: <Widget>[
                        Icon(
                          c == _repo
                              ? Icons.radio_button_checked_rounded
                              : Icons.radio_button_unchecked_rounded,
                          size: 16,
                          color: c == _repo ? pa.brand : pa.fgTertiary,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            c,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: PATypography.code
                                .copyWith(color: pa.fgPrimary),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            if (_repoErrorText.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 2, bottom: 6),
                child: Text(
                  _repoErrorText,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ),
            const SizedBox(height: 4),
            Row(
              children: <Widget>[
                Expanded(
                  child: TextField(
                    key: const ValueKey<String>('git-repo-input'),
                    controller: _repoCtrl,
                    style: PATypography.code.copyWith(color: pa.fgPrimary),
                    onSubmitted: _applyRepo,
                    decoration: InputDecoration(
                      isDense: true,
                      hintText: '或手输仓库绝对路径',
                      hintStyle:
                          PATypography.caption.copyWith(color: pa.fgTertiary),
                      filled: true,
                      fillColor: pa.bgSurface,
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: 10,
                        vertical: 10,
                      ),
                      enabledBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(10),
                        borderSide: BorderSide(color: pa.hairline),
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(10),
                        borderSide: BorderSide(color: pa.brand, width: 1.5),
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                PAButton(
                  label: '打开',
                  expanded: false,
                  onPressed: () => _applyRepo(_repoCtrl.text),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }

  // ── 分支三态 ───────────────────────────────────────────────────────────

  List<Widget> _buildPhaseBody(BuildContext context, PAColors pa) {
    switch (_phase) {
      case _GitPhase.loading:
        return <Widget>[
          const _LoadingView(message: '正在读取分支…'),
        ];
      case _GitPhase.error:
        return <Widget>[
          _ErrorView(message: _errorMessage, onRetry: _load),
        ];
      case _GitPhase.ready:
        return _buildReadyBody(pa);
    }
  }

  List<Widget> _buildReadyBody(PAColors pa) {
    if (_repo.isEmpty) {
      return <Widget>[
        const _EmptyView(
          title: '还没有选仓库',
          hint: '选一个工作区候选，或手输电脑上的 Git 仓库绝对路径',
        ),
      ];
    }
    if (_branches.isEmpty) {
      return <Widget>[
        _EmptyView(
          title: '没有读到分支信息',
          hint: 'daemon 对这个路径没有返回任何分支；'
              '确认它是 Git 仓库后点「重试」，或换一个仓库',
          actionLabel: '重试',
          onAction: _load,
        ),
      ];
    }
    return <Widget>[
      PACard(
        padding: EdgeInsets.zero,
        child: Column(
          children: <Widget>[
            for (int i = 0; i < _branches.length; i++) ...<Widget>[
              if (i > 0)
                const Padding(
                  padding: EdgeInsets.only(left: 44),
                  child: Divider(height: 1),
                ),
              _BranchRow(branch: _branches[i]),
            ],
          ],
        ),
      ),
    ];
  }
}

/// 分支行：当前分支高亮 + 远端标注；**不可点**（本波无写操作，不摆切换入口）。
class _BranchRow extends StatelessWidget {
  const _BranchRow({required this.branch});

  final GitBranchRow branch;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final GitBranchRow b = branch;
    final Color accent = b.isCurrent ? pa.brand : pa.fgTertiary;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      child: Row(
        children: <Widget>[
          Icon(
            b.isCurrent ? Icons.check_circle_rounded : Icons.circle_outlined,
            size: 16,
            color: accent,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              b.name,
              key: ValueKey<String>('git-branch-${b.name}'),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: PATypography.body.copyWith(
                fontFamily: PATypography.monoFont,
                color: b.isCurrent ? pa.brand : pa.fgPrimary,
                fontWeight: b.isCurrent ? FontWeight.w600 : FontWeight.w400,
              ),
            ),
          ),
          if (b.isCurrent)
            Text(
              '当前',
              style: PATypography.caption.copyWith(color: accent),
            )
        else if (b.isRemote)
          Text(
            '远端',
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
      ],
    ),
  );
  }
}

/// 加载态（同 Threads/Files：固定 padding，一眼可见在转）。
class _LoadingView extends StatelessWidget {
  const _LoadingView({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(top: 48),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            SizedBox(
              width: 26,
              height: 26,
              child: CircularProgressIndicator(
                strokeWidth: 2.5,
                color: pa.brand,
              ),
            ),
            const SizedBox(height: 12),
            Text(
              message,
              style: PATypography.caption.copyWith(color: pa.fgSecondary),
            ),
          ],
        ),
      ),
    );
  }
}

/// 错误态：标题 + 人话原因 + 重试出口。
class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final Future<void> Function() onRetry;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return SingleChildScrollView(
      padding: const EdgeInsets.only(top: 40),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Icons.error_outline_rounded, size: 30, color: pa.danger),
            const SizedBox(height: 10),
            Text(
              '分支列表加载失败',
              style: PATypography.body.copyWith(
                color: pa.fgPrimary,
                fontWeight: FontWeight.w600,
              ),
            ),
            if (message.isNotEmpty) ...<Widget>[
              const SizedBox(height: 6),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: Text(
                  message,
                  textAlign: TextAlign.center,
                  style: PATypography.caption.copyWith(color: pa.fgTertiary),
                ),
              ),
            ],
            const SizedBox(height: 14),
            PAButton(
              label: '重试',
              expanded: false,
              onPressed: onRetry,
            ),
          ],
        ),
      ),
    );
  }
}

/// 空态：未选仓库 / 选了但没读到分支（两种原因分两句文案）。
class _EmptyView extends StatelessWidget {
  const _EmptyView({
    required this.title,
    required this.hint,
    this.actionLabel,
    this.onAction,
  });

  final String title;
  final String hint;
  final String? actionLabel;
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.only(top: 40),
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Text(
              title,
              style: PATypography.body.copyWith(
                color: pa.fgPrimary,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 6),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Text(
                hint,
                textAlign: TextAlign.center,
                style: PATypography.caption.copyWith(color: pa.fgTertiary),
              ),
            ),
            if (actionLabel != null) ...<Widget>[
              const SizedBox(height: 12),
              TextButton(
                onPressed: onAction,
                child: Text(
                  actionLabel!,
                  style: PATypography.caption.copyWith(
                    color: pa.brand,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
