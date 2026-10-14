// This is a generated file - do not edit.
//
// Generated from v1/git.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use gitStatusRequestDescriptor instead')
const GitStatusRequest$json = {
  '1': 'GitStatusRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
  ],
};

/// Descriptor for `GitStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitStatusRequestDescriptor = $convert
    .base64Decode('ChBHaXRTdGF0dXNSZXF1ZXN0EhIKBHJlcG8YASABKAlSBHJlcG8=');

@$core.Deprecated('Use gitStatusDescriptor instead')
const GitStatus$json = {
  '1': 'GitStatus',
  '2': [
    {'1': 'branch', '3': 1, '4': 1, '5': 9, '10': 'branch'},
    {'1': 'upstream', '3': 2, '4': 1, '5': 9, '10': 'upstream'},
    {'1': 'ahead', '3': 3, '4': 1, '5': 8, '10': 'ahead'},
    {'1': 'behind', '3': 4, '4': 1, '5': 8, '10': 'behind'},
    {
      '1': 'changes',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.GitFileChange',
      '10': 'changes'
    },
  ],
};

/// Descriptor for `GitStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitStatusDescriptor = $convert.base64Decode(
    'CglHaXRTdGF0dXMSFgoGYnJhbmNoGAEgASgJUgZicmFuY2gSGgoIdXBzdHJlYW0YAiABKAlSCH'
    'Vwc3RyZWFtEhQKBWFoZWFkGAMgASgIUgVhaGVhZBIWCgZiZWhpbmQYBCABKAhSBmJlaGluZBI3'
    'CgdjaGFuZ2VzGAUgAygLMh0ucG9ja2V0YWdlbnQudjEuR2l0RmlsZUNoYW5nZVIHY2hhbmdlcw'
    '==');

@$core.Deprecated('Use gitFileChangeDescriptor instead')
const GitFileChange$json = {
  '1': 'GitFileChange',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
    {'1': 'staged_status', '3': 2, '4': 1, '5': 9, '10': 'stagedStatus'},
    {'1': 'worktree_status', '3': 3, '4': 1, '5': 9, '10': 'worktreeStatus'},
  ],
};

/// Descriptor for `GitFileChange`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitFileChangeDescriptor = $convert.base64Decode(
    'Cg1HaXRGaWxlQ2hhbmdlEhIKBHBhdGgYASABKAlSBHBhdGgSIwoNc3RhZ2VkX3N0YXR1cxgCIA'
    'EoCVIMc3RhZ2VkU3RhdHVzEicKD3dvcmt0cmVlX3N0YXR1cxgDIAEoCVIOd29ya3RyZWVTdGF0'
    'dXM=');

@$core.Deprecated('Use gitDiffRequestDescriptor instead')
const GitDiffRequest$json = {
  '1': 'GitDiffRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'staged', '3': 2, '4': 1, '5': 8, '10': 'staged'},
    {'1': 'path', '3': 3, '4': 1, '5': 9, '10': 'path'},
    {'1': 'context_lines', '3': 4, '4': 1, '5': 5, '10': 'contextLines'},
    {'1': 'commit', '3': 5, '4': 1, '5': 9, '10': 'commit'},
  ],
};

/// Descriptor for `GitDiffRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitDiffRequestDescriptor = $convert.base64Decode(
    'Cg5HaXREaWZmUmVxdWVzdBISCgRyZXBvGAEgASgJUgRyZXBvEhYKBnN0YWdlZBgCIAEoCFIGc3'
    'RhZ2VkEhIKBHBhdGgYAyABKAlSBHBhdGgSIwoNY29udGV4dF9saW5lcxgEIAEoBVIMY29udGV4'
    'dExpbmVzEhYKBmNvbW1pdBgFIAEoCVIGY29tbWl0');

@$core.Deprecated('Use gitDiffResponseDescriptor instead')
const GitDiffResponse$json = {
  '1': 'GitDiffResponse',
  '2': [
    {'1': 'unified_diff', '3': 1, '4': 1, '5': 9, '10': 'unifiedDiff'},
    {'1': 'changed_files', '3': 2, '4': 3, '5': 9, '10': 'changedFiles'},
  ],
};

/// Descriptor for `GitDiffResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitDiffResponseDescriptor = $convert.base64Decode(
    'Cg9HaXREaWZmUmVzcG9uc2USIQoMdW5pZmllZF9kaWZmGAEgASgJUgt1bmlmaWVkRGlmZhIjCg'
    '1jaGFuZ2VkX2ZpbGVzGAIgAygJUgxjaGFuZ2VkRmlsZXM=');

