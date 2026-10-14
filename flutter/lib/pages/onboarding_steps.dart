/// 新手教程的**纯数据 + 纯函数**层（Kotlin `ui/Onboarding.kt` 1:1 迁移）。
///
/// 为什么单独拆一个文件、为什么保持「零副作用纯函数」形态：
/// Kotlin 侧 [onboardSteps] / [onboardActionFor] 是被 `OnboardStepsTest` 与
/// `FirstTaskFunnelFixTest` 两级单测钉死的契约——文案顺序、完成位、禁词表改一个
/// 字符就红。Dart 侧逐条复刻这些断言（见 `test/pages/onboarding_steps_test.dart`），
/// 只有把这些函数从 widget 树里剥出来才测得动。
///
/// 本文件只依赖 `flutter/material.dart` 的 `IconData`（图标标识），其余全是常量
/// 与 `when` 分支——可以在任何测试环境里直接跑。
library;

import 'package:flutter/material.dart';

/// 引导步骤（纯数据，单测钉死 `OnboardStepsTest`）。
///
/// 【一屏一句话一动作】每步只回答两件事：「这步是干什么的」（[sentence]，一句话）和
/// 「现在做什么」（[action]，一个动作）。不堆第二层说明，细节留给浮层与设置页。
class OnboardStep {
  const OnboardStep({
    required this.id,
    required this.done,
    required this.sentence,
    required this.action,
    this.note = '',
  });

  /// 步骤标识（顺序即脊柱：连接 → 配对 → 第一条指令 → 排障）。
  final String id;

  /// 这一步在本机是否已完成 —— 重看时不再逐步打扰，页面上只标注「已完成」。
  final bool done;

  /// 一句话说明。
  final String sentence;

  /// 一个动作。
  final String action;

  /// 已完成时替换正文的一句话（空串 = 无替换）。
  final String note;
}

/// 前三步的页内直达按钮口径（纯函数，覆盖 Kotlin `FirstTaskFunnelFixTest`）。
///
/// 【为什么每步都给直达】首任务漏斗断在「看完教程但不知道去哪」：教程讲完就散，
/// 用户回主页还要自己找入口 —— 前三页各加一个真实按钮，复用 Kotlin 侧
/// `OnboardPage.actionLabel/actionId` 机制（不新增回调）。
class OnboardAction {
  const OnboardAction(this.label, this.target);

  /// 按钮文案。
  final String label;

  /// 去向标识：`"settings"` = 去设置页；`"home"` = 回主页。
  final String target;
}

/// 引导脊柱（纯函数，单测钉死顺序/完成位/无术语）：
/// **连接主机 → 配对 → 发第一条指令**，外加一页「连不上」的排障。
///
/// 【完成位只认真实状态】[connected] 来自 connState —— 连接与配对在协议上是同一件
/// 事（配对成功才有连接），已连接即两步都完成；首条指令有没有发过没有可靠事实
/// （不编造），永远按未完成讲。
///
/// 【不堆术语】面向通用用户：说「桌面端 / 配对码 / WiFi」，不说 token / daemon /
/// tailnet / adb —— 单测按禁词表扫描，回退即红。
List<OnboardStep> onboardSteps(bool connected) => <OnboardStep>[
      OnboardStep(
        id: 'connect',
        done: connected,
        sentence: '手机和电脑连同一个 WiFi,电脑上装好桌面端,它就会出现在手机上。',
        action: '在电脑上运行桌面端安装包里的 install.bat,装完回到这里点「下一步」。'
            '没自动出现的话,到「设置 → 连接与配对」手动填电脑的地址'
            '(形如 192.168.1.20:7246,冒号后面是端口号)。',
        note: '电脑已经连上了,这一步直接跳过。',
      ),
      OnboardStep(
        id: 'pair',
        done: connected,
        sentence: '第一次连接要认个亲:把电脑上显示的 8 位配对码输进手机。',
        action: '在主页点你的电脑,输入那 8 位数字,点「配对并连接」;'
            '用数据线连的时候会自动配对,不用输。',
        note: '已经配对好了,这一步直接跳过。',
      ),
      const OnboardStep(
        id: 'first-task',
        done: false,
        sentence: '在首页点「新建任务」,用一句大白话告诉电脑你想做什么。',
        action: '点「新建任务」,选个文件夹,把想说的话发出去;'
            '它要执行命令前会先弹卡问你,你点头才执行。',
      ),
      const OnboardStep(
        id: 'trouble',
        done: false,
        sentence: '电脑没出现在手机上?按顺序过这几件小事。',
        action: '①电脑开着桌面端;②手机和电脑连同一个 WiFi(或插数据线);'
            '③还不行,到「设置 → 连接与配对」手动填地址(形如 192.168.1.20:7246);'
            '④路由器开了 AP 隔离,或防火墙拦了自动发现,也会搜不到电脑,同样手填地址就能连;'
            '⑤地址没错却连不上,多半是电脑上 7246 端口被别的程序占了,换个端口重新装桌面端。',
      ),
    ];

