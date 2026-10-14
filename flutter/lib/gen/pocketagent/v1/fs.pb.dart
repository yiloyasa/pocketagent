// This is a generated file - do not edit.
//
// Generated from v1/fs.proto.

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

class ListRootsRequest extends $pb.GeneratedMessage {
  factory ListRootsRequest() => ListRootsRequest._();

  ListRootsRequest._();

  factory ListRootsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRootsRequest()..mergeFromBuffer(data, registry);
  factory ListRootsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRootsRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListRootsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListRootsRequest.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRootsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRootsRequest copyWith(void Function(ListRootsRequest) updates) =>
      super.copyWith((message) => updates(message as ListRootsRequest))
          as ListRootsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListRootsRequest() / ListRootsRequest.new instead')
  static ListRootsRequest create() => ListRootsRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListRootsRequest._();
  @$core.override
  ListRootsRequest createEmptyInstance() => ListRootsRequest._();
  @$core.pragma('dart2js:noInline')
  static ListRootsRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListRootsRequest>(
          ListRootsRequest.$_createMessage);
  static ListRootsRequest? _defaultInstance;
}

class ListRootsResponse extends $pb.GeneratedMessage {
  factory ListRootsResponse({
    $core.Iterable<DirectoryEntry>? roots,
  }) {
    final result = ListRootsResponse._();
    if (roots != null) result.roots.addAll(roots);
    return result;
  }

  ListRootsResponse._();

  factory ListRootsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRootsResponse()..mergeFromBuffer(data, registry);
  factory ListRootsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListRootsResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListRootsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListRootsResponse.$_createMessage)
    ..pPM<DirectoryEntry>(1, _omitFieldNames ? '' : 'roots',
        subBuilder: DirectoryEntry.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRootsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRootsResponse copyWith(void Function(ListRootsResponse) updates) =>
      super.copyWith((message) => updates(message as ListRootsResponse))
          as ListRootsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ListRootsResponse() / ListRootsResponse.new instead')
  static ListRootsResponse create() => ListRootsResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListRootsResponse._();
  @$core.override
  ListRootsResponse createEmptyInstance() => ListRootsResponse._();
  @$core.pragma('dart2js:noInline')
  static ListRootsResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListRootsResponse>(
          ListRootsResponse.$_createMessage);
  static ListRootsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<DirectoryEntry> get roots => $_getList(0);
}

class ListDirectoryRequest extends $pb.GeneratedMessage {
  factory ListDirectoryRequest({
    $core.String? path,
    $core.bool? includeHidden,
  }) {
    final result = ListDirectoryRequest._();
    if (path != null) result.path = path;
    if (includeHidden != null) result.includeHidden = includeHidden;
    return result;
  }

  ListDirectoryRequest._();

  factory ListDirectoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListDirectoryRequest()..mergeFromBuffer(data, registry);
  factory ListDirectoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListDirectoryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListDirectoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListDirectoryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..aOB(2, _omitFieldNames ? '' : 'includeHidden')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListDirectoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListDirectoryRequest copyWith(void Function(ListDirectoryRequest) updates) =>
      super.copyWith((message) => updates(message as ListDirectoryRequest))
          as ListDirectoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListDirectoryRequest() / ListDirectoryRequest.new instead')
  static ListDirectoryRequest create() => ListDirectoryRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListDirectoryRequest._();
  @$core.override
  ListDirectoryRequest createEmptyInstance() => ListDirectoryRequest._();
  @$core.pragma('dart2js:noInline')
  static ListDirectoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListDirectoryRequest>(
          ListDirectoryRequest.$_createMessage);
  static ListDirectoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get includeHidden => $_getBF(1);
  @$pb.TagNumber(2)
  set includeHidden($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIncludeHidden() => $_has(1);
  @$pb.TagNumber(2)
  void clearIncludeHidden() => $_clearField(2);
}

class ListDirectoryResponse extends $pb.GeneratedMessage {
  factory ListDirectoryResponse({
    $core.Iterable<DirectoryEntry>? entries,
  }) {
    final result = ListDirectoryResponse._();
    if (entries != null) result.entries.addAll(entries);
    return result;
  }

