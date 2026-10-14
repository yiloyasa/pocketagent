// This is a generated file - do not edit.
//
// Generated from v1/runtime.proto.

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

@$core.Deprecated('Use getRuntimePolicyRequestDescriptor instead')
const GetRuntimePolicyRequest$json = {
  '1': 'GetRuntimePolicyRequest',
};

/// Descriptor for `GetRuntimePolicyRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getRuntimePolicyRequestDescriptor =
    $convert.base64Decode('ChdHZXRSdW50aW1lUG9saWN5UmVxdWVzdA==');

@$core.Deprecated('Use setRuntimePolicyRequestDescriptor instead')
const SetRuntimePolicyRequest$json = {
  '1': 'SetRuntimePolicyRequest',
  '2': [
    {
      '1': 'policy',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimePolicy',
      '10': 'policy'
    },
    {
      '1': 'expected_revision',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'expectedRevision'
    },
  ],
};

/// Descriptor for `SetRuntimePolicyRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setRuntimePolicyRequestDescriptor = $convert.base64Decode(
    'ChdTZXRSdW50aW1lUG9saWN5UmVxdWVzdBI1CgZwb2xpY3kYASABKAsyHS5wb2NrZXRhZ2VudC'
    '52MS5SdW50aW1lUG9saWN5UgZwb2xpY3kSKwoRZXhwZWN0ZWRfcmV2aXNpb24YAiABKARSEGV4'
    'cGVjdGVkUmV2aXNpb24=');

@$core.Deprecated('Use runtimePolicyResponseDescriptor instead')
const RuntimePolicyResponse$json = {
  '1': 'RuntimePolicyResponse',
  '2': [
    {
      '1': 'policy',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimePolicy',
      '10': 'policy'
    },
    {'1': 'revision', '3': 2, '4': 1, '5': 4, '10': 'revision'},
    {'1': 'policy_path', '3': 3, '4': 1, '5': 9, '10': 'policyPath'},
    {
      '1': 'effective_global',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeLimits',
      '10': 'effectiveGlobal'
    },
  ],
};

/// Descriptor for `RuntimePolicyResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runtimePolicyResponseDescriptor = $convert.base64Decode(
    'ChVSdW50aW1lUG9saWN5UmVzcG9uc2USNQoGcG9saWN5GAEgASgLMh0ucG9ja2V0YWdlbnQudj'
    'EuUnVudGltZVBvbGljeVIGcG9saWN5EhoKCHJldmlzaW9uGAIgASgEUghyZXZpc2lvbhIfCgtw'
    'b2xpY3lfcGF0aBgDIAEoCVIKcG9saWN5UGF0aBJIChBlZmZlY3RpdmVfZ2xvYmFsGAQgASgLMh'
    '0ucG9ja2V0YWdlbnQudjEuUnVudGltZUxpbWl0c1IPZWZmZWN0aXZlR2xvYmFs');

@$core.Deprecated('Use runtimeLimitsDescriptor instead')
const RuntimeLimits$json = {
  '1': 'RuntimeLimits',
  '2': [
    {
      '1': 'ram_working_bytes',
      '3': 1,
      '4': 1,
      '5': 3,
      '9': 0,
      '10': 'ramWorkingBytes',
      '17': true
    },
    {
      '1': 'ram_event_buffer_bytes',
      '3': 2,
      '4': 1,
      '5': 3,
      '9': 1,
      '10': 'ramEventBufferBytes',
      '17': true
    },
    {
      '1': 'ram_session_index_bytes',
      '3': 3,
      '4': 1,
      '5': 3,
      '9': 2,
      '10': 'ramSessionIndexBytes',
      '17': true
    },
    {
      '1': 'disk_conversation_bytes',
      '3': 4,
      '4': 1,
      '5': 3,
      '9': 3,
      '10': 'diskConversationBytes',
      '17': true
    },
    {
      '1': 'disk_cache_bytes',
      '3': 5,
      '4': 1,
      '5': 3,
      '9': 4,
      '10': 'diskCacheBytes',
      '17': true
    },
    {
      '1': 'disk_artifact_bytes',
      '3': 6,
      '4': 1,
      '5': 3,
      '9': 5,
      '10': 'diskArtifactBytes',
      '17': true
    },
    {
      '1': 'max_parallel_turns',
      '3': 7,
      '4': 1,
      '5': 5,
      '9': 6,
      '10': 'maxParallelTurns',
      '17': true
    },
  ],
  '8': [
    {'1': '_ram_working_bytes'},
    {'1': '_ram_event_buffer_bytes'},
    {'1': '_ram_session_index_bytes'},
    {'1': '_disk_conversation_bytes'},
    {'1': '_disk_cache_bytes'},
    {'1': '_disk_artifact_bytes'},
    {'1': '_max_parallel_turns'},
  ],
};

/// Descriptor for `RuntimeLimits`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runtimeLimitsDescriptor = $convert.base64Decode(
    'Cg1SdW50aW1lTGltaXRzEi8KEXJhbV93b3JraW5nX2J5dGVzGAEgASgDSABSD3JhbVdvcmtpbm'
    'dCeXRlc4gBARI4ChZyYW1fZXZlbnRfYnVmZmVyX2J5dGVzGAIgASgDSAFSE3JhbUV2ZW50QnVm'
    'ZmVyQnl0ZXOIAQESOgoXcmFtX3Nlc3Npb25faW5kZXhfYnl0ZXMYAyABKANIAlIUcmFtU2Vzc2'
    'lvbkluZGV4Qnl0ZXOIAQESOwoXZGlza19jb252ZXJzYXRpb25fYnl0ZXMYBCABKANIA1IVZGlz'
    'a0NvbnZlcnNhdGlvbkJ5dGVziAEBEi0KEGRpc2tfY2FjaGVfYnl0ZXMYBSABKANIBFIOZGlza0'
    'NhY2hlQnl0ZXOIAQESMwoTZGlza19hcnRpZmFjdF9ieXRlcxgGIAEoA0gFUhFkaXNrQXJ0aWZh'
    'Y3RCeXRlc4gBARIxChJtYXhfcGFyYWxsZWxfdHVybnMYByABKAVIBlIQbWF4UGFyYWxsZWxUdX'
    'Juc4gBAUIUChJfcmFtX3dvcmtpbmdfYnl0ZXNCGQoXX3JhbV9ldmVudF9idWZmZXJfYnl0ZXNC'
    'GgoYX3JhbV9zZXNzaW9uX2luZGV4X2J5dGVzQhoKGF9kaXNrX2NvbnZlcnNhdGlvbl9ieXRlc0'
    'ITChFfZGlza19jYWNoZV9ieXRlc0IWChRfZGlza19hcnRpZmFjdF9ieXRlc0IVChNfbWF4X3Bh'
    'cmFsbGVsX3R1cm5z');

@$core.Deprecated('Use runtimePolicyDescriptor instead')
const RuntimePolicy$json = {
  '1': 'RuntimePolicy',
  '2': [
    {
      '1': 'global',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeLimits',
      '10': 'global'
    },
    {
      '1': 'agents',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.RuntimePolicy.AgentsEntry',
      '10': 'agents'
    },
    {
      '1': 'workspaces',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.RuntimePolicy.WorkspacesEntry',
      '10': 'workspaces'
    },
    {
      '1': 'threads',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.RuntimePolicy.ThreadsEntry',
      '10': 'threads'
    },
    {
      '1': 'collabs',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.RuntimePolicy.CollabsEntry',
      '10': 'collabs'
    },
    {
      '1': 'unknown_request_passthrough',
      '3': 6,
      '4': 1,
      '5': 8,
      '9': 0,
      '10': 'unknownRequestPassthrough',
      '17': true
    },
  ],
  '3': [
    RuntimePolicy_AgentsEntry$json,
    RuntimePolicy_WorkspacesEntry$json,
    RuntimePolicy_ThreadsEntry$json,
    RuntimePolicy_CollabsEntry$json
  ],
  '8': [
    {'1': '_unknown_request_passthrough'},
  ],
};

