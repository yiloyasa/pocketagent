// This is a generated file - do not edit.
//
// Generated from v1/common.proto.

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

@$core.Deprecated('Use emptyDescriptor instead')
const Empty$json = {
  '1': 'Empty',
};

/// Descriptor for `Empty`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List emptyDescriptor =
    $convert.base64Decode('CgVFbXB0eQ==');

@$core.Deprecated('Use errorDescriptor instead')
const Error$json = {
  '1': 'Error',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
    {'1': 'message', '3': 2, '4': 1, '5': 9, '10': 'message'},
    {'1': 'retryable', '3': 3, '4': 1, '5': 8, '10': 'retryable'},
    {'1': 'http_status', '3': 4, '4': 1, '5': 5, '10': 'httpStatus'},
    {'1': 'url', '3': 5, '4': 1, '5': 9, '10': 'url'},
    {'1': 'details', '3': 6, '4': 1, '5': 9, '10': 'details'},
  ],
};

/// Descriptor for `Error`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List errorDescriptor = $convert.base64Decode(
    'CgVFcnJvchISCgRjb2RlGAEgASgJUgRjb2RlEhgKB21lc3NhZ2UYAiABKAlSB21lc3NhZ2USHA'
    'oJcmV0cnlhYmxlGAMgASgIUglyZXRyeWFibGUSHwoLaHR0cF9zdGF0dXMYBCABKAVSCmh0dHBT'
    'dGF0dXMSEAoDdXJsGAUgASgJUgN1cmwSGAoHZGV0YWlscxgGIAEoCVIHZGV0YWlscw==');

@$core.Deprecated('Use contentBlockDescriptor instead')
const ContentBlock$json = {
  '1': 'ContentBlock',
  '2': [
    {
      '1': 'text',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TextBlock',
      '9': 0,
      '10': 'text'
    },
    {
      '1': 'image',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ImageBlock',
      '9': 0,
      '10': 'image'
    },
    {
      '1': 'tool_call',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ToolCallBlock',
      '9': 0,
      '10': 'toolCall'
    },
    {
      '1': 'tool_result',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ToolResultBlock',
      '9': 0,
      '10': 'toolResult'
    },
    {
      '1': 'thinking',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ThinkingBlock',
      '9': 0,
      '10': 'thinking'
    },
  ],
  '8': [
    {'1': 'block'},
  ],
};

/// Descriptor for `ContentBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List contentBlockDescriptor = $convert.base64Decode(
    'CgxDb250ZW50QmxvY2sSLwoEdGV4dBgBIAEoCzIZLnBvY2tldGFnZW50LnYxLlRleHRCbG9ja0'
    'gAUgR0ZXh0EjIKBWltYWdlGAIgASgLMhoucG9ja2V0YWdlbnQudjEuSW1hZ2VCbG9ja0gAUgVp'
    'bWFnZRI8Cgl0b29sX2NhbGwYAyABKAsyHS5wb2NrZXRhZ2VudC52MS5Ub29sQ2FsbEJsb2NrSA'
    'BSCHRvb2xDYWxsEkIKC3Rvb2xfcmVzdWx0GAQgASgLMh8ucG9ja2V0YWdlbnQudjEuVG9vbFJl'
    'c3VsdEJsb2NrSABSCnRvb2xSZXN1bHQSOwoIdGhpbmtpbmcYBSABKAsyHS5wb2NrZXRhZ2VudC'
    '52MS5UaGlua2luZ0Jsb2NrSABSCHRoaW5raW5nQgcKBWJsb2Nr');

@$core.Deprecated('Use textBlockDescriptor instead')
const TextBlock$json = {
  '1': 'TextBlock',
  '2': [
    {'1': 'text', '3': 1, '4': 1, '5': 9, '10': 'text'},
  ],
};

/// Descriptor for `TextBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List textBlockDescriptor =
    $convert.base64Decode('CglUZXh0QmxvY2sSEgoEdGV4dBgBIAEoCVIEdGV4dA==');

@$core.Deprecated('Use imageBlockDescriptor instead')
const ImageBlock$json = {
  '1': 'ImageBlock',
  '2': [
    {'1': 'data', '3': 1, '4': 1, '5': 12, '10': 'data'},
    {'1': 'media_type', '3': 2, '4': 1, '5': 9, '10': 'mediaType'},
  ],
};

