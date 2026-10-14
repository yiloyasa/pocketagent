import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/onboarding_steps.dart';

/// 新手教程脊柱的单测（Kotlin `OnboardStepsTest` + `FirstTaskFunnelFixTest`
/// 中教程相关断言**逐条**直译）。
///
/// 为什么要逐条：这两份 Kotlin 单测是教程文案的顺序/完成位/禁词表契约，改一个
/// 字符即红；Flutter 侧逐条搬过来才能保证迁移不缩水。
void main() {
  // ---- 脊柱顺序 ----

  test('脊柱顺序_连接_配对_首条指令_排障', () {
    final List<String> ids =
        onboardSteps(false).map((OnboardStep s) => s.id).toList();
    expect(ids, <String>['connect', 'pair', 'first-task', 'trouble']);
  });

  test('前三步就是调研钉死的三件事', () {
    final List<String> spine =
        onboardSteps(false).take(3).map((OnboardStep s) => s.id).toList();
    expect(spine, <String>['connect', 'pair', 'first-task']);
  });

  // ---- 完成位只认真实连接状态 ----

  test('已连接_连接与配对两步标记完成_不再展开打扰', () {
    final List<OnboardStep> steps = onboardSteps(true);
    expect(steps.firstWhere((OnboardStep s) => s.id == 'connect').done, isTrue);
    expect(steps.firstWhere((OnboardStep s) => s.id == 'pair').done, isTrue);
  });

  test('未连接_全部步骤按未完成讲', () {
    final List<OnboardStep> steps = onboardSteps(false);
    expect(steps.where((OnboardStep s) => s.done), isEmpty);
  });

  test('首条指令没有可靠完成事实_不编造_恒按未完成', () {
    final List<OnboardStep> steps = onboardSteps(true);
    expect(
      steps.firstWhere((OnboardStep s) => s.id == 'first-task').done,
      isFalse,
    );
  });

  // ---- 一屏一句话一动作 ----

  test('每步都有非空的一句话与一个动作', () {
    for (final OnboardStep step in onboardSteps(false)) {
      expect(step.sentence.trim().isNotEmpty, isTrue,
          reason: '步骤 ${step.id} 缺一句话');
      expect(step.action.trim().isNotEmpty, isTrue,
          reason: '步骤 ${step.id} 缺一个动作');
    }
  });

  test('已完成的步骤带替换话术_首条指令不带', () {
    final List<OnboardStep> steps = onboardSteps(true);
    expect(
      steps.firstWhere((OnboardStep s) => s.id == 'connect').note.trim(),
      isNotEmpty,
    );
    expect(
      steps.firstWhere((OnboardStep s) => s.id == 'pair').note.trim(),
      isNotEmpty,
    );
    expect(
      steps.firstWhere((OnboardStep s) => s.id == 'first-task').note.trim(),
      isEmpty,
    );
  });

  test('手动填地址处必带端口示例_裸IP连不上', () {
    final List<OnboardStep> steps = onboardSteps(false);
    final String connect =
        steps.firstWhere((OnboardStep s) => s.id == 'connect').action;
    final String trouble =
        steps.firstWhere((OnboardStep s) => s.id == 'trouble').action;
    expect(connect.contains(':7246'), isTrue, reason: '连接页缺 :7246 示例');
    expect(trouble.contains(':7246'), isTrue, reason: '排障页缺 :7246 示例');
  });

  // ---- 不堆术语:禁词表扫描(面向通用用户) ----

  test('全部文案不含术语禁词', () {
    const List<String> banned = <String>[
      'token',
      'daemon',
      'tailnet',
      'adb',
      'grpc',
      'api',
      'cli',
      'paguard',
      'onnx',
    ];
    for (final OnboardStep step in onboardSteps(true)) {
      final String all = '${step.sentence} ${step.action} ${step.note}'
          .toLowerCase();
      for (final String word in banned) {
        expect(all.contains(word), isFalse, reason: '步骤 ${step.id} 出现术语「$word」');
      }
    }
  });

  // ---- 前三步 / 排障页的页内直达按钮（Kotlin FirstTaskFunnelFixTest）----

  test('前三页各有真实直达按钮_去向复用既有接线', () {
    final OnboardAction? connect = onboardActionFor('connect');
    final OnboardAction? pair = onboardActionFor('pair');
    final OnboardAction? firstTask = onboardActionFor('first-task');
    expect(connect, isNotNull, reason: '连接页必须有直达按钮');
    expect(pair, isNotNull, reason: '配对页必须有直达按钮');
    expect(firstTask, isNotNull, reason: '首任务页必须有直达按钮');
    // 去向只许 settings/home（Kotlin MainActivity 已有接线，不新增回调）。
    expect(connect!.target, 'home');
    expect(pair!.target, 'home');
    expect(firstTask!.target, 'home');
    expect(<String>{connect.label, pair.label, firstTask.label}.length, 3,
        reason: '三页文案各不相同');
  });

  test('排障页按钮原样保留_未知步骤无按钮', () {
    final OnboardAction? trouble = onboardActionFor('trouble');
    expect(trouble, isNotNull);
    expect(trouble!.label, '去设置填写地址');
    expect(trouble.target, 'settings');
    expect(onboardActionFor('no-such-step'), isNull, reason: '未知步骤不得冒出按钮');
  });

  test('直达按钮文案无术语', () {
    const List<String> banned = <String>[
      'token',
      'daemon',
      'tailnet',
      'adb',
      'grpc',
      'api',
      'cli',
      'paguard',
      'onnx',
    ];
    for (final String id in <String>['connect', 'pair', 'first-task', 'trouble']) {
      final String lower = onboardActionFor(id)!.label.toLowerCase();
      for (final String word in banned) {
        expect(lower.contains(word), isFalse, reason: '教程 $id 直达按钮出现术语「$word」');
      }
    }
  });

  // ---- 教程页清单（Kotlin onboardPages 的行为等价，替代其源码结构钉子）----

  test('教程页清单_欢迎页_脊柱四步_收尾页', () {
    final List<OnboardPage> pages = onboardPages(false);
    expect(pages.length, 6, reason: '欢迎 + 4 脊柱步 + 收尾');
    expect(pages.first.title, '欢迎使用 PocketAgent');
    expect(pages.last.title, '一切就绪');
    expect(
      pages.map((OnboardPage p) => p.title).toList(),
      <String>[
        '欢迎使用 PocketAgent',
        '第一步 · 让手机找到电脑',
        '第二步 · 配对',
        '第三步 · 发第一条指令',
        '连不上?',
        '一切就绪',
      ],
    );
  });

  test('脊柱页经 onboardActionFor 统一接直达按钮', () {
    final List<OnboardPage> pages = onboardPages(false);
    // Kotlin 侧的源码结构钉子（OnboardPageContent 渲染 actionLabel/actionId）
    // 在 Dart 侧用行为断言表达：每个脊柱页都必须拿到按钮，且口径与
    // onboardActionFor 完全一致。
    for (final OnboardStep step in onboardSteps(false)) {
      final OnboardPage page =
          pages.firstWhere((OnboardPage p) => p.title == stepTitle(step.id));
      final OnboardAction? action = onboardActionFor(step.id);
      expect(page.actionLabel, isNotNull, reason: '${step.id} 页必须有直达按钮');
      expect(page.actionId, isNotNull);
      expect(page.actionLabel, action!.label);
      expect(page.actionId, action.target);
    }
    expect(pages.first.actionId, isNull, reason: '欢迎页无直达按钮');
  });

  test('收尾页直达按钮去主页', () {
    final OnboardPage last = onboardPages(false).last;
    expect(last.actionLabel, '去发第一条指令');
    expect(last.actionId, 'home');
  });

  test('已完成步骤_文案换成替换话术_动作标标题换成可以直接下一步', () {
    final List<OnboardPage> pages = onboardPages(true);
    final OnboardPage connect =
        pages.firstWhere((OnboardPage p) => p.title == stepTitle('connect'));
    expect(connect.done, isTrue);
    expect(connect.blocks.first, isA<OnboardLead>());
    expect(
      (connect.blocks.first as OnboardLead).text,
      onboardSteps(true).firstWhere((OnboardStep s) => s.id == 'connect').note,
    );
    expect(
      (connect.blocks[1] as OnboardBullet).title,
      '可以直接下一步',
    );

    // 未连接时同一页讲的是原话术 + 「现在做什么」
    final OnboardPage fresh =
        onboardPages(false).firstWhere((OnboardPage p) => p.title == stepTitle('connect'));
    expect(fresh.done, isFalse);
    expect(
      (fresh.blocks[1] as OnboardBullet).title,
      '现在做什么',
    );
  });

  test('首条指令页_id 恒定不按连接态造假', () {
    final List<OnboardPage> pages = onboardPages(true);
    final OnboardPage page = pages
        .firstWhere((OnboardPage p) => p.title == stepTitle('first-task'));
    expect(page.done, isFalse, reason: '首条指令没有可靠完成事实，不编造');
    expect(
      (page.blocks[1] as OnboardBullet).desc,
      onboardSteps(true).firstWhere((OnboardStep s) => s.id == 'first-task').action,
    );
  });
}
