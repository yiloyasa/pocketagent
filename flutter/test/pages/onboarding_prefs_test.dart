import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/pages/onboarding_prefs.dart';

/// `OnboardingPrefs` 契约测试（Kotlin `OnboardingPrefs.isDone/markDone` 语义对位）。
///
/// 为什么值得单测：启动路由「首启 → Onboarding，否则 → HomeShell」完全由它决定，
/// 默认值错了就是「每次冷启都重看教程」或「新用户永远看不到教程」。
void main() {
  test('默认未完成_首启必须看到教程', () {
    expect(InMemoryOnboardingPrefs().isDone, isFalse);
  });

  test('markDone 后 isDone 恒为 true 且可重复调用', () async {
    final InMemoryOnboardingPrefs prefs = InMemoryOnboardingPrefs();
    expect(prefs.isDone, isFalse);
    await prefs.markDone();
    expect(prefs.isDone, isTrue);
    // 幂等：重复标记不翻回 false
    await prefs.markDone();
    expect(prefs.isDone, isTrue);
  });

  test('可按初始完成态构造_对应「下次启动已看过」', () {
    expect(InMemoryOnboardingPrefs(done: true).isDone, isTrue);
  });

  test('不同实例互不共享_占位实现不是全局单例', () async {
    final InMemoryOnboardingPrefs a = InMemoryOnboardingPrefs();
    final InMemoryOnboardingPrefs b = InMemoryOnboardingPrefs();
    await a.markDone();
    expect(a.isDone, isTrue);
    expect(b.isDone, isFalse);
  });
}
