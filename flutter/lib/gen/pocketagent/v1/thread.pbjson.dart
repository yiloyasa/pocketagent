// This is a generated file - do not edit.
//
// Generated from v1/thread.proto.

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

@$core.Deprecated('Use threadStatusDescriptor instead')
const ThreadStatus$json = {
  '1': 'ThreadStatus',
  '2': [
    {'1': 'THREAD_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'THREAD_STATUS_IDLE', '2': 1},
    {'1': 'THREAD_STATUS_RUNNING', '2': 2},
    {'1': 'THREAD_STATUS_WAITING_APPROVAL', '2': 3},
    {'1': 'THREAD_STATUS_ARCHIVED', '2': 4},
  ],
};

/// Descriptor for `ThreadStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List threadStatusDescriptor = $convert.base64Decode(
    'CgxUaHJlYWRTdGF0dXMSHQoZVEhSRUFEX1NUQVRVU19VTlNQRUNJRklFRBAAEhYKElRIUkVBRF'
    '9TVEFUVVNfSURMRRABEhkKFVRIUkVBRF9TVEFUVVNfUlVOTklORxACEiIKHlRIUkVBRF9TVEFU'
    'VVNfV0FJVElOR19BUFBST1ZBTBADEhoKFlRIUkVBRF9TVEFUVVNfQVJDSElWRUQQBA==');

@$core.Deprecated('Use outputStateDescriptor instead')
const OutputState$json = {
  '1': 'OutputState',
  '2': [
    {'1': 'OUTPUT_STATE_UNSPECIFIED', '2': 0},
    {'1': 'OUTPUT_STATE_TEXT', '2': 1},
    {'1': 'OUTPUT_STATE_EMPTY', '2': 2},
    {'1': 'OUTPUT_STATE_ARTIFACT_ONLY', '2': 3},
    {'1': 'OUTPUT_STATE_ERROR', '2': 4},
    {'1': 'OUTPUT_STATE_RECOVERED_FROM_NATIVE_HISTORY', '2': 5},
    {'1': 'OUTPUT_STATE_ADAPTER_LOST_OUTPUT', '2': 6},
  ],
};

/// Descriptor for `OutputState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List outputStateDescriptor = $convert.base64Decode(
    'CgtPdXRwdXRTdGF0ZRIcChhPVVRQVVRfU1RBVEVfVU5TUEVDSUZJRUQQABIVChFPVVRQVVRfU1'
    'RBVEVfVEVYVBABEhYKEk9VVFBVVF9TVEFURV9FTVBUWRACEh4KGk9VVFBVVF9TVEFURV9BUlRJ'
    'RkFDVF9PTkxZEAMSFgoST1VUUFVUX1NUQVRFX0VSUk9SEAQSLgoqT1VUUFVUX1NUQVRFX1JFQ0'
    '9WRVJFRF9GUk9NX05BVElWRV9ISVNUT1JZEAUSJAogT1VUUFVUX1NUQVRFX0FEQVBURVJfTE9T'
    'VF9PVVRQVVQQBg==');

@$core.Deprecated('Use startReviewRequestDescriptor instead')
const StartReviewRequest$json = {
  '1': 'StartReviewRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'focus', '3': 2, '4': 1, '5': 9, '10': 'focus'},
  ],
};

/// Descriptor for `StartReviewRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startReviewRequestDescriptor = $convert.base64Decode(
    'ChJTdGFydFJldmlld1JlcXVlc3QSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZBIUCgVmb2'
    'N1cxgCIAEoCVIFZm9jdXM=');

@$core.Deprecated('Use startReviewResponseDescriptor instead')
const StartReviewResponse$json = {
  '1': 'StartReviewResponse',
  '2': [
    {'1': 'review_id', '3': 1, '4': 1, '5': 9, '10': 'reviewId'},
    {'1': 'started', '3': 2, '4': 1, '5': 8, '10': 'started'},
  ],
};

/// Descriptor for `StartReviewResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startReviewResponseDescriptor = $convert.base64Decode(
    'ChNTdGFydFJldmlld1Jlc3BvbnNlEhsKCXJldmlld19pZBgBIAEoCVIIcmV2aWV3SWQSGAoHc3'
    'RhcnRlZBgCIAEoCFIHc3RhcnRlZA==');

@$core.Deprecated('Use delegateSubTaskRequestDescriptor instead')
const DelegateSubTaskRequest$json = {
  '1': 'DelegateSubTaskRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'task', '3': 2, '4': 1, '5': 9, '10': 'task'},
    {'1': 'workspace', '3': 3, '4': 1, '5': 9, '10': 'workspace'},
    {'1': 'model', '3': 4, '4': 1, '5': 9, '10': 'model'},
    {'1': 'effort', '3': 5, '4': 1, '5': 9, '10': 'effort'},
    {'1': 'parent_thread_id', '3': 6, '4': 1, '5': 9, '10': 'parentThreadId'},
    {'1': 'role_id', '3': 7, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `DelegateSubTaskRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List delegateSubTaskRequestDescriptor = $convert.base64Decode(
    'ChZEZWxlZ2F0ZVN1YlRhc2tSZXF1ZXN0EhkKCGFnZW50X2lkGAEgASgJUgdhZ2VudElkEhIKBH'
    'Rhc2sYAiABKAlSBHRhc2sSHAoJd29ya3NwYWNlGAMgASgJUgl3b3Jrc3BhY2USFAoFbW9kZWwY'
    'BCABKAlSBW1vZGVsEhYKBmVmZm9ydBgFIAEoCVIGZWZmb3J0EigKEHBhcmVudF90aHJlYWRfaW'
    'QYBiABKAlSDnBhcmVudFRocmVhZElkEhcKB3JvbGVfaWQYByABKAlSBnJvbGVJZA==');

@$core.Deprecated('Use delegateSubTaskResponseDescriptor instead')
const DelegateSubTaskResponse$json = {
  '1': 'DelegateSubTaskResponse',
  '2': [
    {'1': 'child_thread_id', '3': 1, '4': 1, '5': 9, '10': 'childThreadId'},
    {'1': 'summary', '3': 2, '4': 1, '5': 9, '10': 'summary'},
    {'1': 'state', '3': 3, '4': 1, '5': 9, '10': 'state'},
    {'1': 'error', '3': 4, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `DelegateSubTaskResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List delegateSubTaskResponseDescriptor = $convert.base64Decode(
    'ChdEZWxlZ2F0ZVN1YlRhc2tSZXNwb25zZRImCg9jaGlsZF90aHJlYWRfaWQYASABKAlSDWNoaW'
    'xkVGhyZWFkSWQSGAoHc3VtbWFyeRgCIAEoCVIHc3VtbWFyeRIUCgVzdGF0ZRgDIAEoCVIFc3Rh'
    'dGUSFAoFZXJyb3IYBCABKAlSBWVycm9y');

@$core.Deprecated('Use collabRoleDescriptor instead')
const CollabRole$json = {
  '1': 'CollabRole',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'agent_id', '3': 3, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {'1': 'instructions', '3': 5, '4': 1, '5': 9, '10': 'instructions'},
    {'1': 'model', '3': 6, '4': 1, '5': 9, '10': 'model'},
    {'1': 'effort', '3': 7, '4': 1, '5': 9, '10': 'effort'},
    {'1': 'workspace', '3': 8, '4': 1, '5': 9, '10': 'workspace'},
  ],
};

/// Descriptor for `CollabRole`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List collabRoleDescriptor = $convert.base64Decode(
    'CgpDb2xsYWJSb2xlEg4KAmlkGAEgASgJUgJpZBISCgRuYW1lGAIgASgJUgRuYW1lEhkKCGFnZW'
    '50X2lkGAMgASgJUgdhZ2VudElkEiAKC2Rlc2NyaXB0aW9uGAQgASgJUgtkZXNjcmlwdGlvbhIi'
    'CgxpbnN0cnVjdGlvbnMYBSABKAlSDGluc3RydWN0aW9ucxIUCgVtb2RlbBgGIAEoCVIFbW9kZW'
    'wSFgoGZWZmb3J0GAcgASgJUgZlZmZvcnQSHAoJd29ya3NwYWNlGAggASgJUgl3b3Jrc3BhY2U=');

@$core.Deprecated('Use listCollabRolesRequestDescriptor instead')
const ListCollabRolesRequest$json = {
  '1': 'ListCollabRolesRequest',
};

/// Descriptor for `ListCollabRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCollabRolesRequestDescriptor =
    $convert.base64Decode('ChZMaXN0Q29sbGFiUm9sZXNSZXF1ZXN0');

@$core.Deprecated('Use listCollabRolesResponseDescriptor instead')
const ListCollabRolesResponse$json = {
  '1': 'ListCollabRolesResponse',
  '2': [
    {
      '1': 'roles',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.CollabRole',
      '10': 'roles'
    },
  ],
};

/// Descriptor for `ListCollabRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCollabRolesResponseDescriptor =
    $convert.base64Decode(
        'ChdMaXN0Q29sbGFiUm9sZXNSZXNwb25zZRIwCgVyb2xlcxgBIAMoCzIaLnBvY2tldGFnZW50Ln'
        'YxLkNvbGxhYlJvbGVSBXJvbGVz');

@$core.Deprecated('Use setCollabRoleRequestDescriptor instead')
const SetCollabRoleRequest$json = {
  '1': 'SetCollabRoleRequest',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CollabRole',
      '10': 'role'
    },
  ],
};

/// Descriptor for `SetCollabRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setCollabRoleRequestDescriptor = $convert.base64Decode(
    'ChRTZXRDb2xsYWJSb2xlUmVxdWVzdBIuCgRyb2xlGAEgASgLMhoucG9ja2V0YWdlbnQudjEuQ2'
    '9sbGFiUm9sZVIEcm9sZQ==');

@$core.Deprecated('Use setCollabRoleResponseDescriptor instead')
const SetCollabRoleResponse$json = {
  '1': 'SetCollabRoleResponse',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CollabRole',
      '10': 'role'
    },
  ],
};

/// Descriptor for `SetCollabRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setCollabRoleResponseDescriptor = $convert.base64Decode(
    'ChVTZXRDb2xsYWJSb2xlUmVzcG9uc2USLgoEcm9sZRgBIAEoCzIaLnBvY2tldGFnZW50LnYxLk'
    'NvbGxhYlJvbGVSBHJvbGU=');

@$core.Deprecated('Use deleteCollabRoleRequestDescriptor instead')
const DeleteCollabRoleRequest$json = {
  '1': 'DeleteCollabRoleRequest',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
  ],
};

/// Descriptor for `DeleteCollabRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteCollabRoleRequestDescriptor = $convert
    .base64Decode('ChdEZWxldGVDb2xsYWJSb2xlUmVxdWVzdBIOCgJpZBgBIAEoCVICaWQ=');

@$core.Deprecated('Use deleteCollabRoleResponseDescriptor instead')
const DeleteCollabRoleResponse$json = {
  '1': 'DeleteCollabRoleResponse',
};

/// Descriptor for `DeleteCollabRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteCollabRoleResponseDescriptor =
    $convert.base64Decode('ChhEZWxldGVDb2xsYWJSb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use collabMemberDescriptor instead')
const CollabMember$json = {
  '1': 'CollabMember',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'role_id', '3': 2, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `CollabMember`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List collabMemberDescriptor = $convert.base64Decode(
    'CgxDb2xsYWJNZW1iZXISGQoIYWdlbnRfaWQYASABKAlSB2FnZW50SWQSFwoHcm9sZV9pZBgCIA'
    'EoCVIGcm9sZUlk');

@$core.Deprecated('Use dispatchCollabRequestDescriptor instead')
const DispatchCollabRequest$json = {
  '1': 'DispatchCollabRequest',
  '2': [
    {'1': 'instruction', '3': 1, '4': 1, '5': 9, '10': 'instruction'},
    {
      '1': 'members',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.CollabMember',
      '10': 'members'
    },
    {'1': 'workspace', '3': 3, '4': 1, '5': 9, '10': 'workspace'},
  ],
};

/// Descriptor for `DispatchCollabRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List dispatchCollabRequestDescriptor = $convert.base64Decode(
    'ChVEaXNwYXRjaENvbGxhYlJlcXVlc3QSIAoLaW5zdHJ1Y3Rpb24YASABKAlSC2luc3RydWN0aW'
    '9uEjYKB21lbWJlcnMYAiADKAsyHC5wb2NrZXRhZ2VudC52MS5Db2xsYWJNZW1iZXJSB21lbWJl'
    'cnMSHAoJd29ya3NwYWNlGAMgASgJUgl3b3Jrc3BhY2U=');

