// This is a generated file - do not edit.
//
// Generated from v1/common.proto.

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

class Empty extends $pb.GeneratedMessage {
  factory Empty() => Empty._();

  Empty._();

  factory Empty.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Empty()..mergeFromBuffer(data, registry);
  factory Empty.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Empty()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Empty',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: Empty.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Empty clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Empty copyWith(void Function(Empty) updates) =>
      super.copyWith((message) => updates(message as Empty)) as Empty;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Empty() / Empty.new instead')
  static Empty create() => Empty._();
  static $pb.GeneratedMessage $_createMessage() => Empty._();
  @$core.override
  Empty createEmptyInstance() => Empty._();
  @$core.pragma('dart2js:noInline')
  static Empty getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Empty>(Empty.$_createMessage);
  static Empty? _defaultInstance;
}

class Error extends $pb.GeneratedMessage {
  factory Error({
    $core.String? code,
    $core.String? message,
    $core.bool? retryable,
    $core.int? httpStatus,
    $core.String? url,
    $core.String? details,
  }) {
    final result = Error._();
    if (code != null) result.code = code;
    if (message != null) result.message = message;
    if (retryable != null) result.retryable = retryable;
    if (httpStatus != null) result.httpStatus = httpStatus;
    if (url != null) result.url = url;
    if (details != null) result.details = details;
    return result;
  }

  Error._();

