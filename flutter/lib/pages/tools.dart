import 'package:flutter/material.dart';

import '../components/pa_approval_sheet.dart';
import '../components/pa_button.dart';
import '../components/pa_status_chip.dart';
import '../components/pa_tool_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';

/// 工具页：折叠工具卡与审批卡的设计系统演示（纯静态，无网络/原生通道）。
class ToolsPage extends StatelessWidget {
  const ToolsPage({super.key});

  Future<void> _openSheet(
    BuildContext context, {
    required bool destructive,
  }) async {
    final bool? result = await PAApprovalSheet.show(
      context,
      PAApprovalSheet(
        title: destructive ? '删除构建产物' : '执行 shell 命令',
        destructive: destructive,
        riskLabel: destructive ? '破坏性操作 · 不可撤销' : '风险：中',
        detail: destructive
            ? '该操作会永久删除文件，拒绝后可让 agent 改用可逆方案。'
            : 'agent 请求在项目目录内执行一条命令。',
        info: <String, String>{
          '工具': destructive ? 'fs.remove' : 'shell.exec',
          '目录': '/workspace/pocketagent',
          '参数': destructive ? 'rm -rf build/' : 'git status --short',
        },
        onAllow: () => Navigator.of(context).pop(true),
        onDeny: () => Navigator.of(context).pop(false),
        onCancel: () => Navigator.of(context).pop(),
      ),
    );
    if (!context.mounted) return;
    final String text = switch (result) {
      true => '已允许（示例，未真正下发）',
      false => '已拒绝（示例，未真正下发）',
      null => '已取消',
    };
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(text)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.only(left: 4, bottom: 10),
          child: Text(
            '静态演示数据 · 点击卡头展开输出体',
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ),
        const PAToolCard(
          toolName: 'shell.exec',
          argsSummary: 'git status --short',
          duration: '1.2s',
          status: PAStatusKind.done,
          repeatCount: 2,
          output: ' M lib/main.dart\n M lib/theme/palette.dart\n?? flutter/test/',
        ),
        const SizedBox(height: 10),
        const PAToolCard(
          toolName: 'fs.read',
          argsSummary: 'docs/verify/handoff-2026-10-09.md',
          duration: '86ms',
          status: PAStatusKind.running,
        ),
        const SizedBox(height: 10),
        const PAToolCard(
          toolName: 'shell.exec',
          argsSummary: 'go test ./internal/...',
          duration: '12.4s',
          status: PAStatusKind.failed,
          output:
              'FAIL\tdaemon/internal/server\tTestHandshake\twant 200, got 503',
        ),
        const SizedBox(height: 10),
        const PAToolCard(
          toolName: 'fs.write',
          argsSummary: 'android/app/src/main/proto/v1/*.proto',
          duration: '3.0s',
          status: PAStatusKind.warn,
          output: '双副本 MD5 不一致，需执行 tools/proto/gen_proto.py 同步',
        ),
        const SizedBox(height: 22),
        Row(
          children: <Widget>[
            Expanded(
              child: PAButton(
                label: '审批卡 · 常规',
                variant: PAButtonVariant.neutral,
                onPressed: () => _openSheet(context, destructive: false),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: PAButton(
                label: '审批卡 · 破坏性',
                variant: PAButtonVariant.danger,
                onPressed: () => _openSheet(context, destructive: true),
              ),
            ),
          ],
        ),
      ],
    );
  }
}