@$core.Deprecated('Use runtimePolicyDescriptor instead')
const RuntimePolicy_AgentsEntry$json = {
  '1': 'AgentsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeLimits',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

@$core.Deprecated('Use runtimePolicyDescriptor instead')
const RuntimePolicy_WorkspacesEntry$json = {
  '1': 'WorkspacesEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeLimits',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

@$core.Deprecated('Use runtimePolicyDescriptor instead')
const RuntimePolicy_ThreadsEntry$json = {
  '1': 'ThreadsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeLimits',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

@$core.Deprecated('Use runtimePolicyDescriptor instead')
const RuntimePolicy_CollabsEntry$json = {
  '1': 'CollabsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeLimits',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

/// Descriptor for `RuntimePolicy`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runtimePolicyDescriptor = $convert.base64Decode(
    'Cg1SdW50aW1lUG9saWN5EjUKBmdsb2JhbBgBIAEoCzIdLnBvY2tldGFnZW50LnYxLlJ1bnRpbW'
    'VMaW1pdHNSBmdsb2JhbBJBCgZhZ2VudHMYAiADKAsyKS5wb2NrZXRhZ2VudC52MS5SdW50aW1l'
    'UG9saWN5LkFnZW50c0VudHJ5UgZhZ2VudHMSTQoKd29ya3NwYWNlcxgDIAMoCzItLnBvY2tldG'
    'FnZW50LnYxLlJ1bnRpbWVQb2xpY3kuV29ya3NwYWNlc0VudHJ5Ugp3b3Jrc3BhY2VzEkQKB3Ro'
    'cmVhZHMYBCADKAsyKi5wb2NrZXRhZ2VudC52MS5SdW50aW1lUG9saWN5LlRocmVhZHNFbnRyeV'
    'IHdGhyZWFkcxJECgdjb2xsYWJzGAUgAygLMioucG9ja2V0YWdlbnQudjEuUnVudGltZVBvbGlj'
    'eS5Db2xsYWJzRW50cnlSB2NvbGxhYnMSQwobdW5rbm93bl9yZXF1ZXN0X3Bhc3N0aHJvdWdoGA'
    'YgASgISABSGXVua25vd25SZXF1ZXN0UGFzc3Rocm91Z2iIAQEaWAoLQWdlbnRzRW50cnkSEAoD'
    'a2V5GAEgASgJUgNrZXkSMwoFdmFsdWUYAiABKAsyHS5wb2NrZXRhZ2VudC52MS5SdW50aW1lTG'
    'ltaXRzUgV2YWx1ZToCOAEaXAoPV29ya3NwYWNlc0VudHJ5EhAKA2tleRgBIAEoCVIDa2V5EjMK'
    'BXZhbHVlGAIgASgLMh0ucG9ja2V0YWdlbnQudjEuUnVudGltZUxpbWl0c1IFdmFsdWU6AjgBGl'
    'kKDFRocmVhZHNFbnRyeRIQCgNrZXkYASABKAlSA2tleRIzCgV2YWx1ZRgCIAEoCzIdLnBvY2tl'
    'dGFnZW50LnYxLlJ1bnRpbWVMaW1pdHNSBXZhbHVlOgI4ARpZCgxDb2xsYWJzRW50cnkSEAoDa2'
    'V5GAEgASgJUgNrZXkSMwoFdmFsdWUYAiABKAsyHS5wb2NrZXRhZ2VudC52MS5SdW50aW1lTGlt'
    'aXRzUgV2YWx1ZToCOAFCHgocX3Vua25vd25fcmVxdWVzdF9wYXNzdGhyb3VnaA==');

@$core.Deprecated('Use getResourceUsageRequestDescriptor instead')
const GetResourceUsageRequest$json = {
  '1': 'GetResourceUsageRequest',
  '2': [
    {'1': 'scope', '3': 1, '4': 1, '5': 9, '10': 'scope'},
  ],
};

/// Descriptor for `GetResourceUsageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getResourceUsageRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRSZXNvdXJjZVVzYWdlUmVxdWVzdBIUCgVzY29wZRgBIAEoCVIFc2NvcGU=');

@$core.Deprecated('Use resourceUsageResponseDescriptor instead')
const ResourceUsageResponse$json = {
  '1': 'ResourceUsageResponse',
  '2': [
    {
      '1': 'metrics',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ResourceMetric',
      '10': 'metrics'
    },
    {
      '1': 'sampled_at_unix_ms',
      '3': 2,
      '4': 1,
      '5': 3,
      '10': 'sampledAtUnixMs'
    },
  ],
};

/// Descriptor for `ResourceUsageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resourceUsageResponseDescriptor = $convert.base64Decode(
    'ChVSZXNvdXJjZVVzYWdlUmVzcG9uc2USOAoHbWV0cmljcxgBIAMoCzIeLnBvY2tldGFnZW50Ln'
    'YxLlJlc291cmNlTWV0cmljUgdtZXRyaWNzEisKEnNhbXBsZWRfYXRfdW5peF9tcxgCIAEoA1IP'
    'c2FtcGxlZEF0VW5peE1z');

@$core.Deprecated('Use resourceMetricDescriptor instead')
const ResourceMetric$json = {
  '1': 'ResourceMetric',
  '2': [
    {'1': 'lease_id', '3': 1, '4': 1, '5': 9, '10': 'leaseId'},
    {'1': 'scope', '3': 2, '4': 1, '5': 9, '10': 'scope'},
    {'1': 'kind', '3': 3, '4': 1, '5': 9, '10': 'kind'},
    {'1': 'reserved_bytes', '3': 4, '4': 1, '5': 3, '10': 'reservedBytes'},
    {'1': 'used_bytes', '3': 5, '4': 1, '5': 3, '10': 'usedBytes'},
    {'1': 'soft_limit_bytes', '3': 6, '4': 1, '5': 3, '10': 'softLimitBytes'},
    {'1': 'hard_limit_bytes', '3': 7, '4': 1, '5': 3, '10': 'hardLimitBytes'},
    {'1': 'state', '3': 8, '4': 1, '5': 9, '10': 'state'},
    {'1': 'spill_policy', '3': 9, '4': 1, '5': 9, '10': 'spillPolicy'},
    {'1': 'optional', '3': 10, '4': 1, '5': 8, '10': 'optional'},
  ],
};

/// Descriptor for `ResourceMetric`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resourceMetricDescriptor = $convert.base64Decode(
    'Cg5SZXNvdXJjZU1ldHJpYxIZCghsZWFzZV9pZBgBIAEoCVIHbGVhc2VJZBIUCgVzY29wZRgCIA'
    'EoCVIFc2NvcGUSEgoEa2luZBgDIAEoCVIEa2luZBIlCg5yZXNlcnZlZF9ieXRlcxgEIAEoA1IN'
    'cmVzZXJ2ZWRCeXRlcxIdCgp1c2VkX2J5dGVzGAUgASgDUgl1c2VkQnl0ZXMSKAoQc29mdF9saW'
    '1pdF9ieXRlcxgGIAEoA1IOc29mdExpbWl0Qnl0ZXMSKAoQaGFyZF9saW1pdF9ieXRlcxgHIAEo'
    'A1IOaGFyZExpbWl0Qnl0ZXMSFAoFc3RhdGUYCCABKAlSBXN0YXRlEiEKDHNwaWxsX3BvbGljeR'
    'gJIAEoCVILc3BpbGxQb2xpY3kSGgoIb3B0aW9uYWwYCiABKAhSCG9wdGlvbmFs');

@$core.Deprecated('Use listUnknownEventsRequestDescriptor instead')
const ListUnknownEventsRequest$json = {
  '1': 'ListUnknownEventsRequest',
  '2': [
    {'1': 'kind', '3': 1, '4': 1, '5': 9, '10': 'kind'},
    {'1': 'agent_id', '3': 2, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'thread_id', '3': 3, '4': 1, '5': 9, '10': 'threadId'},
    {
      '1': 'processing_status',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'processingStatus'
    },
    {'1': 'limit', '3': 5, '4': 1, '5': 5, '10': 'limit'},
  ],
};

/// Descriptor for `ListUnknownEventsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listUnknownEventsRequestDescriptor = $convert.base64Decode(
    'ChhMaXN0VW5rbm93bkV2ZW50c1JlcXVlc3QSEgoEa2luZBgBIAEoCVIEa2luZBIZCghhZ2VudF'
    '9pZBgCIAEoCVIHYWdlbnRJZBIbCgl0aHJlYWRfaWQYAyABKAlSCHRocmVhZElkEisKEXByb2Nl'
    'c3Npbmdfc3RhdHVzGAQgASgJUhBwcm9jZXNzaW5nU3RhdHVzEhQKBWxpbWl0GAUgASgFUgVsaW'
    '1pdA==');

