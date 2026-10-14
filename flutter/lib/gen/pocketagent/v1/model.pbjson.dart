// This is a generated file - do not edit.
//
// Generated from v1/model.proto.

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

@$core.Deprecated('Use getCodexUsageRequestDescriptor instead')
const GetCodexUsageRequest$json = {
  '1': 'GetCodexUsageRequest',
};

/// Descriptor for `GetCodexUsageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCodexUsageRequestDescriptor =
    $convert.base64Decode('ChRHZXRDb2RleFVzYWdlUmVxdWVzdA==');

@$core.Deprecated('Use rateLimitWindowDescriptor instead')
const RateLimitWindow$json = {
  '1': 'RateLimitWindow',
  '2': [
    {'1': 'used_percent', '3': 1, '4': 1, '5': 5, '10': 'usedPercent'},
    {'1': 'resets_at_unix_s', '3': 2, '4': 1, '5': 3, '10': 'resetsAtUnixS'},
    {
      '1': 'window_duration_mins',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'windowDurationMins'
    },
  ],
};

/// Descriptor for `RateLimitWindow`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List rateLimitWindowDescriptor = $convert.base64Decode(
    'Cg9SYXRlTGltaXRXaW5kb3cSIQoMdXNlZF9wZXJjZW50GAEgASgFUgt1c2VkUGVyY2VudBInCh'
    'ByZXNldHNfYXRfdW5peF9zGAIgASgDUg1yZXNldHNBdFVuaXhTEjAKFHdpbmRvd19kdXJhdGlv'
    'bl9taW5zGAMgASgDUhJ3aW5kb3dEdXJhdGlvbk1pbnM=');

