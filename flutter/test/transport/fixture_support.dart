// transport 测试的共享装配：fixture 字节读取。
//
// 【为什么放在 test/transport/ 而不是 test/support/】test/support/ 归
// pump_helpers 域（widget 测试的 Splash 时钟推进，且 import lib/pages），
// 通道测试不该把 widget 域的依赖拖进来。transport 三份测试文件
// （pa_bridge_test / pa_bridge_threads_test / pa_bridge_files_git_test）
// 各写了一份逐字节相同的 fixture 读取，收在这里做唯一出处。
//
// 【为什么不直接 stub PaBridge】要验的是契约本身（见 pa_bridge_test.dart
// 文件头）；本文件只提供字节来源，不含任何宿主行为假设。

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';

/// 读 fixture 原始字节（b64 包裹防 git autocrlf 改坏）。
///
/// 两条候选路径对应「仓库根跑 flutter test」与「flutter/ 目录里跑」两种
/// cwd（`flutter test` 与 `dart test` 的 cwd 不同，历史证据见
/// pa_bridge_test.dart 的既有实现）。都不中即 fail，绝不静默跳过——
/// 静默跳过会让「字节没读到」伪装成「解码断言通过」（假绿）。
Uint8List fixtureBytes(String name) {
  final candidates = <String>[
    '${Directory.current.path}/tools/protofixture/fixtures/$name.b64',
    '${Directory.current.path}/flutter/tools/protofixture/fixtures/$name.b64',
  ];
  for (final c in candidates) {
    final f = File(c);
    if (f.existsSync()) {
      return base64.decode(f.readAsStringSync().trim());
    }
  }
  fail('找不到 fixture $name（试过：${candidates.join(', ')}；'
      'cwd=${Directory.current.path}）');
}