@$core.Deprecated('Use userActionDescriptorDescriptor instead')
const UserActionDescriptor$json = {
  '1': 'UserActionDescriptor',
  '2': [
    {'1': 'action_id', '3': 1, '4': 1, '5': 9, '10': 'actionId'},
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
    {'1': 'safe', '3': 4, '4': 1, '5': 8, '10': 'safe'},
    {
      '1': 'requires_confirmation',
      '3': 5,
      '4': 1,
      '5': 8,
      '10': 'requiresConfirmation'
    },
    {'1': 'destructive', '3': 6, '4': 1, '5': 8, '10': 'destructive'},
  ],
};

/// Descriptor for `UserActionDescriptor`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userActionDescriptorDescriptor = $convert.base64Decode(
    'ChRVc2VyQWN0aW9uRGVzY3JpcHRvchIbCglhY3Rpb25faWQYASABKAlSCGFjdGlvbklkEhQKBW'
    'xhYmVsGAIgASgJUgVsYWJlbBIgCgtkZXNjcmlwdGlvbhgDIAEoCVILZGVzY3JpcHRpb24SEgoE'
    'c2FmZRgEIAEoCFIEc2FmZRIzChVyZXF1aXJlc19jb25maXJtYXRpb24YBSABKAhSFHJlcXVpcm'
    'VzQ29uZmlybWF0aW9uEiAKC2Rlc3RydWN0aXZlGAYgASgIUgtkZXN0cnVjdGl2ZQ==');

@$core.Deprecated('Use unknownEventSummaryDescriptor instead')
const UnknownEventSummary$json = {
  '1': 'UnknownEventSummary',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {'1': 'sequence', '3': 2, '4': 1, '5': 3, '10': 'sequence'},
    {
      '1': 'received_at_unix_ms',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'receivedAtUnixMs'
    },
    {'1': 'source_agent', '3': 4, '4': 1, '5': 9, '10': 'sourceAgent'},
    {'1': 'source_method', '3': 5, '4': 1, '5': 9, '10': 'sourceMethod'},
    {'1': 'thread_id', '3': 6, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'normalized_kind', '3': 7, '4': 1, '5': 9, '10': 'normalizedKind'},
    {'1': 'safety_class', '3': 8, '4': 1, '5': 9, '10': 'safetyClass'},
    {
      '1': 'processing_status',
      '3': 9,
      '4': 1,
      '5': 9,
      '10': 'processingStatus'
    },
    {'1': 'revision', '3': 10, '4': 1, '5': 4, '10': 'revision'},
    {'1': 'summary', '3': 11, '4': 1, '5': 9, '10': 'summary'},
    {'1': 'plain_title', '3': 12, '4': 1, '5': 9, '10': 'plainTitle'},
    {
      '1': 'plain_explanation',
      '3': 13,
      '4': 1,
      '5': 9,
      '10': 'plainExplanation'
    },
    {'1': 'impact', '3': 14, '4': 1, '5': 9, '10': 'impact'},
    {'1': 'requires_user', '3': 15, '4': 1, '5': 8, '10': 'requiresUser'},
    {'1': 'can_open_native', '3': 16, '4': 1, '5': 8, '10': 'canOpenNative'},
    {
      '1': 'actions',
      '3': 17,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.UserActionDescriptor',
      '10': 'actions'
    },
  ],
};

/// Descriptor for `UnknownEventSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unknownEventSummaryDescriptor = $convert.base64Decode(
    'ChNVbmtub3duRXZlbnRTdW1tYXJ5EhkKCGV2ZW50X2lkGAEgASgJUgdldmVudElkEhoKCHNlcX'
    'VlbmNlGAIgASgDUghzZXF1ZW5jZRItChNyZWNlaXZlZF9hdF91bml4X21zGAMgASgDUhByZWNl'
    'aXZlZEF0VW5peE1zEiEKDHNvdXJjZV9hZ2VudBgEIAEoCVILc291cmNlQWdlbnQSIwoNc291cm'
    'NlX21ldGhvZBgFIAEoCVIMc291cmNlTWV0aG9kEhsKCXRocmVhZF9pZBgGIAEoCVIIdGhyZWFk'
    'SWQSJwoPbm9ybWFsaXplZF9raW5kGAcgASgJUg5ub3JtYWxpemVkS2luZBIhCgxzYWZldHlfY2'
    'xhc3MYCCABKAlSC3NhZmV0eUNsYXNzEisKEXByb2Nlc3Npbmdfc3RhdHVzGAkgASgJUhBwcm9j'
    'ZXNzaW5nU3RhdHVzEhoKCHJldmlzaW9uGAogASgEUghyZXZpc2lvbhIYCgdzdW1tYXJ5GAsgAS'
    'gJUgdzdW1tYXJ5Eh8KC3BsYWluX3RpdGxlGAwgASgJUgpwbGFpblRpdGxlEisKEXBsYWluX2V4'
    'cGxhbmF0aW9uGA0gASgJUhBwbGFpbkV4cGxhbmF0aW9uEhYKBmltcGFjdBgOIAEoCVIGaW1wYW'
    'N0EiMKDXJlcXVpcmVzX3VzZXIYDyABKAhSDHJlcXVpcmVzVXNlchImCg9jYW5fb3Blbl9uYXRp'
    'dmUYECABKAhSDWNhbk9wZW5OYXRpdmUSPgoHYWN0aW9ucxgRIAMoCzIkLnBvY2tldGFnZW50Ln'
    'YxLlVzZXJBY3Rpb25EZXNjcmlwdG9yUgdhY3Rpb25z');

@$core.Deprecated('Use unknownEventFieldDescriptor instead')
const UnknownEventField$json = {
  '1': 'UnknownEventField',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'kind', '3': 2, '4': 1, '5': 9, '10': 'kind'},
    {'1': 'preview', '3': 3, '4': 1, '5': 9, '10': 'preview'},
    {'1': 'meaning', '3': 4, '4': 1, '5': 9, '10': 'meaning'},
  ],
};

/// Descriptor for `UnknownEventField`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unknownEventFieldDescriptor = $convert.base64Decode(
    'ChFVbmtub3duRXZlbnRGaWVsZBIQCgNrZXkYASABKAlSA2tleRISCgRraW5kGAIgASgJUgRraW'
    '5kEhgKB3ByZXZpZXcYAyABKAlSB3ByZXZpZXcSGAoHbWVhbmluZxgEIAEoCVIHbWVhbmluZw==');