@$core.Deprecated('Use codexUsageDescriptor instead')
const CodexUsage$json = {
  '1': 'CodexUsage',
  '2': [
    {'1': 'plan_type', '3': 1, '4': 1, '5': 9, '10': 'planType'},
    {
      '1': 'primary',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RateLimitWindow',
      '10': 'primary'
    },
    {
      '1': 'secondary',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.RateLimitWindow',
      '10': 'secondary'
    },
    {'1': 'has_credits', '3': 4, '4': 1, '5': 8, '10': 'hasCredits'},
    {
      '1': 'unlimited_credits',
      '3': 5,
      '4': 1,
      '5': 8,
      '10': 'unlimitedCredits'
    },
    {'1': 'credit_balance', '3': 6, '4': 1, '5': 9, '10': 'creditBalance'},
    {
      '1': 'spend_control_reached',
      '3': 7,
      '4': 1,
      '5': 8,
      '10': 'spendControlReached'
    },
    {
      '1': 'rate_limit_reached_type',
      '3': 8,
      '4': 1,
      '5': 9,
      '10': 'rateLimitReachedType'
    },
    {'1': 'lifetime_tokens', '3': 9, '4': 1, '5': 3, '10': 'lifetimeTokens'},
    {
      '1': 'peak_daily_tokens',
      '3': 10,
      '4': 1,
      '5': 3,
      '10': 'peakDailyTokens'
    },
    {
      '1': 'current_streak_days',
      '3': 11,
      '4': 1,
      '5': 5,
      '10': 'currentStreakDays'
    },
    {
      '1': 'fetched_at_unix_ms',
      '3': 12,
      '4': 1,
      '5': 3,
      '10': 'fetchedAtUnixMs'
    },
    {'1': 'error', '3': 13, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `CodexUsage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexUsageDescriptor = $convert.base64Decode(
    'CgpDb2RleFVzYWdlEhsKCXBsYW5fdHlwZRgBIAEoCVIIcGxhblR5cGUSOQoHcHJpbWFyeRgCIA'
    'EoCzIfLnBvY2tldGFnZW50LnYxLlJhdGVMaW1pdFdpbmRvd1IHcHJpbWFyeRI9CglzZWNvbmRh'
    'cnkYAyABKAsyHy5wb2NrZXRhZ2VudC52MS5SYXRlTGltaXRXaW5kb3dSCXNlY29uZGFyeRIfCg'
    'toYXNfY3JlZGl0cxgEIAEoCFIKaGFzQ3JlZGl0cxIrChF1bmxpbWl0ZWRfY3JlZGl0cxgFIAEo'
    'CFIQdW5saW1pdGVkQ3JlZGl0cxIlCg5jcmVkaXRfYmFsYW5jZRgGIAEoCVINY3JlZGl0QmFsYW'
    '5jZRIyChVzcGVuZF9jb250cm9sX3JlYWNoZWQYByABKAhSE3NwZW5kQ29udHJvbFJlYWNoZWQS'
    'NQoXcmF0ZV9saW1pdF9yZWFjaGVkX3R5cGUYCCABKAlSFHJhdGVMaW1pdFJlYWNoZWRUeXBlEi'
    'cKD2xpZmV0aW1lX3Rva2VucxgJIAEoA1IObGlmZXRpbWVUb2tlbnMSKgoRcGVha19kYWlseV90'
    'b2tlbnMYCiABKANSD3BlYWtEYWlseVRva2VucxIuChNjdXJyZW50X3N0cmVha19kYXlzGAsgAS'
    'gFUhFjdXJyZW50U3RyZWFrRGF5cxIrChJmZXRjaGVkX2F0X3VuaXhfbXMYDCABKANSD2ZldGNo'
    'ZWRBdFVuaXhNcxIUCgVlcnJvchgNIAEoCVIFZXJyb3I=');

@$core.Deprecated('Use listCodexCapabilitiesRequestDescriptor instead')
const ListCodexCapabilitiesRequest$json = {
  '1': 'ListCodexCapabilitiesRequest',
};

/// Descriptor for `ListCodexCapabilitiesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCodexCapabilitiesRequestDescriptor =
    $convert.base64Decode('ChxMaXN0Q29kZXhDYXBhYmlsaXRpZXNSZXF1ZXN0');

@$core.Deprecated('Use codexCapabilityDescriptor instead')
const CodexCapability$json = {
  '1': 'CodexCapability',
  '2': [
    {'1': 'kind', '3': 1, '4': 1, '5': 9, '10': 'kind'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
    {'1': 'enabled', '3': 4, '4': 1, '5': 8, '10': 'enabled'},
    {'1': 'path', '3': 5, '4': 1, '5': 9, '10': 'path'},
    {'1': 'disabled_reason', '3': 6, '4': 1, '5': 9, '10': 'disabledReason'},
    {'1': 'version', '3': 7, '4': 1, '5': 9, '10': 'version'},
    {'1': 'source', '3': 8, '4': 1, '5': 9, '10': 'source'},
    {'1': 'status', '3': 9, '4': 1, '5': 9, '10': 'status'},
  ],
};

/// Descriptor for `CodexCapability`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexCapabilityDescriptor = $convert.base64Decode(
    'Cg9Db2RleENhcGFiaWxpdHkSEgoEa2luZBgBIAEoCVIEa2luZBISCgRuYW1lGAIgASgJUgRuYW'
    '1lEiAKC2Rlc2NyaXB0aW9uGAMgASgJUgtkZXNjcmlwdGlvbhIYCgdlbmFibGVkGAQgASgIUgdl'
    'bmFibGVkEhIKBHBhdGgYBSABKAlSBHBhdGgSJwoPZGlzYWJsZWRfcmVhc29uGAYgASgJUg5kaX'
    'NhYmxlZFJlYXNvbhIYCgd2ZXJzaW9uGAcgASgJUgd2ZXJzaW9uEhYKBnNvdXJjZRgIIAEoCVIG'
    'c291cmNlEhYKBnN0YXR1cxgJIAEoCVIGc3RhdHVz');

@$core.Deprecated('Use listCodexCapabilitiesResponseDescriptor instead')
const ListCodexCapabilitiesResponse$json = {
  '1': 'ListCodexCapabilitiesResponse',
  '2': [
    {
      '1': 'items',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.CodexCapability',
      '10': 'items'
    },
    {'1': 'error', '3': 2, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `ListCodexCapabilitiesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listCodexCapabilitiesResponseDescriptor =
    $convert.base64Decode(
        'Ch1MaXN0Q29kZXhDYXBhYmlsaXRpZXNSZXNwb25zZRI1CgVpdGVtcxgBIAMoCzIfLnBvY2tldG'
        'FnZW50LnYxLkNvZGV4Q2FwYWJpbGl0eVIFaXRlbXMSFAoFZXJyb3IYAiABKAlSBWVycm9y');

@$core.Deprecated('Use getCodexSettingsRequestDescriptor instead')
const GetCodexSettingsRequest$json = {
  '1': 'GetCodexSettingsRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'thread_id', '3': 2, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `GetCodexSettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCodexSettingsRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRDb2RleFNldHRpbmdzUmVxdWVzdBIZCghhZ2VudF9pZBgBIAEoCVIHYWdlbnRJZBIbCg'
        'l0aHJlYWRfaWQYAiABKAlSCHRocmVhZElk');

@$core.Deprecated('Use codexSettingsDescriptor instead')
const CodexSettings$json = {
  '1': 'CodexSettings',
  '2': [
    {'1': 'approval_policy', '3': 1, '4': 1, '5': 9, '10': 'approvalPolicy'},
    {'1': 'sandbox', '3': 2, '4': 1, '5': 9, '10': 'sandbox'},
    {'1': 'account', '3': 3, '4': 1, '5': 9, '10': 'account'},
    {'1': 'model', '3': 4, '4': 1, '5': 9, '10': 'model'},
    {'1': 'effort', '3': 5, '4': 1, '5': 9, '10': 'effort'},
    {
      '1': 'hide_agent_reasoning',
      '3': 6,
      '4': 1,
      '5': 8,
      '10': 'hideAgentReasoning'
    },
    {'1': 'model_verbosity', '3': 7, '4': 1, '5': 9, '10': 'modelVerbosity'},
    {
      '1': 'model_reasoning_summary',
      '3': 8,
      '4': 1,
      '5': 9,
      '10': 'modelReasoningSummary'
    },
    {
      '1': 'disable_paste_burst',
      '3': 9,
      '4': 1,
      '5': 8,
      '10': 'disablePasteBurst'
    },
    {
      '1': 'allowed_approval_policies',
      '3': 10,
      '4': 3,
      '5': 9,
      '10': 'allowedApprovalPolicies'
    },
    {
      '1': 'allowed_sandbox_modes',
      '3': 11,
      '4': 3,
      '5': 9,
      '10': 'allowedSandboxModes'
    },
    {
      '1': 'allowed_web_search_modes',
      '3': 12,
      '4': 3,
      '5': 9,
      '10': 'allowedWebSearchModes'
    },
    {'1': 'managed', '3': 13, '4': 1, '5': 8, '10': 'managed'},
    {'1': 'managed_note', '3': 14, '4': 1, '5': 9, '10': 'managedNote'},
    {
      '1': 'setting_status',
      '3': 15,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.CodexSettingStatus',
      '10': 'settingStatus'
    },
    {
      '1': 'desktop_config_summary',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CodexDesktopConfigSummary',
      '10': 'desktopConfigSummary'
    },
    {
      '1': 'provider_caps',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.CodexProviderCaps',
      '10': 'providerCaps'
    },
    {
      '1': 'sandbox_readiness',
      '3': 18,
      '4': 1,
      '5': 9,
      '10': 'sandboxReadiness'
    },
    {
      '1': 'desktop_echo_error',
      '3': 19,
      '4': 1,
      '5': 9,
      '10': 'desktopEchoError'
    },
    {'1': 'applied_threads', '3': 20, '4': 1, '5': 5, '10': 'appliedThreads'},
    {'1': 'native_mode', '3': 21, '4': 1, '5': 9, '10': 'nativeMode'},
    {'1': 'native_modes', '3': 22, '4': 3, '5': 9, '10': 'nativeModes'},
    {
      '1': 'native_permissions_note',
      '3': 23,
      '4': 1,
      '5': 9,
      '10': 'nativePermissionsNote'
    },
  ],
};

/// Descriptor for `CodexSettings`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexSettingsDescriptor = $convert.base64Decode(
    'Cg1Db2RleFNldHRpbmdzEicKD2FwcHJvdmFsX3BvbGljeRgBIAEoCVIOYXBwcm92YWxQb2xpY3'
    'kSGAoHc2FuZGJveBgCIAEoCVIHc2FuZGJveBIYCgdhY2NvdW50GAMgASgJUgdhY2NvdW50EhQK'
    'BW1vZGVsGAQgASgJUgVtb2RlbBIWCgZlZmZvcnQYBSABKAlSBmVmZm9ydBIwChRoaWRlX2FnZW'
    '50X3JlYXNvbmluZxgGIAEoCFISaGlkZUFnZW50UmVhc29uaW5nEicKD21vZGVsX3ZlcmJvc2l0'
    'eRgHIAEoCVIObW9kZWxWZXJib3NpdHkSNgoXbW9kZWxfcmVhc29uaW5nX3N1bW1hcnkYCCABKA'
    'lSFW1vZGVsUmVhc29uaW5nU3VtbWFyeRIuChNkaXNhYmxlX3Bhc3RlX2J1cnN0GAkgASgIUhFk'
    'aXNhYmxlUGFzdGVCdXJzdBI6ChlhbGxvd2VkX2FwcHJvdmFsX3BvbGljaWVzGAogAygJUhdhbG'
    'xvd2VkQXBwcm92YWxQb2xpY2llcxIyChVhbGxvd2VkX3NhbmRib3hfbW9kZXMYCyADKAlSE2Fs'
    'bG93ZWRTYW5kYm94TW9kZXMSNwoYYWxsb3dlZF93ZWJfc2VhcmNoX21vZGVzGAwgAygJUhVhbG'
    'xvd2VkV2ViU2VhcmNoTW9kZXMSGAoHbWFuYWdlZBgNIAEoCFIHbWFuYWdlZBIhCgxtYW5hZ2Vk'
    'X25vdGUYDiABKAlSC21hbmFnZWROb3RlEkkKDnNldHRpbmdfc3RhdHVzGA8gAygLMiIucG9ja2'
    'V0YWdlbnQudjEuQ29kZXhTZXR0aW5nU3RhdHVzUg1zZXR0aW5nU3RhdHVzEl8KFmRlc2t0b3Bf'
    'Y29uZmlnX3N1bW1hcnkYECABKAsyKS5wb2NrZXRhZ2VudC52MS5Db2RleERlc2t0b3BDb25maW'
    'dTdW1tYXJ5UhRkZXNrdG9wQ29uZmlnU3VtbWFyeRJGCg1wcm92aWRlcl9jYXBzGBEgASgLMiEu'
    'cG9ja2V0YWdlbnQudjEuQ29kZXhQcm92aWRlckNhcHNSDHByb3ZpZGVyQ2FwcxIrChFzYW5kYm'
    '94X3JlYWRpbmVzcxgSIAEoCVIQc2FuZGJveFJlYWRpbmVzcxIsChJkZXNrdG9wX2VjaG9fZXJy'
    'b3IYEyABKAlSEGRlc2t0b3BFY2hvRXJyb3ISJwoPYXBwbGllZF90aHJlYWRzGBQgASgFUg5hcH'
    'BsaWVkVGhyZWFkcxIfCgtuYXRpdmVfbW9kZRgVIAEoCVIKbmF0aXZlTW9kZRIhCgxuYXRpdmVf'
    'bW9kZXMYFiADKAlSC25hdGl2ZU1vZGVzEjYKF25hdGl2ZV9wZXJtaXNzaW9uc19ub3RlGBcgAS'
    'gJUhVuYXRpdmVQZXJtaXNzaW9uc05vdGU=');

@$core.Deprecated('Use codexDesktopConfigSummaryDescriptor instead')
const CodexDesktopConfigSummary$json = {
  '1': 'CodexDesktopConfigSummary',
  '2': [
    {'1': 'model', '3': 1, '4': 1, '5': 9, '10': 'model'},
    {'1': 'provider', '3': 2, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'effort', '3': 3, '4': 1, '5': 9, '10': 'effort'},
    {'1': 'summary', '3': 4, '4': 1, '5': 9, '10': 'summary'},
    {'1': 'verbosity', '3': 5, '4': 1, '5': 9, '10': 'verbosity'},
    {'1': 'sandbox_mode', '3': 6, '4': 1, '5': 9, '10': 'sandboxMode'},
    {'1': 'approval_policy', '3': 7, '4': 1, '5': 9, '10': 'approvalPolicy'},
  ],
};

/// Descriptor for `CodexDesktopConfigSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexDesktopConfigSummaryDescriptor = $convert.base64Decode(
    'ChlDb2RleERlc2t0b3BDb25maWdTdW1tYXJ5EhQKBW1vZGVsGAEgASgJUgVtb2RlbBIaCghwcm'
    '92aWRlchgCIAEoCVIIcHJvdmlkZXISFgoGZWZmb3J0GAMgASgJUgZlZmZvcnQSGAoHc3VtbWFy'
    'eRgEIAEoCVIHc3VtbWFyeRIcCgl2ZXJib3NpdHkYBSABKAlSCXZlcmJvc2l0eRIhCgxzYW5kYm'
    '94X21vZGUYBiABKAlSC3NhbmRib3hNb2RlEicKD2FwcHJvdmFsX3BvbGljeRgHIAEoCVIOYXBw'
    'cm92YWxQb2xpY3k=');

@$core.Deprecated('Use codexProviderCapsDescriptor instead')
const CodexProviderCaps$json = {
  '1': 'CodexProviderCaps',
  '2': [
    {'1': 'image_generation', '3': 1, '4': 1, '5': 8, '10': 'imageGeneration'},
    {'1': 'web_search', '3': 2, '4': 1, '5': 8, '10': 'webSearch'},
    {'1': 'namespace_tools', '3': 3, '4': 1, '5': 8, '10': 'namespaceTools'},
  ],
};

/// Descriptor for `CodexProviderCaps`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexProviderCapsDescriptor = $convert.base64Decode(
    'ChFDb2RleFByb3ZpZGVyQ2FwcxIpChBpbWFnZV9nZW5lcmF0aW9uGAEgASgIUg9pbWFnZUdlbm'
    'VyYXRpb24SHQoKd2ViX3NlYXJjaBgCIAEoCFIJd2ViU2VhcmNoEicKD25hbWVzcGFjZV90b29s'
    'cxgDIAEoCFIObmFtZXNwYWNlVG9vbHM=');

@$core.Deprecated('Use codexSettingStatusDescriptor instead')
const CodexSettingStatus$json = {
  '1': 'CodexSettingStatus',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'state', '3': 2, '4': 1, '5': 9, '10': 'state'},
    {'1': 'detail', '3': 3, '4': 1, '5': 9, '10': 'detail'},
  ],
};