  ListDirectoryResponse._();

  factory ListDirectoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListDirectoryResponse()..mergeFromBuffer(data, registry);
  factory ListDirectoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListDirectoryResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListDirectoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListDirectoryResponse.$_createMessage)
    ..pPM<DirectoryEntry>(1, _omitFieldNames ? '' : 'entries',
        subBuilder: DirectoryEntry.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListDirectoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListDirectoryResponse copyWith(
          void Function(ListDirectoryResponse) updates) =>
      super.copyWith((message) => updates(message as ListDirectoryResponse))
          as ListDirectoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListDirectoryResponse() / ListDirectoryResponse.new instead')
  static ListDirectoryResponse create() => ListDirectoryResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListDirectoryResponse._();
  @$core.override
  ListDirectoryResponse createEmptyInstance() => ListDirectoryResponse._();
  @$core.pragma('dart2js:noInline')
  static ListDirectoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListDirectoryResponse>(
          ListDirectoryResponse.$_createMessage);
  static ListDirectoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<DirectoryEntry> get entries => $_getList(0);
}

class DirectoryEntry extends $pb.GeneratedMessage {
  factory DirectoryEntry({
    $core.String? name,
    $core.String? path,
    $core.bool? isDir,
    $fixnum.Int64? sizeBytes,
    $fixnum.Int64? modifiedUnixMs,
  }) {
    final result = DirectoryEntry._();
    if (name != null) result.name = name;
    if (path != null) result.path = path;
    if (isDir != null) result.isDir = isDir;
    if (sizeBytes != null) result.sizeBytes = sizeBytes;
    if (modifiedUnixMs != null) result.modifiedUnixMs = modifiedUnixMs;
    return result;
  }

  DirectoryEntry._();

