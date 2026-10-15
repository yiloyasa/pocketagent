// 协同角色配置的数据层：RPC 拉取/保存/删除 + 可观察状态（Flutter 版
// CollabRolesState）。
//
// 【对位 Kotlin object，Dart 侧为什么改成可注入类】Android 的
// `CollabRolesState` 是 app 级 object（跨页共享：CollabRolesScreen 与
// CollabRoleEditorScreen 都直接摸它，CollabScreen 也读它的只读投影）。
// Dart 没有 `object` 单例语法，而全局单例会让测试之间串味（一个测试留下的
// roles/loadState 会影响下一个）——**测试隔离优先**：这里给可注入的 store
// 类，页面构造注入，生产由根装配持一个共享实例（B4 接线点在 main.dart 的
// 入口处）。方法面与 Kotlin 逐条对位（load/save/delete + 三态字段），
// 届时提 app 级单例只是「谁来持有实例」的变化，方法面不动。
//
// 【与 collab_roles_data.dart 的分工】本文件只管「通道 + 状态」，所有展示
// 规则（id 校验/显示名/摘要）都在 data 层，单测不依赖真机/daemon。
//
// 【错误透传】daemon 返回的 InvalidArgument 等人话文案，经 [extractRpcMessage]
// 只取 PaBridgeException.message（干净的人话，不含 "INVALID_ARGUMENT:" 前缀），
// 绝不泛化成「操作失败」（AUDIT §3.3：禁显「未知/错误」裸字样）。

import '../gen/pocketagent/v1/thread.pb.dart';
import '../transport/pa_bridge.dart';

/// 角色列表的加载状态（纯枚举，UI 据此区分 加载中 / 就绪(含空) / 失败）。
///
/// 对位 Kotlin `CollabRolesLoadState`（LOADING/READY/ERROR 三档，一个不少）。
enum CollabRolesLoadState { loading, ready, error }

/// 协同角色 store：三态 + roles + 三个 RPC 动作。
///
/// 【为什么 save/delete 返回错误串而不是抛】页面需要「失败时把原因显示在
/// 原处」（编辑页的 saveError、列表页删除后的提示），且失败**不改变**三态
/// （一次保存失败不该把整页打成错误态）。返回 `null` = 成功；否则返回 daemon
/// 原文（人话），调用方决定摆哪。与 Kotlin `save(): String?` 同款形状。
class CollabRolesStore {
  CollabRolesStore({PaBridge? bridge}) : _bridge = bridge ?? PaBridge();

  final PaBridge _bridge;

  List<CollabRole> _roles = <CollabRole>[];

  /// 角色列表（daemon 按 id 升序返回，这里不重排——与 Kotlin 的
  /// mutableStateOf 字段同语义：写操作后本地同步更新，UI 直接读）。
  ///
  /// 返回不可修改视图：调用方改列表必须走 save/delete（否则与 daemon 真值
  /// 漂移，回显 stale）。
  List<CollabRole> get roles => List<CollabRole>.unmodifiable(_roles);

  CollabRolesLoadState _loadState = CollabRolesLoadState.loading;

  CollabRolesLoadState get loadState => _loadState;

  /// 最近一次加载失败的原文（成功/加载中为空），供 UI 如实展示。
  String? _loadError;

  String? get loadError => _loadError;

  /// 拉取角色列表（幂等：空列表 = 用户还没配过任何角色，不是错误，UI 走
  /// 空态文案）。进列表页与从编辑器返回都会调一次；失败只更新状态，不抛。
  ///
  /// 【为什么 fire-and-forget（返回 void 而不是 Future）】Kotlin 的 load()
  /// 是 `scope.launch`（页面 LaunchedEffect 不等它）；Dart 侧页面同样在
  /// initState 里调，状态变化经 setState 落 UI。测试要等它，直接 await 内部
  /// 的 [_load] 路径——不，测试要确定性：见 [_loadFuture]（进行中的加载只
  /// 保留一个，重复调用返回同一个 Future，测试可 await）。
  void load() {
    _loadState = CollabRolesLoadState.loading;
    _loadError = null;
    // 记录在飞加载：并发调用共享同一个 Future（Kotlin 侧由 LaunchedEffect 的
    // 单次性天然保证，Dart 侧显式记录）。
    _loadFuture = _doLoad();
  }

  late Future<void> _loadFuture = _doLoad();

  /// 当前（或最近一次）加载的 Future——测试据此 await 到状态收敛，
  /// 不用 sleep 碰运气。
  Future<void> get loadCompleted => _loadFuture;