/// Descriptor for `CodexSettingStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List codexSettingStatusDescriptor = $convert.base64Decode(
    'ChJDb2RleFNldHRpbmdTdGF0dXMSEAoDa2V5GAEgASgJUgNrZXkSFAoFc3RhdGUYAiABKAlSBX'
    'N0YXRlEhYKBmRldGFpbBgDIAEoCVIGZGV0YWls');

@$core.Deprecated('Use setCodexSettingsRequestDescriptor instead')
const SetCodexSettingsRequest$json = {
  '1': 'SetCodexSettingsRequest',
  '2': [
    {
      '1': 'approval_policy',
      '3': 1,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'approvalPolicy',
      '17': true
    },
    {
      '1': 'sandbox',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'sandbox',
      '17': true
    },
    {'1': 'model', '3': 3, '4': 1, '5': 9, '9': 2, '10': 'model', '17': true},
    {'1': 'effort', '3': 4, '4': 1, '5': 9, '9': 3, '10': 'effort', '17': true},
    {
      '1': 'hide_agent_reasoning',
      '3': 5,
      '4': 1,
      '5': 8,
      '9': 4,
      '10': 'hideAgentReasoning',
      '17': true
    },
    {
      '1': 'model_verbosity',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 5,
      '10': 'modelVerbosity',
      '17': true
    },
    {
      '1': 'model_reasoning_summary',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 6,
      '10': 'modelReasoningSummary',
      '17': true
    },
    {
      '1': 'disable_paste_burst',
      '3': 8,
      '4': 1,
      '5': 8,
      '9': 7,
      '10': 'disablePasteBurst',
      '17': true
    },
    {
      '1': 'apply_to_existing_threads',
      '3': 9,
      '4': 1,
      '5': 8,
      '9': 8,
      '10': 'applyToExistingThreads',
      '17': true
    },
    {'1': 'agent_id', '3': 10, '4': 1, '5': 9, '10': 'agentId'},
    {
      '1': 'native_mode',
      '3': 11,
      '4': 1,
      '5': 9,
      '9': 9,
      '10': 'nativeMode',
      '17': true
    },
    {'1': 'thread_id', '3': 12, '4': 1, '5': 9, '10': 'threadId'},
    {
      '1': 'native_config_option_id',
      '3': 13,
      '4': 1,
      '5': 9,
      '9': 10,
      '10': 'nativeConfigOptionId',
      '17': true
    },
    {
      '1': 'native_config_value',
      '3': 14,
      '4': 1,
      '5': 9,
      '9': 11,
      '10': 'nativeConfigValue',
      '17': true
    },
  ],
  '8': [
    {'1': '_approval_policy'},
    {'1': '_sandbox'},
    {'1': '_model'},
    {'1': '_effort'},
    {'1': '_hide_agent_reasoning'},
    {'1': '_model_verbosity'},
    {'1': '_model_reasoning_summary'},
    {'1': '_disable_paste_burst'},
    {'1': '_apply_to_existing_threads'},
    {'1': '_native_mode'},
    {'1': '_native_config_option_id'},
    {'1': '_native_config_value'},
  ],
};