@$core.Deprecated('Use dispatchCollabItemDescriptor instead')
const DispatchCollabItem$json = {
  '1': 'DispatchCollabItem',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'thread_id', '3': 2, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'ack', '3': 3, '4': 1, '5': 8, '10': 'ack'},
    {'1': 'error', '3': 4, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `DispatchCollabItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List dispatchCollabItemDescriptor = $convert.base64Decode(
    'ChJEaXNwYXRjaENvbGxhYkl0ZW0SGQoIYWdlbnRfaWQYASABKAlSB2FnZW50SWQSGwoJdGhyZW'
    'FkX2lkGAIgASgJUgh0aHJlYWRJZBIQCgNhY2sYAyABKAhSA2FjaxIUCgVlcnJvchgEIAEoCVIF'
    'ZXJyb3I=');

@$core.Deprecated('Use dispatchCollabResponseDescriptor instead')
const DispatchCollabResponse$json = {
  '1': 'DispatchCollabResponse',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {
      '1': 'items',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.DispatchCollabItem',
      '10': 'items'
    },
  ],
};

/// Descriptor for `DispatchCollabResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List dispatchCollabResponseDescriptor = $convert.base64Decode(
    'ChZEaXNwYXRjaENvbGxhYlJlc3BvbnNlEh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb25JZB'
    'I4CgVpdGVtcxgCIAMoCzIiLnBvY2tldGFnZW50LnYxLkRpc3BhdGNoQ29sbGFiSXRlbVIFaXRl'
    'bXM=');

@$core.Deprecated('Use listSlashCommandsRequestDescriptor instead')
const ListSlashCommandsRequest$json = {
  '1': 'ListSlashCommandsRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'refresh', '3': 2, '4': 1, '5': 8, '10': 'refresh'},
  ],
};

/// Descriptor for `ListSlashCommandsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSlashCommandsRequestDescriptor =
    $convert.base64Decode(
        'ChhMaXN0U2xhc2hDb21tYW5kc1JlcXVlc3QSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZB'
        'IYCgdyZWZyZXNoGAIgASgIUgdyZWZyZXNo');

@$core.Deprecated('Use slashCommandDescriptor instead')
const SlashCommand$json = {
  '1': 'SlashCommand',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'input_hint', '3': 3, '4': 1, '5': 9, '10': 'inputHint'},
  ],
};

/// Descriptor for `SlashCommand`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List slashCommandDescriptor = $convert.base64Decode(
    'CgxTbGFzaENvbW1hbmQSEgoEbmFtZRgBIAEoCVIEbmFtZRIgCgtkZXNjcmlwdGlvbhgCIAEoCV'
    'ILZGVzY3JpcHRpb24SHQoKaW5wdXRfaGludBgDIAEoCVIJaW5wdXRIaW50');

@$core.Deprecated('Use listSlashCommandsResponseDescriptor instead')
const ListSlashCommandsResponse$json = {
  '1': 'ListSlashCommandsResponse',
  '2': [
    {
      '1': 'commands',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.SlashCommand',
      '10': 'commands'
    },
  ],
};

/// Descriptor for `ListSlashCommandsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSlashCommandsResponseDescriptor =
    $convert.base64Decode(
        'ChlMaXN0U2xhc2hDb21tYW5kc1Jlc3BvbnNlEjgKCGNvbW1hbmRzGAEgAygLMhwucG9ja2V0YW'
        'dlbnQudjEuU2xhc2hDb21tYW5kUghjb21tYW5kcw==');

@$core.Deprecated('Use optimizePromptRequestDescriptor instead')
const OptimizePromptRequest$json = {
  '1': 'OptimizePromptRequest',
  '2': [
    {'1': 'text', '3': 1, '4': 1, '5': 9, '10': 'text'},
    {'1': 'style', '3': 2, '4': 1, '5': 9, '10': 'style'},
    {'1': 'agent_id', '3': 3, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'model_id', '3': 4, '4': 1, '5': 9, '10': 'modelId'},
    {'1': 'effort', '3': 5, '4': 1, '5': 9, '10': 'effort'},
  ],
};

/// Descriptor for `OptimizePromptRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List optimizePromptRequestDescriptor = $convert.base64Decode(
    'ChVPcHRpbWl6ZVByb21wdFJlcXVlc3QSEgoEdGV4dBgBIAEoCVIEdGV4dBIUCgVzdHlsZRgCIA'
    'EoCVIFc3R5bGUSGQoIYWdlbnRfaWQYAyABKAlSB2FnZW50SWQSGQoIbW9kZWxfaWQYBCABKAlS'
    'B21vZGVsSWQSFgoGZWZmb3J0GAUgASgJUgZlZmZvcnQ=');

@$core.Deprecated('Use optimizePromptResponseDescriptor instead')
const OptimizePromptResponse$json = {
  '1': 'OptimizePromptResponse',
  '2': [
    {'1': 'optimized', '3': 1, '4': 1, '5': 9, '10': 'optimized'},
  ],
};

/// Descriptor for `OptimizePromptResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List optimizePromptResponseDescriptor =
    $convert.base64Decode(
        'ChZPcHRpbWl6ZVByb21wdFJlc3BvbnNlEhwKCW9wdGltaXplZBgBIAEoCVIJb3B0aW1pemVk');

@$core.Deprecated('Use exportThreadHtmlRequestDescriptor instead')
const ExportThreadHtmlRequest$json = {
  '1': 'ExportThreadHtmlRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `ExportThreadHtmlRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exportThreadHtmlRequestDescriptor =
    $convert.base64Decode(
        'ChdFeHBvcnRUaHJlYWRIdG1sUmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZElk');

@$core.Deprecated('Use exportThreadHtmlResponseDescriptor instead')
const ExportThreadHtmlResponse$json = {
  '1': 'ExportThreadHtmlResponse',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
  ],
};

/// Descriptor for `ExportThreadHtmlResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exportThreadHtmlResponseDescriptor =
    $convert.base64Decode(
        'ChhFeHBvcnRUaHJlYWRIdG1sUmVzcG9uc2USEgoEcGF0aBgBIAEoCVIEcGF0aA==');

@$core.Deprecated('Use approveGuardianDeniedActionRequestDescriptor instead')
const ApproveGuardianDeniedActionRequest$json = {
  '1': 'ApproveGuardianDeniedActionRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'review_id', '3': 2, '4': 1, '5': 9, '10': 'reviewId'},
    {'1': 'target_item_id', '3': 3, '4': 1, '5': 9, '10': 'targetItemId'},
  ],
};

/// Descriptor for `ApproveGuardianDeniedActionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List approveGuardianDeniedActionRequestDescriptor =
    $convert.base64Decode(
        'CiJBcHByb3ZlR3VhcmRpYW5EZW5pZWRBY3Rpb25SZXF1ZXN0EhsKCXRocmVhZF9pZBgBIAEoCV'
        'IIdGhyZWFkSWQSGwoJcmV2aWV3X2lkGAIgASgJUghyZXZpZXdJZBIkCg50YXJnZXRfaXRlbV9p'
        'ZBgDIAEoCVIMdGFyZ2V0SXRlbUlk');

@$core.Deprecated('Use forkThreadRequestDescriptor instead')
const ForkThreadRequest$json = {
  '1': 'ForkThreadRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'last_turn_id', '3': 2, '4': 1, '5': 9, '10': 'lastTurnId'},
  ],
};

/// Descriptor for `ForkThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List forkThreadRequestDescriptor = $convert.base64Decode(
    'ChFGb3JrVGhyZWFkUmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZElkEiAKDGxhc3'
    'RfdHVybl9pZBgCIAEoCVIKbGFzdFR1cm5JZA==');

@$core.Deprecated('Use rollbackThreadRequestDescriptor instead')
const RollbackThreadRequest$json = {
  '1': 'RollbackThreadRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'num_turns', '3': 2, '4': 1, '5': 5, '10': 'numTurns'},
  ],
};

/// Descriptor for `RollbackThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List rollbackThreadRequestDescriptor = $convert.base64Decode(
    'ChVSb2xsYmFja1RocmVhZFJlcXVlc3QSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZBIbCg'
    'ludW1fdHVybnMYAiABKAVSCG51bVR1cm5z');

@$core.Deprecated('Use threadSectionDescriptor instead')
const ThreadSection$json = {
  '1': 'ThreadSection',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'color', '3': 3, '4': 1, '5': 9, '10': 'color'},
    {'1': 'icon', '3': 4, '4': 1, '5': 9, '10': 'icon'},
    {'1': 'thread_count', '3': 5, '4': 1, '5': 5, '10': 'threadCount'},
  ],
};

/// Descriptor for `ThreadSection`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List threadSectionDescriptor = $convert.base64Decode(
    'Cg1UaHJlYWRTZWN0aW9uEg4KAmlkGAEgASgJUgJpZBISCgRuYW1lGAIgASgJUgRuYW1lEhQKBW'
    'NvbG9yGAMgASgJUgVjb2xvchISCgRpY29uGAQgASgJUgRpY29uEiEKDHRocmVhZF9jb3VudBgF'
    'IAEoBVILdGhyZWFkQ291bnQ=');

@$core.Deprecated('Use listThreadSectionsRequestDescriptor instead')
const ListThreadSectionsRequest$json = {
  '1': 'ListThreadSectionsRequest',
};

/// Descriptor for `ListThreadSectionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadSectionsRequestDescriptor =
    $convert.base64Decode('ChlMaXN0VGhyZWFkU2VjdGlvbnNSZXF1ZXN0');

@$core.Deprecated('Use listThreadSectionsResponseDescriptor instead')
const ListThreadSectionsResponse$json = {
  '1': 'ListThreadSectionsResponse',
  '2': [
    {
      '1': 'sections',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ThreadSection',
      '10': 'sections'
    },
  ],
};

/// Descriptor for `ListThreadSectionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadSectionsResponseDescriptor =
    $convert.base64Decode(
        'ChpMaXN0VGhyZWFkU2VjdGlvbnNSZXNwb25zZRI5CghzZWN0aW9ucxgBIAMoCzIdLnBvY2tldG'
        'FnZW50LnYxLlRocmVhZFNlY3Rpb25SCHNlY3Rpb25z');

@$core.Deprecated('Use createThreadSectionRequestDescriptor instead')
const CreateThreadSectionRequest$json = {
  '1': 'CreateThreadSectionRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
  ],
};

/// Descriptor for `CreateThreadSectionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createThreadSectionRequestDescriptor =
    $convert.base64Decode(
        'ChpDcmVhdGVUaHJlYWRTZWN0aW9uUmVxdWVzdBISCgRuYW1lGAEgASgJUgRuYW1l');

@$core.Deprecated('Use renameThreadSectionRequestDescriptor instead')
const RenameThreadSectionRequest$json = {
  '1': 'RenameThreadSectionRequest',
  '2': [
    {'1': 'section_id', '3': 1, '4': 1, '5': 9, '10': 'sectionId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
  ],
};

/// Descriptor for `RenameThreadSectionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List renameThreadSectionRequestDescriptor =
    $convert.base64Decode(
        'ChpSZW5hbWVUaHJlYWRTZWN0aW9uUmVxdWVzdBIdCgpzZWN0aW9uX2lkGAEgASgJUglzZWN0aW'
        '9uSWQSEgoEbmFtZRgCIAEoCVIEbmFtZQ==');

@$core.Deprecated('Use deleteThreadSectionRequestDescriptor instead')
const DeleteThreadSectionRequest$json = {
  '1': 'DeleteThreadSectionRequest',
  '2': [
    {'1': 'section_id', '3': 1, '4': 1, '5': 9, '10': 'sectionId'},
  ],
};

/// Descriptor for `DeleteThreadSectionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteThreadSectionRequestDescriptor =
    $convert.base64Decode(
        'ChpEZWxldGVUaHJlYWRTZWN0aW9uUmVxdWVzdBIdCgpzZWN0aW9uX2lkGAEgASgJUglzZWN0aW'
        '9uSWQ=');

@$core.Deprecated('Use moveThreadToSectionRequestDescriptor instead')
const MoveThreadToSectionRequest$json = {
  '1': 'MoveThreadToSectionRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'section_id', '3': 2, '4': 1, '5': 9, '10': 'sectionId'},
  ],
};

