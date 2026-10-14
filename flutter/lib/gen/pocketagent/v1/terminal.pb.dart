// This is a generated file - do not edit.
//
// Generated from v1/terminal.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

enum TerminalClientMessage_Msg { open, input, resize, notSet }

class TerminalClientMessage extends $pb.GeneratedMessage {
  factory TerminalClientMessage({
    OpenTerminal? open,
    TerminalInput? input,
    TerminalResize? resize,
  }) {
    final result = TerminalClientMessage._();
    if (open != null) result.open = open;
    if (input != null) result.input = input;
    if (resize != null) result.resize = resize;
    return result;
  }

  TerminalClientMessage._();

  factory TerminalClientMessage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalClientMessage()..mergeFromBuffer(data, registry);
  factory TerminalClientMessage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalClientMessage()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, TerminalClientMessage_Msg>
      _TerminalClientMessage_MsgByTag = {
    1: TerminalClientMessage_Msg.open,
    2: TerminalClientMessage_Msg.input,
    3: TerminalClientMessage_Msg.resize,
    0: TerminalClientMessage_Msg.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalClientMessage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalClientMessage.$_createMessage)
    ..oo(0, [1, 2, 3])
    ..aOM<OpenTerminal>(1, _omitFieldNames ? '' : 'open',
        subBuilder: OpenTerminal.$_createMessage)
    ..aOM<TerminalInput>(2, _omitFieldNames ? '' : 'input',
        subBuilder: TerminalInput.$_createMessage)
    ..aOM<TerminalResize>(3, _omitFieldNames ? '' : 'resize',
        subBuilder: TerminalResize.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalClientMessage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalClientMessage copyWith(
          void Function(TerminalClientMessage) updates) =>
      super.copyWith((message) => updates(message as TerminalClientMessage))
          as TerminalClientMessage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use TerminalClientMessage() / TerminalClientMessage.new instead')
  static TerminalClientMessage create() => TerminalClientMessage._();
  static $pb.GeneratedMessage $_createMessage() => TerminalClientMessage._();
  @$core.override
  TerminalClientMessage createEmptyInstance() => TerminalClientMessage._();
  @$core.pragma('dart2js:noInline')
  static TerminalClientMessage getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TerminalClientMessage>(
          TerminalClientMessage.$_createMessage);
  static TerminalClientMessage? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  TerminalClientMessage_Msg whichMsg() =>
      _TerminalClientMessage_MsgByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  void clearMsg() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  OpenTerminal get open => $_getN(0);
  @$pb.TagNumber(1)
  set open(OpenTerminal value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOpen() => $_has(0);
  @$pb.TagNumber(1)
  void clearOpen() => $_clearField(1);
  @$pb.TagNumber(1)
  OpenTerminal ensureOpen() => $_ensure(0);

  @$pb.TagNumber(2)
  TerminalInput get input => $_getN(1);
  @$pb.TagNumber(2)
  set input(TerminalInput value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasInput() => $_has(1);
  @$pb.TagNumber(2)
  void clearInput() => $_clearField(2);
  @$pb.TagNumber(2)
  TerminalInput ensureInput() => $_ensure(1);

  @$pb.TagNumber(3)
  TerminalResize get resize => $_getN(2);
  @$pb.TagNumber(3)
  set resize(TerminalResize value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasResize() => $_has(2);
  @$pb.TagNumber(3)
  void clearResize() => $_clearField(3);
  @$pb.TagNumber(3)
  TerminalResize ensureResize() => $_ensure(2);
}

class OpenTerminal extends $pb.GeneratedMessage {
  factory OpenTerminal({
    $core.String? sessionId,
    $core.String? cwd,
    $core.String? shell,
    $core.int? cols,
    $core.int? rows,
    $core.String? envJson,
  }) {
    final result = OpenTerminal._();
    if (sessionId != null) result.sessionId = sessionId;
    if (cwd != null) result.cwd = cwd;
    if (shell != null) result.shell = shell;
    if (cols != null) result.cols = cols;
    if (rows != null) result.rows = rows;
    if (envJson != null) result.envJson = envJson;
    return result;
  }

  OpenTerminal._();

  factory OpenTerminal.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OpenTerminal()..mergeFromBuffer(data, registry);
  factory OpenTerminal.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      OpenTerminal()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OpenTerminal',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: OpenTerminal.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'cwd')
    ..aOS(3, _omitFieldNames ? '' : 'shell')
    ..aI(4, _omitFieldNames ? '' : 'cols')
    ..aI(5, _omitFieldNames ? '' : 'rows')
    ..aOS(6, _omitFieldNames ? '' : 'envJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OpenTerminal clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OpenTerminal copyWith(void Function(OpenTerminal) updates) =>
      super.copyWith((message) => updates(message as OpenTerminal))
          as OpenTerminal;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use OpenTerminal() / OpenTerminal.new instead')
  static OpenTerminal create() => OpenTerminal._();
  static $pb.GeneratedMessage $_createMessage() => OpenTerminal._();
  @$core.override
  OpenTerminal createEmptyInstance() => OpenTerminal._();
  @$core.pragma('dart2js:noInline')
  static OpenTerminal getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<OpenTerminal>(
          OpenTerminal.$_createMessage);
  static OpenTerminal? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get cwd => $_getSZ(1);
  @$pb.TagNumber(2)
  set cwd($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCwd() => $_has(1);
  @$pb.TagNumber(2)
  void clearCwd() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get shell => $_getSZ(2);
  @$pb.TagNumber(3)
  set shell($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasShell() => $_has(2);
  @$pb.TagNumber(3)
  void clearShell() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get cols => $_getIZ(3);
  @$pb.TagNumber(4)
  set cols($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCols() => $_has(3);
  @$pb.TagNumber(4)
  void clearCols() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get rows => $_getIZ(4);
  @$pb.TagNumber(5)
  set rows($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRows() => $_has(4);
  @$pb.TagNumber(5)
  void clearRows() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get envJson => $_getSZ(5);
  @$pb.TagNumber(6)
  set envJson($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasEnvJson() => $_has(5);
  @$pb.TagNumber(6)
  void clearEnvJson() => $_clearField(6);
}

class TerminalInput extends $pb.GeneratedMessage {
  factory TerminalInput({
    $core.List<$core.int>? data,
  }) {
    final result = TerminalInput._();
    if (data != null) result.data = data;
    return result;
  }

  TerminalInput._();

  factory TerminalInput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalInput()..mergeFromBuffer(data, registry);
  factory TerminalInput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalInput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalInput',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalInput.$_createMessage)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'data', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalInput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalInput copyWith(void Function(TerminalInput) updates) =>
      super.copyWith((message) => updates(message as TerminalInput))
          as TerminalInput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TerminalInput() / TerminalInput.new instead')
  static TerminalInput create() => TerminalInput._();
  static $pb.GeneratedMessage $_createMessage() => TerminalInput._();
  @$core.override
  TerminalInput createEmptyInstance() => TerminalInput._();
  @$core.pragma('dart2js:noInline')
  static TerminalInput getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TerminalInput>(
          TerminalInput.$_createMessage);
  static TerminalInput? _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.int> get data => $_getN(0);
  @$pb.TagNumber(1)
  set data($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasData() => $_has(0);
  @$pb.TagNumber(1)
  void clearData() => $_clearField(1);
}

class TerminalResize extends $pb.GeneratedMessage {
  factory TerminalResize({
    $core.int? cols,
    $core.int? rows,
  }) {
    final result = TerminalResize._();
    if (cols != null) result.cols = cols;
    if (rows != null) result.rows = rows;
    return result;
  }

  TerminalResize._();

  factory TerminalResize.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalResize()..mergeFromBuffer(data, registry);
  factory TerminalResize.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalResize()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalResize',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalResize.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'cols')
    ..aI(2, _omitFieldNames ? '' : 'rows')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalResize clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalResize copyWith(void Function(TerminalResize) updates) =>
      super.copyWith((message) => updates(message as TerminalResize))
          as TerminalResize;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TerminalResize() / TerminalResize.new instead')
  static TerminalResize create() => TerminalResize._();
  static $pb.GeneratedMessage $_createMessage() => TerminalResize._();
  @$core.override
  TerminalResize createEmptyInstance() => TerminalResize._();
  @$core.pragma('dart2js:noInline')
  static TerminalResize getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TerminalResize>(
          TerminalResize.$_createMessage);
  static TerminalResize? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get cols => $_getIZ(0);
  @$pb.TagNumber(1)
  set cols($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCols() => $_has(0);
  @$pb.TagNumber(1)
  void clearCols() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get rows => $_getIZ(1);
  @$pb.TagNumber(2)
  set rows($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRows() => $_has(1);
  @$pb.TagNumber(2)
  void clearRows() => $_clearField(2);
}

enum TerminalServerMessage_Msg { opened, output, exit, error, resumed, notSet }

class TerminalServerMessage extends $pb.GeneratedMessage {
  factory TerminalServerMessage({
    TerminalOpened? opened,
    TerminalOutput? output,
    TerminalExit? exit,
    TerminalError? error,
    Resumed? resumed,
  }) {
    final result = TerminalServerMessage._();
    if (opened != null) result.opened = opened;
    if (output != null) result.output = output;
    if (exit != null) result.exit = exit;
    if (error != null) result.error = error;
    if (resumed != null) result.resumed = resumed;
    return result;
  }

  TerminalServerMessage._();

  factory TerminalServerMessage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalServerMessage()..mergeFromBuffer(data, registry);
  factory TerminalServerMessage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalServerMessage()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, TerminalServerMessage_Msg>
      _TerminalServerMessage_MsgByTag = {
    1: TerminalServerMessage_Msg.opened,
    2: TerminalServerMessage_Msg.output,
    3: TerminalServerMessage_Msg.exit,
    4: TerminalServerMessage_Msg.error,
    5: TerminalServerMessage_Msg.resumed,
    0: TerminalServerMessage_Msg.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalServerMessage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalServerMessage.$_createMessage)
    ..oo(0, [1, 2, 3, 4, 5])
    ..aOM<TerminalOpened>(1, _omitFieldNames ? '' : 'opened',
        subBuilder: TerminalOpened.$_createMessage)
    ..aOM<TerminalOutput>(2, _omitFieldNames ? '' : 'output',
        subBuilder: TerminalOutput.$_createMessage)
    ..aOM<TerminalExit>(3, _omitFieldNames ? '' : 'exit',
        subBuilder: TerminalExit.$_createMessage)
    ..aOM<TerminalError>(4, _omitFieldNames ? '' : 'error',
        subBuilder: TerminalError.$_createMessage)
    ..aOM<Resumed>(5, _omitFieldNames ? '' : 'resumed',
        subBuilder: Resumed.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalServerMessage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalServerMessage copyWith(
          void Function(TerminalServerMessage) updates) =>
      super.copyWith((message) => updates(message as TerminalServerMessage))
          as TerminalServerMessage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use TerminalServerMessage() / TerminalServerMessage.new instead')
  static TerminalServerMessage create() => TerminalServerMessage._();
  static $pb.GeneratedMessage $_createMessage() => TerminalServerMessage._();
  @$core.override
  TerminalServerMessage createEmptyInstance() => TerminalServerMessage._();
  @$core.pragma('dart2js:noInline')
  static TerminalServerMessage getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TerminalServerMessage>(
          TerminalServerMessage.$_createMessage);
  static TerminalServerMessage? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  TerminalServerMessage_Msg whichMsg() =>
      _TerminalServerMessage_MsgByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  void clearMsg() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  TerminalOpened get opened => $_getN(0);
  @$pb.TagNumber(1)
  set opened(TerminalOpened value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOpened() => $_has(0);
  @$pb.TagNumber(1)
  void clearOpened() => $_clearField(1);
  @$pb.TagNumber(1)
  TerminalOpened ensureOpened() => $_ensure(0);

  @$pb.TagNumber(2)
  TerminalOutput get output => $_getN(1);
  @$pb.TagNumber(2)
  set output(TerminalOutput value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOutput() => $_has(1);
  @$pb.TagNumber(2)
  void clearOutput() => $_clearField(2);
  @$pb.TagNumber(2)
  TerminalOutput ensureOutput() => $_ensure(1);

  @$pb.TagNumber(3)
  TerminalExit get exit => $_getN(2);
  @$pb.TagNumber(3)
  set exit(TerminalExit value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasExit() => $_has(2);
  @$pb.TagNumber(3)
  void clearExit() => $_clearField(3);
  @$pb.TagNumber(3)
  TerminalExit ensureExit() => $_ensure(2);

  @$pb.TagNumber(4)
  TerminalError get error => $_getN(3);
  @$pb.TagNumber(4)
  set error(TerminalError value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasError() => $_has(3);
  @$pb.TagNumber(4)
  void clearError() => $_clearField(4);
  @$pb.TagNumber(4)
  TerminalError ensureError() => $_ensure(3);

  @$pb.TagNumber(5)
  Resumed get resumed => $_getN(4);
  @$pb.TagNumber(5)
  set resumed(Resumed value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasResumed() => $_has(4);
  @$pb.TagNumber(5)
  void clearResumed() => $_clearField(5);
  @$pb.TagNumber(5)
  Resumed ensureResumed() => $_ensure(4);
}

class TerminalOpened extends $pb.GeneratedMessage {
  factory TerminalOpened({
    $core.String? sessionId,
    $core.String? shell,
    $core.String? cwd,
  }) {
    final result = TerminalOpened._();
    if (sessionId != null) result.sessionId = sessionId;
    if (shell != null) result.shell = shell;
    if (cwd != null) result.cwd = cwd;
    return result;
  }

  TerminalOpened._();

  factory TerminalOpened.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalOpened()..mergeFromBuffer(data, registry);
  factory TerminalOpened.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalOpened()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalOpened',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalOpened.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'shell')
    ..aOS(3, _omitFieldNames ? '' : 'cwd')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalOpened clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalOpened copyWith(void Function(TerminalOpened) updates) =>
      super.copyWith((message) => updates(message as TerminalOpened))
          as TerminalOpened;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TerminalOpened() / TerminalOpened.new instead')
  static TerminalOpened create() => TerminalOpened._();
  static $pb.GeneratedMessage $_createMessage() => TerminalOpened._();
  @$core.override
  TerminalOpened createEmptyInstance() => TerminalOpened._();
  @$core.pragma('dart2js:noInline')
  static TerminalOpened getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TerminalOpened>(
          TerminalOpened.$_createMessage);
  static TerminalOpened? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get shell => $_getSZ(1);
  @$pb.TagNumber(2)
  set shell($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasShell() => $_has(1);
  @$pb.TagNumber(2)
  void clearShell() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get cwd => $_getSZ(2);
  @$pb.TagNumber(3)
  set cwd($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCwd() => $_has(2);
  @$pb.TagNumber(3)
  void clearCwd() => $_clearField(3);
}

class TerminalOutput extends $pb.GeneratedMessage {
  factory TerminalOutput({
    $core.List<$core.int>? data,
  }) {
    final result = TerminalOutput._();
    if (data != null) result.data = data;
    return result;
  }

  TerminalOutput._();

  factory TerminalOutput.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalOutput()..mergeFromBuffer(data, registry);
  factory TerminalOutput.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalOutput()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalOutput',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalOutput.$_createMessage)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'data', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalOutput clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalOutput copyWith(void Function(TerminalOutput) updates) =>
      super.copyWith((message) => updates(message as TerminalOutput))
          as TerminalOutput;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TerminalOutput() / TerminalOutput.new instead')
  static TerminalOutput create() => TerminalOutput._();
  static $pb.GeneratedMessage $_createMessage() => TerminalOutput._();
  @$core.override
  TerminalOutput createEmptyInstance() => TerminalOutput._();
  @$core.pragma('dart2js:noInline')
  static TerminalOutput getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TerminalOutput>(
          TerminalOutput.$_createMessage);
  static TerminalOutput? _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.int> get data => $_getN(0);
  @$pb.TagNumber(1)
  set data($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasData() => $_has(0);
  @$pb.TagNumber(1)
  void clearData() => $_clearField(1);
}

class TerminalExit extends $pb.GeneratedMessage {
  factory TerminalExit({
    $core.String? sessionId,
    $core.int? code,
  }) {
    final result = TerminalExit._();
    if (sessionId != null) result.sessionId = sessionId;
    if (code != null) result.code = code;
    return result;
  }

  TerminalExit._();

  factory TerminalExit.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalExit()..mergeFromBuffer(data, registry);
  factory TerminalExit.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalExit()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalExit',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalExit.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aI(2, _omitFieldNames ? '' : 'code')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalExit clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalExit copyWith(void Function(TerminalExit) updates) =>
      super.copyWith((message) => updates(message as TerminalExit))
          as TerminalExit;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TerminalExit() / TerminalExit.new instead')
  static TerminalExit create() => TerminalExit._();
  static $pb.GeneratedMessage $_createMessage() => TerminalExit._();
  @$core.override
  TerminalExit createEmptyInstance() => TerminalExit._();
  @$core.pragma('dart2js:noInline')
  static TerminalExit getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TerminalExit>(
          TerminalExit.$_createMessage);
  static TerminalExit? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get code => $_getIZ(1);
  @$pb.TagNumber(2)
  set code($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearCode() => $_clearField(2);
}

class TerminalError extends $pb.GeneratedMessage {
  factory TerminalError({
    $core.String? sessionId,
    $core.String? message,
  }) {
    final result = TerminalError._();
    if (sessionId != null) result.sessionId = sessionId;
    if (message != null) result.message = message;
    return result;
  }

  TerminalError._();

  factory TerminalError.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalError()..mergeFromBuffer(data, registry);
  factory TerminalError.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalError()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalError',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalError.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'message')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalError clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalError copyWith(void Function(TerminalError) updates) =>
      super.copyWith((message) => updates(message as TerminalError))
          as TerminalError;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TerminalError() / TerminalError.new instead')
  static TerminalError create() => TerminalError._();
  static $pb.GeneratedMessage $_createMessage() => TerminalError._();
  @$core.override
  TerminalError createEmptyInstance() => TerminalError._();
  @$core.pragma('dart2js:noInline')
  static TerminalError getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<TerminalError>(
          TerminalError.$_createMessage);
  static TerminalError? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get message => $_getSZ(1);
  @$pb.TagNumber(2)
  set message($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);
}

/// Resumed:Attach 复用了已存在的存活会话时,服务端在 Opened 前先发这一帧,
/// 提示客户端"这是恢复旧会话"(PTY 无回放,静默 shell 不会有首字节输出)。
/// 旧客户端忽略未知 oneof 字段即可,天然兼容。
class Resumed extends $pb.GeneratedMessage {
  factory Resumed({
    $core.String? sessionId,
    $fixnum.Int64? attachedMs,
  }) {
    final result = Resumed._();
    if (sessionId != null) result.sessionId = sessionId;
    if (attachedMs != null) result.attachedMs = attachedMs;
    return result;
  }

  Resumed._();

  factory Resumed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Resumed()..mergeFromBuffer(data, registry);
  factory Resumed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Resumed()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Resumed',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: Resumed.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aInt64(2, _omitFieldNames ? '' : 'attachedMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Resumed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Resumed copyWith(void Function(Resumed) updates) =>
      super.copyWith((message) => updates(message as Resumed)) as Resumed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Resumed() / Resumed.new instead')
  static Resumed create() => Resumed._();
  static $pb.GeneratedMessage $_createMessage() => Resumed._();
  @$core.override
  Resumed createEmptyInstance() => Resumed._();
  @$core.pragma('dart2js:noInline')
  static Resumed getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Resumed>(Resumed.$_createMessage);
  static Resumed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get attachedMs => $_getI64(1);
  @$pb.TagNumber(2)
  set attachedMs($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAttachedMs() => $_has(1);
  @$pb.TagNumber(2)
  void clearAttachedMs() => $_clearField(2);
}

class ListTerminalsRequest extends $pb.GeneratedMessage {
  factory ListTerminalsRequest() => ListTerminalsRequest._();

  ListTerminalsRequest._();

  factory ListTerminalsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTerminalsRequest()..mergeFromBuffer(data, registry);
  factory ListTerminalsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTerminalsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListTerminalsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListTerminalsRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTerminalsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTerminalsRequest copyWith(void Function(ListTerminalsRequest) updates) =>
      super.copyWith((message) => updates(message as ListTerminalsRequest))
          as ListTerminalsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListTerminalsRequest() / ListTerminalsRequest.new instead')
  static ListTerminalsRequest create() => ListTerminalsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListTerminalsRequest._();
  @$core.override
  ListTerminalsRequest createEmptyInstance() => ListTerminalsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListTerminalsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListTerminalsRequest>(
          ListTerminalsRequest.$_createMessage);
  static ListTerminalsRequest? _defaultInstance;
}

class ListTerminalsResponse extends $pb.GeneratedMessage {
  factory ListTerminalsResponse({
    $core.Iterable<TerminalSessionInfo>? sessions,
  }) {
    final result = ListTerminalsResponse._();
    if (sessions != null) result.sessions.addAll(sessions);
    return result;
  }

  ListTerminalsResponse._();

  factory ListTerminalsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTerminalsResponse()..mergeFromBuffer(data, registry);
  factory ListTerminalsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListTerminalsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListTerminalsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListTerminalsResponse.$_createMessage)
    ..pPM<TerminalSessionInfo>(1, _omitFieldNames ? '' : 'sessions',
        subBuilder: TerminalSessionInfo.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTerminalsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTerminalsResponse copyWith(
          void Function(ListTerminalsResponse) updates) =>
      super.copyWith((message) => updates(message as ListTerminalsResponse))
          as ListTerminalsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListTerminalsResponse() / ListTerminalsResponse.new instead')
  static ListTerminalsResponse create() => ListTerminalsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListTerminalsResponse._();
  @$core.override
  ListTerminalsResponse createEmptyInstance() => ListTerminalsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListTerminalsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListTerminalsResponse>(
          ListTerminalsResponse.$_createMessage);
  static ListTerminalsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<TerminalSessionInfo> get sessions => $_getList(0);
}

class TerminalSessionInfo extends $pb.GeneratedMessage {
  factory TerminalSessionInfo({
    $core.String? sessionId,
    $core.String? shell,
    $core.String? cwd,
    $fixnum.Int64? openedAtUnixMs,
    $core.bool? alive,
  }) {
    final result = TerminalSessionInfo._();
    if (sessionId != null) result.sessionId = sessionId;
    if (shell != null) result.shell = shell;
    if (cwd != null) result.cwd = cwd;
    if (openedAtUnixMs != null) result.openedAtUnixMs = openedAtUnixMs;
    if (alive != null) result.alive = alive;
    return result;
  }

  TerminalSessionInfo._();

  factory TerminalSessionInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalSessionInfo()..mergeFromBuffer(data, registry);
  factory TerminalSessionInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TerminalSessionInfo()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TerminalSessionInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TerminalSessionInfo.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'shell')
    ..aOS(3, _omitFieldNames ? '' : 'cwd')
    ..aInt64(4, _omitFieldNames ? '' : 'openedAtUnixMs')
    ..aOB(5, _omitFieldNames ? '' : 'alive')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalSessionInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TerminalSessionInfo copyWith(void Function(TerminalSessionInfo) updates) =>
      super.copyWith((message) => updates(message as TerminalSessionInfo))
          as TerminalSessionInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use TerminalSessionInfo() / TerminalSessionInfo.new instead')
  static TerminalSessionInfo create() => TerminalSessionInfo._();
  static $pb.GeneratedMessage $_createMessage() => TerminalSessionInfo._();
  @$core.override
  TerminalSessionInfo createEmptyInstance() => TerminalSessionInfo._();
  @$core.pragma('dart2js:noInline')
  static TerminalSessionInfo getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TerminalSessionInfo>(
          TerminalSessionInfo.$_createMessage);
  static TerminalSessionInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get shell => $_getSZ(1);
  @$pb.TagNumber(2)
  set shell($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasShell() => $_has(1);
  @$pb.TagNumber(2)
  void clearShell() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get cwd => $_getSZ(2);
  @$pb.TagNumber(3)
  set cwd($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCwd() => $_has(2);
  @$pb.TagNumber(3)
  void clearCwd() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get openedAtUnixMs => $_getI64(3);
  @$pb.TagNumber(4)
  set openedAtUnixMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOpenedAtUnixMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearOpenedAtUnixMs() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get alive => $_getBF(4);
  @$pb.TagNumber(5)
  set alive($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAlive() => $_has(4);
  @$pb.TagNumber(5)
  void clearAlive() => $_clearField(5);
}

class CloseTerminalRequest extends $pb.GeneratedMessage {
  factory CloseTerminalRequest({
    $core.String? sessionId,
  }) {
    final result = CloseTerminalRequest._();
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  CloseTerminalRequest._();

  factory CloseTerminalRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CloseTerminalRequest()..mergeFromBuffer(data, registry);
  factory CloseTerminalRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CloseTerminalRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CloseTerminalRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CloseTerminalRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseTerminalRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CloseTerminalRequest copyWith(void Function(CloseTerminalRequest) updates) =>
      super.copyWith((message) => updates(message as CloseTerminalRequest))
          as CloseTerminalRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CloseTerminalRequest() / CloseTerminalRequest.new instead')
  static CloseTerminalRequest create() => CloseTerminalRequest._();
  static $pb.GeneratedMessage $_createMessage() => CloseTerminalRequest._();
  @$core.override
  CloseTerminalRequest createEmptyInstance() => CloseTerminalRequest._();
  @$core.pragma('dart2js:noInline')
  static CloseTerminalRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CloseTerminalRequest>(
          CloseTerminalRequest.$_createMessage);
  static CloseTerminalRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
