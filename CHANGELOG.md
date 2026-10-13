# CHANGELOG

格式：`Keep a Changelog` 简化版。`[App vX / daemon vX / versionCode N]` 头注明双端版本。
`MinApp` = 该 daemon 要求的最低 App 版本（`HostInfo.min_app_version`）。

## [0.3.1] — 2026-10-13

`[App v0.3.1 / daemon v0.3.1 / versionCode 7 / MinApp 0.3.1]`

补丁版：无契约变更（proto 零改动）。**本仓库只维护最新版，不兼容旧格式**：
`MinAppVersion` 与 `Version` 同步为 0.3.1，旧 App / 旧 daemon 不再保证可连
（App 侧仅提示不硬拦）。双端下载见 GitHub Releases 页（`v0.3.1`：
总 zip + `app-release.apk` + `SHA256SUMS`，见 `tools/release/publish_github.py`）。

### 新增

- 版本双副本同步门禁 `tools/release/check_version.py`（daemon `Version`
  与 App `versionName` 相等断言 + `MinAppVersion <= Version` + 三段式校验）。
- GitHub Release 发布脚本 `tools/release/publish_github.py`（总 zip +
  `app-release.apk` + `SHA256SUMS` 三件一次上传，补上此前缺失的双端下载连接；
  备注取本文件对应版本节）。
- 发版流程：`bump(version.go + gradle)` → `check_version.py` → 测试 →
  `dist/release.ps1` → `publish_github.py --dry-run` 核对 → `git tag vX.Y.Z` →
  `gh release create` + `git push origin vX.Y.Z`。
- 设置 → 关于新增「桌面端」行（daemon 版本，断连时如实标「需连接主机」）。
- 新手教程操作化：排障页地址示例补 `:7246`；收尾页加「去发第一条指令」直达；
  重看教程返回不再重进首启流程。

### 修复

- 新手教程两处「手动填电脑的地址」补端口示例（裸 IP 必连不上）。
- 桌面端 `install.bat` 收尾追加三行指引（等 1 分钟自动发现 → 手填 `IP:7246` →
  `pair-code.txt` 路径再打一遍）。
- `QUICKSTART.md` 安装窗口说明与 `install.bat` 的 `pause` 对齐（按提示按键退出）。

### 已知事项（下轮契约修）

- `proto/v1/host.proto` `AuthorizeRequest.pair_code` 注释残留「6 位」，
  全仓实际为 8 位。注释改动需双副本同步 + 重新生成，为避开补丁版甲级审核，
  留到下次契约变更时一并改。

### 兼容矩阵

见 `docs/verify/compat-matrix-0.3.1.md`。

## [0.3.0] — 2026-10-11

`[App v0.3.0 / daemon v0.3.0 / versionCode 6 / MinApp 0.3.0]`

- UI 收尾轮：撤销互斥 / 48dp 触控 / 减弱动画即时响应 / 键盘避让 /
  小屏大字适配 / 底部运行条让位；oaHTTP 看门狗竞态修复。
- Android 1799 例绿，daemon 29 包绿、12 包 race 通过。
- L2 最终真机验收 14 PASS（`docs/verify/evidence/l2-final-20261011/`）。
- 引入 `MinAppVersion` 版本协商闸（仅提示不阻断）。
- 已知：`dist/windows/build/version.txt` 残留 `0.2.2` 为本地暂存产物，
  以仓库 `internal/version` 为准（0.3.1 起由 `check_version.py` 守护，
  不再依赖该暂存文件）。
