import 'package:flutter/material.dart';

import '../components/pa_card.dart';
import '../theme/palette.dart';
import '../theme/pocket_theme.dart';
import '../theme/typography.dart';

/// 文件页：静态占位（设计系统分组行展示），B3 阶段接真实文件树。
class FilesPage extends StatelessWidget {
  const FilesPage({super.key});

  static const List<(IconData, String, String)> _files =
      <(IconData, String, String)>[
    (Icons.folder_rounded, 'android/', '12 项'),
    (Icons.folder_rounded, 'daemon/', '31 项'),
    (Icons.folder_rounded, 'docs/', '48 项'),
    (Icons.folder_rounded, 'proto/', '9 项'),
    (Icons.description_rounded, 'README.md', '12 KB'),
    (Icons.description_rounded, 'CHANGELOG.md', '38 KB'),
    (Icons.data_object_rounded, 'go.mod', '2 KB'),
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
            '静态占位 · 文件树将在 B3 阶段迁移',
            style: PATypography.caption.copyWith(color: pa.fgTertiary),
          ),
        ),
        PACard(
          padding: EdgeInsets.zero,
          child: Column(
            children: <Widget>[
              for (int i = 0; i < _files.length; i++) ...<Widget>[
                if (i > 0)
                  const Padding(
                    padding: EdgeInsets.only(left: 52),
                    child: Divider(),
                  ),
                Padding(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                  child: Row(
                    children: <Widget>[
                      Icon(
                        _files[i].$1,
                        size: 18,
                        color: pa.fgSecondary,
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          _files[i].$2,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: PATypography.code
                              .copyWith(color: pa.fgPrimary),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        _files[i].$3,
                        style:
                            PATypography.caption.copyWith(color: pa.fgTertiary),
                      ),
                    ],
                  ),
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}