/// 步骤的页内直达按钮口径（Kotlin `onboardActionFor` 直译）。
///
/// @return 文案 to 去向标识（`"settings"` = 去设置页；`"home"` = 回主页），
///         null = 无按钮。
OnboardAction? onboardActionFor(String stepId) => switch (stepId) {
      'connect' => const OnboardAction('去首页发现列表', 'home'),
      'pair' => const OnboardAction('去首页', 'home'),
      'first-task' => const OnboardAction('去发第一条指令', 'home'),
      'trouble' => const OnboardAction('去设置填写地址', 'settings'),
      _ => null,
    };

/// 步骤图标（Kotlin `stepIcon`：细线 lucide → Flutter Material 语义对位图标）。
IconData stepIcon(String id) => switch (id) {
      'connect' => Icons.desktop_windows_rounded,
      'pair' => Icons.shield_rounded,
      'first-task' => Icons.send_rounded,
      _ => Icons.info_outline_rounded,
    };

/// 步骤标题（口语化；编号即顺序）。
String stepTitle(String id) => switch (id) {
      'connect' => '第一步 · 让手机找到电脑',
      'pair' => '第二步 · 配对',
      'first-task' => '第三步 · 发第一条指令',
      _ => '连不上?',
    };

/// 教程内容块（数据驱动，便于后续加页）。
sealed class OnboardBlock {
  const OnboardBlock();
}

/// 一段引导说明文字。
class OnboardLead extends OnboardBlock {
  const OnboardLead(this.text);

  final String text;
}

/// 带图标的小卡片：标题 + 副说明。
class OnboardBullet extends OnboardBlock {
  const OnboardBullet({this.icon, required this.title, this.desc});

  final IconData? icon;
  final String title;
  final String? desc;
}

/// 提示卡（Info 样式）。
class OnboardNote extends OnboardBlock {
  const OnboardNote(this.text);

  final String text;
}

/// 单页教程（数据驱动）；[done] = 这一步已完成，重看时只标注、不再展开打扰。
class OnboardPage {
  const OnboardPage({
    required this.icon,
    required this.title,
    required this.blocks,
    this.done = false,
    this.actionLabel,
    this.actionId,
  });

  final IconData icon;
  final String title;
  final List<OnboardBlock> blocks;
  final bool done;

  /// 主操作按钮文案（null = 无按钮）。按钮只做页内直达，不承载业务逻辑。
  final String? actionLabel;

  /// 按钮去向标识（`"settings"` = 去设置页；`"home"` = 回主页）。调用方按标识接线。
  final String? actionId;
}

/// 教程页清单：欢迎页 + 引导脊柱（[onboardSteps]）+ 收尾页。
/// 后续加页只需在这里追加一个 [OnboardPage]，UI 与指示点会自动适配。
List<OnboardPage> onboardPages(bool connected) => <OnboardPage>[
      const OnboardPage(
        icon: Icons.auto_awesome_rounded,
        title: '欢迎使用 PocketAgent',
        blocks: <OnboardBlock>[
          OnboardLead('用手机,遥控你电脑上的 AI 干活。'),
          OnboardBullet(
            icon: Icons.smartphone_rounded,
            title: '一共三步',
            desc: '连上电脑 → 配对 → 发第一条指令,每步一分钟。',
          ),
          OnboardNote('不着急的话,随时点右上角「跳过」;以后在侧边栏的「新手教程」可以重看。'),
        ],
      ),
      // 脊柱页逐条展开（顺序 = onboardingSteps 的顺序）
      for (final OnboardStep step in onboardSteps(connected)) _stepPage(step),
      const OnboardPage(
        icon: Icons.check_circle_rounded,
        title: '一切就绪',
        // 收尾页给直达按钮：点后回主页发第一条指令（调用方接线）。
        actionLabel: '去发第一条指令',
        actionId: 'home',
        blocks: <OnboardBlock>[
          OnboardLead('三步走完,去发第一条指令吧。'),
          OnboardNote('以后想重看这份教程:侧边栏 →「新手教程」。'),
        ],
      ),
    ];

/// 一个引导步骤 → 一页教程。
///
/// 前三步 + 排障页直达按钮口径见 [onboardActionFor]（复用 Kotlin 的
/// actionLabel/actionId 机制，调用方按标识接线，排障→设置、其余→主页）。
OnboardPage _stepPage(OnboardStep step) {
  final OnboardAction? onboardAction = onboardActionFor(step.id);
  return OnboardPage(
    icon: stepIcon(step.id),
    title: stepTitle(step.id),
    done: step.done,
    actionLabel: onboardAction?.label,
    actionId: onboardAction?.target,
    blocks: <OnboardBlock>[
      // 已完成的步骤：一句话告知并请用户直接翻页，不重复展开讲
      if (step.done && step.note.isNotEmpty)
        OnboardLead(step.note)
      else
        OnboardLead(step.sentence),
      OnboardBullet(
        icon: step.done ? Icons.check_circle_rounded : Icons.check_rounded,
        title: step.done ? '可以直接下一步' : '现在做什么',
        desc: step.action,
      ),
    ],
  );
}
