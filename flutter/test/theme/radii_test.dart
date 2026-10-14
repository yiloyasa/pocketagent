import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pocketagent_ui/theme/radii.dart';

void main() {
  test('圆角 token 与方案一致：按钮 8 / 卡片 14 / 弹层 22 / 头像 10', () {
    expect(PARadii.button, 8);
    expect(PARadii.card, 14);
    expect(PARadii.sheet, 22);
    expect(PARadii.avatar, 10);
  });

  test('圆角 token 全部为正且单调有序， BorderRadius 常量对得上', () {
    for (final double r in <double>[
      PARadii.button,
      PARadii.card,
      PARadii.sheet,
      PARadii.avatar,
    ]) {
      expect(r, greaterThan(0), reason: '圆角必须为正值');
      expect(r.isFinite, isTrue);
    }

    expect(PARadii.button, lessThan(PARadii.avatar));
    expect(PARadii.avatar, lessThan(PARadii.card));
    expect(PARadii.card, lessThan(PARadii.sheet));

    expect(
      PARadii.buttonBR,
      const BorderRadius.all(Radius.circular(8)),
    );
    expect(
      PARadii.cardBR,
      const BorderRadius.all(Radius.circular(14)),
    );
    expect(
      PARadii.sheetBR,
      const BorderRadius.all(Radius.circular(22)),
    );
    expect(
      PARadii.avatarBR,
      const BorderRadius.all(Radius.circular(10)),
    );
  });
}