/// Descriptor for `ImageBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List imageBlockDescriptor = $convert.base64Decode(
    'CgpJbWFnZUJsb2NrEhIKBGRhdGEYASABKAxSBGRhdGESHQoKbWVkaWFfdHlwZRgCIAEoCVIJbW'
    'VkaWFUeXBl');

@$core.Deprecated('Use toolCallBlockDescriptor instead')
const ToolCallBlock$json = {
  '1': 'ToolCallBlock',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'json_args', '3': 3, '4': 1, '5': 9, '10': 'jsonArgs'},
  ],
};

/// Descriptor for `ToolCallBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List toolCallBlockDescriptor = $convert.base64Decode(
    'Cg1Ub29sQ2FsbEJsb2NrEg4KAmlkGAEgASgJUgJpZBISCgRuYW1lGAIgASgJUgRuYW1lEhsKCW'
    'pzb25fYXJncxgDIAEoCVIIanNvbkFyZ3M=');

@$core.Deprecated('Use toolResultBlockDescriptor instead')
const ToolResultBlock$json = {
  '1': 'ToolResultBlock',
  '2': [
    {'1': 'call_id', '3': 1, '4': 1, '5': 9, '10': 'callId'},
    {'1': 'ok', '3': 2, '4': 1, '5': 8, '10': 'ok'},
    {'1': 'output', '3': 3, '4': 1, '5': 9, '10': 'output'},
    {'1': 'elapsed_ms', '3': 4, '4': 1, '5': 1, '10': 'elapsedMs'},
  ],
};

/// Descriptor for `ToolResultBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List toolResultBlockDescriptor = $convert.base64Decode(
    'Cg9Ub29sUmVzdWx0QmxvY2sSFwoHY2FsbF9pZBgBIAEoCVIGY2FsbElkEg4KAm9rGAIgASgIUg'
    'JvaxIWCgZvdXRwdXQYAyABKAlSBm91dHB1dBIdCgplbGFwc2VkX21zGAQgASgBUgllbGFwc2Vk'
    'TXM=');

@$core.Deprecated('Use thinkingBlockDescriptor instead')
const ThinkingBlock$json = {
  '1': 'ThinkingBlock',
  '2': [
    {'1': 'text', '3': 1, '4': 1, '5': 9, '10': 'text'},
    {'1': 'signature', '3': 2, '4': 1, '5': 9, '10': 'signature'},
  ],
};

/// Descriptor for `ThinkingBlock`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List thinkingBlockDescriptor = $convert.base64Decode(
    'Cg1UaGlua2luZ0Jsb2NrEhIKBHRleHQYASABKAlSBHRleHQSHAoJc2lnbmF0dXJlGAIgASgJUg'
    'lzaWduYXR1cmU=');

@$core.Deprecated('Use usageDescriptor instead')
const Usage$json = {
  '1': 'Usage',
  '2': [
    {'1': 'input_tokens', '3': 1, '4': 1, '5': 3, '10': 'inputTokens'},
    {'1': 'output_tokens', '3': 2, '4': 1, '5': 3, '10': 'outputTokens'},
    {'1': 'cache_read_tokens', '3': 3, '4': 1, '5': 3, '10': 'cacheReadTokens'},
    {
      '1': 'cache_write_tokens',
      '3': 4,
      '4': 1,
      '5': 3,
      '10': 'cacheWriteTokens'
    },
    {'1': 'cost_usd', '3': 5, '4': 1, '5': 1, '10': 'costUsd'},
    {'1': 'reasoning_tokens', '3': 6, '4': 1, '5': 3, '10': 'reasoningTokens'},
  ],
};

/// Descriptor for `Usage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List usageDescriptor = $convert.base64Decode(
    'CgVVc2FnZRIhCgxpbnB1dF90b2tlbnMYASABKANSC2lucHV0VG9rZW5zEiMKDW91dHB1dF90b2'
    'tlbnMYAiABKANSDG91dHB1dFRva2VucxIqChFjYWNoZV9yZWFkX3Rva2VucxgDIAEoA1IPY2Fj'
    'aGVSZWFkVG9rZW5zEiwKEmNhY2hlX3dyaXRlX3Rva2VucxgEIAEoA1IQY2FjaGVXcml0ZVRva2'
    'VucxIZCghjb3N0X3VzZBgFIAEoAVIHY29zdFVzZBIpChByZWFzb25pbmdfdG9rZW5zGAYgASgD'
    'Ug9yZWFzb25pbmdUb2tlbnM=');
