import 'package:flutter/material.dart';

import 'palette.dart';
import 'radii.dart';
import 'typography.dart';

/// 把 [PAColors] 挂进 [ThemeData]，业务组件经 `context.pa` 取整套 token。
@immutable
class PAColorsExtension extends ThemeExtension<PAColorsExtension> {
  const PAColorsExtension({required this.colors});

  final PAColors colors;

  @override
  PAColorsExtension copyWith({PAColors? colors}) =>
      PAColorsExtension(colors: colors ?? this.colors);

  /// 亮暗切换按整套 token 整体替换，不做逐色插值（避免中间态脏色）。
  @override
  PAColorsExtension lerp(
    ThemeExtension<PAColorsExtension>? other,
    double t,
  ) {
    if (other is! PAColorsExtension) return this;
    return t < 0.5 ? this : other;
  }
}

/// 从当前 [Theme] 取设计系统颜色 token；缺失时按亮度回退，保证测试可用。
extension PAContextTheme on BuildContext {
  PAColors get pa {
    final ThemeData theme = Theme.of(this);
    final PAColorsExtension? ext = theme.extension<PAColorsExtension>();
    if (ext != null) return ext.colors;
    return theme.brightness == Brightness.light ? PAColors.light : PAColors.dark;
  }
}

/// ThemeData 组装：Material 3、暗色为默认、浅色可用。
abstract final class PocketTheme {
  /// 工程默认亮度（方案 §二「暗色为主」）。
  static const PABrightness defaultBrightness = PABrightness.dark;

  static ThemeData fromBrightness(PABrightness brightness) {
    final bool isDark = brightness == PABrightness.dark;
    final PAColors pa = PAColors.of(brightness);
    final Brightness br = isDark ? Brightness.dark : Brightness.light;

    final ColorScheme scheme = ColorScheme.fromSeed(
      seedColor: pa.brand,
      brightness: br,
    ).copyWith(
      primary: pa.brand,
      onPrimary: pa.onBrand,
      secondary: pa.agentAccent,
      surface: pa.bgSurface,
      onSurface: pa.fgPrimary,
      outline: pa.hairline,
      error: pa.danger,
      onError: pa.onBrand,
      surfaceContainer: pa.bgSurface,
      surfaceContainerHigh: pa.bgSurface2,
      surfaceContainerHighest: pa.bgSurface2,
    );

    final ThemeData base = ThemeData(
      useMaterial3: true,
      brightness: br,
      colorScheme: scheme,
      scaffoldBackgroundColor: pa.bgBase,
      canvasColor: pa.bgBase,
      cardColor: pa.bgSurface,
      dividerColor: pa.hairline,
      splashFactory: InkSparkle.splashFactory,
    );

    return base.copyWith(
      textTheme: _textTheme(base.textTheme, pa),
      appBarTheme: AppBarTheme(
        backgroundColor: pa.bgBase,
        foregroundColor: pa.fgPrimary,
        elevation: 0,
        scrolledUnderElevation: 0,
        surfaceTintColor: Colors.transparent,
        centerTitle: false,
        titleTextStyle: PATypography.titleM.copyWith(color: pa.fgPrimary),
      ),
      navigationBarTheme: NavigationBarThemeData(
        backgroundColor: pa.bgSurface,
        surfaceTintColor: Colors.transparent,
        indicatorColor: pa.brand.withValues(alpha: 0.16),
        height: 64,
        labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
        iconTheme: WidgetStateProperty.resolveWith<IconThemeData>(
          (Set<WidgetState> states) => IconThemeData(
            size: 22,
            color: states.contains(WidgetState.selected)
                ? pa.brand
                : pa.fgTertiary,
          ),
        ),
        labelTextStyle: WidgetStateProperty.resolveWith<TextStyle>(
          (Set<WidgetState> states) => PATypography.caption.copyWith(
            fontWeight:
                states.contains(WidgetState.selected) ? FontWeight.w600 : null,
            color: states.contains(WidgetState.selected)
                ? pa.brand
                : pa.fgTertiary,
          ),
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: pa.brand,
          foregroundColor: pa.onBrand,
          disabledBackgroundColor: pa.fgTertiary.withValues(alpha: 0.3),
          disabledForegroundColor: pa.fgTertiary,
          textStyle: PATypography.bodyEmphasis,
          minimumSize: const Size(64, 44),
          padding: const EdgeInsets.symmetric(horizontal: 18),
          shape: const RoundedRectangleBorder(
            borderRadius: PARadii.buttonBR,
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: pa.fgPrimary,
          side: BorderSide(color: pa.hairline, width: 1),
          textStyle: PATypography.bodyEmphasis,
          minimumSize: const Size(64, 44),
          padding: const EdgeInsets.symmetric(horizontal: 18),
          shape: const RoundedRectangleBorder(
            borderRadius: PARadii.buttonBR,
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: pa.brand,
          textStyle: PATypography.bodyEmphasis,
        ),
      ),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: pa.bgSurface,
        surfaceTintColor: Colors.transparent,
        shape: const RoundedRectangleBorder(
          borderRadius: PARadii.sheetTopBR,
        ),
      ),
      chipTheme: base.chipTheme.copyWith(
        backgroundColor: pa.bgSurface2,
        side: BorderSide.none,
        labelStyle: PATypography.caption.copyWith(color: pa.fgSecondary),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: pa.bgSurface2,
        hintStyle: PATypography.body.copyWith(color: pa.fgTertiary),
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        border: const OutlineInputBorder(
          borderRadius: PARadii.buttonBR,
          borderSide: BorderSide.none,
        ),
      ),
      dividerTheme: DividerThemeData(
        color: pa.hairline,
        thickness: 1,
        space: 1,
      ),
      extensions: <ThemeExtension<dynamic>>[
        PAColorsExtension(colors: pa),
      ],
    );
  }

  static TextTheme _textTheme(TextTheme base, PAColors pa) {
    TextStyle apply(TextStyle s, Color color) => s.copyWith(color: color);
    return base.copyWith(
      headlineSmall: apply(PATypography.titleL, pa.fgPrimary),
      titleLarge: apply(PATypography.titleL, pa.fgPrimary),
      titleMedium: apply(PATypography.titleM, pa.fgPrimary),
      titleSmall: apply(PATypography.titleM, pa.fgPrimary),
      bodyLarge: apply(PATypography.body, pa.fgPrimary),
      bodyMedium: apply(PATypography.bodySmall, pa.fgPrimary),
      bodySmall: apply(PATypography.caption, pa.fgSecondary),
      labelLarge: apply(PATypography.body, pa.fgPrimary),
      labelMedium: apply(PATypography.caption, pa.fgSecondary),
      labelSmall: apply(PATypography.caption, pa.fgTertiary),
    );
  }
}