/// Descriptor for `SetCodexSettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setCodexSettingsRequestDescriptor = $convert.base64Decode(
    'ChdTZXRDb2RleFNldHRpbmdzUmVxdWVzdBIsCg9hcHByb3ZhbF9wb2xpY3kYASABKAlIAFIOYX'
    'Bwcm92YWxQb2xpY3mIAQESHQoHc2FuZGJveBgCIAEoCUgBUgdzYW5kYm94iAEBEhkKBW1vZGVs'
    'GAMgASgJSAJSBW1vZGVsiAEBEhsKBmVmZm9ydBgEIAEoCUgDUgZlZmZvcnSIAQESNQoUaGlkZV'
    '9hZ2VudF9yZWFzb25pbmcYBSABKAhIBFISaGlkZUFnZW50UmVhc29uaW5niAEBEiwKD21vZGVs'
    'X3ZlcmJvc2l0eRgGIAEoCUgFUg5tb2RlbFZlcmJvc2l0eYgBARI7Chdtb2RlbF9yZWFzb25pbm'
    'dfc3VtbWFyeRgHIAEoCUgGUhVtb2RlbFJlYXNvbmluZ1N1bW1hcnmIAQESMwoTZGlzYWJsZV9w'
    'YXN0ZV9idXJzdBgIIAEoCEgHUhFkaXNhYmxlUGFzdGVCdXJzdIgBARI+ChlhcHBseV90b19leG'
    'lzdGluZ190aHJlYWRzGAkgASgISAhSFmFwcGx5VG9FeGlzdGluZ1RocmVhZHOIAQESGQoIYWdl'
    'bnRfaWQYCiABKAlSB2FnZW50SWQSJAoLbmF0aXZlX21vZGUYCyABKAlICVIKbmF0aXZlTW9kZY'
    'gBARIbCgl0aHJlYWRfaWQYDCABKAlSCHRocmVhZElkEjoKF25hdGl2ZV9jb25maWdfb3B0aW9u'
    'X2lkGA0gASgJSApSFG5hdGl2ZUNvbmZpZ09wdGlvbklkiAEBEjMKE25hdGl2ZV9jb25maWdfdm'
    'FsdWUYDiABKAlIC1IRbmF0aXZlQ29uZmlnVmFsdWWIAQFCEgoQX2FwcHJvdmFsX3BvbGljeUIK'
    'Cghfc2FuZGJveEIICgZfbW9kZWxCCQoHX2VmZm9ydEIXChVfaGlkZV9hZ2VudF9yZWFzb25pbm'
    'dCEgoQX21vZGVsX3ZlcmJvc2l0eUIaChhfbW9kZWxfcmVhc29uaW5nX3N1bW1hcnlCFgoUX2Rp'
    'c2FibGVfcGFzdGVfYnVyc3RCHAoaX2FwcGx5X3RvX2V4aXN0aW5nX3RocmVhZHNCDgoMX25hdG'
    'l2ZV9tb2RlQhoKGF9uYXRpdmVfY29uZmlnX29wdGlvbl9pZEIWChRfbmF0aXZlX2NvbmZpZ192'
    'YWx1ZQ==');