@$core.Deprecated('Use listUnknownEventsResponseDescriptor instead')
const ListUnknownEventsResponse$json = {
  '1': 'ListUnknownEventsResponse',
  '2': [
    {
      '1': 'items',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.UnknownEventSummary',
      '10': 'items'
    },
    {'1': 'total', '3': 2, '4': 1, '5': 3, '10': 'total'},
    {'1': 'dropped', '3': 3, '4': 1, '5': 3, '10': 'dropped'},
  ],
};

/// Descriptor for `ListUnknownEventsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listUnknownEventsResponseDescriptor = $convert.base64Decode(
    'ChlMaXN0VW5rbm93bkV2ZW50c1Jlc3BvbnNlEjkKBWl0ZW1zGAEgAygLMiMucG9ja2V0YWdlbn'
    'QudjEuVW5rbm93bkV2ZW50U3VtbWFyeVIFaXRlbXMSFAoFdG90YWwYAiABKANSBXRvdGFsEhgK'
    'B2Ryb3BwZWQYAyABKANSB2Ryb3BwZWQ=');

@$core.Deprecated('Use getUnknownEventRequestDescriptor instead')
const GetUnknownEventRequest$json = {
  '1': 'GetUnknownEventRequest',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
  ],
};

/// Descriptor for `GetUnknownEventRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getUnknownEventRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRVbmtub3duRXZlbnRSZXF1ZXN0EhkKCGV2ZW50X2lkGAEgASgJUgdldmVudElk');

@$core.Deprecated('Use unknownEventResponseDescriptor instead')
const UnknownEventResponse$json = {
  '1': 'UnknownEventResponse',
  '2': [
    {
      '1': 'summary',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.UnknownEventSummary',
      '10': 'summary'
    },
    {'1': 'raw_json', '3': 2, '4': 1, '5': 9, '10': 'rawJson'},
    {
      '1': 'decoded_fields_json',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'decodedFieldsJson'
    },
    {'1': 'note', '3': 4, '4': 1, '5': 9, '10': 'note'},
    {
      '1': 'digest',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.UnknownEventField',
      '10': 'digest'
    },
    {'1': 'translation', '3': 6, '4': 1, '5': 9, '10': 'translation'},
    {
      '1': 'recommended_next_step',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'recommendedNextStep'
    },
    {
      '1': 'support_bundle_hint',
      '3': 8,
      '4': 1,
      '5': 9,
      '10': 'supportBundleHint'
    },
  ],
};

/// Descriptor for `UnknownEventResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unknownEventResponseDescriptor = $convert.base64Decode(
    'ChRVbmtub3duRXZlbnRSZXNwb25zZRI9CgdzdW1tYXJ5GAEgASgLMiMucG9ja2V0YWdlbnQudj'
    'EuVW5rbm93bkV2ZW50U3VtbWFyeVIHc3VtbWFyeRIZCghyYXdfanNvbhgCIAEoCVIHcmF3SnNv'
    'bhIuChNkZWNvZGVkX2ZpZWxkc19qc29uGAMgASgJUhFkZWNvZGVkRmllbGRzSnNvbhISCgRub3'
    'RlGAQgASgJUgRub3RlEjkKBmRpZ2VzdBgFIAMoCzIhLnBvY2tldGFnZW50LnYxLlVua25vd25F'
    'dmVudEZpZWxkUgZkaWdlc3QSIAoLdHJhbnNsYXRpb24YBiABKAlSC3RyYW5zbGF0aW9uEjIKFX'
    'JlY29tbWVuZGVkX25leHRfc3RlcBgHIAEoCVITcmVjb21tZW5kZWROZXh0U3RlcBIuChNzdXBw'
    'b3J0X2J1bmRsZV9oaW50GAggASgJUhFzdXBwb3J0QnVuZGxlSGludA==');

@$core.Deprecated('Use acknowledgeUnknownEventRequestDescriptor instead')
const AcknowledgeUnknownEventRequest$json = {
  '1': 'AcknowledgeUnknownEventRequest',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {'1': 'action', '3': 2, '4': 1, '5': 9, '10': 'action'},
    {'1': 'note', '3': 3, '4': 1, '5': 9, '10': 'note'},
  ],
};

/// Descriptor for `AcknowledgeUnknownEventRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acknowledgeUnknownEventRequestDescriptor =
    $convert.base64Decode(
        'Ch5BY2tub3dsZWRnZVVua25vd25FdmVudFJlcXVlc3QSGQoIZXZlbnRfaWQYASABKAlSB2V2ZW'
        '50SWQSFgoGYWN0aW9uGAIgASgJUgZhY3Rpb24SEgoEbm90ZRgDIAEoCVIEbm90ZQ==');

@$core.Deprecated('Use acknowledgeUnknownEventResponseDescriptor instead')
const AcknowledgeUnknownEventResponse$json = {
  '1': 'AcknowledgeUnknownEventResponse',
  '2': [
    {
      '1': 'item',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.UnknownEventSummary',
      '10': 'item'
    },
  ],
};

/// Descriptor for `AcknowledgeUnknownEventResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acknowledgeUnknownEventResponseDescriptor =
    $convert.base64Decode(
        'Ch9BY2tub3dsZWRnZVVua25vd25FdmVudFJlc3BvbnNlEjcKBGl0ZW0YASABKAsyIy5wb2NrZX'
        'RhZ2VudC52MS5Vbmtub3duRXZlbnRTdW1tYXJ5UgRpdGVt');

@$core.Deprecated('Use unknownEventActionRequestDescriptor instead')
const UnknownEventActionRequest$json = {
  '1': 'UnknownEventActionRequest',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {
      '1': 'expected_revision',
      '3': 2,
      '4': 1,
      '5': 3,
      '10': 'expectedRevision'
    },
    {'1': 'action_id', '3': 3, '4': 1, '5': 9, '10': 'actionId'},
    {'1': 'note', '3': 4, '4': 1, '5': 9, '10': 'note'},
  ],
};

/// Descriptor for `UnknownEventActionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unknownEventActionRequestDescriptor = $convert.base64Decode(
    'ChlVbmtub3duRXZlbnRBY3Rpb25SZXF1ZXN0EhkKCGV2ZW50X2lkGAEgASgJUgdldmVudElkEi'
    'sKEWV4cGVjdGVkX3JldmlzaW9uGAIgASgDUhBleHBlY3RlZFJldmlzaW9uEhsKCWFjdGlvbl9p'
    'ZBgDIAEoCVIIYWN0aW9uSWQSEgoEbm90ZRgEIAEoCVIEbm90ZQ==');

@$core.Deprecated('Use unknownEventActionResultDescriptor instead')
const UnknownEventActionResult$json = {
  '1': 'UnknownEventActionResult',
  '2': [
    {'1': 'ok', '3': 1, '4': 1, '5': 8, '10': 'ok'},
    {'1': 'conflict', '3': 2, '4': 1, '5': 8, '10': 'conflict'},
    {'1': 'detail', '3': 3, '4': 1, '5': 9, '10': 'detail'},
    {
      '1': 'summary',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.UnknownEventSummary',
      '10': 'summary'
    },
    {
      '1': 'technical',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.UnknownEventResponse',
      '10': 'technical'
    },
    {'1': 'new_revision', '3': 6, '4': 1, '5': 3, '10': 'newRevision'},
  ],
};