  factory DirectoryEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DirectoryEntry()..mergeFromBuffer(data, registry);
  factory DirectoryEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DirectoryEntry()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DirectoryEntry',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DirectoryEntry.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'path')
    ..aOB(3, _omitFieldNames ? '' : 'isDir')
    ..aInt64(4, _omitFieldNames ? '' : 'sizeBytes')
    ..aInt64(5, _omitFieldNames ? '' : 'modifiedUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DirectoryEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DirectoryEntry copyWith(void Function(DirectoryEntry) updates) =>
      super.copyWith((message) => updates(message as DirectoryEntry))
          as DirectoryEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DirectoryEntry() / DirectoryEntry.new instead')
  static DirectoryEntry create() => DirectoryEntry._();
  static $pb.GeneratedMessage $_createMessage() => DirectoryEntry._();
  @$core.override
  DirectoryEntry createEmptyInstance() => DirectoryEntry._();
  @$core.pragma('dart2js:noInline')
  static DirectoryEntry getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<DirectoryEntry>(
          DirectoryEntry.$_createMessage);
  static DirectoryEntry? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get path => $_getSZ(1);
  @$pb.TagNumber(2)
  set path($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPath() => $_has(1);
  @$pb.TagNumber(2)
  void clearPath() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isDir => $_getBF(2);
  @$pb.TagNumber(3)
  set isDir($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsDir() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsDir() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get sizeBytes => $_getI64(3);
  @$pb.TagNumber(4)
  set sizeBytes($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSizeBytes() => $_has(3);
  @$pb.TagNumber(4)
  void clearSizeBytes() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get modifiedUnixMs => $_getI64(4);
  @$pb.TagNumber(5)
  set modifiedUnixMs($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasModifiedUnixMs() => $_has(4);
  @$pb.TagNumber(5)
  void clearModifiedUnixMs() => $_clearField(5);
}

class ReadFileRequest extends $pb.GeneratedMessage {
  factory ReadFileRequest({
    $core.String? path,
    $fixnum.Int64? maxBytes,
  }) {
    final result = ReadFileRequest._();
    if (path != null) result.path = path;
    if (maxBytes != null) result.maxBytes = maxBytes;
    return result;
  }

  ReadFileRequest._();

  factory ReadFileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReadFileRequest()..mergeFromBuffer(data, registry);
  factory ReadFileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReadFileRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadFileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ReadFileRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..aInt64(2, _omitFieldNames ? '' : 'maxBytes')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadFileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadFileRequest copyWith(void Function(ReadFileRequest) updates) =>
      super.copyWith((message) => updates(message as ReadFileRequest))
          as ReadFileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ReadFileRequest() / ReadFileRequest.new instead')
  static ReadFileRequest create() => ReadFileRequest._();
  static $pb.GeneratedMessage $_createMessage() => ReadFileRequest._();
  @$core.override
  ReadFileRequest createEmptyInstance() => ReadFileRequest._();
  @$core.pragma('dart2js:noInline')
  static ReadFileRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReadFileRequest>(
          ReadFileRequest.$_createMessage);
  static ReadFileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get maxBytes => $_getI64(1);
  @$pb.TagNumber(2)
  set maxBytes($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMaxBytes() => $_has(1);
  @$pb.TagNumber(2)
  void clearMaxBytes() => $_clearField(2);
}

class FileChunk extends $pb.GeneratedMessage {
  factory FileChunk({
    $core.List<$core.int>? data,
    $fixnum.Int64? totalSize,
    $core.bool? text,
    $fixnum.Int64? modifiedUnixMs,
  }) {
    final result = FileChunk._();
    if (data != null) result.data = data;
    if (totalSize != null) result.totalSize = totalSize;
    if (text != null) result.text = text;
    if (modifiedUnixMs != null) result.modifiedUnixMs = modifiedUnixMs;
    return result;
  }

  FileChunk._();

  factory FileChunk.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FileChunk()..mergeFromBuffer(data, registry);
  factory FileChunk.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FileChunk()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileChunk',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: FileChunk.$_createMessage)
    ..a<$core.List<$core.int>>(
        1, _omitFieldNames ? '' : 'data', $pb.PbFieldType.OY)
    ..aInt64(2, _omitFieldNames ? '' : 'totalSize')
    ..aOB(3, _omitFieldNames ? '' : 'text')
    ..aInt64(4, _omitFieldNames ? '' : 'modifiedUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileChunk clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileChunk copyWith(void Function(FileChunk) updates) =>
      super.copyWith((message) => updates(message as FileChunk)) as FileChunk;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use FileChunk() / FileChunk.new instead')
  static FileChunk create() => FileChunk._();
  static $pb.GeneratedMessage $_createMessage() => FileChunk._();
  @$core.override
  FileChunk createEmptyInstance() => FileChunk._();
  @$core.pragma('dart2js:noInline')
  static FileChunk getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileChunk>(FileChunk.$_createMessage);
  static FileChunk? _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.int> get data => $_getN(0);
  @$pb.TagNumber(1)
  set data($core.List<$core.int> value) => $_setBytes(0, value);
  @$pb.TagNumber(1)
  $core.bool hasData() => $_has(0);
  @$pb.TagNumber(1)
  void clearData() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get totalSize => $_getI64(1);
  @$pb.TagNumber(2)
  set totalSize($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTotalSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearTotalSize() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get text => $_getBF(2);
  @$pb.TagNumber(3)
  set text($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasText() => $_has(2);
  @$pb.TagNumber(3)
  void clearText() => $_clearField(3);

  /// 【乐观锁配套】该文件读取时刻的修改时间(UnixMilli)。每个分块都带同一值,
  /// 客户端取任意一块即可拿到;手机端编辑保存时回填 WriteFileRequest 的
  /// expected_modified_at_ms。目录/不存在语义照旧(不读文件就没有该值)。
  @$pb.TagNumber(4)
  $fixnum.Int64 get modifiedUnixMs => $_getI64(3);
  @$pb.TagNumber(4)
  set modifiedUnixMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasModifiedUnixMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearModifiedUnixMs() => $_clearField(4);
}

class WriteFileRequest extends $pb.GeneratedMessage {
  factory WriteFileRequest({
    $core.String? path,
    $core.List<$core.int>? data,
    $core.bool? create_3,
    $core.bool? truncate,
    $fixnum.Int64? expectedModifiedAtMs,
  }) {
    final result = WriteFileRequest._();
    if (path != null) result.path = path;
    if (data != null) result.data = data;
    if (create_3 != null) result.create_3 = create_3;
    if (truncate != null) result.truncate = truncate;
    if (expectedModifiedAtMs != null)
      result.expectedModifiedAtMs = expectedModifiedAtMs;
    return result;
  }

  WriteFileRequest._();

  factory WriteFileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WriteFileRequest()..mergeFromBuffer(data, registry);
  factory WriteFileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WriteFileRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WriteFileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: WriteFileRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'data', $pb.PbFieldType.OY)
    ..aOB(3, _omitFieldNames ? '' : 'create')
    ..aOB(4, _omitFieldNames ? '' : 'truncate')
    ..aInt64(5, _omitFieldNames ? '' : 'expectedModifiedAtMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteFileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteFileRequest copyWith(void Function(WriteFileRequest) updates) =>
      super.copyWith((message) => updates(message as WriteFileRequest))
          as WriteFileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use WriteFileRequest() / WriteFileRequest.new instead')
  static WriteFileRequest create() => WriteFileRequest._();
  static $pb.GeneratedMessage $_createMessage() => WriteFileRequest._();
  @$core.override
  WriteFileRequest createEmptyInstance() => WriteFileRequest._();
  @$core.pragma('dart2js:noInline')
  static WriteFileRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WriteFileRequest>(
          WriteFileRequest.$_createMessage);
  static WriteFileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get data => $_getN(1);
  @$pb.TagNumber(2)
  set data($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasData() => $_has(1);
  @$pb.TagNumber(2)
  void clearData() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get create_3 => $_getBF(2);
  @$pb.TagNumber(3)
  set create_3($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCreate_3() => $_has(2);
  @$pb.TagNumber(3)
  void clearCreate_3() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get truncate => $_getBF(3);
  @$pb.TagNumber(4)
  set truncate($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTruncate() => $_has(3);
  @$pb.TagNumber(4)
  void clearTruncate() => $_clearField(4);

  /// 【乐观锁】调用方读文件时记下的修改时间(UnixMilli)。大于 0 时服务端校验:
  /// 与磁盘当前值不等(含文件已被删)则拒绝写入,防止手机与电脑并行编辑互相覆盖。
  /// 0 / 缺省 = 不校验(兼容旧客户端与"强制覆盖"场景)。对齐原作 WriteWorkspaceTextFile。
  @$pb.TagNumber(5)
  $fixnum.Int64 get expectedModifiedAtMs => $_getI64(4);
  @$pb.TagNumber(5)
  set expectedModifiedAtMs($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedModifiedAtMs() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedModifiedAtMs() => $_clearField(5);
}

class WriteFileResponse extends $pb.GeneratedMessage {
  factory WriteFileResponse({
    $fixnum.Int64? newModifiedAtMs,
  }) {
    final result = WriteFileResponse._();
    if (newModifiedAtMs != null) result.newModifiedAtMs = newModifiedAtMs;
    return result;
  }

  WriteFileResponse._();

  factory WriteFileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WriteFileResponse()..mergeFromBuffer(data, registry);
  factory WriteFileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      WriteFileResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WriteFileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: WriteFileResponse.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'newModifiedAtMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteFileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WriteFileResponse copyWith(void Function(WriteFileResponse) updates) =>
      super.copyWith((message) => updates(message as WriteFileResponse))
          as WriteFileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use WriteFileResponse() / WriteFileResponse.new instead')
  static WriteFileResponse create() => WriteFileResponse._();
  static $pb.GeneratedMessage $_createMessage() => WriteFileResponse._();
  @$core.override
  WriteFileResponse createEmptyInstance() => WriteFileResponse._();
  @$core.pragma('dart2js:noInline')
  static WriteFileResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WriteFileResponse>(
          WriteFileResponse.$_createMessage);
  static WriteFileResponse? _defaultInstance;

  /// 写入完成后磁盘上的新修改时间(UnixMilli)。客户端保存它,作为下一次
  /// 乐观锁写入的 expected_modified_at_ms。读取失败时为 0。
  @$pb.TagNumber(1)
  $fixnum.Int64 get newModifiedAtMs => $_getI64(0);
  @$pb.TagNumber(1)
  set newModifiedAtMs($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNewModifiedAtMs() => $_has(0);
  @$pb.TagNumber(1)
  void clearNewModifiedAtMs() => $_clearField(1);
}

class ReadFileDiffRequest extends $pb.GeneratedMessage {
  factory ReadFileDiffRequest({
    $core.String? path,
  }) {
    final result = ReadFileDiffRequest._();
    if (path != null) result.path = path;
    return result;
  }

  ReadFileDiffRequest._();

  factory ReadFileDiffRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReadFileDiffRequest()..mergeFromBuffer(data, registry);
  factory ReadFileDiffRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ReadFileDiffRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadFileDiffRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ReadFileDiffRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadFileDiffRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadFileDiffRequest copyWith(void Function(ReadFileDiffRequest) updates) =>
      super.copyWith((message) => updates(message as ReadFileDiffRequest))
          as ReadFileDiffRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ReadFileDiffRequest() / ReadFileDiffRequest.new instead')
  static ReadFileDiffRequest create() => ReadFileDiffRequest._();
  static $pb.GeneratedMessage $_createMessage() => ReadFileDiffRequest._();
  @$core.override
  ReadFileDiffRequest createEmptyInstance() => ReadFileDiffRequest._();
  @$core.pragma('dart2js:noInline')
  static ReadFileDiffRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadFileDiffRequest>(
          ReadFileDiffRequest.$_createMessage);
  static ReadFileDiffRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);
}

class FileDiff extends $pb.GeneratedMessage {
  factory FileDiff({
    $core.String? unifiedDiff,
    $core.bool? hasChanges,
  }) {
    final result = FileDiff._();
    if (unifiedDiff != null) result.unifiedDiff = unifiedDiff;
    if (hasChanges != null) result.hasChanges = hasChanges;
    return result;
  }

  FileDiff._();

  factory FileDiff.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FileDiff()..mergeFromBuffer(data, registry);
  factory FileDiff.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      FileDiff()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileDiff',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: FileDiff.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'unifiedDiff')
    ..aOB(2, _omitFieldNames ? '' : 'hasChanges')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileDiff clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileDiff copyWith(void Function(FileDiff) updates) =>
      super.copyWith((message) => updates(message as FileDiff)) as FileDiff;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use FileDiff() / FileDiff.new instead')
  static FileDiff create() => FileDiff._();
  static $pb.GeneratedMessage $_createMessage() => FileDiff._();
  @$core.override
  FileDiff createEmptyInstance() => FileDiff._();
  @$core.pragma('dart2js:noInline')
  static FileDiff getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileDiff>(FileDiff.$_createMessage);
  static FileDiff? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get unifiedDiff => $_getSZ(0);
  @$pb.TagNumber(1)
  set unifiedDiff($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUnifiedDiff() => $_has(0);
  @$pb.TagNumber(1)
  void clearUnifiedDiff() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get hasChanges => $_getBF(1);
  @$pb.TagNumber(2)
  set hasChanges($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHasChanges() => $_has(1);
  @$pb.TagNumber(2)
  void clearHasChanges() => $_clearField(2);
}

class CreateDirectoryRequest extends $pb.GeneratedMessage {
  factory CreateDirectoryRequest({
    $core.String? path,
  }) {
    final result = CreateDirectoryRequest._();
    if (path != null) result.path = path;
    return result;
  }

  CreateDirectoryRequest._();

  factory CreateDirectoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateDirectoryRequest()..mergeFromBuffer(data, registry);
  factory CreateDirectoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateDirectoryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateDirectoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CreateDirectoryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateDirectoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateDirectoryRequest copyWith(
          void Function(CreateDirectoryRequest) updates) =>
      super.copyWith((message) => updates(message as CreateDirectoryRequest))
          as CreateDirectoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CreateDirectoryRequest() / CreateDirectoryRequest.new instead')
  static CreateDirectoryRequest create() => CreateDirectoryRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateDirectoryRequest._();
  @$core.override
  CreateDirectoryRequest createEmptyInstance() => CreateDirectoryRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateDirectoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateDirectoryRequest>(
          CreateDirectoryRequest.$_createMessage);
  static CreateDirectoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);
}

class DeleteEntryRequest extends $pb.GeneratedMessage {
  factory DeleteEntryRequest({
    $core.String? path,
    $core.bool? recursive,
  }) {
    final result = DeleteEntryRequest._();
    if (path != null) result.path = path;
    if (recursive != null) result.recursive = recursive;
    return result;
  }

  DeleteEntryRequest._();

  factory DeleteEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteEntryRequest()..mergeFromBuffer(data, registry);
  factory DeleteEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DeleteEntryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteEntryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DeleteEntryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..aOB(2, _omitFieldNames ? '' : 'recursive')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteEntryRequest copyWith(void Function(DeleteEntryRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteEntryRequest))
          as DeleteEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use DeleteEntryRequest() / DeleteEntryRequest.new instead')
  static DeleteEntryRequest create() => DeleteEntryRequest._();
  static $pb.GeneratedMessage $_createMessage() => DeleteEntryRequest._();
  @$core.override
  DeleteEntryRequest createEmptyInstance() => DeleteEntryRequest._();
  @$core.pragma('dart2js:noInline')
  static DeleteEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteEntryRequest>(
          DeleteEntryRequest.$_createMessage);
  static DeleteEntryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get recursive => $_getBF(1);
  @$pb.TagNumber(2)
  set recursive($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRecursive() => $_has(1);
  @$pb.TagNumber(2)
  void clearRecursive() => $_clearField(2);
}

class MoveEntryRequest extends $pb.GeneratedMessage {
  factory MoveEntryRequest({
    $core.String? fromPath,
    $core.String? toPath,
  }) {
    final result = MoveEntryRequest._();
    if (fromPath != null) result.fromPath = fromPath;
    if (toPath != null) result.toPath = toPath;
    return result;
  }

  MoveEntryRequest._();

  factory MoveEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MoveEntryRequest()..mergeFromBuffer(data, registry);
  factory MoveEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      MoveEntryRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveEntryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: MoveEntryRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'fromPath')
    ..aOS(2, _omitFieldNames ? '' : 'toPath')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveEntryRequest copyWith(void Function(MoveEntryRequest) updates) =>
      super.copyWith((message) => updates(message as MoveEntryRequest))
          as MoveEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use MoveEntryRequest() / MoveEntryRequest.new instead')
  static MoveEntryRequest create() => MoveEntryRequest._();
  static $pb.GeneratedMessage $_createMessage() => MoveEntryRequest._();
  @$core.override
  MoveEntryRequest createEmptyInstance() => MoveEntryRequest._();
  @$core.pragma('dart2js:noInline')
  static MoveEntryRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MoveEntryRequest>(
          MoveEntryRequest.$_createMessage);
  static MoveEntryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fromPath => $_getSZ(0);
  @$pb.TagNumber(1)
  set fromPath($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFromPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearFromPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get toPath => $_getSZ(1);
  @$pb.TagNumber(2)
  set toPath($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasToPath() => $_has(1);
  @$pb.TagNumber(2)
  void clearToPath() => $_clearField(2);
}

class SearchContentRequest extends $pb.GeneratedMessage {
  factory SearchContentRequest({
    $core.String? root,
    $core.String? query,
    $core.bool? regex,
    $core.String? glob,
    $core.int? maxResults,
  }) {
    final result = SearchContentRequest._();
    if (root != null) result.root = root;
    if (query != null) result.query = query;
    if (regex != null) result.regex = regex;
    if (glob != null) result.glob = glob;
    if (maxResults != null) result.maxResults = maxResults;
    return result;
  }

  SearchContentRequest._();

  factory SearchContentRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchContentRequest()..mergeFromBuffer(data, registry);
  factory SearchContentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchContentRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchContentRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SearchContentRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'root')
    ..aOS(2, _omitFieldNames ? '' : 'query')
    ..aOB(3, _omitFieldNames ? '' : 'regex')
    ..aOS(4, _omitFieldNames ? '' : 'glob')
    ..aI(5, _omitFieldNames ? '' : 'maxResults')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchContentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchContentRequest copyWith(void Function(SearchContentRequest) updates) =>
      super.copyWith((message) => updates(message as SearchContentRequest))
          as SearchContentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use SearchContentRequest() / SearchContentRequest.new instead')
  static SearchContentRequest create() => SearchContentRequest._();
  static $pb.GeneratedMessage $_createMessage() => SearchContentRequest._();
  @$core.override
  SearchContentRequest createEmptyInstance() => SearchContentRequest._();
  @$core.pragma('dart2js:noInline')
  static SearchContentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchContentRequest>(
          SearchContentRequest.$_createMessage);
  static SearchContentRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get root => $_getSZ(0);
  @$pb.TagNumber(1)
  set root($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoot() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoot() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get query => $_getSZ(1);
  @$pb.TagNumber(2)
  set query($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasQuery() => $_has(1);
  @$pb.TagNumber(2)
  void clearQuery() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get regex => $_getBF(2);
  @$pb.TagNumber(3)
  set regex($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRegex() => $_has(2);
  @$pb.TagNumber(3)
  void clearRegex() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get glob => $_getSZ(3);
  @$pb.TagNumber(4)
  set glob($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGlob() => $_has(3);
  @$pb.TagNumber(4)
  void clearGlob() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get maxResults => $_getIZ(4);
  @$pb.TagNumber(5)
  set maxResults($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMaxResults() => $_has(4);
  @$pb.TagNumber(5)
  void clearMaxResults() => $_clearField(5);
}

class SearchMatch extends $pb.GeneratedMessage {
  factory SearchMatch({
    $core.String? path,
    $core.int? lineNumber,
    $core.String? lineText,
  }) {
    final result = SearchMatch._();
    if (path != null) result.path = path;
    if (lineNumber != null) result.lineNumber = lineNumber;
    if (lineText != null) result.lineText = lineText;
    return result;
  }

  SearchMatch._();

  factory SearchMatch.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchMatch()..mergeFromBuffer(data, registry);
  factory SearchMatch.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      SearchMatch()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchMatch',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: SearchMatch.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..aI(2, _omitFieldNames ? '' : 'lineNumber')
    ..aOS(3, _omitFieldNames ? '' : 'lineText')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchMatch clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchMatch copyWith(void Function(SearchMatch) updates) =>
      super.copyWith((message) => updates(message as SearchMatch))
          as SearchMatch;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use SearchMatch() / SearchMatch.new instead')
  static SearchMatch create() => SearchMatch._();
  static $pb.GeneratedMessage $_createMessage() => SearchMatch._();
  @$core.override
  SearchMatch createEmptyInstance() => SearchMatch._();
  @$core.pragma('dart2js:noInline')
  static SearchMatch getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SearchMatch>(
          SearchMatch.$_createMessage);
  static SearchMatch? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get lineNumber => $_getIZ(1);
  @$pb.TagNumber(2)
  set lineNumber($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLineNumber() => $_has(1);
  @$pb.TagNumber(2)
  void clearLineNumber() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get lineText => $_getSZ(2);
  @$pb.TagNumber(3)
  set lineText($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLineText() => $_has(2);
  @$pb.TagNumber(3)
  void clearLineText() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