/// Descriptor for `MoveThreadToSectionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List moveThreadToSectionRequestDescriptor =
    $convert.base64Decode(
        'ChpNb3ZlVGhyZWFkVG9TZWN0aW9uUmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZE'
        'lkEh0KCnNlY3Rpb25faWQYAiABKAlSCXNlY3Rpb25JZA==');

@$core.Deprecated('Use listCodexThreadsRequestDescriptor instead')
const ListCodexThreadsRequest$json = {
  '1': 'ListCodexThreadsRequest',
};

/// Descriptor for `ListCodexThreadsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCodexThreadsRequestDescriptor =
    $convert.base64Decode('ChdMaXN0Q29kZXhUaHJlYWRzUmVxdWVzdA==');

@$core.Deprecated('Use codexThreadInfoDescriptor instead')
const CodexThreadInfo$json = {
  '1': 'CodexThreadInfo',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '10': 'title'},
    {'1': 'cwd', '3': 3, '4': 1, '5': 9, '10': 'cwd'},
    {
      '1': 'updated_at_unix_ms',
      '3': 4,
      '4': 1,
      '5': 3,
      '10': 'updatedAtUnixMs'
    },
  ],
};

/// Descriptor for `CodexThreadInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexThreadInfoDescriptor = $convert.base64Decode(
    'Cg9Db2RleFRocmVhZEluZm8SDgoCaWQYASABKAlSAmlkEhQKBXRpdGxlGAIgASgJUgV0aXRsZR'
    'IQCgNjd2QYAyABKAlSA2N3ZBIrChJ1cGRhdGVkX2F0X3VuaXhfbXMYBCABKANSD3VwZGF0ZWRB'
    'dFVuaXhNcw==');

@$core.Deprecated('Use listCodexThreadsResponseDescriptor instead')
const ListCodexThreadsResponse$json = {
  '1': 'ListCodexThreadsResponse',
  '2': [
    {
      '1': 'threads',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.CodexThreadInfo',
      '10': 'threads'
    },
  ],
};

/// Descriptor for `ListCodexThreadsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCodexThreadsResponseDescriptor =
    $convert.base64Decode(
        'ChhMaXN0Q29kZXhUaHJlYWRzUmVzcG9uc2USOQoHdGhyZWFkcxgBIAMoCzIfLnBvY2tldGFnZW'
        '50LnYxLkNvZGV4VGhyZWFkSW5mb1IHdGhyZWFkcw==');

@$core.Deprecated('Use importCodexThreadRequestDescriptor instead')
const ImportCodexThreadRequest$json = {
  '1': 'ImportCodexThreadRequest',
  '2': [
    {'1': 'codex_thread_id', '3': 1, '4': 1, '5': 9, '10': 'codexThreadId'},
  ],
};

/// Descriptor for `ImportCodexThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importCodexThreadRequestDescriptor =
    $convert.base64Decode(
        'ChhJbXBvcnRDb2RleFRocmVhZFJlcXVlc3QSJgoPY29kZXhfdGhyZWFkX2lkGAEgASgJUg1jb2'
        'RleFRocmVhZElk');

@$core.Deprecated('Use importCodexThreadResponseDescriptor instead')
const ImportCodexThreadResponse$json = {
  '1': 'ImportCodexThreadResponse',
  '2': [
    {'1': 'thread_id', '3': 2, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `ImportCodexThreadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importCodexThreadResponseDescriptor =
    $convert.base64Decode(
        'ChlJbXBvcnRDb2RleFRocmVhZFJlc3BvbnNlEhsKCXRocmVhZF9pZBgCIAEoCVIIdGhyZWFkSW'
        'Q=');

@$core.Deprecated('Use listAgentThreadsRequestDescriptor instead')
const ListAgentThreadsRequest$json = {
  '1': 'ListAgentThreadsRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
  ],
};

/// Descriptor for `ListAgentThreadsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAgentThreadsRequestDescriptor =
    $convert.base64Decode(
        'ChdMaXN0QWdlbnRUaHJlYWRzUmVxdWVzdBIZCghhZ2VudF9pZBgBIAEoCVIHYWdlbnRJZA==');

@$core.Deprecated('Use agentThreadInfoDescriptor instead')
const AgentThreadInfo$json = {
  '1': 'AgentThreadInfo',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'session_id', '3': 2, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'session_file', '3': 3, '4': 1, '5': 9, '10': 'sessionFile'},
    {'1': 'cwd', '3': 4, '4': 1, '5': 9, '10': 'cwd'},
    {'1': 'title', '3': 5, '4': 1, '5': 9, '10': 'title'},
    {
      '1': 'updated_at_unix_ms',
      '3': 6,
      '4': 1,
      '5': 3,
      '10': 'updatedAtUnixMs'
    },
  ],
};

/// Descriptor for `AgentThreadInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentThreadInfoDescriptor = $convert.base64Decode(
    'Cg9BZ2VudFRocmVhZEluZm8SGQoIYWdlbnRfaWQYASABKAlSB2FnZW50SWQSHQoKc2Vzc2lvbl'
    '9pZBgCIAEoCVIJc2Vzc2lvbklkEiEKDHNlc3Npb25fZmlsZRgDIAEoCVILc2Vzc2lvbkZpbGUS'
    'EAoDY3dkGAQgASgJUgNjd2QSFAoFdGl0bGUYBSABKAlSBXRpdGxlEisKEnVwZGF0ZWRfYXRfdW'
    '5peF9tcxgGIAEoA1IPdXBkYXRlZEF0VW5peE1z');

@$core.Deprecated('Use listAgentThreadsResponseDescriptor instead')
const ListAgentThreadsResponse$json = {
  '1': 'ListAgentThreadsResponse',
  '2': [
    {
      '1': 'threads',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.AgentThreadInfo',
      '10': 'threads'
    },
  ],
};

/// Descriptor for `ListAgentThreadsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAgentThreadsResponseDescriptor =
    $convert.base64Decode(
        'ChhMaXN0QWdlbnRUaHJlYWRzUmVzcG9uc2USOQoHdGhyZWFkcxgBIAMoCzIfLnBvY2tldGFnZW'
        '50LnYxLkFnZW50VGhyZWFkSW5mb1IHdGhyZWFkcw==');

@$core.Deprecated('Use importAgentThreadRequestDescriptor instead')
const ImportAgentThreadRequest$json = {
  '1': 'ImportAgentThreadRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'session_id', '3': 2, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'session_file', '3': 3, '4': 1, '5': 9, '10': 'sessionFile'},
    {'1': 'cwd', '3': 4, '4': 1, '5': 9, '10': 'cwd'},
    {'1': 'title', '3': 5, '4': 1, '5': 9, '10': 'title'},
  ],
};

/// Descriptor for `ImportAgentThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importAgentThreadRequestDescriptor = $convert.base64Decode(
    'ChhJbXBvcnRBZ2VudFRocmVhZFJlcXVlc3QSGQoIYWdlbnRfaWQYASABKAlSB2FnZW50SWQSHQ'
    'oKc2Vzc2lvbl9pZBgCIAEoCVIJc2Vzc2lvbklkEiEKDHNlc3Npb25fZmlsZRgDIAEoCVILc2Vz'
    'c2lvbkZpbGUSEAoDY3dkGAQgASgJUgNjd2QSFAoFdGl0bGUYBSABKAlSBXRpdGxl');

@$core.Deprecated('Use importAgentThreadResponseDescriptor instead')
const ImportAgentThreadResponse$json = {
  '1': 'ImportAgentThreadResponse',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `ImportAgentThreadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importAgentThreadResponseDescriptor =
    $convert.base64Decode(
        'ChlJbXBvcnRBZ2VudFRocmVhZFJlc3BvbnNlEhsKCXRocmVhZF9pZBgBIAEoCVIIdGhyZWFkSW'
        'Q=');

@$core.Deprecated('Use getThreadHistoryRequestDescriptor instead')
const GetThreadHistoryRequest$json = {
  '1': 'GetThreadHistoryRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'limit', '3': 2, '4': 1, '5': 5, '10': 'limit'},
    {'1': 'rounds', '3': 3, '4': 1, '5': 5, '10': 'rounds'},
  ],
};

/// Descriptor for `GetThreadHistoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getThreadHistoryRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRUaHJlYWRIaXN0b3J5UmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZElkEh'
        'QKBWxpbWl0GAIgASgFUgVsaW1pdBIWCgZyb3VuZHMYAyABKAVSBnJvdW5kcw==');

@$core.Deprecated('Use getThreadHistoryResponseDescriptor instead')
const GetThreadHistoryResponse$json = {
  '1': 'GetThreadHistoryResponse',
  '2': [
    {
      '1': 'messages',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ThreadMessage',
      '10': 'messages'
    },
    {
      '1': 'goal_detail',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CodexGoalDetail',
      '10': 'goalDetail'
    },
  ],
};

/// Descriptor for `GetThreadHistoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getThreadHistoryResponseDescriptor = $convert.base64Decode(
    'ChhHZXRUaHJlYWRIaXN0b3J5UmVzcG9uc2USOQoIbWVzc2FnZXMYASADKAsyHS5wb2NrZXRhZ2'
    'VudC52MS5UaHJlYWRNZXNzYWdlUghtZXNzYWdlcxJACgtnb2FsX2RldGFpbBgCIAEoCzIfLnBv'
    'Y2tldGFnZW50LnYxLkNvZGV4R29hbERldGFpbFIKZ29hbERldGFpbA==');

@$core.Deprecated('Use threadMessageDescriptor instead')
const ThreadMessage$json = {
  '1': 'ThreadMessage',
  '2': [
    {'1': 'role', '3': 1, '4': 1, '5': 9, '10': 'role'},
    {
      '1': 'blocks',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ContentBlock',
      '10': 'blocks'
    },
    {
      '1': 'created_at_unix_ms',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'createdAtUnixMs'
    },
    {'1': 'message_id', '3': 4, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'turn_id', '3': 5, '4': 1, '5': 9, '10': 'turnId'},
  ],
};

/// Descriptor for `ThreadMessage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List threadMessageDescriptor = $convert.base64Decode(
    'Cg1UaHJlYWRNZXNzYWdlEhIKBHJvbGUYASABKAlSBHJvbGUSNAoGYmxvY2tzGAIgAygLMhwucG'
    '9ja2V0YWdlbnQudjEuQ29udGVudEJsb2NrUgZibG9ja3MSKwoSY3JlYXRlZF9hdF91bml4X21z'
    'GAMgASgDUg9jcmVhdGVkQXRVbml4TXMSHQoKbWVzc2FnZV9pZBgEIAEoCVIJbWVzc2FnZUlkEh'
    'cKB3R1cm5faWQYBSABKAlSBnR1cm5JZA==');

