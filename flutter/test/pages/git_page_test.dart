// Git 页测试：三态 + 仓库选择 + 分支渲染（当前分支高亮）+ 真通道默认接线。
//
// 【两条必须分家的路径】宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」；
// 宿主正常回空分支列表 → 空态「没有读到分支信息」——各钉一条，互斥断言
// （红线：不留假绿，「通道没就绪」绝不许伪装成「没有分支」）。
//
// 【本波无写操作】页面不渲染切换/新建分支入口；测试反向钉死：分支行不可点
// （没有 onTap），也没有「新建分支」按钮——没有就是没有，不摆假按钮。
//
// 【仓库候选来源】getHostInfo 的 workspaces（proto HostInfo 字段 6）；
// 候选读失败（含 HOST_UNAVAILABLE）时如实显示原因，但**仍可手输路径**
// ——快捷入口缺席不等于功能缺席，两条都测。

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/git.pb.dart';
import 'package:pocketagent_ui/gen/pocketagent/v1/host.pb.dart';
import 'package:pocketagent_ui/pages/git.dart';
import 'package:pocketagent_ui/pages/git_bridge.dart';
import 'package:pocketagent_ui/theme/palette.dart';
import 'package:pocketagent_ui/theme/pocket_theme.dart';
import 'package:pocketagent_ui/transport/pa_bridge.dart';

/// 卡住不返回的分支源：把「加载中」钉在屏幕上。
class _PendingSource implements GitBranchesSource {
  final Completer<List<GitBranchRow>> completer =
      Completer<List<GitBranchRow>>();

  @override
  Future<List<GitBranchRow>> load(String repo) => completer.future;
}

/// 第一次必失败、之后成功：验证错误态的「重试」是真的重新拉。
class _RetrySource implements GitBranchesSource {
  _RetrySource(this.branches);

  final List<GitBranchRow> branches;
  final List<String> seenRepos = <String>[];
  int calls = 0;

  @override
  Future<List<GitBranchRow>> load(String repo) async {
    seenRepos.add(repo);
    calls += 1;
    if (calls == 1) {
      throw const GitLoadFailure('git branch 失败: not a git repository');
    }
    return branches;
  }
}

class _StaticSource implements GitBranchesSource {
  const _StaticSource(this.branches);

  final List<GitBranchRow> branches;

  @override
  Future<List<GitBranchRow>> load(String repo) async => branches;
}

