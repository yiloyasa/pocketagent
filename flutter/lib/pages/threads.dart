import 'package:flutter/material.dart';

import '../components/pa_card.dart';
import '../components/pa_status_chip.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';

/// 会话列表：agent 圆角矩形头像 + 标题 + 摘要 + 状态 chip + 时间。
/// 静态骨架，数据为示例，B3 阶段接真实会话流。
class ThreadsPage extends StatelessWidget {
  const ThreadsPage({super.key});

  static const List<_ThreadItem> _threads = <_ThreadItem>[
    _ThreadItem(
      tag: 'cx',
      title: '修复登录页 token 过期跳转',
      summary: '已改完 refresh 拦截器，正在跑单测',
      status: PAStatusKind.running,
      time: '14:02',
    ),
    _ThreadItem(
      tag: 'pi',
      title: '给 daemon 加配对码速率限制',
      summary: '5 次失败后指数退避，proto 无需改动',
      status: PAStatusKind.done,
      time: '13:41',
    ),
    _ThreadItem(
      tag: 'ds',
      title: '重构文件树虚拟列表',
      summary: '超出 5000 节点时首屏卡顿，需要分片加载',
      status: PAStatusKind.warn,
      time: '11:27',
    ),
    _ThreadItem(
      tag: 'zc',
      title: '同步 android 副本 proto',
      summary: 'gen_proto.py --check 报 MD5 不一致',
      status: PAStatusKind.failed,
      time: '昨天',
    ),
    _ThreadItem(
      tag: 'hm',
      title: '写 0.4.0 发布说明',
      summary: '等 daemon 侧文档定稿后合并',
      status: PAStatusKind.pending,
      time: '昨天',
    ),
    _ThreadItem(
      tag: 'cx',
      title: '终端页 ANSI 色彩还原',
      summary: 'OSC 256 色已支持，真彩序列待补',
      status: PAStatusKind.done,
      time: '10-12',
    ),
  ];

  @override
  Widget build(BuildContext context) {
    final PAColors pa = context.pa;
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.only(left: 4, bottom: 10),
          child: Text(
            '静态示例数据 · B3 阶段接入真实会话',
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ),
        for (final _ThreadItem t in _threads)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: PACard(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
              onTap: () {},
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Container(
                    width: 40,
                    height: 40,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: pa.agentAccent.withValues(alpha: 0.16),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Text(
                      t.tag,
                      style: PATypography.bodySmall.copyWith(
                        color: pa.agentAccent,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        Row(
                          children: <Widget>[
                            Expanded(
                              child: Text(
                                t.title,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: PATypography.body.copyWith(
                                  color: pa.fgPrimary,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                            ),
                            const SizedBox(width: 8),
                            Text(
                              t.time,
                              style: PATypography.caption
                                  .copyWith(color: pa.fgTertiary),
                            ),
                          ],
                        ),
                        const SizedBox(height: 3),
                        Row(
                          children: <Widget>[
                            Expanded(
                              child: Text(
                                t.summary,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: PATypography.bodySmall
                                    .copyWith(color: pa.fgSecondary),
                              ),
                            ),
                            const SizedBox(width: 8),
                            PAStatusChip(status: t.status),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

class _ThreadItem {
  const _ThreadItem({
    required this.tag,
    required this.title,
    required this.summary,
    required this.status,
    required this.time,
  });

  final String tag;
  final String title;
  final String summary;
  final PAStatusKind status;
  final String time;
}