@$core.Deprecated('Use threadDescriptor instead')
const Thread$json = {
  '1': 'Thread',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '10': 'title'},
    {'1': 'workspace', '3': 3, '4': 1, '5': 9, '10': 'workspace'},
    {'1': 'goal', '3': 4, '4': 1, '5': 9, '10': 'goal'},
    {'1': 'model_id', '3': 5, '4': 1, '5': 9, '10': 'modelId'},
    {
      '1': 'status',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.pocketagent.v1.ThreadStatus',
      '10': 'status'
    },
    {
      '1': 'created_at_unix_ms',
      '3': 7,
      '4': 1,
      '5': 3,
      '10': 'createdAtUnixMs'
    },
    {
      '1': 'updated_at_unix_ms',
      '3': 8,
      '4': 1,
      '5': 3,
      '10': 'updatedAtUnixMs'
    },
    {'1': 'message_count', '3': 9, '4': 1, '5': 5, '10': 'messageCount'},
    {'1': 'codex_thread_id', '3': 10, '4': 1, '5': 9, '10': 'codexThreadId'},
    {'1': 'section_id', '3': 11, '4': 1, '5': 9, '10': 'sectionId'},
    {'1': 'archived', '3': 12, '4': 1, '5': 8, '10': 'archived'},
    {
      '1': 'guardian_denied_review_id',
      '3': 13,
      '4': 1,
      '5': 9,
      '10': 'guardianDeniedReviewId'
    },
    {
      '1': 'guardian_denied_item_id',
      '3': 14,
      '4': 1,
      '5': 9,
      '10': 'guardianDeniedItemId'
    },
    {
      '1': 'guardian_denied_reason',
      '3': 15,
      '4': 1,
      '5': 9,
      '10': 'guardianDeniedReason'
    },
    {'1': 'agent_id', '3': 16, '4': 1, '5': 9, '10': 'agentId'},
    {
      '1': 'external_session',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ExternalSessionRef',
      '10': 'externalSession'
    },
    {
      '1': 'goal_detail',
      '3': 18,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CodexGoalDetail',
      '10': 'goalDetail'
    },
    {'1': 'effort', '3': 19, '4': 1, '5': 9, '10': 'effort'},
    {'1': 'last_turn_error', '3': 20, '4': 1, '5': 9, '10': 'lastTurnError'},
    {
      '1': 'pending_approvals',
      '3': 21,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ApprovalRequest',
      '10': 'pendingApprovals'
    },
    {'1': 'parent_thread_id', '3': 22, '4': 1, '5': 9, '10': 'parentThreadId'},
    {
      '1': 'turn_started_unix_ms',
      '3': 23,
      '4': 1,
      '5': 3,
      '9': 0,
      '10': 'turnStartedUnixMs',
      '17': true
    },
    {
      '1': 'effective_selection',
      '3': 24,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.EffectiveModelSelection',
      '10': 'effectiveSelection'
    },
  ],
  '8': [
    {'1': '_turn_started_unix_ms'},
  ],
};

/// Descriptor for `Thread`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List threadDescriptor = $convert.base64Decode(
    'CgZUaHJlYWQSDgoCaWQYASABKAlSAmlkEhQKBXRpdGxlGAIgASgJUgV0aXRsZRIcCgl3b3Jrc3'
    'BhY2UYAyABKAlSCXdvcmtzcGFjZRISCgRnb2FsGAQgASgJUgRnb2FsEhkKCG1vZGVsX2lkGAUg'
    'ASgJUgdtb2RlbElkEjQKBnN0YXR1cxgGIAEoDjIcLnBvY2tldGFnZW50LnYxLlRocmVhZFN0YX'
    'R1c1IGc3RhdHVzEisKEmNyZWF0ZWRfYXRfdW5peF9tcxgHIAEoA1IPY3JlYXRlZEF0VW5peE1z'
    'EisKEnVwZGF0ZWRfYXRfdW5peF9tcxgIIAEoA1IPdXBkYXRlZEF0VW5peE1zEiMKDW1lc3NhZ2'
    'VfY291bnQYCSABKAVSDG1lc3NhZ2VDb3VudBImCg9jb2RleF90aHJlYWRfaWQYCiABKAlSDWNv'
    'ZGV4VGhyZWFkSWQSHQoKc2VjdGlvbl9pZBgLIAEoCVIJc2VjdGlvbklkEhoKCGFyY2hpdmVkGA'
    'wgASgIUghhcmNoaXZlZBI5ChlndWFyZGlhbl9kZW5pZWRfcmV2aWV3X2lkGA0gASgJUhZndWFy'
    'ZGlhbkRlbmllZFJldmlld0lkEjUKF2d1YXJkaWFuX2RlbmllZF9pdGVtX2lkGA4gASgJUhRndW'
    'FyZGlhbkRlbmllZEl0ZW1JZBI0ChZndWFyZGlhbl9kZW5pZWRfcmVhc29uGA8gASgJUhRndWFy'
    'ZGlhbkRlbmllZFJlYXNvbhIZCghhZ2VudF9pZBgQIAEoCVIHYWdlbnRJZBJNChBleHRlcm5hbF'
    '9zZXNzaW9uGBEgASgLMiIucG9ja2V0YWdlbnQudjEuRXh0ZXJuYWxTZXNzaW9uUmVmUg9leHRl'
    'cm5hbFNlc3Npb24SQAoLZ29hbF9kZXRhaWwYEiABKAsyHy5wb2NrZXRhZ2VudC52MS5Db2RleE'
    'dvYWxEZXRhaWxSCmdvYWxEZXRhaWwSFgoGZWZmb3J0GBMgASgJUgZlZmZvcnQSJgoPbGFzdF90'
    'dXJuX2Vycm9yGBQgASgJUg1sYXN0VHVybkVycm9yEkwKEXBlbmRpbmdfYXBwcm92YWxzGBUgAy'
    'gLMh8ucG9ja2V0YWdlbnQudjEuQXBwcm92YWxSZXF1ZXN0UhBwZW5kaW5nQXBwcm92YWxzEigK'
    'EHBhcmVudF90aHJlYWRfaWQYFiABKAlSDnBhcmVudFRocmVhZElkEjQKFHR1cm5fc3RhcnRlZF'
    '91bml4X21zGBcgASgDSABSEXR1cm5TdGFydGVkVW5peE1ziAEBElgKE2VmZmVjdGl2ZV9zZWxl'
    'Y3Rpb24YGCABKAsyJy5wb2NrZXRhZ2VudC52MS5FZmZlY3RpdmVNb2RlbFNlbGVjdGlvblISZW'
    'ZmZWN0aXZlU2VsZWN0aW9uQhcKFV90dXJuX3N0YXJ0ZWRfdW5peF9tcw==');

@$core.Deprecated('Use codexGoalDetailDescriptor instead')
const CodexGoalDetail$json = {
  '1': 'CodexGoalDetail',
  '2': [
    {'1': 'objective', '3': 1, '4': 1, '5': 9, '10': 'objective'},
    {'1': 'status', '3': 2, '4': 1, '5': 9, '10': 'status'},
    {'1': 'tokens_used', '3': 3, '4': 1, '5': 3, '10': 'tokensUsed'},
    {'1': 'token_budget', '3': 4, '4': 1, '5': 3, '10': 'tokenBudget'},
    {'1': 'time_used_seconds', '3': 5, '4': 1, '5': 3, '10': 'timeUsedSeconds'},
  ],
};

/// Descriptor for `CodexGoalDetail`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexGoalDetailDescriptor = $convert.base64Decode(
    'Cg9Db2RleEdvYWxEZXRhaWwSHAoJb2JqZWN0aXZlGAEgASgJUglvYmplY3RpdmUSFgoGc3RhdH'
    'VzGAIgASgJUgZzdGF0dXMSHwoLdG9rZW5zX3VzZWQYAyABKANSCnRva2Vuc1VzZWQSIQoMdG9r'
    'ZW5fYnVkZ2V0GAQgASgDUgt0b2tlbkJ1ZGdldBIqChF0aW1lX3VzZWRfc2Vjb25kcxgFIAEoA1'
    'IPdGltZVVzZWRTZWNvbmRz');

@$core.Deprecated('Use externalSessionRefDescriptor instead')
const ExternalSessionRef$json = {
  '1': 'ExternalSessionRef',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'session_file', '3': 2, '4': 1, '5': 9, '10': 'sessionFile'},
    {'1': 'cwd', '3': 3, '4': 1, '5': 9, '10': 'cwd'},
  ],
};

/// Descriptor for `ExternalSessionRef`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List externalSessionRefDescriptor = $convert.base64Decode(
    'ChJFeHRlcm5hbFNlc3Npb25SZWYSHQoKc2Vzc2lvbl9pZBgBIAEoCVIJc2Vzc2lvbklkEiEKDH'
    'Nlc3Npb25fZmlsZRgCIAEoCVILc2Vzc2lvbkZpbGUSEAoDY3dkGAMgASgJUgNjd2Q=');

@$core.Deprecated('Use createThreadRequestDescriptor instead')
const CreateThreadRequest$json = {
  '1': 'CreateThreadRequest',
  '2': [
    {'1': 'workspace', '3': 1, '4': 1, '5': 9, '10': 'workspace'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '10': 'title'},
    {'1': 'model_id', '3': 3, '4': 1, '5': 9, '10': 'modelId'},
    {'1': 'agent_id', '3': 4, '4': 1, '5': 9, '10': 'agentId'},
  ],
};

/// Descriptor for `CreateThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createThreadRequestDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVUaHJlYWRSZXF1ZXN0EhwKCXdvcmtzcGFjZRgBIAEoCVIJd29ya3NwYWNlEhQKBX'
    'RpdGxlGAIgASgJUgV0aXRsZRIZCghtb2RlbF9pZBgDIAEoCVIHbW9kZWxJZBIZCghhZ2VudF9p'
    'ZBgEIAEoCVIHYWdlbnRJZA==');

@$core.Deprecated('Use listThreadsRequestDescriptor instead')
const ListThreadsRequest$json = {
  '1': 'ListThreadsRequest',
  '2': [
    {'1': 'include_archived', '3': 1, '4': 1, '5': 8, '10': 'includeArchived'},
    {'1': 'limit', '3': 2, '4': 1, '5': 5, '10': 'limit'},
    {'1': 'search_term', '3': 3, '4': 1, '5': 9, '10': 'searchTerm'},
    {'1': 'section_id', '3': 4, '4': 1, '5': 9, '10': 'sectionId'},
    {'1': 'sort_key', '3': 5, '4': 1, '5': 9, '10': 'sortKey'},
    {'1': 'sort_ascending', '3': 6, '4': 1, '5': 8, '10': 'sortAscending'},
  ],
};

/// Descriptor for `ListThreadsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadsRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0VGhyZWFkc1JlcXVlc3QSKQoQaW5jbHVkZV9hcmNoaXZlZBgBIAEoCFIPaW5jbHVkZU'
    'FyY2hpdmVkEhQKBWxpbWl0GAIgASgFUgVsaW1pdBIfCgtzZWFyY2hfdGVybRgDIAEoCVIKc2Vh'
    'cmNoVGVybRIdCgpzZWN0aW9uX2lkGAQgASgJUglzZWN0aW9uSWQSGQoIc29ydF9rZXkYBSABKA'
    'lSB3NvcnRLZXkSJQoOc29ydF9hc2NlbmRpbmcYBiABKAhSDXNvcnRBc2NlbmRpbmc=');

@$core.Deprecated('Use listThreadsResponseDescriptor instead')
const ListThreadsResponse$json = {
  '1': 'ListThreadsResponse',
  '2': [
    {
      '1': 'threads',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.Thread',
      '10': 'threads'
    },
  ],
};

/// Descriptor for `ListThreadsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadsResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0VGhyZWFkc1Jlc3BvbnNlEjAKB3RocmVhZHMYASADKAsyFi5wb2NrZXRhZ2VudC52MS'
    '5UaHJlYWRSB3RocmVhZHM=');

@$core.Deprecated('Use getThreadRequestDescriptor instead')
const GetThreadRequest$json = {
  '1': 'GetThreadRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `GetThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getThreadRequestDescriptor = $convert.base64Decode(
    'ChBHZXRUaHJlYWRSZXF1ZXN0EhsKCXRocmVhZF9pZBgBIAEoCVIIdGhyZWFkSWQ=');

@$core.Deprecated('Use deleteThreadRequestDescriptor instead')
const DeleteThreadRequest$json = {
  '1': 'DeleteThreadRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'local_only', '3': 2, '4': 1, '5': 8, '10': 'localOnly'},
  ],
};

/// Descriptor for `DeleteThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteThreadRequestDescriptor = $convert.base64Decode(
    'ChNEZWxldGVUaHJlYWRSZXF1ZXN0EhsKCXRocmVhZF9pZBgBIAEoCVIIdGhyZWFkSWQSHQoKbG'
    '9jYWxfb25seRgCIAEoCFIJbG9jYWxPbmx5');

@$core.Deprecated('Use archiveThreadRequestDescriptor instead')
const ArchiveThreadRequest$json = {
  '1': 'ArchiveThreadRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'archived', '3': 2, '4': 1, '5': 8, '10': 'archived'},
  ],
};

/// Descriptor for `ArchiveThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List archiveThreadRequestDescriptor = $convert.base64Decode(
    'ChRBcmNoaXZlVGhyZWFkUmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZElkEhoKCG'
    'FyY2hpdmVkGAIgASgIUghhcmNoaXZlZA==');

