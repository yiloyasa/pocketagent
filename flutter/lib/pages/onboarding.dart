import 'package:flutter/material.dart';

import '../components/pa_button.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/radii.dart';
import '../theme/typography.dart';

/// 3 页横滑引导：这台电脑是什么 / 怎么连 / 发第一条指令。
/// 底部 = 跳过 + 圆点 + 下一步（末页「开始使用」）。
class OnboardingPage extends StatefulWidget {
  const OnboardingPage({super.key, this.onFinished});

  /// 完成回调；为空时关闭本页。
  final VoidCallback? onFinished;

  @override
  State<OnboardingPage> createState() => _OnboardingPageState();
}

class _OnboardingStep {
  const _OnboardingStep({
    required this.icon,
    required this.title,
    required this.body,
    required this.bullets,
  });

  final IconData icon;
  final String title;
  final String body;
  final List<String> bullets;
}

const List<_OnboardingStep> _steps = <_OnboardingStep>[
  _OnboardingStep(
    icon: Icons.computer_rounded,
    title: '这台电脑是什么',
    body: 'PocketAgent 电脑端 daemon 跑在你的电脑上，手机只是遥控器。',
    bullets: <String>[
      '零云服务器：手机直连本机，不经过任何第三方',
      '桥接 codex / pi / dsh / zcode / hermes 等原生 agent',
      '你的代码与凭据始终留在电脑里',
    ],
  ),
  _OnboardingStep(
    icon: Icons.link_rounded,
    title: '怎么连',
    body: '电脑端启动后生成 8 位配对码，手机输入配对码即可建立 TLS 连接。',
    bullets: <String>[],
  ),
  _OnboardingStep(
    icon: Icons.rocket_launch_rounded,
    title: '发第一条指令',
    body: '选中一个会话，像发消息一样把任务交给 agent。',
    bullets: <String>[
      '工具调用与审批都会以折叠卡片回显在手机上',
      '危险操作走底部审批卡，由你决定放行还是拒绝',
    ],
  ),
];

class _OnboardingPageState extends State<OnboardingPage> {
  final PageController _controller = PageController();
  int _index = 0;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _finish() {
    final VoidCallback? cb = widget.onFinished;
    if (cb != null) {
      cb();
      return;
    }
    Navigator.of(context).maybePop();
  }

  void _next() {
    if (_index >= _steps.length - 1) {
      _finish();
      return;
    }
    _controller.nextPage(
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
    );
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    final bool last = _index == _steps.length - 1;

    return Scaffold(
      backgroundColor: pa.bgBase,
      body: SafeArea(
        child: Column(
          children: <Widget>[
            Expanded(
              child: PageView.builder(
                controller: _controller,
                itemCount: _steps.length,
                onPageChanged: (int i) => setState(() => _index = i),
                itemBuilder: (BuildContext context, int i) =>
                    _OnboardingStepView(step: _steps[i]),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 20),
              child: Row(
                children: <Widget>[
                  TextButton(
                    onPressed: _finish,
                    child: Text(
                      '跳过',
                      style: PATypography.body.copyWith(color: pa.fgTertiary),
                    ),
                  ),
                  Expanded(
                    child: Center(
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: List<Widget>.generate(
                          _steps.length,
                          (int i) => AnimatedContainer(
                            duration: const Duration(milliseconds: 200),
                            margin: const EdgeInsets.symmetric(horizontal: 4),
                            width: i == _index ? 18 : 6,
                            height: 6,
                            decoration: BoxDecoration(
                              color:
                                  i == _index ? pa.brand : pa.fgTertiary.withValues(alpha: 0.4),
                              borderRadius: BorderRadius.circular(3),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                  SizedBox(
                    width: 116,
                    child: PAButton(
                      label: last ? '开始使用' : '下一步',
                      onPressed: _next,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _OnboardingStepView extends StatelessWidget {
  const _OnboardingStepView({required this.step});

  final _OnboardingStep step;

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 28),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Container(
            width: 88,
            height: 88,
            decoration: BoxDecoration(
              color: pa.bgSurface2,
              borderRadius: BorderRadius.circular(PARadii.card),
              border: Border.all(color: pa.hairline, width: 1),
            ),
            child: Icon(step.icon, size: 40, color: pa.agentAccent),
          ),
          const SizedBox(height: 28),
          Text(
            step.title,
            style: PATypography.titleL.copyWith(color: pa.fgPrimary),
          ),
          const SizedBox(height: 12),
          Text(
            step.body,
            style: PATypography.body.copyWith(color: pa.fgSecondary),
          ),
          if (step.bullets.isNotEmpty) ...<Widget>[
            const SizedBox(height: 20),
            for (final String b in step.bullets)
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Icon(Icons.check_rounded, size: 16, color: pa.ok),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        b,
                        style:
                            PATypography.bodySmall.copyWith(color: pa.fgSecondary),
                      ),
                    ),
                  ],
                ),
              ),
          ],
        ],
      ),
    );
  }
}