@$core.Deprecated('Use gitStageRequestDescriptor instead')
const GitStageRequest$json = {
  '1': 'GitStageRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'paths', '3': 2, '4': 3, '5': 9, '10': 'paths'},
  ],
};

/// Descriptor for `GitStageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitStageRequestDescriptor = $convert.base64Decode(
    'Cg9HaXRTdGFnZVJlcXVlc3QSEgoEcmVwbxgBIAEoCVIEcmVwbxIUCgVwYXRocxgCIAMoCVIFcG'
    'F0aHM=');

@$core.Deprecated('Use gitUnstageRequestDescriptor instead')
const GitUnstageRequest$json = {
  '1': 'GitUnstageRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'paths', '3': 2, '4': 3, '5': 9, '10': 'paths'},
  ],
};

/// Descriptor for `GitUnstageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitUnstageRequestDescriptor = $convert.base64Decode(
    'ChFHaXRVbnN0YWdlUmVxdWVzdBISCgRyZXBvGAEgASgJUgRyZXBvEhQKBXBhdGhzGAIgAygJUg'
    'VwYXRocw==');

@$core.Deprecated('Use gitCommitRequestDescriptor instead')
const GitCommitRequest$json = {
  '1': 'GitCommitRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'message', '3': 2, '4': 1, '5': 9, '10': 'message'},
    {'1': 'paths', '3': 3, '4': 3, '5': 9, '10': 'paths'},
  ],
};

/// Descriptor for `GitCommitRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitCommitRequestDescriptor = $convert.base64Decode(
    'ChBHaXRDb21taXRSZXF1ZXN0EhIKBHJlcG8YASABKAlSBHJlcG8SGAoHbWVzc2FnZRgCIAEoCV'
    'IHbWVzc2FnZRIUCgVwYXRocxgDIAMoCVIFcGF0aHM=');

@$core.Deprecated('Use gitCommitResponseDescriptor instead')
const GitCommitResponse$json = {
  '1': 'GitCommitResponse',
  '2': [
    {'1': 'commit_hash', '3': 1, '4': 1, '5': 9, '10': 'commitHash'},
    {'1': 'summary', '3': 2, '4': 1, '5': 9, '10': 'summary'},
  ],
};

/// Descriptor for `GitCommitResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitCommitResponseDescriptor = $convert.base64Decode(
    'ChFHaXRDb21taXRSZXNwb25zZRIfCgtjb21taXRfaGFzaBgBIAEoCVIKY29tbWl0SGFzaBIYCg'
    'dzdW1tYXJ5GAIgASgJUgdzdW1tYXJ5');

@$core.Deprecated('Use gitLogRequestDescriptor instead')
const GitLogRequest$json = {
  '1': 'GitLogRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'limit', '3': 2, '4': 1, '5': 5, '10': 'limit'},
    {'1': 'path', '3': 3, '4': 1, '5': 9, '10': 'path'},
  ],
};

/// Descriptor for `GitLogRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitLogRequestDescriptor = $convert.base64Decode(
    'Cg1HaXRMb2dSZXF1ZXN0EhIKBHJlcG8YASABKAlSBHJlcG8SFAoFbGltaXQYAiABKAVSBWxpbW'
    'l0EhIKBHBhdGgYAyABKAlSBHBhdGg=');

@$core.Deprecated('Use gitLogResponseDescriptor instead')
const GitLogResponse$json = {
  '1': 'GitLogResponse',
  '2': [
    {
      '1': 'commits',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.GitCommitInfo',
      '10': 'commits'
    },
  ],
};

/// Descriptor for `GitLogResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitLogResponseDescriptor = $convert.base64Decode(
    'Cg5HaXRMb2dSZXNwb25zZRI3Cgdjb21taXRzGAEgAygLMh0ucG9ja2V0YWdlbnQudjEuR2l0Q2'
    '9tbWl0SW5mb1IHY29tbWl0cw==');

@$core.Deprecated('Use gitCommitInfoDescriptor instead')
const GitCommitInfo$json = {
  '1': 'GitCommitInfo',
  '2': [
    {'1': 'hash', '3': 1, '4': 1, '5': 9, '10': 'hash'},
    {'1': 'short_hash', '3': 2, '4': 1, '5': 9, '10': 'shortHash'},
    {'1': 'author', '3': 3, '4': 1, '5': 9, '10': 'author'},
    {'1': 'message', '3': 4, '4': 1, '5': 9, '10': 'message'},
    {'1': 'timestamp_unix_ms', '3': 5, '4': 1, '5': 3, '10': 'timestampUnixMs'},
  ],
};