@$core.Deprecated('Use renameThreadRequestDescriptor instead')
const RenameThreadRequest$json = {
  '1': 'RenameThreadRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '10': 'title'},
  ],
};

/// Descriptor for `RenameThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List renameThreadRequestDescriptor = $convert.base64Decode(
    'ChNSZW5hbWVUaHJlYWRSZXF1ZXN0EhsKCXRocmVhZF9pZBgBIAEoCVIIdGhyZWFkSWQSFAoFdG'
    'l0bGUYAiABKAlSBXRpdGxl');

@$core.Deprecated('Use setThreadGoalRequestDescriptor instead')
const SetThreadGoalRequest$json = {
  '1': 'SetThreadGoalRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'goal', '3': 2, '4': 1, '5': 9, '10': 'goal'},
  ],
};

/// Descriptor for `SetThreadGoalRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setThreadGoalRequestDescriptor = $convert.base64Decode(
    'ChRTZXRUaHJlYWRHb2FsUmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZElkEhIKBG'
    'dvYWwYAiABKAlSBGdvYWw=');

@$core.Deprecated('Use compactThreadRequestDescriptor instead')
const CompactThreadRequest$json = {
  '1': 'CompactThreadRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `CompactThreadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List compactThreadRequestDescriptor =
    $convert.base64Decode(
        'ChRDb21wYWN0VGhyZWFkUmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZElk');

@$core.Deprecated('Use turnClientMessageDescriptor instead')
const TurnClientMessage$json = {
  '1': 'TurnClientMessage',
  '2': [
    {
      '1': 'input',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TurnInput',
      '9': 0,
      '10': 'input'
    },
    {
      '1': 'supplement',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TurnSupplement',
      '9': 0,
      '10': 'supplement'
    },
    {
      '1': 'approval',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ApprovalDecision',
      '9': 0,
      '10': 'approval'
    },
    {
      '1': 'cancel',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CancelSignal',
      '9': 0,
      '10': 'cancel'
    },
  ],
  '8': [
    {'1': 'msg'},
  ],
};

/// Descriptor for `TurnClientMessage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnClientMessageDescriptor = $convert.base64Decode(
    'ChFUdXJuQ2xpZW50TWVzc2FnZRIxCgVpbnB1dBgBIAEoCzIZLnBvY2tldGFnZW50LnYxLlR1cm'
    '5JbnB1dEgAUgVpbnB1dBJACgpzdXBwbGVtZW50GAIgASgLMh4ucG9ja2V0YWdlbnQudjEuVHVy'
    'blN1cHBsZW1lbnRIAFIKc3VwcGxlbWVudBI+CghhcHByb3ZhbBgDIAEoCzIgLnBvY2tldGFnZW'
    '50LnYxLkFwcHJvdmFsRGVjaXNpb25IAFIIYXBwcm92YWwSNgoGY2FuY2VsGAQgASgLMhwucG9j'
    'a2V0YWdlbnQudjEuQ2FuY2VsU2lnbmFsSABSBmNhbmNlbEIFCgNtc2c=');

@$core.Deprecated('Use turnInputDescriptor instead')
const TurnInput$json = {
  '1': 'TurnInput',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {
      '1': 'blocks',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ContentBlock',
      '10': 'blocks'
    },
    {'1': 'background', '3': 3, '4': 1, '5': 8, '10': 'background'},
    {'1': 'model_id', '3': 4, '4': 1, '5': 9, '10': 'modelId'},
    {'1': 'effort', '3': 5, '4': 1, '5': 9, '10': 'effort'},
  ],
};

/// Descriptor for `TurnInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnInputDescriptor = $convert.base64Decode(
    'CglUdXJuSW5wdXQSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZBI0CgZibG9ja3MYAiADKA'
    'syHC5wb2NrZXRhZ2VudC52MS5Db250ZW50QmxvY2tSBmJsb2NrcxIeCgpiYWNrZ3JvdW5kGAMg'
    'ASgIUgpiYWNrZ3JvdW5kEhkKCG1vZGVsX2lkGAQgASgJUgdtb2RlbElkEhYKBmVmZm9ydBgFIA'
    'EoCVIGZWZmb3J0');

@$core.Deprecated('Use turnSupplementDescriptor instead')
const TurnSupplement$json = {
  '1': 'TurnSupplement',
  '2': [
    {
      '1': 'blocks',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ContentBlock',
      '10': 'blocks'
    },
  ],
};

/// Descriptor for `TurnSupplement`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnSupplementDescriptor = $convert.base64Decode(
    'Cg5UdXJuU3VwcGxlbWVudBI0CgZibG9ja3MYASADKAsyHC5wb2NrZXRhZ2VudC52MS5Db250ZW'
    '50QmxvY2tSBmJsb2Nrcw==');

@$core.Deprecated('Use approvalDecisionDescriptor instead')
const ApprovalDecision$json = {
  '1': 'ApprovalDecision',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'approved', '3': 2, '4': 1, '5': 8, '10': 'approved'},
    {'1': 'note', '3': 3, '4': 1, '5': 9, '10': 'note'},
    {'1': 'option', '3': 4, '4': 1, '5': 9, '10': 'option'},
  ],
};

/// Descriptor for `ApprovalDecision`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List approvalDecisionDescriptor = $convert.base64Decode(
    'ChBBcHByb3ZhbERlY2lzaW9uEh0KCnJlcXVlc3RfaWQYASABKAlSCXJlcXVlc3RJZBIaCghhcH'
    'Byb3ZlZBgCIAEoCFIIYXBwcm92ZWQSEgoEbm90ZRgDIAEoCVIEbm90ZRIWCgZvcHRpb24YBCAB'
    'KAlSBm9wdGlvbg==');

@$core.Deprecated('Use cancelSignalDescriptor instead')
const CancelSignal$json = {
  '1': 'CancelSignal',
  '2': [
    {'1': 'reason', '3': 1, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `CancelSignal`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelSignalDescriptor = $convert
    .base64Decode('CgxDYW5jZWxTaWduYWwSFgoGcmVhc29uGAEgASgJUgZyZWFzb24=');

@$core.Deprecated('Use turnServerMessageDescriptor instead')
const TurnServerMessage$json = {
  '1': 'TurnServerMessage',
  '2': [
    {
      '1': 'started',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TurnStarted',
      '9': 0,
      '10': 'started'
    },
    {
      '1': 'text_delta',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TextDelta',
      '9': 0,
      '10': 'textDelta'
    },
    {
      '1': 'thinking_delta',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ThinkingDelta',
      '9': 0,
      '10': 'thinkingDelta'
    },
    {
      '1': 'tool_call',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ToolCallUpdate',
      '9': 0,
      '10': 'toolCall'
    },
    {
      '1': 'approval_request',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ApprovalRequest',
      '9': 0,
      '10': 'approvalRequest'
    },
    {
      '1': 'usage',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.UsageUpdate',
      '9': 0,
      '10': 'usage'
    },
    {
      '1': 'completed',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TurnCompleted',
      '9': 0,
      '10': 'completed'
    },
    {
      '1': 'failed',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TurnFailed',
      '9': 0,
      '10': 'failed'
    },
    {
      '1': 'codex_event',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CodexEvent',
      '9': 0,
      '10': 'codexEvent'
    },
    {
      '1': 'approval_resolved',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ApprovalResolved',
      '9': 0,
      '10': 'approvalResolved'
    },
    {
      '1': 'steered',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.Steered',
      '9': 0,
      '10': 'steered'
    },
    {
      '1': 'queued',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.Queued',
      '9': 0,
      '10': 'queued'
    },
    {
      '1': 'queue_flushed',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.QueueFlushed',
      '9': 0,
      '10': 'queueFlushed'
    },
    {
      '1': 'resync_required',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ResyncRequired',
      '9': 0,
      '10': 'resyncRequired'
    },
    {'1': 'sequence', '3': 14, '4': 1, '5': 3, '10': 'sequence'},
  ],
  '8': [
    {'1': 'msg'},
  ],
};

/// Descriptor for `TurnServerMessage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnServerMessageDescriptor = $convert.base64Decode(
    'ChFUdXJuU2VydmVyTWVzc2FnZRI3CgdzdGFydGVkGAEgASgLMhsucG9ja2V0YWdlbnQudjEuVH'
    'VyblN0YXJ0ZWRIAFIHc3RhcnRlZBI6Cgp0ZXh0X2RlbHRhGAIgASgLMhkucG9ja2V0YWdlbnQu'
    'djEuVGV4dERlbHRhSABSCXRleHREZWx0YRJGCg50aGlua2luZ19kZWx0YRgDIAEoCzIdLnBvY2'
    'tldGFnZW50LnYxLlRoaW5raW5nRGVsdGFIAFINdGhpbmtpbmdEZWx0YRI9Cgl0b29sX2NhbGwY'
    'BCABKAsyHi5wb2NrZXRhZ2VudC52MS5Ub29sQ2FsbFVwZGF0ZUgAUgh0b29sQ2FsbBJMChBhcH'
    'Byb3ZhbF9yZXF1ZXN0GAUgASgLMh8ucG9ja2V0YWdlbnQudjEuQXBwcm92YWxSZXF1ZXN0SABS'
    'D2FwcHJvdmFsUmVxdWVzdBIzCgV1c2FnZRgGIAEoCzIbLnBvY2tldGFnZW50LnYxLlVzYWdlVX'
    'BkYXRlSABSBXVzYWdlEj0KCWNvbXBsZXRlZBgHIAEoCzIdLnBvY2tldGFnZW50LnYxLlR1cm5D'
    'b21wbGV0ZWRIAFIJY29tcGxldGVkEjQKBmZhaWxlZBgIIAEoCzIaLnBvY2tldGFnZW50LnYxLl'
    'R1cm5GYWlsZWRIAFIGZmFpbGVkEj0KC2NvZGV4X2V2ZW50GAkgASgLMhoucG9ja2V0YWdlbnQu'
    'djEuQ29kZXhFdmVudEgAUgpjb2RleEV2ZW50Ek8KEWFwcHJvdmFsX3Jlc29sdmVkGAogASgLMi'
    'AucG9ja2V0YWdlbnQudjEuQXBwcm92YWxSZXNvbHZlZEgAUhBhcHByb3ZhbFJlc29sdmVkEjMK'
    'B3N0ZWVyZWQYCyABKAsyFy5wb2NrZXRhZ2VudC52MS5TdGVlcmVkSABSB3N0ZWVyZWQSMAoGcX'
    'VldWVkGAwgASgLMhYucG9ja2V0YWdlbnQudjEuUXVldWVkSABSBnF1ZXVlZBJDCg1xdWV1ZV9m'
    'bHVzaGVkGA0gASgLMhwucG9ja2V0YWdlbnQudjEuUXVldWVGbHVzaGVkSABSDHF1ZXVlRmx1c2'
    'hlZBJJCg9yZXN5bmNfcmVxdWlyZWQYDyABKAsyHi5wb2NrZXRhZ2VudC52MS5SZXN5bmNSZXF1'
    'aXJlZEgAUg5yZXN5bmNSZXF1aXJlZBIaCghzZXF1ZW5jZRgOIAEoA1IIc2VxdWVuY2VCBQoDbX'
    'Nn');

@$core.Deprecated('Use getThreadEventsSinceRequestDescriptor instead')
const GetThreadEventsSinceRequest$json = {
  '1': 'GetThreadEventsSinceRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'after_sequence', '3': 2, '4': 1, '5': 3, '10': 'afterSequence'},
  ],
};

/// Descriptor for `GetThreadEventsSinceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getThreadEventsSinceRequestDescriptor =
    $convert.base64Decode(
        'ChtHZXRUaHJlYWRFdmVudHNTaW5jZVJlcXVlc3QSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYW'
        'RJZBIlCg5hZnRlcl9zZXF1ZW5jZRgCIAEoA1INYWZ0ZXJTZXF1ZW5jZQ==');

@$core.Deprecated('Use resyncRequiredDescriptor instead')
const ResyncRequired$json = {
  '1': 'ResyncRequired',
  '2': [
    {'1': 'reason', '3': 1, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `ResyncRequired`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resyncRequiredDescriptor = $convert
    .base64Decode('Cg5SZXN5bmNSZXF1aXJlZBIWCgZyZWFzb24YASABKAlSBnJlYXNvbg==');

@$core.Deprecated('Use steeredDescriptor instead')
const Steered$json = {
  '1': 'Steered',
  '2': [
    {'1': 'turn_id', '3': 1, '4': 1, '5': 9, '10': 'turnId'},
    {'1': 'codex_turn_id', '3': 2, '4': 1, '5': 9, '10': 'codexTurnId'},
  ],
};

/// Descriptor for `Steered`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List steeredDescriptor = $convert.base64Decode(
    'CgdTdGVlcmVkEhcKB3R1cm5faWQYASABKAlSBnR1cm5JZBIiCg1jb2RleF90dXJuX2lkGAIgAS'
    'gJUgtjb2RleFR1cm5JZA==');

@$core.Deprecated('Use queuedDescriptor instead')
const Queued$json = {
  '1': 'Queued',
  '2': [
    {'1': 'position', '3': 1, '4': 1, '5': 5, '10': 'position'},
    {'1': 'depth', '3': 2, '4': 1, '5': 5, '10': 'depth'},
  ],
};

/// Descriptor for `Queued`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List queuedDescriptor = $convert.base64Decode(
    'CgZRdWV1ZWQSGgoIcG9zaXRpb24YASABKAVSCHBvc2l0aW9uEhQKBWRlcHRoGAIgASgFUgVkZX'
    'B0aA==');

@$core.Deprecated('Use queueFlushedDescriptor instead')
const QueueFlushed$json = {
  '1': 'QueueFlushed',
  '2': [
    {'1': 'turn_id', '3': 1, '4': 1, '5': 9, '10': 'turnId'},
    {'1': 'remaining', '3': 2, '4': 1, '5': 5, '10': 'remaining'},
  ],
};

/// Descriptor for `QueueFlushed`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List queueFlushedDescriptor = $convert.base64Decode(
    'CgxRdWV1ZUZsdXNoZWQSFwoHdHVybl9pZBgBIAEoCVIGdHVybklkEhwKCXJlbWFpbmluZxgCIA'
    'EoBVIJcmVtYWluaW5n');

@$core.Deprecated('Use approvalResolvedDescriptor instead')
const ApprovalResolved$json = {
  '1': 'ApprovalResolved',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'note', '3': 2, '4': 1, '5': 9, '10': 'note'},
  ],
};

