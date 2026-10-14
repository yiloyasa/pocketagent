import 'package:flutter/material.dart';

/// 圆角 token：按钮 8 / 卡片 14 / 弹层顶 22 / 头像 10（圆角矩形）。
/// 设计红线：**单屏 ≤3 种圆角**，新增圆角前先看这行。
abstract final class PARadii {
  static const double button = 8;
  static const double card = 14;
  static const double sheet = 22;
  static const double avatar = 10;

  static const BorderRadius buttonBR = BorderRadius.all(Radius.circular(button));
  static const BorderRadius cardBR = BorderRadius.all(Radius.circular(card));
  static const BorderRadius sheetBR = BorderRadius.all(Radius.circular(sheet));
  static const BorderRadius avatarBR = BorderRadius.all(Radius.circular(avatar));

  /// 弹层只圆顶部（底边贴屏，底部不圆角）。
  static const BorderRadius sheetTopBR =
      BorderRadius.vertical(top: Radius.circular(sheet));
}