/// Descriptor for `UnknownEventActionResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unknownEventActionResultDescriptor = $convert.base64Decode(
    'ChhVbmtub3duRXZlbnRBY3Rpb25SZXN1bHQSDgoCb2sYASABKAhSAm9rEhoKCGNvbmZsaWN0GA'
    'IgASgIUghjb25mbGljdBIWCgZkZXRhaWwYAyABKAlSBmRldGFpbBI9CgdzdW1tYXJ5GAQgASgL'
    'MiMucG9ja2V0YWdlbnQudjEuVW5rbm93bkV2ZW50U3VtbWFyeVIHc3VtbWFyeRJCCgl0ZWNobm'
    'ljYWwYBSABKAsyJC5wb2NrZXRhZ2VudC52MS5Vbmtub3duRXZlbnRSZXNwb25zZVIJdGVjaG5p'
    'Y2FsEiEKDG5ld19yZXZpc2lvbhgGIAEoA1ILbmV3UmV2aXNpb24=');

@$core.Deprecated('Use collabPolicyDescriptor instead')
const CollabPolicy$json = {
  '1': 'CollabPolicy',
  '2': [
    {
      '1': 'collaboration_enabled',
      '3': 1,
      '4': 1,
      '5': 8,
      '9': 0,
      '10': 'collaborationEnabled',
      '17': true
    },
    {
      '1': 'native_delegate_enabled',
      '3': 2,
      '4': 1,
      '5': 8,
      '9': 1,
      '10': 'nativeDelegateEnabled',
      '17': true
    },
    {
      '1': 'fanout_enabled',
      '3': 3,
      '4': 1,
      '5': 8,
      '9': 2,
      '10': 'fanoutEnabled',
      '17': true
    },
    {
      '1': 'max_parallel_members',
      '3': 4,
      '4': 1,
      '5': 5,
      '9': 3,
      '10': 'maxParallelMembers',
      '17': true
    },
  ],
  '8': [
    {'1': '_collaboration_enabled'},
    {'1': '_native_delegate_enabled'},
    {'1': '_fanout_enabled'},
    {'1': '_max_parallel_members'},
  ],
};

/// Descriptor for `CollabPolicy`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List collabPolicyDescriptor = $convert.base64Decode(
    'CgxDb2xsYWJQb2xpY3kSOAoVY29sbGFib3JhdGlvbl9lbmFibGVkGAEgASgISABSFGNvbGxhYm'
    '9yYXRpb25FbmFibGVkiAEBEjsKF25hdGl2ZV9kZWxlZ2F0ZV9lbmFibGVkGAIgASgISAFSFW5h'
    'dGl2ZURlbGVnYXRlRW5hYmxlZIgBARIqCg5mYW5vdXRfZW5hYmxlZBgDIAEoCEgCUg1mYW5vdX'
    'RFbmFibGVkiAEBEjUKFG1heF9wYXJhbGxlbF9tZW1iZXJzGAQgASgFSANSEm1heFBhcmFsbGVs'
    'TWVtYmVyc4gBAUIYChZfY29sbGFib3JhdGlvbl9lbmFibGVkQhoKGF9uYXRpdmVfZGVsZWdhdG'
    'VfZW5hYmxlZEIRCg9fZmFub3V0X2VuYWJsZWRCFwoVX21heF9wYXJhbGxlbF9tZW1iZXJz');

@$core.Deprecated('Use getCollabPolicyRequestDescriptor instead')
const GetCollabPolicyRequest$json = {
  '1': 'GetCollabPolicyRequest',
};

/// Descriptor for `GetCollabPolicyRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCollabPolicyRequestDescriptor =
    $convert.base64Decode('ChZHZXRDb2xsYWJQb2xpY3lSZXF1ZXN0');

@$core.Deprecated('Use setCollabPolicyRequestDescriptor instead')
const SetCollabPolicyRequest$json = {
  '1': 'SetCollabPolicyRequest',
  '2': [
    {
      '1': 'policy',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CollabPolicy',
      '10': 'policy'
    },
    {
      '1': 'expected_revision',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'expectedRevision'
    },
  ],
};

/// Descriptor for `SetCollabPolicyRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setCollabPolicyRequestDescriptor = $convert.base64Decode(
    'ChZTZXRDb2xsYWJQb2xpY3lSZXF1ZXN0EjQKBnBvbGljeRgBIAEoCzIcLnBvY2tldGFnZW50Ln'
    'YxLkNvbGxhYlBvbGljeVIGcG9saWN5EisKEWV4cGVjdGVkX3JldmlzaW9uGAIgASgEUhBleHBl'
    'Y3RlZFJldmlzaW9u');

@$core.Deprecated('Use collabPolicyResponseDescriptor instead')
const CollabPolicyResponse$json = {
  '1': 'CollabPolicyResponse',
  '2': [
    {
      '1': 'policy',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CollabPolicy',
      '10': 'policy'
    },
    {'1': 'revision', '3': 2, '4': 1, '5': 4, '10': 'revision'},
    {'1': 'policy_path', '3': 3, '4': 1, '5': 9, '10': 'policyPath'},
    {
      '1': 'effective',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CollabPolicy',
      '10': 'effective'
    },
  ],
};

/// Descriptor for `CollabPolicyResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List collabPolicyResponseDescriptor = $convert.base64Decode(
    'ChRDb2xsYWJQb2xpY3lSZXNwb25zZRI0CgZwb2xpY3kYASABKAsyHC5wb2NrZXRhZ2VudC52MS'
    '5Db2xsYWJQb2xpY3lSBnBvbGljeRIaCghyZXZpc2lvbhgCIAEoBFIIcmV2aXNpb24SHwoLcG9s'
    'aWN5X3BhdGgYAyABKAlSCnBvbGljeVBhdGgSOgoJZWZmZWN0aXZlGAQgASgLMhwucG9ja2V0YW'
    'dlbnQudjEuQ29sbGFiUG9saWN5UgllZmZlY3RpdmU=');

@$core.Deprecated('Use getRuntimeStateRequestDescriptor instead')
const GetRuntimeStateRequest$json = {
  '1': 'GetRuntimeStateRequest',
};

/// Descriptor for `GetRuntimeStateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getRuntimeStateRequestDescriptor =
    $convert.base64Decode('ChZHZXRSdW50aW1lU3RhdGVSZXF1ZXN0');

@$core.Deprecated('Use runtimeStateResponseDescriptor instead')
const RuntimeStateResponse$json = {
  '1': 'RuntimeStateResponse',
  '2': [
    {'1': 'installation_id', '3': 1, '4': 1, '5': 9, '10': 'installationId'},
    {'1': 'host_id', '3': 2, '4': 1, '5': 9, '10': 'hostId'},
    {'1': 'daemon_version', '3': 3, '4': 1, '5': 9, '10': 'daemonVersion'},
    {
      '1': 'budget_policy_revision',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'budgetPolicyRevision'
    },
    {
      '1': 'collab_policy_revision',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'collabPolicyRevision'
    },
    {'1': 'runtime_state', '3': 6, '4': 1, '5': 9, '10': 'runtimeState'},
    {
      '1': 'sampled_at_unix_ms',
      '3': 7,
      '4': 1,
      '5': 3,
      '10': 'sampledAtUnixMs'
    },
    {
      '1': 'contracts',
      '3': 8,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeContractReport',
      '10': 'contracts'
    },
    {'1': 'data_dir', '3': 9, '4': 1, '5': 9, '10': 'dataDir'},
  ],
};