/// Descriptor for `ApprovalResolved`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List approvalResolvedDescriptor = $convert.base64Decode(
    'ChBBcHByb3ZhbFJlc29sdmVkEh0KCnJlcXVlc3RfaWQYASABKAlSCXJlcXVlc3RJZBISCgRub3'
    'RlGAIgASgJUgRub3Rl');

@$core.Deprecated('Use codexEventDescriptor instead')
const CodexEvent$json = {
  '1': 'CodexEvent',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.pocketagent.v1.CodexEvent.Kind',
      '10': 'kind'
    },
    {'1': 'code', '3': 2, '4': 1, '5': 9, '10': 'code'},
    {'1': 'title', '3': 3, '4': 1, '5': 9, '10': 'title'},
    {'1': 'message', '3': 4, '4': 1, '5': 9, '10': 'message'},
    {'1': 'url', '3': 5, '4': 1, '5': 9, '10': 'url'},
    {'1': 'http_status', '3': 6, '4': 1, '5': 5, '10': 'httpStatus'},
    {'1': 'will_retry', '3': 7, '4': 1, '5': 8, '10': 'willRetry'},
    {'1': 'attempt', '3': 8, '4': 1, '5': 5, '10': 'attempt'},
    {'1': 'retry_after_ms', '3': 9, '4': 1, '5': 3, '10': 'retryAfterMs'},
    {'1': 'thread_id', '3': 10, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'turn_id', '3': 11, '4': 1, '5': 9, '10': 'turnId'},
    {'1': 'risk_level', '3': 12, '4': 1, '5': 9, '10': 'riskLevel'},
    {'1': 'review_status', '3': 13, '4': 1, '5': 9, '10': 'reviewStatus'},
    {'1': 'review_id', '3': 14, '4': 1, '5': 9, '10': 'reviewId'},
    {'1': 'target_item_id', '3': 15, '4': 1, '5': 9, '10': 'targetItemId'},
    {'1': 'rationale', '3': 16, '4': 1, '5': 9, '10': 'rationale'},
    {'1': 'diff', '3': 17, '4': 1, '5': 9, '10': 'diff'},
    {'1': 'raw_json', '3': 18, '4': 1, '5': 9, '10': 'rawJson'},
    {'1': 'plan_steps', '3': 19, '4': 3, '5': 9, '10': 'planSteps'},
  ],
  '4': [CodexEvent_Kind$json],
};

@$core.Deprecated('Use codexEventDescriptor instead')
const CodexEvent_Kind$json = {
  '1': 'Kind',
  '2': [
    {'1': 'CODEX_EVENT_UNSPECIFIED', '2': 0},
    {'1': 'ERROR', '2': 1},
    {'1': 'RETRY_STARTED', '2': 2},
    {'1': 'RETRY_STOPPED', '2': 3},
    {'1': 'STATUS', '2': 4},
    {'1': 'WARNING', '2': 5},
    {'1': 'MODEL_REROUTED', '2': 6},
    {'1': 'RATE_LIMIT', '2': 7},
    {'1': 'CONTEXT_COMPACTED', '2': 8},
    {'1': 'GUARDIAN_REVIEW', '2': 9},
    {'1': 'TURN_DIFF', '2': 10},
    {'1': 'DEPRECATION', '2': 11},
    {'1': 'CONFIG_WARNING', '2': 12},
    {'1': 'SANDBOX_WARNING', '2': 13},
    {'1': 'HOOK', '2': 14},
    {'1': 'SKILLS_CHANGED', '2': 15},
    {'1': 'QUEUE', '2': 16},
    {'1': 'MODEL_SAFETY', '2': 17},
  ],
};

/// Descriptor for `CodexEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexEventDescriptor = $convert.base64Decode(
    'CgpDb2RleEV2ZW50EjMKBGtpbmQYASABKA4yHy5wb2NrZXRhZ2VudC52MS5Db2RleEV2ZW50Lk'
    'tpbmRSBGtpbmQSEgoEY29kZRgCIAEoCVIEY29kZRIUCgV0aXRsZRgDIAEoCVIFdGl0bGUSGAoH'
    'bWVzc2FnZRgEIAEoCVIHbWVzc2FnZRIQCgN1cmwYBSABKAlSA3VybBIfCgtodHRwX3N0YXR1cx'
    'gGIAEoBVIKaHR0cFN0YXR1cxIdCgp3aWxsX3JldHJ5GAcgASgIUgl3aWxsUmV0cnkSGAoHYXR0'
    'ZW1wdBgIIAEoBVIHYXR0ZW1wdBIkCg5yZXRyeV9hZnRlcl9tcxgJIAEoA1IMcmV0cnlBZnRlck'
    '1zEhsKCXRocmVhZF9pZBgKIAEoCVIIdGhyZWFkSWQSFwoHdHVybl9pZBgLIAEoCVIGdHVybklk'
    'Eh0KCnJpc2tfbGV2ZWwYDCABKAlSCXJpc2tMZXZlbBIjCg1yZXZpZXdfc3RhdHVzGA0gASgJUg'
    'xyZXZpZXdTdGF0dXMSGwoJcmV2aWV3X2lkGA4gASgJUghyZXZpZXdJZBIkCg50YXJnZXRfaXRl'
    'bV9pZBgPIAEoCVIMdGFyZ2V0SXRlbUlkEhwKCXJhdGlvbmFsZRgQIAEoCVIJcmF0aW9uYWxlEh'
    'IKBGRpZmYYESABKAlSBGRpZmYSGQoIcmF3X2pzb24YEiABKAlSB3Jhd0pzb24SHQoKcGxhbl9z'
    'dGVwcxgTIAMoCVIJcGxhblN0ZXBzIsECCgRLaW5kEhsKF0NPREVYX0VWRU5UX1VOU1BFQ0lGSU'
    'VEEAASCQoFRVJST1IQARIRCg1SRVRSWV9TVEFSVEVEEAISEQoNUkVUUllfU1RPUFBFRBADEgoK'
    'BlNUQVRVUxAEEgsKB1dBUk5JTkcQBRISCg5NT0RFTF9SRVJPVVRFRBAGEg4KClJBVEVfTElNSV'
    'QQBxIVChFDT05URVhUX0NPTVBBQ1RFRBAIEhMKD0dVQVJESUFOX1JFVklFVxAJEg0KCVRVUk5f'
    'RElGRhAKEg8KC0RFUFJFQ0FUSU9OEAsSEgoOQ09ORklHX1dBUk5JTkcQDBITCg9TQU5EQk9YX1'
    'dBUk5JTkcQDRIICgRIT09LEA4SEgoOU0tJTExTX0NIQU5HRUQQDxIJCgVRVUVVRRAQEhAKDE1P'
    'REVMX1NBRkVUWRAR');

@$core.Deprecated('Use turnStartedDescriptor instead')
const TurnStarted$json = {
  '1': 'TurnStarted',
  '2': [
    {'1': 'turn_id', '3': 1, '4': 1, '5': 9, '10': 'turnId'},
    {'1': 'model_id', '3': 2, '4': 1, '5': 9, '10': 'modelId'},
    {'1': 'effort', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'effort', '17': true},
    {
      '1': 'configuration_confirmed',
      '3': 4,
      '4': 1,
      '5': 8,
      '10': 'configurationConfirmed'
    },
    {'1': 'source', '3': 5, '4': 1, '5': 9, '10': 'source'},
  ],
  '8': [
    {'1': '_effort'},
  ],
};

/// Descriptor for `TurnStarted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnStartedDescriptor = $convert.base64Decode(
    'CgtUdXJuU3RhcnRlZBIXCgd0dXJuX2lkGAEgASgJUgZ0dXJuSWQSGQoIbW9kZWxfaWQYAiABKA'
    'lSB21vZGVsSWQSGwoGZWZmb3J0GAMgASgJSABSBmVmZm9ydIgBARI3Chdjb25maWd1cmF0aW9u'
    'X2NvbmZpcm1lZBgEIAEoCFIWY29uZmlndXJhdGlvbkNvbmZpcm1lZBIWCgZzb3VyY2UYBSABKA'
    'lSBnNvdXJjZUIJCgdfZWZmb3J0');

@$core.Deprecated('Use textDeltaDescriptor instead')
const TextDelta$json = {
  '1': 'TextDelta',
  '2': [
    {'1': 'delta', '3': 1, '4': 1, '5': 9, '10': 'delta'},
  ],
};

/// Descriptor for `TextDelta`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List textDeltaDescriptor =
    $convert.base64Decode('CglUZXh0RGVsdGESFAoFZGVsdGEYASABKAlSBWRlbHRh');

@$core.Deprecated('Use thinkingDeltaDescriptor instead')
const ThinkingDelta$json = {
  '1': 'ThinkingDelta',
  '2': [
    {'1': 'delta', '3': 1, '4': 1, '5': 9, '10': 'delta'},
    {'1': 'snapshot', '3': 2, '4': 1, '5': 9, '10': 'snapshot'},
  ],
};

/// Descriptor for `ThinkingDelta`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List thinkingDeltaDescriptor = $convert.base64Decode(
    'Cg1UaGlua2luZ0RlbHRhEhQKBWRlbHRhGAEgASgJUgVkZWx0YRIaCghzbmFwc2hvdBgCIAEoCV'
    'IIc25hcHNob3Q=');

@$core.Deprecated('Use toolCallUpdateDescriptor instead')
const ToolCallUpdate$json = {
  '1': 'ToolCallUpdate',
  '2': [
    {
      '1': 'phase',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.pocketagent.v1.ToolCallUpdate.Phase',
      '10': 'phase'
    },
    {
      '1': 'call',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ToolCallBlock',
      '10': 'call'
    },
    {
      '1': 'result',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ToolResultBlock',
      '10': 'result'
    },
  ],
  '4': [ToolCallUpdate_Phase$json],
};

