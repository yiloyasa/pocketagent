/// 新手教程「看过没看过」的持久化标志（Kotlin `OnboardingPrefs` 等价抽象）。
///
/// Kotlin 侧是独立 SharedPreferences 文件（`pa_onboarding` / `done`），`isDone` 读、
/// `markDone` 写，`RootNav` 的 startDestination 由它决定（MainActivity.kt:167 / :227）。
///
/// 【为什么不直接用 shared_preferences】本波是 B3 第 1 波的**低依赖部分**：任务红线
/// 明确「不改 pubspec.yaml、不加任何依赖」。因此这里只定义**接口契约** + 一个内存
/// 占位实现；等 B2 通道接入波次引入 `shared_preferences` 后，把
/// `SharedPrefsOnboardingPrefs implements OnboardingPrefs` 挂到 `PocketAgentApp`
/// 即可，调用点（路由判定）一行都不用改。
library;

/// 教程完成标志的持久化来源。
///
/// 为什么抽接口而不写死内存单例：启动路由（首启 → 教程，否则 → 主页）读它，
/// widget 测试要同时覆盖「未完成」与「已完成」两条路径，没有注入点就只能改全局状态。
abstract interface class OnboardingPrefs {
  /// 本机是否已看过教程（false = 首启，走 Onboarding）。
  bool get isDone;

  /// 标记为已看过。
  ///
  /// 返回 Future 是为了让持久化实现（写 SharedPreferences / 文件）可以声明成
  /// 异步；内存占位实现同步完成。
  Future<void> markDone();
}

/// 内存占位实现（B2 持久化接入前的默认值）。
///
/// TODO(B3 后续波次 · 持久化)：引入 `shared_preferences` 后新增
/// `SharedPrefsOnboardingPrefs`（对应 Kotlin 的 `pa_onboarding` 独立偏好文件），
/// 由 `PocketAgentApp` 注入；本类保留给单测与预览使用。
class InMemoryOnboardingPrefs implements OnboardingPrefs {
  // 字段必须可变（markDone 要写），不能用 `this._done` 初始化形参；
  // 改公开参数名又会破坏调用点语义（done 更直白）。
  // ignore: prefer_initializing_formals
  InMemoryOnboardingPrefs({bool done = false}) : _done = done;

  bool _done;

  @override
  bool get isDone => _done;

  @override
  Future<void> markDone() async {
    _done = true;
  }
}