@$core.Deprecated('Use listAgentConfigOptionsRequestDescriptor instead')
const ListAgentConfigOptionsRequest$json = {
  '1': 'ListAgentConfigOptionsRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'thread_id', '3': 2, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `ListAgentConfigOptionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAgentConfigOptionsRequestDescriptor =
    $convert.base64Decode(
        'Ch1MaXN0QWdlbnRDb25maWdPcHRpb25zUmVxdWVzdBIZCghhZ2VudF9pZBgBIAEoCVIHYWdlbn'
        'RJZBIbCgl0aHJlYWRfaWQYAiABKAlSCHRocmVhZElk');

@$core.Deprecated('Use agentConfigOptionLeafDescriptor instead')
const AgentConfigOptionLeaf$json = {
  '1': 'AgentConfigOptionLeaf',
  '2': [
    {'1': 'value', '3': 1, '4': 1, '5': 9, '10': 'value'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
    {'1': 'group', '3': 4, '4': 1, '5': 9, '10': 'group'},
  ],
};

/// Descriptor for `AgentConfigOptionLeaf`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentConfigOptionLeafDescriptor = $convert.base64Decode(
    'ChVBZ2VudENvbmZpZ09wdGlvbkxlYWYSFAoFdmFsdWUYASABKAlSBXZhbHVlEhIKBG5hbWUYAi'
    'ABKAlSBG5hbWUSIAoLZGVzY3JpcHRpb24YAyABKAlSC2Rlc2NyaXB0aW9uEhQKBWdyb3VwGAQg'
    'ASgJUgVncm91cA==');

@$core.Deprecated('Use agentConfigOptionDescriptor instead')
const AgentConfigOption$json = {
  '1': 'AgentConfigOption',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'category', '3': 3, '4': 1, '5': 9, '10': 'category'},
    {'1': 'type', '3': 4, '4': 1, '5': 9, '10': 'type'},
    {'1': 'current_value', '3': 5, '4': 1, '5': 9, '10': 'currentValue'},
    {
      '1': 'options',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.AgentConfigOptionLeaf',
      '10': 'options'
    },
    {'1': 'writable', '3': 7, '4': 1, '5': 8, '10': 'writable'},
    {'1': 'note', '3': 8, '4': 1, '5': 9, '10': 'note'},
  ],
};

