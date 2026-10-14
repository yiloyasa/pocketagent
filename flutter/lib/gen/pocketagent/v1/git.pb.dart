// This is a generated file - do not edit.
//
// Generated from v1/git.proto.

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

class GitStatusRequest extends $pb.GeneratedMessage {
  factory GitStatusRequest({
    $core.String? repo,
  }) {
    final result = GitStatusRequest._();
    if (repo != null) result.repo = repo;
    return result;
  }

  GitStatusRequest._();

  factory GitStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitStatusRequest()..mergeFromBuffer(data, registry);
  factory GitStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitStatusRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitStatusRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitStatusRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitStatusRequest copyWith(void Function(GitStatusRequest) updates) =>
      super.copyWith((message) => updates(message as GitStatusRequest))
          as GitStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitStatusRequest() / GitStatusRequest.new instead')
  static GitStatusRequest create() => GitStatusRequest._();
  static $pb.GeneratedMessage $_createMessage() => GitStatusRequest._();
  @$core.override
  GitStatusRequest createEmptyInstance() => GitStatusRequest._();
  @$core.pragma('dart2js:noInline')
  static GitStatusRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitStatusRequest>(
          GitStatusRequest.$_createMessage);
  static GitStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);
}

class GitStatus extends $pb.GeneratedMessage {
  factory GitStatus({
    $core.String? branch,
    $core.String? upstream,
    $core.bool? ahead,
    $core.bool? behind,
    $core.Iterable<GitFileChange>? changes,
  }) {
    final result = GitStatus._();
    if (branch != null) result.branch = branch;
    if (upstream != null) result.upstream = upstream;
    if (ahead != null) result.ahead = ahead;
    if (behind != null) result.behind = behind;
    if (changes != null) result.changes.addAll(changes);
    return result;
  }

  GitStatus._();