@$core.Deprecated('Use toolCallUpdateDescriptor instead')
const ToolCallUpdate_Phase$json = {
  '1': 'Phase',
  '2': [
    {'1': 'UNSPECIFIED', '2': 0},
    {'1': 'STARTED', '2': 1},
    {'1': 'FINISHED', '2': 2},
    {'1': 'UPDATE', '2': 3},
  ],
};

/// Descriptor for `ToolCallUpdate`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List toolCallUpdateDescriptor = $convert.base64Decode(
    'Cg5Ub29sQ2FsbFVwZGF0ZRI6CgVwaGFzZRgBIAEoDjIkLnBvY2tldGFnZW50LnYxLlRvb2xDYW'
    'xsVXBkYXRlLlBoYXNlUgVwaGFzZRIxCgRjYWxsGAIgASgLMh0ucG9ja2V0YWdlbnQudjEuVG9v'
    'bENhbGxCbG9ja1IEY2FsbBI3CgZyZXN1bHQYAyABKAsyHy5wb2NrZXRhZ2VudC52MS5Ub29sUm'
    'VzdWx0QmxvY2tSBnJlc3VsdCI/CgVQaGFzZRIPCgtVTlNQRUNJRklFRBAAEgsKB1NUQVJURUQQ'
    'ARIMCghGSU5JU0hFRBACEgoKBlVQREFURRAD');

@$core.Deprecated('Use approvalOptionDescriptor instead')
const ApprovalOption$json = {
  '1': 'ApprovalOption',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
  ],
};

/// Descriptor for `ApprovalOption`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List approvalOptionDescriptor = $convert.base64Decode(
    'Cg5BcHByb3ZhbE9wdGlvbhIOCgJpZBgBIAEoCVICaWQSFAoFbGFiZWwYAiABKAlSBWxhYmVsEi'
    'AKC2Rlc2NyaXB0aW9uGAMgASgJUgtkZXNjcmlwdGlvbg==');

@$core.Deprecated('Use approvalRequestDescriptor instead')
const ApprovalRequest$json = {
  '1': 'ApprovalRequest',
  '2': [
    {'1': 'request_id', '3': 1, '4': 1, '5': 9, '10': 'requestId'},
    {
      '1': 'action',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ToolCallBlock',
      '10': 'action'
    },
    {'1': 'reason', '3': 3, '4': 1, '5': 9, '10': 'reason'},
    {
      '1': 'options',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ApprovalOption',
      '10': 'options'
    },
    {'1': 'details', '3': 5, '4': 1, '5': 9, '10': 'details'},
  ],
};

/// Descriptor for `ApprovalRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List approvalRequestDescriptor = $convert.base64Decode(
    'Cg9BcHByb3ZhbFJlcXVlc3QSHQoKcmVxdWVzdF9pZBgBIAEoCVIJcmVxdWVzdElkEjUKBmFjdG'
    'lvbhgCIAEoCzIdLnBvY2tldGFnZW50LnYxLlRvb2xDYWxsQmxvY2tSBmFjdGlvbhIWCgZyZWFz'
    'b24YAyABKAlSBnJlYXNvbhI4CgdvcHRpb25zGAQgAygLMh4ucG9ja2V0YWdlbnQudjEuQXBwcm'
    '92YWxPcHRpb25SB29wdGlvbnMSGAoHZGV0YWlscxgFIAEoCVIHZGV0YWlscw==');

@$core.Deprecated('Use usageUpdateDescriptor instead')
const UsageUpdate$json = {
  '1': 'UsageUpdate',
  '2': [
    {
      '1': 'usage',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.Usage',
      '10': 'usage'
    },
    {
      '1': 'total_usage',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.Usage',
      '10': 'totalUsage'
    },
    {
      '1': 'model_context_window',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'modelContextWindow'
    },
  ],
};

/// Descriptor for `UsageUpdate`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List usageUpdateDescriptor = $convert.base64Decode(
    'CgtVc2FnZVVwZGF0ZRIrCgV1c2FnZRgBIAEoCzIVLnBvY2tldGFnZW50LnYxLlVzYWdlUgV1c2'
    'FnZRI2Cgt0b3RhbF91c2FnZRgCIAEoCzIVLnBvY2tldGFnZW50LnYxLlVzYWdlUgp0b3RhbFVz'
    'YWdlEjAKFG1vZGVsX2NvbnRleHRfd2luZG93GAMgASgDUhJtb2RlbENvbnRleHRXaW5kb3c=');

@$core.Deprecated('Use turnCompletedDescriptor instead')
const TurnCompleted$json = {
  '1': 'TurnCompleted',
  '2': [
    {'1': 'turn_id', '3': 1, '4': 1, '5': 9, '10': 'turnId'},
    {
      '1': 'final_blocks',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ContentBlock',
      '10': 'finalBlocks'
    },
    {
      '1': 'usage',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.Usage',
      '10': 'usage'
    },
    {'1': 'elapsed_ms', '3': 4, '4': 1, '5': 3, '10': 'elapsedMs'},
    {
      '1': 'output_state',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.pocketagent.v1.OutputState',
      '10': 'outputState'
    },
    {'1': 'output_source', '3': 6, '4': 1, '5': 9, '10': 'outputSource'},
    {'1': 'diagnostic_id', '3': 7, '4': 1, '5': 9, '10': 'diagnosticId'},
  ],
};

/// Descriptor for `TurnCompleted`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnCompletedDescriptor = $convert.base64Decode(
    'Cg1UdXJuQ29tcGxldGVkEhcKB3R1cm5faWQYASABKAlSBnR1cm5JZBI/CgxmaW5hbF9ibG9ja3'
    'MYAiADKAsyHC5wb2NrZXRhZ2VudC52MS5Db250ZW50QmxvY2tSC2ZpbmFsQmxvY2tzEisKBXVz'
    'YWdlGAMgASgLMhUucG9ja2V0YWdlbnQudjEuVXNhZ2VSBXVzYWdlEh0KCmVsYXBzZWRfbXMYBC'
    'ABKANSCWVsYXBzZWRNcxI+CgxvdXRwdXRfc3RhdGUYBSABKA4yGy5wb2NrZXRhZ2VudC52MS5P'
    'dXRwdXRTdGF0ZVILb3V0cHV0U3RhdGUSIwoNb3V0cHV0X3NvdXJjZRgGIAEoCVIMb3V0cHV0U2'
    '91cmNlEiMKDWRpYWdub3N0aWNfaWQYByABKAlSDGRpYWdub3N0aWNJZA==');

@$core.Deprecated('Use turnFailedDescriptor instead')
const TurnFailed$json = {
  '1': 'TurnFailed',
  '2': [
    {'1': 'turn_id', '3': 1, '4': 1, '5': 9, '10': 'turnId'},
    {
      '1': 'error',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.Error',
      '10': 'error'
    },
    {'1': 'partial_text', '3': 3, '4': 1, '5': 9, '10': 'partialText'},
  ],
};

/// Descriptor for `TurnFailed`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnFailedDescriptor = $convert.base64Decode(
    'CgpUdXJuRmFpbGVkEhcKB3R1cm5faWQYASABKAlSBnR1cm5JZBIrCgVlcnJvchgCIAEoCzIVLn'
    'BvY2tldGFnZW50LnYxLkVycm9yUgVlcnJvchIhCgxwYXJ0aWFsX3RleHQYAyABKAlSC3BhcnRp'
    'YWxUZXh0');

@$core.Deprecated('Use cancelTurnRequestDescriptor instead')
const CancelTurnRequest$json = {
  '1': 'CancelTurnRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `CancelTurnRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelTurnRequestDescriptor = $convert.base64Decode(
    'ChFDYW5jZWxUdXJuUmVxdWVzdBIbCgl0aHJlYWRfaWQYASABKAlSCHRocmVhZElk');

@$core.Deprecated('Use clearQueuedInputsRequestDescriptor instead')
const ClearQueuedInputsRequest$json = {
  '1': 'ClearQueuedInputsRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `ClearQueuedInputsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearQueuedInputsRequestDescriptor =
    $convert.base64Decode(
        'ChhDbGVhclF1ZXVlZElucHV0c1JlcXVlc3QSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZA'
        '==');

@$core.Deprecated('Use clearQueuedInputsResponseDescriptor instead')
const ClearQueuedInputsResponse$json = {
  '1': 'ClearQueuedInputsResponse',
  '2': [
    {'1': 'steered', '3': 1, '4': 1, '5': 5, '10': 'steered'},
    {'1': 'follow_up', '3': 2, '4': 1, '5': 5, '10': 'followUp'},
  ],
};

/// Descriptor for `ClearQueuedInputsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearQueuedInputsResponseDescriptor =
    $convert.base64Decode(
        'ChlDbGVhclF1ZXVlZElucHV0c1Jlc3BvbnNlEhgKB3N0ZWVyZWQYASABKAVSB3N0ZWVyZWQSGw'
        'oJZm9sbG93X3VwGAIgASgFUghmb2xsb3dVcA==');

@$core.Deprecated('Use submitApprovalRequestDescriptor instead')
const SubmitApprovalRequest$json = {
  '1': 'SubmitApprovalRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {
      '1': 'decision',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ApprovalDecision',
      '10': 'decision'
    },
  ],
};

/// Descriptor for `SubmitApprovalRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitApprovalRequestDescriptor = $convert.base64Decode(
    'ChVTdWJtaXRBcHByb3ZhbFJlcXVlc3QSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZBI8Cg'
    'hkZWNpc2lvbhgCIAEoCzIgLnBvY2tldGFnZW50LnYxLkFwcHJvdmFsRGVjaXNpb25SCGRlY2lz'
    'aW9u');

@$core.Deprecated('Use searchThreadMessagesRequestDescriptor instead')
const SearchThreadMessagesRequest$json = {
  '1': 'SearchThreadMessagesRequest',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'query', '3': 2, '4': 1, '5': 9, '10': 'query'},
    {'1': 'limit', '3': 3, '4': 1, '5': 5, '10': 'limit'},
  ],
};

/// Descriptor for `SearchThreadMessagesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchThreadMessagesRequestDescriptor =
    $convert.base64Decode(
        'ChtTZWFyY2hUaHJlYWRNZXNzYWdlc1JlcXVlc3QSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYW'
        'RJZBIUCgVxdWVyeRgCIAEoCVIFcXVlcnkSFAoFbGltaXQYAyABKAVSBWxpbWl0');

@$core.Deprecated('Use matchRangeDescriptor instead')
const MatchRange$json = {
  '1': 'MatchRange',
  '2': [
    {'1': 'start', '3': 1, '4': 1, '5': 5, '10': 'start'},
    {'1': 'end', '3': 2, '4': 1, '5': 5, '10': 'end'},
  ],
};

/// Descriptor for `MatchRange`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchRangeDescriptor = $convert.base64Decode(
    'CgpNYXRjaFJhbmdlEhQKBXN0YXJ0GAEgASgFUgVzdGFydBIQCgNlbmQYAiABKAVSA2VuZA==');

@$core.Deprecated('Use threadMessageHitDescriptor instead')
const ThreadMessageHit$json = {
  '1': 'ThreadMessageHit',
  '2': [
    {'1': 'sequence', '3': 1, '4': 1, '5': 5, '10': 'sequence'},
    {'1': 'role', '3': 2, '4': 1, '5': 9, '10': 'role'},
    {'1': 'snippet', '3': 3, '4': 1, '5': 9, '10': 'snippet'},
    {
      '1': 'created_at_unix_ms',
      '3': 4,
      '4': 1,
      '5': 3,
      '10': 'createdAtUnixMs'
    },
    {'1': 'message_id', '3': 6, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'turn_id', '3': 7, '4': 1, '5': 9, '10': 'turnId'},
    {
      '1': 'ranges',
      '3': 8,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.MatchRange',
      '10': 'ranges'
    },
    {
      '1': 'from_loaded_history',
      '3': 9,
      '4': 1,
      '5': 8,
      '10': 'fromLoadedHistory'
    },
  ],
};