/// Descriptor for `RuntimeStateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runtimeStateResponseDescriptor = $convert.base64Decode(
    'ChRSdW50aW1lU3RhdGVSZXNwb25zZRInCg9pbnN0YWxsYXRpb25faWQYASABKAlSDmluc3RhbG'
    'xhdGlvbklkEhcKB2hvc3RfaWQYAiABKAlSBmhvc3RJZBIlCg5kYWVtb25fdmVyc2lvbhgDIAEo'
    'CVINZGFlbW9uVmVyc2lvbhI0ChZidWRnZXRfcG9saWN5X3JldmlzaW9uGAQgASgEUhRidWRnZX'
    'RQb2xpY3lSZXZpc2lvbhI0ChZjb2xsYWJfcG9saWN5X3JldmlzaW9uGAUgASgEUhRjb2xsYWJQ'
    'b2xpY3lSZXZpc2lvbhIjCg1ydW50aW1lX3N0YXRlGAYgASgJUgxydW50aW1lU3RhdGUSKwoSc2'
    'FtcGxlZF9hdF91bml4X21zGAcgASgDUg9zYW1wbGVkQXRVbml4TXMSQwoJY29udHJhY3RzGAgg'
    'AygLMiUucG9ja2V0YWdlbnQudjEuUnVudGltZUNvbnRyYWN0UmVwb3J0Ugljb250cmFjdHMSGQ'
    'oIZGF0YV9kaXIYCSABKAlSB2RhdGFEaXI=');

@$core.Deprecated('Use runtimeContractReportDescriptor instead')
const RuntimeContractReport$json = {
  '1': 'RuntimeContractReport',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'display_name', '3': 2, '4': 1, '5': 9, '10': 'displayName'},
    {'1': 'version', '3': 3, '4': 1, '5': 9, '10': 'version'},
    {'1': 'declared', '3': 4, '4': 1, '5': 8, '10': 'declared'},
    {'1': 'capabilities', '3': 5, '4': 3, '5': 9, '10': 'capabilities'},
    {
      '1': 'issues',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeContractIssue',
      '10': 'issues'
    },
    {
      '1': 'limitations',
      '3': 7,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeLimitation',
      '10': 'limitations'
    },
    {'1': 'session_model', '3': 8, '4': 1, '5': 9, '10': 'sessionModel'},
    {'1': 'resume_model', '3': 9, '4': 1, '5': 9, '10': 'resumeModel'},
    {'1': 'approval_model', '3': 10, '4': 1, '5': 9, '10': 'approvalModel'},
    {'1': 'subagent_model', '3': 11, '4': 1, '5': 9, '10': 'subagentModel'},
    {'1': 'memory_model', '3': 12, '4': 1, '5': 9, '10': 'memoryModel'},
    {'1': 'cache_model', '3': 13, '4': 1, '5': 9, '10': 'cacheModel'},
  ],
};

/// Descriptor for `RuntimeContractReport`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runtimeContractReportDescriptor = $convert.base64Decode(
    'ChVSdW50aW1lQ29udHJhY3RSZXBvcnQSGQoIYWdlbnRfaWQYASABKAlSB2FnZW50SWQSIQoMZG'
    'lzcGxheV9uYW1lGAIgASgJUgtkaXNwbGF5TmFtZRIYCgd2ZXJzaW9uGAMgASgJUgd2ZXJzaW9u'
    'EhoKCGRlY2xhcmVkGAQgASgIUghkZWNsYXJlZBIiCgxjYXBhYmlsaXRpZXMYBSADKAlSDGNhcG'
    'FiaWxpdGllcxI8CgZpc3N1ZXMYBiADKAsyJC5wb2NrZXRhZ2VudC52MS5SdW50aW1lQ29udHJh'
    'Y3RJc3N1ZVIGaXNzdWVzEkMKC2xpbWl0YXRpb25zGAcgAygLMiEucG9ja2V0YWdlbnQudjEuUn'
    'VudGltZUxpbWl0YXRpb25SC2xpbWl0YXRpb25zEiMKDXNlc3Npb25fbW9kZWwYCCABKAlSDHNl'
    'c3Npb25Nb2RlbBIhCgxyZXN1bWVfbW9kZWwYCSABKAlSC3Jlc3VtZU1vZGVsEiUKDmFwcHJvdm'
    'FsX21vZGVsGAogASgJUg1hcHByb3ZhbE1vZGVsEiUKDnN1YmFnZW50X21vZGVsGAsgASgJUg1z'
    'dWJhZ2VudE1vZGVsEiEKDG1lbW9yeV9tb2RlbBgMIAEoCVILbWVtb3J5TW9kZWwSHwoLY2FjaG'
    'VfbW9kZWwYDSABKAlSCmNhY2hlTW9kZWw=');

@$core.Deprecated('Use runtimeContractIssueDescriptor instead')
const RuntimeContractIssue$json = {
  '1': 'RuntimeContractIssue',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
    {'1': 'detail', '3': 2, '4': 1, '5': 9, '10': 'detail'},
  ],
};

/// Descriptor for `RuntimeContractIssue`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runtimeContractIssueDescriptor = $convert.base64Decode(
    'ChRSdW50aW1lQ29udHJhY3RJc3N1ZRISCgRjb2RlGAEgASgJUgRjb2RlEhYKBmRldGFpbBgCIA'
    'EoCVIGZGV0YWls');