  Future<void> _doLoad() async {
    try {
      final ListCollabRolesResponse response = await _bridge.listCollabRoles();
      _roles = response.roles.toList(growable: false);
      _loadState = CollabRolesLoadState.ready;
      _loadError = null;
    } on PaBridgeException catch (e) {
      _loadError = extractRpcMessage(e);
      _loadState = CollabRolesLoadState.error;
    } catch (e) {
      // 兜底：非通道异常（理论上 _invoke 已全部收敛成 PaBridgeException）也
      // 不许裸抛给页面——进错误态，文案如实带类型名（可核查的真实信息）。
      _loadError = extractRpcMessage(e);
      _loadState = CollabRolesLoadState.error;
    }
  }

  /// 保存（新增/编辑共用）：按 id 幂等 upsert。成功返回 null，失败返回
  /// daemon 原文。
  ///
  /// 调用方负责客户端前置校验（见 [validateCollabRoleId]），这里只透传后端
  /// 错误。成功后用落库后的真值（daemon 可能 trim 过 id）刷新本地列表，
  /// 避免回显 stale——与 Kotlin `save()` 同款（`roles.filter { it.id !=
  /// saved.id } + saved`）。
  Future<String?> save(CollabRole role) async {
    try {
      final SetCollabRoleResponse response = await _bridge.setCollabRole(role);
      final CollabRole saved = response.role;
      _roles = <CollabRole>[
        ..._roles.where((CollabRole r) => r.id != saved.id),
        saved,
      ];
      return null;
    } on PaBridgeException catch (e) {
      return extractRpcMessage(e);
    } catch (e) {
      return extractRpcMessage(e);
    }
  }

  /// 删除（幂等：删不存在的 id，daemon 不报错；本地直接移除）。
  /// 成功返回 null；极端传输错误返回原文。
  Future<String?> delete(String id) async {
    try {
      await _bridge.deleteCollabRole(id);
      _roles = _roles.where((CollabRole r) => r.id != id).toList(growable: false);
      return null;
    } on PaBridgeException catch (e) {
      return extractRpcMessage(e);
    } catch (e) {
      return extractRpcMessage(e);
    }
  }

  /// 按 id 找角色（编辑器进页预填用；找不到 = 可能已被删除）。
  ///
  /// 【为什么 store 提供这个而不是页面自己扫 roles】「找不到角色」的降级
  /// 路径（Kotlin 显示「找不到该角色(可能已被删除)」）两处都要用（进页预填
  /// + 深链），收在 store 里口径只有一个。
  CollabRole? findById(String id) {
    for (final CollabRole r in _roles) {
      if (r.id == id) return r;
    }
    return null;
  }

  /// 异常 → 可透传原文：优先 PaBridgeException.message（daemon 人话原文，
  /// gRPC 状态码前缀已在 Kotlin 宿主侧剥掉）；空白退 `toString()`；
  /// 再空白退类名（可核查的真实信息，非「未知错误」）。
  ///
  /// 对位 Kotlin `extractRpcMessage`（CollabRolesState.kt:104-108）：
  /// 那边优先取 `Status.description`，这边优先取异常 message——两种宿主里
  /// 「干净人话」的落点不同（gRPC Status vs PlatformException），语义同源。
  static String extractRpcMessage(Object error) {
    if (error is PaBridgeException) {
      final String message = error.message;
      if (message.trim().isNotEmpty) return message;
    }
    final String text = error.toString();
    if (text.trim().isNotEmpty) return text;
    return error.runtimeType.toString();
  }
}

/// store 的 load/save/delete 错误 → 页面错误态文案（三处动作的错误区共用这一条）。
///
/// 【HOST_UNAVAILABLE 单独定制】B4 前这是常态：点名「协同角色通道未就绪
/// （B4 波次）」让用户知道不是自己操作错了，也不是「没有角色」。其余（含
/// daemon 的业务拒绝）原样透传——不改写、不吞（与 settings/threads/files/
/// git/collab 页同一套透传纪律）；原文空白时退一句兜底人话，不留空条。
String collabRolesErrorText(String? errorText) {
  if (errorText == null || errorText.trim().isEmpty) {
    return '电脑端没有给出原因,请稍后重试。';
  }
  if (errorText.contains(PaBridgeErrorCodes.hostUnavailable) ||
      errorText.contains('B4 波次') ||
      errorText.contains('宿主桥未注册') ||
      // flutter_tester 复刻宿主未注册时的 MissingPluginException 原文
      // （真实设备上 Android 引擎回的也是这句）——同一条诚实路径。
      errorText.contains('No implementation found')) {
    return '桌面端协同角色通道未就绪（B4 波次）：Kotlin 宿主尚未注册 bridge 通道，'
        '协同角色暂不可用。';
  }
  return errorText;
}