/// Descriptor for `ThreadMessageHit`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List threadMessageHitDescriptor = $convert.base64Decode(
    'ChBUaHJlYWRNZXNzYWdlSGl0EhoKCHNlcXVlbmNlGAEgASgFUghzZXF1ZW5jZRISCgRyb2xlGA'
    'IgASgJUgRyb2xlEhgKB3NuaXBwZXQYAyABKAlSB3NuaXBwZXQSKwoSY3JlYXRlZF9hdF91bml4'
    'X21zGAQgASgDUg9jcmVhdGVkQXRVbml4TXMSHQoKbWVzc2FnZV9pZBgGIAEoCVIJbWVzc2FnZU'
    'lkEhcKB3R1cm5faWQYByABKAlSBnR1cm5JZBIyCgZyYW5nZXMYCCADKAsyGi5wb2NrZXRhZ2Vu'
    'dC52MS5NYXRjaFJhbmdlUgZyYW5nZXMSLgoTZnJvbV9sb2FkZWRfaGlzdG9yeRgJIAEoCFIRZn'
    'JvbUxvYWRlZEhpc3Rvcnk=');

@$core.Deprecated('Use searchThreadMessagesResponseDescriptor instead')
const SearchThreadMessagesResponse$json = {
  '1': 'SearchThreadMessagesResponse',
  '2': [
    {
      '1': 'hits',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ThreadMessageHit',
      '10': 'hits'
    },
    {'1': 'has_more', '3': 2, '4': 1, '5': 8, '10': 'hasMore'},
    {
      '1': 'searched_messages',
      '3': 3,
      '4': 1,
      '5': 5,
      '10': 'searchedMessages'
    },
    {'1': 'scope', '3': 4, '4': 1, '5': 9, '10': 'scope'},
  ],
};

/// Descriptor for `SearchThreadMessagesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchThreadMessagesResponseDescriptor = $convert.base64Decode(
    'ChxTZWFyY2hUaHJlYWRNZXNzYWdlc1Jlc3BvbnNlEjQKBGhpdHMYASADKAsyIC5wb2NrZXRhZ2'
    'VudC52MS5UaHJlYWRNZXNzYWdlSGl0UgRoaXRzEhkKCGhhc19tb3JlGAIgASgIUgdoYXNNb3Jl'
    'EisKEXNlYXJjaGVkX21lc3NhZ2VzGAMgASgFUhBzZWFyY2hlZE1lc3NhZ2VzEhQKBXNjb3BlGA'
    'QgASgJUgVzY29wZQ==');

@$core.Deprecated('Use collabSessionSnapshotDescriptor instead')
const CollabSessionSnapshot$json = {
  '1': 'CollabSessionSnapshot',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'instruction', '3': 2, '4': 1, '5': 9, '10': 'instruction'},
    {'1': 'status', '3': 3, '4': 1, '5': 9, '10': 'status'},
    {'1': 'workspace', '3': 4, '4': 1, '5': 9, '10': 'workspace'},
    {
      '1': 'created_at_unix_ms',
      '3': 5,
      '4': 1,
      '5': 3,
      '10': 'createdAtUnixMs'
    },
    {
      '1': 'updated_at_unix_ms',
      '3': 6,
      '4': 1,
      '5': 3,
      '10': 'updatedAtUnixMs'
    },
    {
      '1': 'members',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.CollabMemberSnapshot',
      '10': 'members'
    },
    {'1': 'error', '3': 8, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `CollabSessionSnapshot`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List collabSessionSnapshotDescriptor = $convert.base64Decode(
    'ChVDb2xsYWJTZXNzaW9uU25hcHNob3QSHQoKc2Vzc2lvbl9pZBgBIAEoCVIJc2Vzc2lvbklkEi'
    'AKC2luc3RydWN0aW9uGAIgASgJUgtpbnN0cnVjdGlvbhIWCgZzdGF0dXMYAyABKAlSBnN0YXR1'
    'cxIcCgl3b3Jrc3BhY2UYBCABKAlSCXdvcmtzcGFjZRIrChJjcmVhdGVkX2F0X3VuaXhfbXMYBS'
    'ABKANSD2NyZWF0ZWRBdFVuaXhNcxIrChJ1cGRhdGVkX2F0X3VuaXhfbXMYBiABKANSD3VwZGF0'
    'ZWRBdFVuaXhNcxI+CgdtZW1iZXJzGAcgAygLMiQucG9ja2V0YWdlbnQudjEuQ29sbGFiTWVtYm'
    'VyU25hcHNob3RSB21lbWJlcnMSFAoFZXJyb3IYCCABKAlSBWVycm9y');

@$core.Deprecated('Use collabMemberSnapshotDescriptor instead')
const CollabMemberSnapshot$json = {
  '1': 'CollabMemberSnapshot',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'role_id', '3': 2, '4': 1, '5': 9, '10': 'roleId'},
    {'1': 'thread_id', '3': 3, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'status', '3': 4, '4': 1, '5': 9, '10': 'status'},
    {'1': 'model_id', '3': 5, '4': 1, '5': 9, '10': 'modelId'},
    {'1': 'effort', '3': 6, '4': 1, '5': 9, '10': 'effort'},
    {'1': 'progress', '3': 7, '4': 1, '5': 9, '10': 'progress'},
    {'1': 'summary', '3': 8, '4': 1, '5': 9, '10': 'summary'},
    {'1': 'error', '3': 9, '4': 1, '5': 9, '10': 'error'},
    {
      '1': 'pending_approvals',
      '3': 10,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ApprovalRequest',
      '10': 'pendingApprovals'
    },
    {'1': 'workspace', '3': 11, '4': 1, '5': 9, '10': 'workspace'},
  ],
};

/// Descriptor for `CollabMemberSnapshot`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List collabMemberSnapshotDescriptor = $convert.base64Decode(
    'ChRDb2xsYWJNZW1iZXJTbmFwc2hvdBIZCghhZ2VudF9pZBgBIAEoCVIHYWdlbnRJZBIXCgdyb2'
    'xlX2lkGAIgASgJUgZyb2xlSWQSGwoJdGhyZWFkX2lkGAMgASgJUgh0aHJlYWRJZBIWCgZzdGF0'
    'dXMYBCABKAlSBnN0YXR1cxIZCghtb2RlbF9pZBgFIAEoCVIHbW9kZWxJZBIWCgZlZmZvcnQYBi'
    'ABKAlSBmVmZm9ydBIaCghwcm9ncmVzcxgHIAEoCVIIcHJvZ3Jlc3MSGAoHc3VtbWFyeRgIIAEo'
    'CVIHc3VtbWFyeRIUCgVlcnJvchgJIAEoCVIFZXJyb3ISTAoRcGVuZGluZ19hcHByb3ZhbHMYCi'
    'ADKAsyHy5wb2NrZXRhZ2VudC52MS5BcHByb3ZhbFJlcXVlc3RSEHBlbmRpbmdBcHByb3ZhbHMS'
    'HAoJd29ya3NwYWNlGAsgASgJUgl3b3Jrc3BhY2U=');

@$core.Deprecated('Use watchCollabSessionRequestDescriptor instead')
const WatchCollabSessionRequest$json = {
  '1': 'WatchCollabSessionRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'after_sequence', '3': 2, '4': 1, '5': 3, '10': 'afterSequence'},
  ],
};

/// Descriptor for `WatchCollabSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchCollabSessionRequestDescriptor =
    $convert.base64Decode(
        'ChlXYXRjaENvbGxhYlNlc3Npb25SZXF1ZXN0Eh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb2'
        '5JZBIlCg5hZnRlcl9zZXF1ZW5jZRgCIAEoA1INYWZ0ZXJTZXF1ZW5jZQ==');

@$core.Deprecated('Use collabSessionEventDescriptor instead')
const CollabSessionEvent$json = {
  '1': 'CollabSessionEvent',
  '2': [
    {'1': 'sequence', '3': 1, '4': 1, '5': 3, '10': 'sequence'},
    {'1': 'session_id', '3': 2, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'member_thread_id', '3': 3, '4': 1, '5': 9, '10': 'memberThreadId'},
    {'1': 'kind', '3': 4, '4': 1, '5': 9, '10': 'kind'},
    {
      '1': 'snapshot',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CollabSessionSnapshot',
      '10': 'snapshot'
    },
    {
      '1': 'member',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CollabMemberSnapshot',
      '10': 'member'
    },
  ],
};

/// Descriptor for `CollabSessionEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List collabSessionEventDescriptor = $convert.base64Decode(
    'ChJDb2xsYWJTZXNzaW9uRXZlbnQSGgoIc2VxdWVuY2UYASABKANSCHNlcXVlbmNlEh0KCnNlc3'
    'Npb25faWQYAiABKAlSCXNlc3Npb25JZBIoChBtZW1iZXJfdGhyZWFkX2lkGAMgASgJUg5tZW1i'
    'ZXJUaHJlYWRJZBISCgRraW5kGAQgASgJUgRraW5kEkEKCHNuYXBzaG90GAUgASgLMiUucG9ja2'
    'V0YWdlbnQudjEuQ29sbGFiU2Vzc2lvblNuYXBzaG90UghzbmFwc2hvdBI8CgZtZW1iZXIYBiAB'
    'KAsyJC5wb2NrZXRhZ2VudC52MS5Db2xsYWJNZW1iZXJTbmFwc2hvdFIGbWVtYmVy');

@$core.Deprecated('Use getCollabSessionRequestDescriptor instead')
const GetCollabSessionRequest$json = {
  '1': 'GetCollabSessionRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `GetCollabSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCollabSessionRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRDb2xsYWJTZXNzaW9uUmVxdWVzdBIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW9uSW'
        'Q=');

@$core.Deprecated('Use listCollabSessionsRequestDescriptor instead')
const ListCollabSessionsRequest$json = {
  '1': 'ListCollabSessionsRequest',
  '2': [
    {'1': 'include_finished', '3': 1, '4': 1, '5': 8, '10': 'includeFinished'},
  ],
};

/// Descriptor for `ListCollabSessionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCollabSessionsRequestDescriptor =
    $convert.base64Decode(
        'ChlMaXN0Q29sbGFiU2Vzc2lvbnNSZXF1ZXN0EikKEGluY2x1ZGVfZmluaXNoZWQYASABKAhSD2'
        'luY2x1ZGVGaW5pc2hlZA==');

@$core.Deprecated('Use listCollabSessionsResponseDescriptor instead')
const ListCollabSessionsResponse$json = {
  '1': 'ListCollabSessionsResponse',
  '2': [
    {
      '1': 'sessions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.CollabSessionSnapshot',
      '10': 'sessions'
    },
  ],
};

/// Descriptor for `ListCollabSessionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCollabSessionsResponseDescriptor =
    $convert.base64Decode(
        'ChpMaXN0Q29sbGFiU2Vzc2lvbnNSZXNwb25zZRJBCghzZXNzaW9ucxgBIAMoCzIlLnBvY2tldG'
        'FnZW50LnYxLkNvbGxhYlNlc3Npb25TbmFwc2hvdFIIc2Vzc2lvbnM=');

@$core.Deprecated('Use cancelCollabSessionRequestDescriptor instead')
const CancelCollabSessionRequest$json = {
  '1': 'CancelCollabSessionRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `CancelCollabSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelCollabSessionRequestDescriptor =
    $convert.base64Decode(
        'ChpDYW5jZWxDb2xsYWJTZXNzaW9uUmVxdWVzdBIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW'
        '9uSWQ=');

@$core.Deprecated('Use cancelCollabMemberRequestDescriptor instead')
const CancelCollabMemberRequest$json = {
  '1': 'CancelCollabMemberRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'thread_id', '3': 2, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `CancelCollabMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelCollabMemberRequestDescriptor =
    $convert.base64Decode(
        'ChlDYW5jZWxDb2xsYWJNZW1iZXJSZXF1ZXN0Eh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb2'
        '5JZBIbCgl0aHJlYWRfaWQYAiABKAlSCHRocmVhZElk');

@$core.Deprecated('Use retryCollabMemberRequestDescriptor instead')
const RetryCollabMemberRequest$json = {
  '1': 'RetryCollabMemberRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'thread_id', '3': 2, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `RetryCollabMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List retryCollabMemberRequestDescriptor =
    $convert.base64Decode(
        'ChhSZXRyeUNvbGxhYk1lbWJlclJlcXVlc3QSHQoKc2Vzc2lvbl9pZBgBIAEoCVIJc2Vzc2lvbk'
        'lkEhsKCXRocmVhZF9pZBgCIAEoCVIIdGhyZWFkSWQ=');