/// Descriptor for `AgentConfigOption`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List agentConfigOptionDescriptor = $convert.base64Decode(
    'ChFBZ2VudENvbmZpZ09wdGlvbhIOCgJpZBgBIAEoCVICaWQSEgoEbmFtZRgCIAEoCVIEbmFtZR'
    'IaCghjYXRlZ29yeRgDIAEoCVIIY2F0ZWdvcnkSEgoEdHlwZRgEIAEoCVIEdHlwZRIjCg1jdXJy'
    'ZW50X3ZhbHVlGAUgASgJUgxjdXJyZW50VmFsdWUSPwoHb3B0aW9ucxgGIAMoCzIlLnBvY2tldG'
    'FnZW50LnYxLkFnZW50Q29uZmlnT3B0aW9uTGVhZlIHb3B0aW9ucxIaCgh3cml0YWJsZRgHIAEo'
    'CFIId3JpdGFibGUSEgoEbm90ZRgIIAEoCVIEbm90ZQ==');

@$core.Deprecated('Use listAgentConfigOptionsResponseDescriptor instead')
const ListAgentConfigOptionsResponse$json = {
  '1': 'ListAgentConfigOptionsResponse',
  '2': [
    {
      '1': 'items',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.AgentConfigOption',
      '10': 'items'
    },
    {'1': 'error', '3': 2, '4': 1, '5': 9, '10': 'error'},
    {
      '1': 'sampled_at_unix_ms',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'sampledAtUnixMs'
    },
  ],
};

/// Descriptor for `ListAgentConfigOptionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listAgentConfigOptionsResponseDescriptor =
    $convert.base64Decode(
        'Ch5MaXN0QWdlbnRDb25maWdPcHRpb25zUmVzcG9uc2USNwoFaXRlbXMYASADKAsyIS5wb2NrZX'
        'RhZ2VudC52MS5BZ2VudENvbmZpZ09wdGlvblIFaXRlbXMSFAoFZXJyb3IYAiABKAlSBWVycm9y'
        'EisKEnNhbXBsZWRfYXRfdW5peF9tcxgDIAEoA1IPc2FtcGxlZEF0VW5peE1z');

@$core.Deprecated('Use listMcpServersRequestDescriptor instead')
const ListMcpServersRequest$json = {
  '1': 'ListMcpServersRequest',
};

/// Descriptor for `ListMcpServersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMcpServersRequestDescriptor =
    $convert.base64Decode('ChVMaXN0TWNwU2VydmVyc1JlcXVlc3Q=');