  factory Error.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Error()..mergeFromBuffer(data, registry);
  factory Error.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Error()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Error',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: Error.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..aOS(2, _omitFieldNames ? '' : 'message')
    ..aOB(3, _omitFieldNames ? '' : 'retryable')
    ..aI(4, _omitFieldNames ? '' : 'httpStatus')
    ..aOS(5, _omitFieldNames ? '' : 'url')
    ..aOS(6, _omitFieldNames ? '' : 'details')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Error clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Error copyWith(void Function(Error) updates) =>
      super.copyWith((message) => updates(message as Error)) as Error;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Error() / Error.new instead')
  static Error create() => Error._();
  static $pb.GeneratedMessage $_createMessage() => Error._();
  @$core.override
  Error createEmptyInstance() => Error._();
  @$core.pragma('dart2js:noInline')
  static Error getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Error>(Error.$_createMessage);
  static Error? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get message => $_getSZ(1);
  @$pb.TagNumber(2)
  set message($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get retryable => $_getBF(2);
  @$pb.TagNumber(3)
  set retryable($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRetryable() => $_has(2);
  @$pb.TagNumber(3)
  void clearRetryable() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get httpStatus => $_getIZ(3);
  @$pb.TagNumber(4)
  set httpStatus($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasHttpStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearHttpStatus() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get url => $_getSZ(4);
  @$pb.TagNumber(5)
  set url($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasUrl() => $_has(4);
  @$pb.TagNumber(5)
  void clearUrl() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get details => $_getSZ(5);
  @$pb.TagNumber(6)
  set details($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDetails() => $_has(5);
  @$pb.TagNumber(6)
  void clearDetails() => $_clearField(6);
}

enum ContentBlock_Block { text, image, toolCall, toolResult, thinking, notSet }

/// 会话内容块:消息由块序列构成(对齐 Claude/OpenAI 的 content blocks)
class ContentBlock extends $pb.GeneratedMessage {
  factory ContentBlock({
    TextBlock? text,
    ImageBlock? image,
    ToolCallBlock? toolCall,
    ToolResultBlock? toolResult,
    ThinkingBlock? thinking,
  }) {
    final result = ContentBlock._();
    if (text != null) result.text = text;
    if (image != null) result.image = image;
    if (toolCall != null) result.toolCall = toolCall;
    if (toolResult != null) result.toolResult = toolResult;
    if (thinking != null) result.thinking = thinking;
    return result;
  }

  ContentBlock._();

  factory ContentBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ContentBlock()..mergeFromBuffer(data, registry);
  factory ContentBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ContentBlock()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, ContentBlock_Block>
      _ContentBlock_BlockByTag = {
    1: ContentBlock_Block.text,
    2: ContentBlock_Block.image,
    3: ContentBlock_Block.toolCall,
    4: ContentBlock_Block.toolResult,
    5: ContentBlock_Block.thinking,
    0: ContentBlock_Block.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ContentBlock',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ContentBlock.$_createMessage)
    ..oo(0, [1, 2, 3, 4, 5])
    ..aOM<TextBlock>(1, _omitFieldNames ? '' : 'text',
        subBuilder: TextBlock.$_createMessage)
    ..aOM<ImageBlock>(2, _omitFieldNames ? '' : 'image',
        subBuilder: ImageBlock.$_createMessage)
    ..aOM<ToolCallBlock>(3, _omitFieldNames ? '' : 'toolCall',
        subBuilder: ToolCallBlock.$_createMessage)
    ..aOM<ToolResultBlock>(4, _omitFieldNames ? '' : 'toolResult',
        subBuilder: ToolResultBlock.$_createMessage)
    ..aOM<ThinkingBlock>(5, _omitFieldNames ? '' : 'thinking',
        subBuilder: ThinkingBlock.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContentBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContentBlock copyWith(void Function(ContentBlock) updates) =>
      super.copyWith((message) => updates(message as ContentBlock))
          as ContentBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ContentBlock() / ContentBlock.new instead')
  static ContentBlock create() => ContentBlock._();
  static $pb.GeneratedMessage $_createMessage() => ContentBlock._();
  @$core.override
  ContentBlock createEmptyInstance() => ContentBlock._();
  @$core.pragma('dart2js:noInline')
  static ContentBlock getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ContentBlock>(
          ContentBlock.$_createMessage);
  static ContentBlock? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  ContentBlock_Block whichBlock() => _ContentBlock_BlockByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  void clearBlock() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  TextBlock get text => $_getN(0);
  @$pb.TagNumber(1)
  set text(TextBlock value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasText() => $_has(0);
  @$pb.TagNumber(1)
  void clearText() => $_clearField(1);
  @$pb.TagNumber(1)
  TextBlock ensureText() => $_ensure(0);

  @$pb.TagNumber(2)
  ImageBlock get image => $_getN(1);
  @$pb.TagNumber(2)
  set image(ImageBlock value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasImage() => $_has(1);
  @$pb.TagNumber(2)
  void clearImage() => $_clearField(2);
  @$pb.TagNumber(2)
  ImageBlock ensureImage() => $_ensure(1);

  @$pb.TagNumber(3)
  ToolCallBlock get toolCall => $_getN(2);
  @$pb.TagNumber(3)
  set toolCall(ToolCallBlock value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasToolCall() => $_has(2);
  @$pb.TagNumber(3)
  void clearToolCall() => $_clearField(3);
  @$pb.TagNumber(3)
  ToolCallBlock ensureToolCall() => $_ensure(2);

  @$pb.TagNumber(4)
  ToolResultBlock get toolResult => $_getN(3);
  @$pb.TagNumber(4)
  set toolResult(ToolResultBlock value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasToolResult() => $_has(3);
  @$pb.TagNumber(4)
  void clearToolResult() => $_clearField(4);
  @$pb.TagNumber(4)
  ToolResultBlock ensureToolResult() => $_ensure(3);

  @$pb.TagNumber(5)
  ThinkingBlock get thinking => $_getN(4);
  @$pb.TagNumber(5)
  set thinking(ThinkingBlock value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasThinking() => $_has(4);
  @$pb.TagNumber(5)
  void clearThinking() => $_clearField(5);
  @$pb.TagNumber(5)
  ThinkingBlock ensureThinking() => $_ensure(4);
}

class TextBlock extends $pb.GeneratedMessage {
  factory TextBlock({
    $core.String? text,
  }) {
    final result = TextBlock._();
    if (text != null) result.text = text;
    return result;
  }

  TextBlock._();

  factory TextBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TextBlock()..mergeFromBuffer(data, registry);
  factory TextBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      TextBlock()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TextBlock',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: TextBlock.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'text')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TextBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TextBlock copyWith(void Function(TextBlock) updates) =>
      super.copyWith((message) => updates(message as TextBlock)) as TextBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use TextBlock() / TextBlock.new instead')
  static TextBlock create() => TextBlock._();
  static $pb.GeneratedMessage $_createMessage() => TextBlock._();
  @$core.override
  TextBlock createEmptyInstance() => TextBlock._();
  @$core.pragma('dart2js:noInline')
  static TextBlock getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TextBlock>(TextBlock.$_createMessage);
  static TextBlock? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get text => $_getSZ(0);
  @$pb.TagNumber(1)
  set text($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasText() => $_has(0);
  @$pb.TagNumber(1)
  void clearText() => $_clearField(1);
}

class ImageBlock extends $pb.GeneratedMessage {
  factory ImageBlock({
    $core.List<$core.int>? data,
    $core.String? mediaType,
  }) {
    final result = ImageBlock._();
    if (data != null) result.data = data;
    if (mediaType != null) result.mediaType = mediaType;
    return result;
  }

  ImageBlock._();

  factory ImageBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImageBlock()..mergeFromBuffer(data, registry);
  factory ImageBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ImageBlock()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ImageBlock',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ImageBlock.$_createMessage)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'data', $pb.PbFieldType.OY)
    ..aOS(2, _omitFieldNames ? '' : 'mediaType')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImageBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ImageBlock copyWith(void Function(ImageBlock) updates) =>
      super.copyWith((message) => updates(message as ImageBlock)) as ImageBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ImageBlock() / ImageBlock.new instead')
  static ImageBlock create() => ImageBlock._();
  static $pb.GeneratedMessage $_createMessage() => ImageBlock._();
  @$core.override
  ImageBlock createEmptyInstance() => ImageBlock._();
  @$core.pragma('dart2js:noInline')
  static ImageBlock getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ImageBlock>(ImageBlock.$_createMessage);
  static ImageBlock? _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.int> get data => $_getN(0);
  @$pb.TagNumber(1)
  set data($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasData() => $_has(0);
  @$pb.TagNumber(1)
  void clearData() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get mediaType => $_getSZ(1);
  @$pb.TagNumber(2)
  set mediaType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMediaType() => $_has(1);
  @$pb.TagNumber(2)
  void clearMediaType() => $_clearField(2);
}

class ToolCallBlock extends $pb.GeneratedMessage {
  factory ToolCallBlock({
    $core.String? id,
    $core.String? name,
    $core.String? jsonArgs,
  }) {
    final result = ToolCallBlock._();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (jsonArgs != null) result.jsonArgs = jsonArgs;
    return result;
  }

  ToolCallBlock._();

  factory ToolCallBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolCallBlock()..mergeFromBuffer(data, registry);
  factory ToolCallBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolCallBlock()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ToolCallBlock',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ToolCallBlock.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'jsonArgs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolCallBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolCallBlock copyWith(void Function(ToolCallBlock) updates) =>
      super.copyWith((message) => updates(message as ToolCallBlock))
          as ToolCallBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ToolCallBlock() / ToolCallBlock.new instead')
  static ToolCallBlock create() => ToolCallBlock._();
  static $pb.GeneratedMessage $_createMessage() => ToolCallBlock._();
  @$core.override
  ToolCallBlock createEmptyInstance() => ToolCallBlock._();
  @$core.pragma('dart2js:noInline')
  static ToolCallBlock getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ToolCallBlock>(
          ToolCallBlock.$_createMessage);
  static ToolCallBlock? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get jsonArgs => $_getSZ(2);
  @$pb.TagNumber(3)
  set jsonArgs($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasJsonArgs() => $_has(2);
  @$pb.TagNumber(3)
  void clearJsonArgs() => $_clearField(3);
}

class ToolResultBlock extends $pb.GeneratedMessage {
  factory ToolResultBlock({
    $core.String? callId,
    $core.bool? ok,
    $core.String? output,
    $core.double? elapsedMs,
  }) {
    final result = ToolResultBlock._();
    if (callId != null) result.callId = callId;
    if (ok != null) result.ok = ok;
    if (output != null) result.output = output;
    if (elapsedMs != null) result.elapsedMs = elapsedMs;
    return result;
  }

  ToolResultBlock._();

  factory ToolResultBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolResultBlock()..mergeFromBuffer(data, registry);
  factory ToolResultBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ToolResultBlock()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ToolResultBlock',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ToolResultBlock.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'callId')
    ..aOB(2, _omitFieldNames ? '' : 'ok')
    ..aOS(3, _omitFieldNames ? '' : 'output')
    ..aD(4, _omitFieldNames ? '' : 'elapsedMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolResultBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ToolResultBlock copyWith(void Function(ToolResultBlock) updates) =>
      super.copyWith((message) => updates(message as ToolResultBlock))
          as ToolResultBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ToolResultBlock() / ToolResultBlock.new instead')
  static ToolResultBlock create() => ToolResultBlock._();
  static $pb.GeneratedMessage $_createMessage() => ToolResultBlock._();
  @$core.override
  ToolResultBlock createEmptyInstance() => ToolResultBlock._();
  @$core.pragma('dart2js:noInline')
  static ToolResultBlock getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ToolResultBlock>(
          ToolResultBlock.$_createMessage);
  static ToolResultBlock? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get callId => $_getSZ(0);
  @$pb.TagNumber(1)
  set callId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCallId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get ok => $_getBF(1);
  @$pb.TagNumber(2)
  set ok($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOk() => $_has(1);
  @$pb.TagNumber(2)
  void clearOk() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get output => $_getSZ(2);
  @$pb.TagNumber(3)
  set output($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOutput() => $_has(2);
  @$pb.TagNumber(3)
  void clearOutput() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.double get elapsedMs => $_getN(3);
  @$pb.TagNumber(4)
  set elapsedMs($core.double value) => $_setDouble(3, value);
  @$pb.TagNumber(4)
  $core.bool hasElapsedMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearElapsedMs() => $_clearField(4);
}

class ThinkingBlock extends $pb.GeneratedMessage {
  factory ThinkingBlock({
    $core.String? text,
    $core.String? signature,
  }) {
    final result = ThinkingBlock._();
    if (text != null) result.text = text;
    if (signature != null) result.signature = signature;
    return result;
  }

  ThinkingBlock._();

  factory ThinkingBlock.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThinkingBlock()..mergeFromBuffer(data, registry);
  factory ThinkingBlock.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ThinkingBlock()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ThinkingBlock',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ThinkingBlock.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'text')
    ..aOS(2, _omitFieldNames ? '' : 'signature')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThinkingBlock clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ThinkingBlock copyWith(void Function(ThinkingBlock) updates) =>
      super.copyWith((message) => updates(message as ThinkingBlock))
          as ThinkingBlock;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ThinkingBlock() / ThinkingBlock.new instead')
  static ThinkingBlock create() => ThinkingBlock._();
  static $pb.GeneratedMessage $_createMessage() => ThinkingBlock._();
  @$core.override
  ThinkingBlock createEmptyInstance() => ThinkingBlock._();
  @$core.pragma('dart2js:noInline')
  static ThinkingBlock getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ThinkingBlock>(
          ThinkingBlock.$_createMessage);
  static ThinkingBlock? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get text => $_getSZ(0);
  @$pb.TagNumber(1)
  set text($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasText() => $_has(0);
  @$pb.TagNumber(1)
  void clearText() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get signature => $_getSZ(1);
  @$pb.TagNumber(2)
  set signature($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSignature() => $_has(1);
  @$pb.TagNumber(2)
  void clearSignature() => $_clearField(2);
}

/// Token 用量统计
class Usage extends $pb.GeneratedMessage {
  factory Usage({
    $fixnum.Int64? inputTokens,
    $fixnum.Int64? outputTokens,
    $fixnum.Int64? cacheReadTokens,
    $fixnum.Int64? cacheWriteTokens,
    $core.double? costUsd,
    $fixnum.Int64? reasoningTokens,
  }) {
    final result = Usage._();
    if (inputTokens != null) result.inputTokens = inputTokens;
    if (outputTokens != null) result.outputTokens = outputTokens;
    if (cacheReadTokens != null) result.cacheReadTokens = cacheReadTokens;
    if (cacheWriteTokens != null) result.cacheWriteTokens = cacheWriteTokens;
    if (costUsd != null) result.costUsd = costUsd;
    if (reasoningTokens != null) result.reasoningTokens = reasoningTokens;
    return result;
  }

  Usage._();

  factory Usage.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Usage()..mergeFromBuffer(data, registry);
  factory Usage.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Usage()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Usage',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: Usage.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'inputTokens')
    ..aInt64(2, _omitFieldNames ? '' : 'outputTokens')
    ..aInt64(3, _omitFieldNames ? '' : 'cacheReadTokens')
    ..aInt64(4, _omitFieldNames ? '' : 'cacheWriteTokens')
    ..aD(5, _omitFieldNames ? '' : 'costUsd')
    ..aInt64(6, _omitFieldNames ? '' : 'reasoningTokens')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Usage clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Usage copyWith(void Function(Usage) updates) =>
      super.copyWith((message) => updates(message as Usage)) as Usage;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Usage() / Usage.new instead')
  static Usage create() => Usage._();
  static $pb.GeneratedMessage $_createMessage() => Usage._();
  @$core.override
  Usage createEmptyInstance() => Usage._();
  @$core.pragma('dart2js:noInline')
  static Usage getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Usage>(Usage.$_createMessage);
  static Usage? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get inputTokens => $_getI64(0);
  @$pb.TagNumber(1)
  set inputTokens($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInputTokens() => $_has(0);
  @$pb.TagNumber(1)
  void clearInputTokens() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get outputTokens => $_getI64(1);
  @$pb.TagNumber(2)
  set outputTokens($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOutputTokens() => $_has(1);
  @$pb.TagNumber(2)
  void clearOutputTokens() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get cacheReadTokens => $_getI64(2);
  @$pb.TagNumber(3)
  set cacheReadTokens($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCacheReadTokens() => $_has(2);
  @$pb.TagNumber(3)
  void clearCacheReadTokens() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get cacheWriteTokens => $_getI64(3);
  @$pb.TagNumber(4)
  set cacheWriteTokens($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCacheWriteTokens() => $_has(3);
  @$pb.TagNumber(4)
  void clearCacheWriteTokens() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.double get costUsd => $_getN(4);
  @$pb.TagNumber(5)
  set costUsd($core.double value) => $_setDouble(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCostUsd() => $_has(4);
  @$pb.TagNumber(5)
  void clearCostUsd() => $_clearField(5);

  @$pb.TagNumber(6)
  $fixnum.Int64 get reasoningTokens => $_getI64(5);
  @$pb.TagNumber(6)
  set reasoningTokens($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasReasoningTokens() => $_has(5);
  @$pb.TagNumber(6)
  void clearReasoningTokens() => $_clearField(6);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