  factory GitStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitStatus()..mergeFromBuffer(data, registry);
  factory GitStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitStatus()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitStatus',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitStatus.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'branch')
    ..aOS(2, _omitFieldNames ? '' : 'upstream')
    ..aOB(3, _omitFieldNames ? '' : 'ahead')
    ..aOB(4, _omitFieldNames ? '' : 'behind')
    ..pPM<GitFileChange>(5, _omitFieldNames ? '' : 'changes',
        subBuilder: GitFileChange.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitStatus copyWith(void Function(GitStatus) updates) =>
      super.copyWith((message) => updates(message as GitStatus)) as GitStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitStatus() / GitStatus.new instead')
  static GitStatus create() => GitStatus._();
  static $pb.GeneratedMessage $_createMessage() => GitStatus._();
  @$core.override
  GitStatus createEmptyInstance() => GitStatus._();
  @$core.pragma('dart2js:noInline')
  static GitStatus getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GitStatus>(GitStatus.$_createMessage);
  static GitStatus? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get branch => $_getSZ(0);
  @$pb.TagNumber(1)
  set branch($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBranch() => $_has(0);
  @$pb.TagNumber(1)
  void clearBranch() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get upstream => $_getSZ(1);
  @$pb.TagNumber(2)
  set upstream($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUpstream() => $_has(1);
  @$pb.TagNumber(2)
  void clearUpstream() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get ahead => $_getBF(2);
  @$pb.TagNumber(3)
  set ahead($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAhead() => $_has(2);
  @$pb.TagNumber(3)
  void clearAhead() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get behind => $_getBF(3);
  @$pb.TagNumber(4)
  set behind($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBehind() => $_has(3);
  @$pb.TagNumber(4)
  void clearBehind() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<GitFileChange> get changes => $_getList(4);
}

class GitFileChange extends $pb.GeneratedMessage {
  factory GitFileChange({
    $core.String? path,
    $core.String? stagedStatus,
    $core.String? worktreeStatus,
  }) {
    final result = GitFileChange._();
    if (path != null) result.path = path;
    if (stagedStatus != null) result.stagedStatus = stagedStatus;
    if (worktreeStatus != null) result.worktreeStatus = worktreeStatus;
    return result;
  }

  GitFileChange._();

  factory GitFileChange.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitFileChange()..mergeFromBuffer(data, registry);
  factory GitFileChange.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitFileChange()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitFileChange',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitFileChange.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'path')
    ..aOS(2, _omitFieldNames ? '' : 'stagedStatus')
    ..aOS(3, _omitFieldNames ? '' : 'worktreeStatus')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitFileChange clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitFileChange copyWith(void Function(GitFileChange) updates) =>
      super.copyWith((message) => updates(message as GitFileChange))
          as GitFileChange;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitFileChange() / GitFileChange.new instead')
  static GitFileChange create() => GitFileChange._();
  static $pb.GeneratedMessage $_createMessage() => GitFileChange._();
  @$core.override
  GitFileChange createEmptyInstance() => GitFileChange._();
  @$core.pragma('dart2js:noInline')
  static GitFileChange getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitFileChange>(
          GitFileChange.$_createMessage);
  static GitFileChange? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get path => $_getSZ(0);
  @$pb.TagNumber(1)
  set path($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPath() => $_has(0);
  @$pb.TagNumber(1)
  void clearPath() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get stagedStatus => $_getSZ(1);
  @$pb.TagNumber(2)
  set stagedStatus($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStagedStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStagedStatus() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get worktreeStatus => $_getSZ(2);
  @$pb.TagNumber(3)
  set worktreeStatus($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasWorktreeStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearWorktreeStatus() => $_clearField(3);
}

class GitDiffRequest extends $pb.GeneratedMessage {
  factory GitDiffRequest({
    $core.String? repo,
    $core.bool? staged,
    $core.String? path,
    $core.int? contextLines,
    $core.String? commit,
  }) {
    final result = GitDiffRequest._();
    if (repo != null) result.repo = repo;
    if (staged != null) result.staged = staged;
    if (path != null) result.path = path;
    if (contextLines != null) result.contextLines = contextLines;
    if (commit != null) result.commit = commit;
    return result;
  }

  GitDiffRequest._();

  factory GitDiffRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitDiffRequest()..mergeFromBuffer(data, registry);
  factory GitDiffRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitDiffRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitDiffRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitDiffRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..aOB(2, _omitFieldNames ? '' : 'staged')
    ..aOS(3, _omitFieldNames ? '' : 'path')
    ..aI(4, _omitFieldNames ? '' : 'contextLines')
    ..aOS(5, _omitFieldNames ? '' : 'commit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitDiffRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitDiffRequest copyWith(void Function(GitDiffRequest) updates) =>
      super.copyWith((message) => updates(message as GitDiffRequest))
          as GitDiffRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitDiffRequest() / GitDiffRequest.new instead')
  static GitDiffRequest create() => GitDiffRequest._();
  static $pb.GeneratedMessage $_createMessage() => GitDiffRequest._();
  @$core.override
  GitDiffRequest createEmptyInstance() => GitDiffRequest._();
  @$core.pragma('dart2js:noInline')
  static GitDiffRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitDiffRequest>(
          GitDiffRequest.$_createMessage);
  static GitDiffRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get staged => $_getBF(1);
  @$pb.TagNumber(2)
  set staged($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStaged() => $_has(1);
  @$pb.TagNumber(2)
  void clearStaged() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get path => $_getSZ(2);
  @$pb.TagNumber(3)
  set path($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPath() => $_has(2);
  @$pb.TagNumber(3)
  void clearPath() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get contextLines => $_getIZ(3);
  @$pb.TagNumber(4)
  set contextLines($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasContextLines() => $_has(3);
  @$pb.TagNumber(4)
  void clearContextLines() => $_clearField(4);

  /// 非空 = 查看某个提交的 patch(git show <commit>);此时忽略 staged。
  @$pb.TagNumber(5)
  $core.String get commit => $_getSZ(4);
  @$pb.TagNumber(5)
  set commit($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCommit() => $_has(4);
  @$pb.TagNumber(5)
  void clearCommit() => $_clearField(5);
}

class GitDiffResponse extends $pb.GeneratedMessage {
  factory GitDiffResponse({
    $core.String? unifiedDiff,
    $core.Iterable<$core.String>? changedFiles,
  }) {
    final result = GitDiffResponse._();
    if (unifiedDiff != null) result.unifiedDiff = unifiedDiff;
    if (changedFiles != null) result.changedFiles.addAll(changedFiles);
    return result;
  }

  GitDiffResponse._();

  factory GitDiffResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitDiffResponse()..mergeFromBuffer(data, registry);
  factory GitDiffResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitDiffResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitDiffResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitDiffResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'unifiedDiff')
    ..pPS(2, _omitFieldNames ? '' : 'changedFiles')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitDiffResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitDiffResponse copyWith(void Function(GitDiffResponse) updates) =>
      super.copyWith((message) => updates(message as GitDiffResponse))
          as GitDiffResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitDiffResponse() / GitDiffResponse.new instead')
  static GitDiffResponse create() => GitDiffResponse._();
  static $pb.GeneratedMessage $_createMessage() => GitDiffResponse._();
  @$core.override
  GitDiffResponse createEmptyInstance() => GitDiffResponse._();
  @$core.pragma('dart2js:noInline')
  static GitDiffResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitDiffResponse>(
          GitDiffResponse.$_createMessage);
  static GitDiffResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get unifiedDiff => $_getSZ(0);
  @$pb.TagNumber(1)
  set unifiedDiff($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUnifiedDiff() => $_has(0);
  @$pb.TagNumber(1)
  void clearUnifiedDiff() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get changedFiles => $_getList(1);
}

class GitStageRequest extends $pb.GeneratedMessage {
  factory GitStageRequest({
    $core.String? repo,
    $core.Iterable<$core.String>? paths,
  }) {
    final result = GitStageRequest._();
    if (repo != null) result.repo = repo;
    if (paths != null) result.paths.addAll(paths);
    return result;
  }

  GitStageRequest._();

  factory GitStageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitStageRequest()..mergeFromBuffer(data, registry);
  factory GitStageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitStageRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitStageRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitStageRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..pPS(2, _omitFieldNames ? '' : 'paths')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitStageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitStageRequest copyWith(void Function(GitStageRequest) updates) =>
      super.copyWith((message) => updates(message as GitStageRequest))
          as GitStageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitStageRequest() / GitStageRequest.new instead')
  static GitStageRequest create() => GitStageRequest._();
  static $pb.GeneratedMessage $_createMessage() => GitStageRequest._();
  @$core.override
  GitStageRequest createEmptyInstance() => GitStageRequest._();
  @$core.pragma('dart2js:noInline')
  static GitStageRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitStageRequest>(
          GitStageRequest.$_createMessage);
  static GitStageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get paths => $_getList(1);
}

class GitUnstageRequest extends $pb.GeneratedMessage {
  factory GitUnstageRequest({
    $core.String? repo,
    $core.Iterable<$core.String>? paths,
  }) {
    final result = GitUnstageRequest._();
    if (repo != null) result.repo = repo;
    if (paths != null) result.paths.addAll(paths);
    return result;
  }

  GitUnstageRequest._();

  factory GitUnstageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitUnstageRequest()..mergeFromBuffer(data, registry);
  factory GitUnstageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitUnstageRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitUnstageRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitUnstageRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..pPS(2, _omitFieldNames ? '' : 'paths')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitUnstageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitUnstageRequest copyWith(void Function(GitUnstageRequest) updates) =>
      super.copyWith((message) => updates(message as GitUnstageRequest))
          as GitUnstageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitUnstageRequest() / GitUnstageRequest.new instead')
  static GitUnstageRequest create() => GitUnstageRequest._();
  static $pb.GeneratedMessage $_createMessage() => GitUnstageRequest._();
  @$core.override
  GitUnstageRequest createEmptyInstance() => GitUnstageRequest._();
  @$core.pragma('dart2js:noInline')
  static GitUnstageRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitUnstageRequest>(
          GitUnstageRequest.$_createMessage);
  static GitUnstageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get paths => $_getList(1);
}

class GitCommitRequest extends $pb.GeneratedMessage {
  factory GitCommitRequest({
    $core.String? repo,
    $core.String? message,
    $core.Iterable<$core.String>? paths,
  }) {
    final result = GitCommitRequest._();
    if (repo != null) result.repo = repo;
    if (message != null) result.message = message;
    if (paths != null) result.paths.addAll(paths);
    return result;
  }

  GitCommitRequest._();

  factory GitCommitRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitCommitRequest()..mergeFromBuffer(data, registry);
  factory GitCommitRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitCommitRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitCommitRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitCommitRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..aOS(2, _omitFieldNames ? '' : 'message')
    ..pPS(3, _omitFieldNames ? '' : 'paths')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitCommitRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitCommitRequest copyWith(void Function(GitCommitRequest) updates) =>
      super.copyWith((message) => updates(message as GitCommitRequest))
          as GitCommitRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitCommitRequest() / GitCommitRequest.new instead')
  static GitCommitRequest create() => GitCommitRequest._();
  static $pb.GeneratedMessage $_createMessage() => GitCommitRequest._();
  @$core.override
  GitCommitRequest createEmptyInstance() => GitCommitRequest._();
  @$core.pragma('dart2js:noInline')
  static GitCommitRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitCommitRequest>(
          GitCommitRequest.$_createMessage);
  static GitCommitRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get message => $_getSZ(1);
  @$pb.TagNumber(2)
  set message($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessage() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessage() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get paths => $_getList(2);
}

class GitCommitResponse extends $pb.GeneratedMessage {
  factory GitCommitResponse({
    $core.String? commitHash,
    $core.String? summary,
  }) {
    final result = GitCommitResponse._();
    if (commitHash != null) result.commitHash = commitHash;
    if (summary != null) result.summary = summary;
    return result;
  }

  GitCommitResponse._();

  factory GitCommitResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitCommitResponse()..mergeFromBuffer(data, registry);
  factory GitCommitResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitCommitResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitCommitResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitCommitResponse.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'commitHash')
    ..aOS(2, _omitFieldNames ? '' : 'summary')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitCommitResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitCommitResponse copyWith(void Function(GitCommitResponse) updates) =>
      super.copyWith((message) => updates(message as GitCommitResponse))
          as GitCommitResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitCommitResponse() / GitCommitResponse.new instead')
  static GitCommitResponse create() => GitCommitResponse._();
  static $pb.GeneratedMessage $_createMessage() => GitCommitResponse._();
  @$core.override
  GitCommitResponse createEmptyInstance() => GitCommitResponse._();
  @$core.pragma('dart2js:noInline')
  static GitCommitResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitCommitResponse>(
          GitCommitResponse.$_createMessage);
  static GitCommitResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get commitHash => $_getSZ(0);
  @$pb.TagNumber(1)
  set commitHash($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCommitHash() => $_has(0);
  @$pb.TagNumber(1)
  void clearCommitHash() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get summary => $_getSZ(1);
  @$pb.TagNumber(2)
  set summary($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSummary() => $_has(1);
  @$pb.TagNumber(2)
  void clearSummary() => $_clearField(2);
}

class GitLogRequest extends $pb.GeneratedMessage {
  factory GitLogRequest({
    $core.String? repo,
    $core.int? limit,
    $core.String? path,
  }) {
    final result = GitLogRequest._();
    if (repo != null) result.repo = repo;
    if (limit != null) result.limit = limit;
    if (path != null) result.path = path;
    return result;
  }

  GitLogRequest._();

  factory GitLogRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitLogRequest()..mergeFromBuffer(data, registry);
  factory GitLogRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitLogRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitLogRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitLogRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..aI(2, _omitFieldNames ? '' : 'limit')
    ..aOS(3, _omitFieldNames ? '' : 'path')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitLogRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitLogRequest copyWith(void Function(GitLogRequest) updates) =>
      super.copyWith((message) => updates(message as GitLogRequest))
          as GitLogRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitLogRequest() / GitLogRequest.new instead')
  static GitLogRequest create() => GitLogRequest._();
  static $pb.GeneratedMessage $_createMessage() => GitLogRequest._();
  @$core.override
  GitLogRequest createEmptyInstance() => GitLogRequest._();
  @$core.pragma('dart2js:noInline')
  static GitLogRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitLogRequest>(
          GitLogRequest.$_createMessage);
  static GitLogRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get limit => $_getIZ(1);
  @$pb.TagNumber(2)
  set limit($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLimit() => $_has(1);
  @$pb.TagNumber(2)
  void clearLimit() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get path => $_getSZ(2);
  @$pb.TagNumber(3)
  set path($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPath() => $_has(2);
  @$pb.TagNumber(3)
  void clearPath() => $_clearField(3);
}

class GitLogResponse extends $pb.GeneratedMessage {
  factory GitLogResponse({
    $core.Iterable<GitCommitInfo>? commits,
  }) {
    final result = GitLogResponse._();
    if (commits != null) result.commits.addAll(commits);
    return result;
  }

  GitLogResponse._();

  factory GitLogResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitLogResponse()..mergeFromBuffer(data, registry);
  factory GitLogResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitLogResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitLogResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitLogResponse.$_createMessage)
    ..pPM<GitCommitInfo>(1, _omitFieldNames ? '' : 'commits',
        subBuilder: GitCommitInfo.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitLogResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitLogResponse copyWith(void Function(GitLogResponse) updates) =>
      super.copyWith((message) => updates(message as GitLogResponse))
          as GitLogResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitLogResponse() / GitLogResponse.new instead')
  static GitLogResponse create() => GitLogResponse._();
  static $pb.GeneratedMessage $_createMessage() => GitLogResponse._();
  @$core.override
  GitLogResponse createEmptyInstance() => GitLogResponse._();
  @$core.pragma('dart2js:noInline')
  static GitLogResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitLogResponse>(
          GitLogResponse.$_createMessage);
  static GitLogResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<GitCommitInfo> get commits => $_getList(0);
}

class GitCommitInfo extends $pb.GeneratedMessage {
  factory GitCommitInfo({
    $core.String? hash,
    $core.String? shortHash,
    $core.String? author,
    $core.String? message,
    $fixnum.Int64? timestampUnixMs,
  }) {
    final result = GitCommitInfo._();
    if (hash != null) result.hash = hash;
    if (shortHash != null) result.shortHash = shortHash;
    if (author != null) result.author = author;
    if (message != null) result.message = message;
    if (timestampUnixMs != null) result.timestampUnixMs = timestampUnixMs;
    return result;
  }

  GitCommitInfo._();

  factory GitCommitInfo.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitCommitInfo()..mergeFromBuffer(data, registry);
  factory GitCommitInfo.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitCommitInfo()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitCommitInfo',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitCommitInfo.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'hash')
    ..aOS(2, _omitFieldNames ? '' : 'shortHash')
    ..aOS(3, _omitFieldNames ? '' : 'author')
    ..aOS(4, _omitFieldNames ? '' : 'message')
    ..aInt64(5, _omitFieldNames ? '' : 'timestampUnixMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitCommitInfo clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitCommitInfo copyWith(void Function(GitCommitInfo) updates) =>
      super.copyWith((message) => updates(message as GitCommitInfo))
          as GitCommitInfo;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitCommitInfo() / GitCommitInfo.new instead')
  static GitCommitInfo create() => GitCommitInfo._();
  static $pb.GeneratedMessage $_createMessage() => GitCommitInfo._();
  @$core.override
  GitCommitInfo createEmptyInstance() => GitCommitInfo._();
  @$core.pragma('dart2js:noInline')
  static GitCommitInfo getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GitCommitInfo>(
          GitCommitInfo.$_createMessage);
  static GitCommitInfo? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get hash => $_getSZ(0);
  @$pb.TagNumber(1)
  set hash($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHash() => $_has(0);
  @$pb.TagNumber(1)
  void clearHash() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get shortHash => $_getSZ(1);
  @$pb.TagNumber(2)
  set shortHash($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasShortHash() => $_has(1);
  @$pb.TagNumber(2)
  void clearShortHash() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get author => $_getSZ(2);
  @$pb.TagNumber(3)
  set author($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAuthor() => $_has(2);
  @$pb.TagNumber(3)
  void clearAuthor() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get message => $_getSZ(3);
  @$pb.TagNumber(4)
  set message($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMessage() => $_has(3);
  @$pb.TagNumber(4)
  void clearMessage() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get timestampUnixMs => $_getI64(4);
  @$pb.TagNumber(5)
  set timestampUnixMs($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTimestampUnixMs() => $_has(4);
  @$pb.TagNumber(5)
  void clearTimestampUnixMs() => $_clearField(5);
}

class ListBranchesRequest extends $pb.GeneratedMessage {
  factory ListBranchesRequest({
    $core.String? repo,
  }) {
    final result = ListBranchesRequest._();
    if (repo != null) result.repo = repo;
    return result;
  }

  ListBranchesRequest._();

  factory ListBranchesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListBranchesRequest()..mergeFromBuffer(data, registry);
  factory ListBranchesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListBranchesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBranchesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListBranchesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBranchesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBranchesRequest copyWith(void Function(ListBranchesRequest) updates) =>
      super.copyWith((message) => updates(message as ListBranchesRequest))
          as ListBranchesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use ListBranchesRequest() / ListBranchesRequest.new instead')
  static ListBranchesRequest create() => ListBranchesRequest._();
  static $pb.GeneratedMessage $_createMessage() => ListBranchesRequest._();
  @$core.override
  ListBranchesRequest createEmptyInstance() => ListBranchesRequest._();
  @$core.pragma('dart2js:noInline')
  static ListBranchesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBranchesRequest>(
          ListBranchesRequest.$_createMessage);
  static ListBranchesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);
}

class ListBranchesResponse extends $pb.GeneratedMessage {
  factory ListBranchesResponse({
    $core.Iterable<GitBranch>? branches,
  }) {
    final result = ListBranchesResponse._();
    if (branches != null) result.branches.addAll(branches);
    return result;
  }

  ListBranchesResponse._();

  factory ListBranchesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListBranchesResponse()..mergeFromBuffer(data, registry);
  factory ListBranchesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ListBranchesResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBranchesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: ListBranchesResponse.$_createMessage)
    ..pPM<GitBranch>(1, _omitFieldNames ? '' : 'branches',
        subBuilder: GitBranch.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBranchesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBranchesResponse copyWith(void Function(ListBranchesResponse) updates) =>
      super.copyWith((message) => updates(message as ListBranchesResponse))
          as ListBranchesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use ListBranchesResponse() / ListBranchesResponse.new instead')
  static ListBranchesResponse create() => ListBranchesResponse._();
  static $pb.GeneratedMessage $_createMessage() => ListBranchesResponse._();
  @$core.override
  ListBranchesResponse createEmptyInstance() => ListBranchesResponse._();
  @$core.pragma('dart2js:noInline')
  static ListBranchesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBranchesResponse>(
          ListBranchesResponse.$_createMessage);
  static ListBranchesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<GitBranch> get branches => $_getList(0);
}

class GitBranch extends $pb.GeneratedMessage {
  factory GitBranch({
    $core.String? name,
    $core.bool? isCurrent,
    $core.bool? isRemote,
  }) {
    final result = GitBranch._();
    if (name != null) result.name = name;
    if (isCurrent != null) result.isCurrent = isCurrent;
    if (isRemote != null) result.isRemote = isRemote;
    return result;
  }

  GitBranch._();

  factory GitBranch.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitBranch()..mergeFromBuffer(data, registry);
  factory GitBranch.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      GitBranch()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GitBranch',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: GitBranch.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOB(2, _omitFieldNames ? '' : 'isCurrent')
    ..aOB(3, _omitFieldNames ? '' : 'isRemote')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitBranch clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GitBranch copyWith(void Function(GitBranch) updates) =>
      super.copyWith((message) => updates(message as GitBranch)) as GitBranch;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use GitBranch() / GitBranch.new instead')
  static GitBranch create() => GitBranch._();
  static $pb.GeneratedMessage $_createMessage() => GitBranch._();
  @$core.override
  GitBranch createEmptyInstance() => GitBranch._();
  @$core.pragma('dart2js:noInline')
  static GitBranch getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GitBranch>(GitBranch.$_createMessage);
  static GitBranch? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get isCurrent => $_getBF(1);
  @$pb.TagNumber(2)
  set isCurrent($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIsCurrent() => $_has(1);
  @$pb.TagNumber(2)
  void clearIsCurrent() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isRemote => $_getBF(2);
  @$pb.TagNumber(3)
  set isRemote($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsRemote() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsRemote() => $_clearField(3);
}

class CheckoutBranchRequest extends $pb.GeneratedMessage {
  factory CheckoutBranchRequest({
    $core.String? repo,
    $core.String? branch,
    $core.bool? create_3,
  }) {
    final result = CheckoutBranchRequest._();
    if (repo != null) result.repo = repo;
    if (branch != null) result.branch = branch;
    if (create_3 != null) result.create_3 = create_3;
    return result;
  }

  CheckoutBranchRequest._();

  factory CheckoutBranchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckoutBranchRequest()..mergeFromBuffer(data, registry);
  factory CheckoutBranchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CheckoutBranchRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckoutBranchRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CheckoutBranchRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..aOS(2, _omitFieldNames ? '' : 'branch')
    ..aOB(3, _omitFieldNames ? '' : 'create')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckoutBranchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckoutBranchRequest copyWith(
          void Function(CheckoutBranchRequest) updates) =>
      super.copyWith((message) => updates(message as CheckoutBranchRequest))
          as CheckoutBranchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use CheckoutBranchRequest() / CheckoutBranchRequest.new instead')
  static CheckoutBranchRequest create() => CheckoutBranchRequest._();
  static $pb.GeneratedMessage $_createMessage() => CheckoutBranchRequest._();
  @$core.override
  CheckoutBranchRequest createEmptyInstance() => CheckoutBranchRequest._();
  @$core.pragma('dart2js:noInline')
  static CheckoutBranchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckoutBranchRequest>(
          CheckoutBranchRequest.$_createMessage);
  static CheckoutBranchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get branch => $_getSZ(1);
  @$pb.TagNumber(2)
  set branch($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBranch() => $_has(1);
  @$pb.TagNumber(2)
  void clearBranch() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get create_3 => $_getBF(2);
  @$pb.TagNumber(3)
  set create_3($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCreate_3() => $_has(2);
  @$pb.TagNumber(3)
  void clearCreate_3() => $_clearField(3);
}

class CreateBranchRequest extends $pb.GeneratedMessage {
  factory CreateBranchRequest({
    $core.String? repo,
    $core.String? name,
    $core.String? fromBranch,
  }) {
    final result = CreateBranchRequest._();
    if (repo != null) result.repo = repo;
    if (name != null) result.name = name;
    if (fromBranch != null) result.fromBranch = fromBranch;
    return result;
  }

  CreateBranchRequest._();

  factory CreateBranchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateBranchRequest()..mergeFromBuffer(data, registry);
  factory CreateBranchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CreateBranchRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateBranchRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: CreateBranchRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'fromBranch')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateBranchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateBranchRequest copyWith(void Function(CreateBranchRequest) updates) =>
      super.copyWith((message) => updates(message as CreateBranchRequest))
          as CreateBranchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core
      .Deprecated('Use CreateBranchRequest() / CreateBranchRequest.new instead')
  static CreateBranchRequest create() => CreateBranchRequest._();
  static $pb.GeneratedMessage $_createMessage() => CreateBranchRequest._();
  @$core.override
  CreateBranchRequest createEmptyInstance() => CreateBranchRequest._();
  @$core.pragma('dart2js:noInline')
  static CreateBranchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateBranchRequest>(
          CreateBranchRequest.$_createMessage);
  static CreateBranchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get fromBranch => $_getSZ(2);
  @$pb.TagNumber(3)
  set fromBranch($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFromBranch() => $_has(2);
  @$pb.TagNumber(3)
  void clearFromBranch() => $_clearField(3);
}

class DiscardChangesRequest extends $pb.GeneratedMessage {
  factory DiscardChangesRequest({
    $core.String? repo,
    $core.Iterable<$core.String>? paths,
  }) {
    final result = DiscardChangesRequest._();
    if (repo != null) result.repo = repo;
    if (paths != null) result.paths.addAll(paths);
    return result;
  }

  DiscardChangesRequest._();

  factory DiscardChangesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DiscardChangesRequest()..mergeFromBuffer(data, registry);
  factory DiscardChangesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      DiscardChangesRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DiscardChangesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'pocketagent.v1'),
      createEmptyInstance: DiscardChangesRequest.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'repo')
    ..pPS(2, _omitFieldNames ? '' : 'paths')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DiscardChangesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DiscardChangesRequest copyWith(
          void Function(DiscardChangesRequest) updates) =>
      super.copyWith((message) => updates(message as DiscardChangesRequest))
          as DiscardChangesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated(
      'Use DiscardChangesRequest() / DiscardChangesRequest.new instead')
  static DiscardChangesRequest create() => DiscardChangesRequest._();
  static $pb.GeneratedMessage $_createMessage() => DiscardChangesRequest._();
  @$core.override
  DiscardChangesRequest createEmptyInstance() => DiscardChangesRequest._();
  @$core.pragma('dart2js:noInline')
  static DiscardChangesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DiscardChangesRequest>(
          DiscardChangesRequest.$_createMessage);
  static DiscardChangesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get repo => $_getSZ(0);
  @$pb.TagNumber(1)
  set repo($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRepo() => $_has(0);
  @$pb.TagNumber(1)
  void clearRepo() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get paths => $_getList(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