@$core.Deprecated('Use mcpServerStatusDescriptor instead')
const McpServerStatus$json = {
  '1': 'McpServerStatus',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'tools', '3': 2, '4': 1, '5': 5, '10': 'tools'},
    {'1': 'status', '3': 3, '4': 1, '5': 9, '10': 'status'},
    {'1': 'auth_status', '3': 4, '4': 1, '5': 9, '10': 'authStatus'},
    {'1': 'server_version', '3': 5, '4': 1, '5': 9, '10': 'serverVersion'},
    {'1': 'server_title', '3': 6, '4': 1, '5': 9, '10': 'serverTitle'},
    {'1': 'resource_count', '3': 7, '4': 1, '5': 5, '10': 'resourceCount'},
    {
      '1': 'resource_template_count',
      '3': 8,
      '4': 1,
      '5': 5,
      '10': 'resourceTemplateCount'
    },
    {'1': 'error', '3': 9, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `McpServerStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List mcpServerStatusDescriptor = $convert.base64Decode(
    'Cg9NY3BTZXJ2ZXJTdGF0dXMSEgoEbmFtZRgBIAEoCVIEbmFtZRIUCgV0b29scxgCIAEoBVIFdG'
    '9vbHMSFgoGc3RhdHVzGAMgASgJUgZzdGF0dXMSHwoLYXV0aF9zdGF0dXMYBCABKAlSCmF1dGhT'
    'dGF0dXMSJQoOc2VydmVyX3ZlcnNpb24YBSABKAlSDXNlcnZlclZlcnNpb24SIQoMc2VydmVyX3'
    'RpdGxlGAYgASgJUgtzZXJ2ZXJUaXRsZRIlCg5yZXNvdXJjZV9jb3VudBgHIAEoBVINcmVzb3Vy'
    'Y2VDb3VudBI2ChdyZXNvdXJjZV90ZW1wbGF0ZV9jb3VudBgIIAEoBVIVcmVzb3VyY2VUZW1wbG'
    'F0ZUNvdW50EhQKBWVycm9yGAkgASgJUgVlcnJvcg==');

@$core.Deprecated('Use listMcpServersResponseDescriptor instead')
const ListMcpServersResponse$json = {
  '1': 'ListMcpServersResponse',
  '2': [
    {
      '1': 'servers',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.McpServerStatus',
      '10': 'servers'
    },
    {'1': 'error', '3': 2, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `ListMcpServersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMcpServersResponseDescriptor =
    $convert.base64Decode(
        'ChZMaXN0TWNwU2VydmVyc1Jlc3BvbnNlEjkKB3NlcnZlcnMYASADKAsyHy5wb2NrZXRhZ2VudC'
        '52MS5NY3BTZXJ2ZXJTdGF0dXNSB3NlcnZlcnMSFAoFZXJyb3IYAiABKAlSBWVycm9y');

@$core.Deprecated('Use listModelsRequestDescriptor instead')
const ListModelsRequest$json = {
  '1': 'ListModelsRequest',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'refresh', '3': 2, '4': 1, '5': 8, '10': 'refresh'},
    {'1': 'thread_id', '3': 3, '4': 1, '5': 9, '10': 'threadId'},
  ],
};

/// Descriptor for `ListModelsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listModelsRequestDescriptor = $convert.base64Decode(
    'ChFMaXN0TW9kZWxzUmVxdWVzdBIZCghhZ2VudF9pZBgBIAEoCVIHYWdlbnRJZBIYCgdyZWZyZX'
    'NoGAIgASgIUgdyZWZyZXNoEhsKCXRocmVhZF9pZBgDIAEoCVIIdGhyZWFkSWQ=');

@$core.Deprecated('Use listModelsResponseDescriptor instead')
const ListModelsResponse$json = {
  '1': 'ListModelsResponse',
  '2': [
    {
      '1': 'models',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.ModelInfo',
      '10': 'models'
    },
    {'1': 'current_model_id', '3': 2, '4': 1, '5': 9, '10': 'currentModelId'},
    {
      '1': 'current_effort',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'currentEffort',
      '17': true
    },
    {'1': 'state_error', '3': 4, '4': 1, '5': 9, '10': 'stateError'},
    {
      '1': 'effective_selection',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.EffectiveModelSelection',
      '10': 'effectiveSelection'
    },
  ],
  '8': [
    {'1': '_current_effort'},
  ],
};

/// Descriptor for `ListModelsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listModelsResponseDescriptor = $convert.base64Decode(
    'ChJMaXN0TW9kZWxzUmVzcG9uc2USMQoGbW9kZWxzGAEgAygLMhkucG9ja2V0YWdlbnQudjEuTW'
    '9kZWxJbmZvUgZtb2RlbHMSKAoQY3VycmVudF9tb2RlbF9pZBgCIAEoCVIOY3VycmVudE1vZGVs'
    'SWQSKgoOY3VycmVudF9lZmZvcnQYAyABKAlIAFINY3VycmVudEVmZm9ydIgBARIfCgtzdGF0ZV'
    '9lcnJvchgEIAEoCVIKc3RhdGVFcnJvchJYChNlZmZlY3RpdmVfc2VsZWN0aW9uGAUgASgLMicu'
    'cG9ja2V0YWdlbnQudjEuRWZmZWN0aXZlTW9kZWxTZWxlY3Rpb25SEmVmZmVjdGl2ZVNlbGVjdG'
    'lvbkIRCg9fY3VycmVudF9lZmZvcnQ=');

@$core.Deprecated('Use modelInfoDescriptor instead')
const ModelInfo$json = {
  '1': 'ModelInfo',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'provider', '3': 2, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'display_name', '3': 3, '4': 1, '5': 9, '10': 'displayName'},
    {'1': 'context_window', '3': 4, '4': 1, '5': 5, '10': 'contextWindow'},
    {
      '1': 'supports_thinking',
      '3': 5,
      '4': 1,
      '5': 8,
      '10': 'supportsThinking'
    },
    {'1': 'supports_images', '3': 6, '4': 1, '5': 8, '10': 'supportsImages'},
    {'1': 'available', '3': 7, '4': 1, '5': 8, '10': 'available'},
    {
      '1': 'available_efforts',
      '3': 8,
      '4': 3,
      '5': 9,
      '10': 'availableEfforts'
    },
  ],
};