@$core.Deprecated('Use runtimeLimitationDescriptor instead')
const RuntimeLimitation$json = {
  '1': 'RuntimeLimitation',
  '2': [
    {'1': 'capability', '3': 1, '4': 1, '5': 9, '10': 'capability'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `RuntimeLimitation`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List runtimeLimitationDescriptor = $convert.base64Decode(
    'ChFSdW50aW1lTGltaXRhdGlvbhIeCgpjYXBhYmlsaXR5GAEgASgJUgpjYXBhYmlsaXR5EhYKBn'
    'JlYXNvbhgCIAEoCVIGcmVhc29u');

@$core.Deprecated('Use getAgentRuntimeRequestDescriptor instead')
const GetAgentRuntimeRequest$json = {
  '1': 'GetAgentRuntimeRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
  ],
};

/// Descriptor for `GetAgentRuntimeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAgentRuntimeRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRBZ2VudFJ1bnRpbWVSZXF1ZXN0EhkKCGFnZW50X2lkGAEgASgJUgdhZ2VudElk');

@$core.Deprecated('Use agentRuntimeResponseDescriptor instead')
const AgentRuntimeResponse$json = {
  '1': 'AgentRuntimeResponse',
  '2': [
    {
      '1': 'contract',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RuntimeContractReport',
      '10': 'contract'
    },
    {
      '1': 'resources',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ResourceMetric',
      '10': 'resources'
    },
    {
      '1': 'sampled_at_unix_ms',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'sampledAtUnixMs'
    },
    {
      '1': 'memory',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.AgentMemoryUsage',
      '10': 'memory'
    },
    {
      '1': 'cache',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.AgentCacheUsage',
      '10': 'cache'
    },
    {
      '1': 'checkpoint',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.AgentCheckpointUsage',
      '10': 'checkpoint'
    },
    {
      '1': 'recovery',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.AgentRecoveryState',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `AgentRuntimeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentRuntimeResponseDescriptor = $convert.base64Decode(
    'ChRBZ2VudFJ1bnRpbWVSZXNwb25zZRJBCghjb250cmFjdBgBIAEoCzIlLnBvY2tldGFnZW50Ln'
    'YxLlJ1bnRpbWVDb250cmFjdFJlcG9ydFIIY29udHJhY3QSPAoJcmVzb3VyY2VzGAIgAygLMh4u'
    'cG9ja2V0YWdlbnQudjEuUmVzb3VyY2VNZXRyaWNSCXJlc291cmNlcxIrChJzYW1wbGVkX2F0X3'
    'VuaXhfbXMYAyABKANSD3NhbXBsZWRBdFVuaXhNcxI4CgZtZW1vcnkYBCABKAsyIC5wb2NrZXRh'
    'Z2VudC52MS5BZ2VudE1lbW9yeVVzYWdlUgZtZW1vcnkSNQoFY2FjaGUYBSABKAsyHy5wb2NrZX'
    'RhZ2VudC52MS5BZ2VudENhY2hlVXNhZ2VSBWNhY2hlEkQKCmNoZWNrcG9pbnQYBiABKAsyJC5w'
    'b2NrZXRhZ2VudC52MS5BZ2VudENoZWNrcG9pbnRVc2FnZVIKY2hlY2twb2ludBI+CghyZWNvdm'
    'VyeRgHIAEoCzIiLnBvY2tldGFnZW50LnYxLkFnZW50UmVjb3ZlcnlTdGF0ZVIIcmVjb3Zlcnk=');

@$core.Deprecated('Use agentMemoryUsageDescriptor instead')
const AgentMemoryUsage$json = {
  '1': 'AgentMemoryUsage',
  '2': [
    {'1': 'threads', '3': 1, '4': 1, '5': 5, '10': 'threads'},
    {'1': 'items', '3': 2, '4': 1, '5': 5, '10': 'items'},
    {'1': 'bytes', '3': 3, '4': 1, '5': 3, '10': 'bytes'},
    {
      '1': 'kinds',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.AgentMemoryUsage.KindsEntry',
      '10': 'kinds'
    },
  ],
  '3': [AgentMemoryUsage_KindsEntry$json],
};

@$core.Deprecated('Use agentMemoryUsageDescriptor instead')
const AgentMemoryUsage_KindsEntry$json = {
  '1': 'KindsEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 5, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `AgentMemoryUsage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentMemoryUsageDescriptor = $convert.base64Decode(
    'ChBBZ2VudE1lbW9yeVVzYWdlEhgKB3RocmVhZHMYASABKAVSB3RocmVhZHMSFAoFaXRlbXMYAi'
    'ABKAVSBWl0ZW1zEhQKBWJ5dGVzGAMgASgDUgVieXRlcxJBCgVraW5kcxgEIAMoCzIrLnBvY2tl'
    'dGFnZW50LnYxLkFnZW50TWVtb3J5VXNhZ2UuS2luZHNFbnRyeVIFa2luZHMaOAoKS2luZHNFbn'
    'RyeRIQCgNrZXkYASABKAlSA2tleRIUCgV2YWx1ZRgCIAEoBVIFdmFsdWU6AjgB');

@$core.Deprecated('Use agentCacheUsageDescriptor instead')
const AgentCacheUsage$json = {
  '1': 'AgentCacheUsage',
  '2': [
    {'1': 'namespaces', '3': 1, '4': 1, '5': 5, '10': 'namespaces'},
    {'1': 'entries', '3': 2, '4': 1, '5': 5, '10': 'entries'},
    {'1': 'bytes', '3': 3, '4': 1, '5': 3, '10': 'bytes'},
    {'1': 'hits', '3': 4, '4': 1, '5': 3, '10': 'hits'},
    {'1': 'misses', '3': 5, '4': 1, '5': 3, '10': 'misses'},
    {'1': 'evictions', '3': 6, '4': 1, '5': 3, '10': 'evictions'},
  ],
};

/// Descriptor for `AgentCacheUsage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentCacheUsageDescriptor = $convert.base64Decode(
    'Cg9BZ2VudENhY2hlVXNhZ2USHgoKbmFtZXNwYWNlcxgBIAEoBVIKbmFtZXNwYWNlcxIYCgdlbn'
    'RyaWVzGAIgASgFUgdlbnRyaWVzEhQKBWJ5dGVzGAMgASgDUgVieXRlcxISCgRoaXRzGAQgASgD'
    'UgRoaXRzEhYKBm1pc3NlcxgFIAEoA1IGbWlzc2VzEhwKCWV2aWN0aW9ucxgGIAEoA1IJZXZpY3'
    'Rpb25z');

@$core.Deprecated('Use agentCheckpointUsageDescriptor instead')
const AgentCheckpointUsage$json = {
  '1': 'AgentCheckpointUsage',
  '2': [
    {'1': 'count', '3': 1, '4': 1, '5': 5, '10': 'count'},
    {'1': 'bytes', '3': 2, '4': 1, '5': 3, '10': 'bytes'},
  ],
};

/// Descriptor for `AgentCheckpointUsage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentCheckpointUsageDescriptor = $convert.base64Decode(
    'ChRBZ2VudENoZWNrcG9pbnRVc2FnZRIUCgVjb3VudBgBIAEoBVIFY291bnQSFAoFYnl0ZXMYAi'
    'ABKANSBWJ5dGVz');

@$core.Deprecated('Use agentRecoveryStateDescriptor instead')
const AgentRecoveryState$json = {
  '1': 'AgentRecoveryState',
  '2': [
    {'1': 'reported', '3': 1, '4': 1, '5': 8, '10': 'reported'},
    {'1': 'attempted', '3': 2, '4': 1, '5': 8, '10': 'attempted'},
    {'1': 'succeeded', '3': 3, '4': 1, '5': 8, '10': 'succeeded'},
    {'1': 'from', '3': 4, '4': 1, '5': 9, '10': 'from'},
    {'1': 'reason', '3': 5, '4': 1, '5': 9, '10': 'reason'},
    {'1': 'at', '3': 6, '4': 1, '5': 9, '10': 'at'},
  ],
};

/// Descriptor for `AgentRecoveryState`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentRecoveryStateDescriptor = $convert.base64Decode(
    'ChJBZ2VudFJlY292ZXJ5U3RhdGUSGgoIcmVwb3J0ZWQYASABKAhSCHJlcG9ydGVkEhwKCWF0dG'
    'VtcHRlZBgCIAEoCFIJYXR0ZW1wdGVkEhwKCXN1Y2NlZWRlZBgDIAEoCFIJc3VjY2VlZGVkEhIK'
    'BGZyb20YBCABKAlSBGZyb20SFgoGcmVhc29uGAUgASgJUgZyZWFzb24SDgoCYXQYBiABKAlSAm'
    'F0');

@$core.Deprecated('Use listThreadMemoriesRequestDescriptor instead')
const ListThreadMemoriesRequest$json = {
  '1': 'ListThreadMemoriesRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
  ],
};

/// Descriptor for `ListThreadMemoriesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadMemoriesRequestDescriptor =
    $convert.base64Decode(
        'ChlMaXN0VGhyZWFkTWVtb3JpZXNSZXF1ZXN0EhkKCGFnZW50X2lkGAEgASgJUgdhZ2VudElk');

@$core.Deprecated('Use threadMemoryRowDescriptor instead')
const ThreadMemoryRow$json = {
  '1': 'ThreadMemoryRow',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'entries', '3': 2, '4': 1, '5': 5, '10': 'entries'},
    {'1': 'bytes', '3': 3, '4': 1, '5': 3, '10': 'bytes'},
    {'1': 'seq', '3': 4, '4': 1, '5': 3, '10': 'seq'},
    {
      '1': 'last_checkpoint_unix_ms',
      '3': 5,
      '4': 1,
      '5': 3,
      '10': 'lastCheckpointUnixMs'
    },
    {'1': 'loaded', '3': 6, '4': 1, '5': 8, '10': 'loaded'},
  ],
};

/// Descriptor for `ThreadMemoryRow`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List threadMemoryRowDescriptor = $convert.base64Decode(
    'Cg9UaHJlYWRNZW1vcnlSb3cSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZBIYCgdlbnRyaW'
    'VzGAIgASgFUgdlbnRyaWVzEhQKBWJ5dGVzGAMgASgDUgVieXRlcxIQCgNzZXEYBCABKANSA3Nl'
    'cRI1ChdsYXN0X2NoZWNrcG9pbnRfdW5peF9tcxgFIAEoA1IUbGFzdENoZWNrcG9pbnRVbml4TX'
    'MSFgoGbG9hZGVkGAYgASgIUgZsb2FkZWQ=');