/// Descriptor for `GitCommitInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitCommitInfoDescriptor = $convert.base64Decode(
    'Cg1HaXRDb21taXRJbmZvEhIKBGhhc2gYASABKAlSBGhhc2gSHQoKc2hvcnRfaGFzaBgCIAEoCV'
    'IJc2hvcnRIYXNoEhYKBmF1dGhvchgDIAEoCVIGYXV0aG9yEhgKB21lc3NhZ2UYBCABKAlSB21l'
    'c3NhZ2USKgoRdGltZXN0YW1wX3VuaXhfbXMYBSABKANSD3RpbWVzdGFtcFVuaXhNcw==');

@$core.Deprecated('Use listBranchesRequestDescriptor instead')
const ListBranchesRequest$json = {
  '1': 'ListBranchesRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
  ],
};

/// Descriptor for `ListBranchesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBranchesRequestDescriptor = $convert
    .base64Decode('ChNMaXN0QnJhbmNoZXNSZXF1ZXN0EhIKBHJlcG8YASABKAlSBHJlcG8=');

@$core.Deprecated('Use listBranchesResponseDescriptor instead')
const ListBranchesResponse$json = {
  '1': 'ListBranchesResponse',
  '2': [
    {
      '1': 'branches',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.GitBranch',
      '10': 'branches'
    },
  ],
};

/// Descriptor for `ListBranchesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBranchesResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0QnJhbmNoZXNSZXNwb25zZRI1CghicmFuY2hlcxgBIAMoCzIZLnBvY2tldGFnZW50Ln'
    'YxLkdpdEJyYW5jaFIIYnJhbmNoZXM=');

@$core.Deprecated('Use gitBranchDescriptor instead')
const GitBranch$json = {
  '1': 'GitBranch',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'is_current', '3': 2, '4': 1, '5': 8, '10': 'isCurrent'},
    {'1': 'is_remote', '3': 3, '4': 1, '5': 8, '10': 'isRemote'},
  ],
};

/// Descriptor for `GitBranch`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gitBranchDescriptor = $convert.base64Decode(
    'CglHaXRCcmFuY2gSEgoEbmFtZRgBIAEoCVIEbmFtZRIdCgppc19jdXJyZW50GAIgASgIUglpc0'
    'N1cnJlbnQSGwoJaXNfcmVtb3RlGAMgASgIUghpc1JlbW90ZQ==');

@$core.Deprecated('Use checkoutBranchRequestDescriptor instead')
const CheckoutBranchRequest$json = {
  '1': 'CheckoutBranchRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'branch', '3': 2, '4': 1, '5': 9, '10': 'branch'},
    {'1': 'create', '3': 3, '4': 1, '5': 8, '10': 'create'},
  ],
};

/// Descriptor for `CheckoutBranchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkoutBranchRequestDescriptor = $convert.base64Decode(
    'ChVDaGVja291dEJyYW5jaFJlcXVlc3QSEgoEcmVwbxgBIAEoCVIEcmVwbxIWCgZicmFuY2gYAi'
    'ABKAlSBmJyYW5jaBIWCgZjcmVhdGUYAyABKAhSBmNyZWF0ZQ==');

@$core.Deprecated('Use createBranchRequestDescriptor instead')
const CreateBranchRequest$json = {
  '1': 'CreateBranchRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'from_branch', '3': 3, '4': 1, '5': 9, '10': 'fromBranch'},
  ],
};

/// Descriptor for `CreateBranchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createBranchRequestDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVCcmFuY2hSZXF1ZXN0EhIKBHJlcG8YASABKAlSBHJlcG8SEgoEbmFtZRgCIAEoCV'
    'IEbmFtZRIfCgtmcm9tX2JyYW5jaBgDIAEoCVIKZnJvbUJyYW5jaA==');

@$core.Deprecated('Use discardChangesRequestDescriptor instead')
const DiscardChangesRequest$json = {
  '1': 'DiscardChangesRequest',
  '2': [
    {'1': 'repo', '3': 1, '4': 1, '5': 9, '10': 'repo'},
    {'1': 'paths', '3': 2, '4': 3, '5': 9, '10': 'paths'},
  ],
};

/// Descriptor for `DiscardChangesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List discardChangesRequestDescriptor = $convert.base64Decode(
    'ChVEaXNjYXJkQ2hhbmdlc1JlcXVlc3QSEgoEcmVwbxgBIAEoCVIEcmVwbxIUCgVwYXRocxgCIA'
    'MoCVIFcGF0aHM=');