/// Descriptor for `ModelInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List modelInfoDescriptor = $convert.base64Decode(
    'CglNb2RlbEluZm8SDgoCaWQYASABKAlSAmlkEhoKCHByb3ZpZGVyGAIgASgJUghwcm92aWRlch'
    'IhCgxkaXNwbGF5X25hbWUYAyABKAlSC2Rpc3BsYXlOYW1lEiUKDmNvbnRleHRfd2luZG93GAQg'
    'ASgFUg1jb250ZXh0V2luZG93EisKEXN1cHBvcnRzX3RoaW5raW5nGAUgASgIUhBzdXBwb3J0c1'
    'RoaW5raW5nEicKD3N1cHBvcnRzX2ltYWdlcxgGIAEoCFIOc3VwcG9ydHNJbWFnZXMSHAoJYXZh'
    'aWxhYmxlGAcgASgIUglhdmFpbGFibGUSKwoRYXZhaWxhYmxlX2VmZm9ydHMYCCADKAlSEGF2YW'
    'lsYWJsZUVmZm9ydHM=');

@$core.Deprecated('Use effectiveModelSelectionDescriptor instead')
const EffectiveModelSelection$json = {
  '1': 'EffectiveModelSelection',
  '2': [
    {'1': 'agent_id', '3': 1, '4': 1, '5': 9, '10': 'agentId'},
    {'1': 'model_id', '3': 2, '4': 1, '5': 9, '10': 'modelId'},
    {'1': 'effort', '3': 3, '4': 1, '5': 9, '10': 'effort'},
    {'1': 'source', '3': 4, '4': 1, '5': 9, '10': 'source'},
    {
      '1': 'applies_to_next_turn',
      '3': 5,
      '4': 1,
      '5': 8,
      '10': 'appliesToNextTurn'
    },
    {
      '1': 'configuration_confirmed',
      '3': 6,
      '4': 1,
      '5': 8,
      '10': 'configurationConfirmed'
    },
    {'1': 'explanation', '3': 7, '4': 1, '5': 9, '10': 'explanation'},
    {
      '1': 'available_efforts',
      '3': 8,
      '4': 3,
      '5': 9,
      '10': 'availableEfforts'
    },
  ],
};

/// Descriptor for `EffectiveModelSelection`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List effectiveModelSelectionDescriptor = $convert.base64Decode(
    'ChdFZmZlY3RpdmVNb2RlbFNlbGVjdGlvbhIZCghhZ2VudF9pZBgBIAEoCVIHYWdlbnRJZBIZCg'
    'htb2RlbF9pZBgCIAEoCVIHbW9kZWxJZBIWCgZlZmZvcnQYAyABKAlSBmVmZm9ydBIWCgZzb3Vy'
    'Y2UYBCABKAlSBnNvdXJjZRIvChRhcHBsaWVzX3RvX25leHRfdHVybhgFIAEoCFIRYXBwbGllc1'
    'RvTmV4dFR1cm4SNwoXY29uZmlndXJhdGlvbl9jb25maXJtZWQYBiABKAhSFmNvbmZpZ3VyYXRp'
    'b25Db25maXJtZWQSIAoLZXhwbGFuYXRpb24YByABKAlSC2V4cGxhbmF0aW9uEisKEWF2YWlsYW'
    'JsZV9lZmZvcnRzGAggAygJUhBhdmFpbGFibGVFZmZvcnRz');

@$core.Deprecated('Use setDefaultModelRequestDescriptor instead')
const SetDefaultModelRequest$json = {
  '1': 'SetDefaultModelRequest',
  '2': [
    {'1': 'model_id', '3': 1, '4': 1, '5': 9, '10': 'modelId'},
  ],
};

/// Descriptor for `SetDefaultModelRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setDefaultModelRequestDescriptor =
    $convert.base64Decode(
        'ChZTZXREZWZhdWx0TW9kZWxSZXF1ZXN0EhkKCG1vZGVsX2lkGAEgASgJUgdtb2RlbElk');

@$core.Deprecated('Use testModelRequestDescriptor instead')
const TestModelRequest$json = {
  '1': 'TestModelRequest',
  '2': [
    {'1': 'model_id', '3': 1, '4': 1, '5': 9, '10': 'modelId'},
    {'1': 'prompt', '3': 2, '4': 1, '5': 9, '10': 'prompt'},
  ],
};

/// Descriptor for `TestModelRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List testModelRequestDescriptor = $convert.base64Decode(
    'ChBUZXN0TW9kZWxSZXF1ZXN0EhkKCG1vZGVsX2lkGAEgASgJUgdtb2RlbElkEhYKBnByb21wdB'
    'gCIAEoCVIGcHJvbXB0');

@$core.Deprecated('Use testModelResponseDescriptor instead')
const TestModelResponse$json = {
  '1': 'TestModelResponse',
  '2': [
    {'1': 'ok', '3': 1, '4': 1, '5': 8, '10': 'ok'},
    {'1': 'reply', '3': 2, '4': 1, '5': 9, '10': 'reply'},
    {'1': 'latency_ms', '3': 3, '4': 1, '5': 3, '10': 'latencyMs'},
    {'1': 'error', '3': 4, '4': 1, '5': 9, '10': 'error'},
  ],
};

/// Descriptor for `TestModelResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List testModelResponseDescriptor = $convert.base64Decode(
    'ChFUZXN0TW9kZWxSZXNwb25zZRIOCgJvaxgBIAEoCFICb2sSFAoFcmVwbHkYAiABKAlSBXJlcG'
    'x5Eh0KCmxhdGVuY3lfbXMYAyABKANSCWxhdGVuY3lNcxIUCgVlcnJvchgEIAEoCVIFZXJyb3I=');