@$core.Deprecated('Use listThreadMemoriesResponseDescriptor instead')
const ListThreadMemoriesResponse$json = {
  '1': 'ListThreadMemoriesResponse',
  '2': [
    {
      '1': 'rows',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ThreadMemoryRow',
      '10': 'rows'
    },
    {'1': 'total_threads', '3': 2, '4': 1, '5': 5, '10': 'totalThreads'},
    {'1': 'total_entries', '3': 3, '4': 1, '5': 5, '10': 'totalEntries'},
    {'1': 'total_bytes', '3': 4, '4': 1, '5': 3, '10': 'totalBytes'},
    {
      '1': 'sampled_at_unix_ms',
      '3': 5,
      '4': 1,
      '5': 3,
      '10': 'sampledAtUnixMs'
    },
  ],
};

/// Descriptor for `ListThreadMemoriesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadMemoriesResponseDescriptor = $convert.base64Decode(
    'ChpMaXN0VGhyZWFkTWVtb3JpZXNSZXNwb25zZRIzCgRyb3dzGAEgAygLMh8ucG9ja2V0YWdlbn'
    'QudjEuVGhyZWFkTWVtb3J5Um93UgRyb3dzEiMKDXRvdGFsX3RocmVhZHMYAiABKAVSDHRvdGFs'
    'VGhyZWFkcxIjCg10b3RhbF9lbnRyaWVzGAMgASgFUgx0b3RhbEVudHJpZXMSHwoLdG90YWxfYn'
    'l0ZXMYBCABKANSCnRvdGFsQnl0ZXMSKwoSc2FtcGxlZF9hdF91bml4X21zGAUgASgDUg9zYW1w'
    'bGVkQXRVbml4TXM=');

@$core.Deprecated('Use clearThreadMemoryRequestDescriptor instead')
const ClearThreadMemoryRequest$json = {
  '1': 'ClearThreadMemoryRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'thread_id', '3': 2, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `ClearThreadMemoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearThreadMemoryRequestDescriptor =
    $convert.base64Decode(
        'ChhDbGVhclRocmVhZE1lbW9yeVJlcXVlc3QSGQoIYWdlbnRfaWQYASABKAlSB2FnZW50SWQSGw'
        'oJdGhyZWFkX2lkGAIgASgJUgh0aHJlYWRJZA==');

@$core.Deprecated('Use clearThreadMemoryResponseDescriptor instead')
const ClearThreadMemoryResponse$json = {
  '1': 'ClearThreadMemoryResponse',
  '2': [
    {'1': 'total_threads', '3': 1, '4': 1, '5': 5, '10': 'totalThreads'},
    {'1': 'total_entries', '3': 2, '4': 1, '5': 5, '10': 'totalEntries'},
    {'1': 'total_bytes', '3': 3, '4': 1, '5': 3, '10': 'totalBytes'},
  ],
};

/// Descriptor for `ClearThreadMemoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearThreadMemoryResponseDescriptor = $convert.base64Decode(
    'ChlDbGVhclRocmVhZE1lbW9yeVJlc3BvbnNlEiMKDXRvdGFsX3RocmVhZHMYASABKAVSDHRvdG'
    'FsVGhyZWFkcxIjCg10b3RhbF9lbnRyaWVzGAIgASgFUgx0b3RhbEVudHJpZXMSHwoLdG90YWxf'
    'Ynl0ZXMYAyABKANSCnRvdGFsQnl0ZXM=');

@$core.Deprecated('Use getControlPlaneRequestDescriptor instead')
const GetControlPlaneRequest$json = {
  '1': 'GetControlPlaneRequest',
};

/// Descriptor for `GetControlPlaneRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getControlPlaneRequestDescriptor =
    $convert.base64Decode('ChZHZXRDb250cm9sUGxhbmVSZXF1ZXN0');

@$core.Deprecated('Use controlPlaneDescriptor instead')
const ControlPlane$json = {
  '1': 'ControlPlane',
  '2': [
    {'1': 'daemon_version', '3': 1, '4': 1, '5': 9, '10': 'daemonVersion'},
    {'1': 'host_name', '3': 2, '4': 1, '5': 9, '10': 'hostName'},
    {'1': 'os', '3': 3, '4': 1, '5': 9, '10': 'os'},
    {'1': 'uptime_unix_ms', '3': 4, '4': 1, '5': 3, '10': 'uptimeUnixMs'},
    {
      '1': 'domains',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ControlDomain',
      '10': 'domains'
    },
    {'1': 'data_dir_label', '3': 6, '4': 1, '5': 9, '10': 'dataDirLabel'},
  ],
};

/// Descriptor for `ControlPlane`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List controlPlaneDescriptor = $convert.base64Decode(
    'CgxDb250cm9sUGxhbmUSJQoOZGFlbW9uX3ZlcnNpb24YASABKAlSDWRhZW1vblZlcnNpb24SGw'
    'oJaG9zdF9uYW1lGAIgASgJUghob3N0TmFtZRIOCgJvcxgDIAEoCVICb3MSJAoOdXB0aW1lX3Vu'
    'aXhfbXMYBCABKANSDHVwdGltZVVuaXhNcxI3Cgdkb21haW5zGAUgAygLMh0ucG9ja2V0YWdlbn'
    'QudjEuQ29udHJvbERvbWFpblIHZG9tYWlucxIkCg5kYXRhX2Rpcl9sYWJlbBgGIAEoCVIMZGF0'
    'YURpckxhYmVs');

@$core.Deprecated('Use controlDomainDescriptor instead')
const ControlDomain$json = {
  '1': 'ControlDomain',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'title', '3': 2, '4': 1, '5': 9, '10': 'title'},
    {'1': 'summary', '3': 3, '4': 1, '5': 9, '10': 'summary'},
    {'1': 'writable', '3': 4, '4': 1, '5': 8, '10': 'writable'},
    {'1': 'status_code', '3': 5, '4': 1, '5': 5, '10': 'statusCode'},
    {
      '1': 'items',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ControlItem',
      '10': 'items'
    },
  ],
};

/// Descriptor for `ControlDomain`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List controlDomainDescriptor = $convert.base64Decode(
    'Cg1Db250cm9sRG9tYWluEg4KAmlkGAEgASgJUgJpZBIUCgV0aXRsZRgCIAEoCVIFdGl0bGUSGA'
    'oHc3VtbWFyeRgDIAEoCVIHc3VtbWFyeRIaCgh3cml0YWJsZRgEIAEoCFIId3JpdGFibGUSHwoL'
    'c3RhdHVzX2NvZGUYBSABKAVSCnN0YXR1c0NvZGUSMQoFaXRlbXMYBiADKAsyGy5wb2NrZXRhZ2'
    'VudC52MS5Db250cm9sSXRlbVIFaXRlbXM=');

@$core.Deprecated('Use controlItemDescriptor instead')
const ControlItem$json = {
  '1': 'ControlItem',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'label', '3': 2, '4': 1, '5': 9, '10': 'label'},
    {'1': 'current', '3': 3, '4': 1, '5': 9, '10': 'current'},
    {'1': 'kind', '3': 4, '4': 1, '5': 9, '10': 'kind'},
    {'1': 'options', '3': 5, '4': 3, '5': 9, '10': 'options'},
    {'1': 'hint', '3': 6, '4': 1, '5': 9, '10': 'hint'},
  ],
};

/// Descriptor for `ControlItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List controlItemDescriptor = $convert.base64Decode(
    'CgtDb250cm9sSXRlbRIOCgJpZBgBIAEoCVICaWQSFAoFbGFiZWwYAiABKAlSBWxhYmVsEhgKB2'
    'N1cnJlbnQYAyABKAlSB2N1cnJlbnQSEgoEa2luZBgEIAEoCVIEa2luZBIYCgdvcHRpb25zGAUg'
    'AygJUgdvcHRpb25zEhIKBGhpbnQYBiABKAlSBGhpbnQ=');