const List<GitBranchRow> demoBranches = <GitBranchRow>[
  GitBranchRow(name: 'main', isCurrent: true, isRemote: false),
  GitBranchRow(name: 'feature/login', isCurrent: false, isRemote: false),
  GitBranchRow(name: 'origin/main', isCurrent: false, isRemote: true),
];

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  // 仓库候选走 getHostInfo（宿主 wire 行为）；不 mock 宿主的用例给一个
  // 「空候选」应答——分支源是注入点，与通道无关。这与生产路径同构：候选可能
  // 为空，用户靠预设仓库/手输路径继续（不猜、不摆假列表）。
  const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
  setUp(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
      methodChannel,
      (MethodCall call) async {
        if (call.method == 'getHostInfo') {
          return HostInfo(workspaces: <String>[]).writeToBuffer();
        }
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      },
    );
  });

  Future<void> pumpGit(
    WidgetTester tester, {
    GitBranchesSource? source,
    PaBridge? bridge,
    String initialRepo = r'G:\repo',
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: PocketTheme.fromBrightness(PABrightness.dark),
        home: Scaffold(
          body: GitPage(
            source: source,
            bridge: bridge,
            initialRepo: initialRepo,
          ),
        ),
      ),
    );
  }

  group('三态', () {
    testWidgets('加载中 → 「正在读取分支…」，数据到了切换成列表', (tester) async {
      final _PendingSource src = _PendingSource();
      await pumpGit(tester, source: src);
      await tester.pump();

      expect(find.text('正在读取分支…'), findsOneWidget);
      expect(find.text('main'), findsNothing);

      src.completer.complete(demoBranches);
      await tester.pumpAndSettle();

      expect(find.text('正在读取分支…'), findsNothing);
      expect(find.text('main'), findsOneWidget);
    });

    testWidgets('加载失败 → 错误态带原因；重试后真拉到列表', (tester) async {
      final _RetrySource src = _RetrySource(demoBranches);
      await pumpGit(tester, source: src);
      await tester.pumpAndSettle();

      expect(find.text('分支列表加载失败'), findsOneWidget);
      expect(find.textContaining('git branch 失败'), findsOneWidget);
      expect(find.text('main'), findsNothing);

      await tester.tap(find.text('重试'));
      await tester.pumpAndSettle();

      expect(src.calls, 2, reason: '重试必须真的重新调用数据源');
      expect(find.text('分支列表加载失败'), findsNothing);
      expect(find.text('main'), findsOneWidget);
    });

    testWidgets('空分支列表 → 「没有读到分支信息」（真空态，不是通道错误的伪装）',
        (tester) async {
      await pumpGit(tester, source: const _StaticSource(<GitBranchRow>[]));
      await tester.pumpAndSettle();

      expect(find.text('没有读到分支信息'), findsOneWidget);
      expect(find.textContaining('B4 波次'), findsNothing,
          reason: '空态不许出现「未就绪」字样——那是错误路径的词');
    });
  });

  group('分支渲染', () {
    testWidgets('三型行：当前分支高亮 + 本地 + 远端；行不可点（无写操作）',
        (tester) async {
      await pumpGit(tester, source: const _StaticSource(demoBranches));
      await tester.pumpAndSettle();

      // 当前分支标注 + 高亮（brand 色由页面决定，测试只验文案与键）
      expect(find.text('当前分支：main'), findsOneWidget);
      expect(find.text('当前'), findsOneWidget);
      expect(find.text('feature/login'), findsOneWidget);
      expect(find.text('origin/main'), findsOneWidget);
      expect(find.text('远端'), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('git-branch-main')), findsOneWidget);
      expect(find.byKey(const ValueKey<String>('git-branch-feature/login')),
          findsOneWidget);

      // 没有写操作入口：页面不摆切换/新建分支
      expect(find.text('切换'), findsNothing);
      expect(find.text('新建分支'), findsNothing);
    });

    testWidgets('没有当前分支（daemon 没回 * 行）→ 不显示「当前分支」行', (tester) async {
      await pumpGit(
        tester,
        source: const _StaticSource(<GitBranchRow>[
          GitBranchRow(name: 'main', isCurrent: false, isRemote: false),
        ]),
      );
      await tester.pumpAndSettle();

      expect(find.text('main'), findsOneWidget);
      expect(find.textContaining('当前分支：'), findsNothing,
          reason: 'currentBranchNameOf 空串时页面不编一个「当前分支」');
    });

    testWidgets('无预设仓库 + 无候选 → 「还没有选仓库」诚实空态（不猜路径）',
        (tester) async {
      await pumpGit(
        tester,
        source: const _StaticSource(demoBranches),
        initialRepo: '',
      );
      await tester.pumpAndSettle();

      expect(find.text('还没有选仓库'), findsOneWidget);
      expect(find.text('main'), findsNothing,
          reason: '没有仓库就不拉分支——不对空气发请求，也不摆假分支列表');
      expect(find.byKey(const ValueKey<String>('git-repo-input')), findsOneWidget,
          reason: '空态给手输路径出口');
    });
  });

  group('仓库选择', () {
    testWidgets('候选来自 getHostInfo.workspaces；点候选真以该路径重拉', (tester) async {
      final _RetrySource src = _RetrySource(demoBranches);
      final PaBridge bridge = PaBridge();
      // 只 mock 宿主 wire：候选走 getHostInfo，分支走 listBranches。
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(kPaBridgeMethodChannel),
        (MethodCall call) async {
          if (call.method == 'getHostInfo') {
            return HostInfo(workspaces: <String>[r'G:\repo-a', r'G:\repo-b'])
                .writeToBuffer();
          }
          if (call.method == 'listBranches') {
            return ListBranchesResponse(
              branches: <GitBranch>[
                GitBranch(name: 'main', isCurrent: true),
              ],
            ).writeToBuffer();
          }
          throw MissingPluginException(
              'No implementation found for method ${call.method}');
        },
      );
      // 让候选加载完再断言（首帧 _loadCandidates 是 async）
      await pumpGit(tester, source: src, bridge: bridge);
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('git-candidate-${r'G:\repo-a'}')),
          findsOneWidget);
      expect(find.byKey(const ValueKey<String>('git-candidate-${r'G:\repo-b'}')),
          findsOneWidget);
      // 默认候选 = 第一个（Android resolveGitRepo 同策略）
      expect(find.text(r'G:\repo-a'), findsWidgets);

      // 点第二个候选 → 分支源以该 repo 重新 load
      await tester.tap(
          find.byKey(const ValueKey<String>('git-candidate-${r'G:\repo-b'}')));
      await tester.pumpAndSettle();
      expect(src.seenRepos, contains(r'G:\repo-b'));
    });

    testWidgets('手输路径 + 「打开」→ 以手输路径重拉（候选缺席也能用）', (tester) async {
      final _RetrySource src = _RetrySource(demoBranches);
      await pumpGit(tester, source: src);
      await tester.pumpAndSettle();

      await tester.enterText(
        find.byKey(const ValueKey<String>('git-repo-input')),
        r'G:\hand-typed',
      );
      await tester.tap(find.text('打开'));
      await tester.pumpAndSettle();

      expect(src.seenRepos, contains(r'G:\hand-typed'));
    });

    testWidgets('候选读失败（HOST_UNAVAILABLE）→ 如实显示原因，手输入口仍在',
        (tester) async {
      final _RetrySource src = _RetrySource(demoBranches);
      final PaBridge bridge = PaBridge();
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
        const MethodChannel(kPaBridgeMethodChannel),
        (MethodCall call) async {
          throw MissingPluginException(
              'No implementation found for method ${call.method}');
        },
      );

      await pumpGit(tester, source: src, bridge: bridge);
      await tester.pumpAndSettle();

      expect(find.textContaining('桌面端 Git 通道未就绪'), findsWidgets,
          reason: '候选读不到 = 宿主桥未注册，如实说明');
      expect(find.byKey(const ValueKey<String>('git-repo-input')), findsOneWidget,
          reason: '快捷入口缺席不等于功能缺席：手输路径仍可用');
      // 有预设仓库：候选读失败后仍拉分支（串行首帧的第二步），失败如实进错误态。
      expect(src.calls, 1, reason: '候选失败后用预设仓库拉的第一次（_load 被调用一次）');
      expect(find.text('分支列表加载失败'), findsOneWidget);
    });
  });

  group('默认接线：真通道（不注入 source）', () {
    const MethodChannel methodChannel = MethodChannel(kPaBridgeMethodChannel);
    late TestDefaultBinaryMessenger messenger;

    setUp(() {
      messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
    });

    testWidgets('宿主未注册（B4 前）→ 错误态「未就绪（B4 波次）」，绝不是空态',
        (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        throw MissingPluginException(
            'No implementation found for method ${call.method}');
      });

      await pumpGit(tester);
      await tester.pumpAndSettle();

      expect(find.text('分支列表加载失败'), findsOneWidget);
      expect(find.textContaining('未就绪（B4 波次）'), findsWidgets,
          reason: '同一句人话在候选区小字与错误态正文各一份（findsWidgets 不是'
              ' findsOneWidget：两处都在说真话，不是重复渲染 bug）');
      expect(find.text('没有读到分支信息'), findsNothing,
          reason: '通道没就绪不许走空态——那是「真空分支」的伪装版（红线：不留假绿）');
    });

    testWidgets('宿主正常回空分支 → 空态（与错误态分家的另一条）', (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'getHostInfo') {
          return HostInfo(workspaces: <String>[r'G:\repo']).writeToBuffer();
        }
        return ListBranchesResponse().writeToBuffer();
      });

      await pumpGit(tester);
      await tester.pumpAndSettle();

      expect(find.text('没有读到分支信息'), findsOneWidget);
      expect(find.text('分支列表加载失败'), findsNothing);
      expect(find.textContaining('未就绪'), findsNothing,
          reason: '空态不许出现错误路径的词——两条路径断言必须互相排斥');
    });

    testWidgets('宿主正常回分支 → 行渲染（当前分支高亮）', (tester) async {
      messenger.setMockMethodCallHandler(methodChannel, (MethodCall call) async {
        if (call.method == 'getHostInfo') {
          return HostInfo(workspaces: <String>[r'G:\repo']).writeToBuffer();
        }
        return ListBranchesResponse(
          branches: <GitBranch>[
            GitBranch(name: 'main', isCurrent: true),
            GitBranch(name: 'dev', isCurrent: false, isRemote: false),
          ],
        ).writeToBuffer();
      });

      await pumpGit(tester);
      await tester.pumpAndSettle();

      expect(find.text('当前分支：main'), findsOneWidget);
      expect(find.text('当前'), findsOneWidget);
      expect(find.text('dev'), findsOneWidget);
    });
  });
}
