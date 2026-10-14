// This is a generated file - do not edit.
//
// Generated from v1/terminal.proto.

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

@$core.Deprecated('Use terminalClientMessageDescriptor instead')
const TerminalClientMessage$json = {
  '1': 'TerminalClientMessage',
  '2': [
    {
      '1': 'open',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.OpenTerminal',
      '9': 0,
      '10': 'open'
    },
    {
      '1': 'input',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TerminalInput',
      '9': 0,
      '10': 'input'
    },
    {
      '1': 'resize',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TerminalResize',
      '9': 0,
      '10': 'resize'
    },
  ],
  '8': [
    {'1': 'msg'},
  ],
};

/// Descriptor for `TerminalClientMessage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalClientMessageDescriptor = $convert.base64Decode(
    'ChVUZXJtaW5hbENsaWVudE1lc3NhZ2USMgoEb3BlbhgBIAEoCzIcLnBvY2tldGFnZW50LnYxLk'
    '9wZW5UZXJtaW5hbEgAUgRvcGVuEjUKBWlucHV0GAIgASgLMh0ucG9ja2V0YWdlbnQudjEuVGVy'
    'bWluYWxJbnB1dEgAUgVpbnB1dBI4CgZyZXNpemUYAyABKAsyHi5wb2NrZXRhZ2VudC52MS5UZX'
    'JtaW5hbFJlc2l6ZUgAUgZyZXNpemVCBQoDbXNn');

@$core.Deprecated('Use openTerminalDescriptor instead')
const OpenTerminal$json = {
  '1': 'OpenTerminal',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'cwd', '3': 2, '4': 1, '5': 9, '10': 'cwd'},
    {'1': 'shell', '3': 3, '4': 1, '5': 9, '10': 'shell'},
    {'1': 'cols', '3': 4, '4': 1, '5': 5, '10': 'cols'},
    {'1': 'rows', '3': 5, '4': 1, '5': 5, '10': 'rows'},
    {'1': 'env_json', '3': 6, '4': 1, '5': 9, '10': 'envJson'},
  ],
};

/// Descriptor for `OpenTerminal`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List openTerminalDescriptor = $convert.base64Decode(
    'CgxPcGVuVGVybWluYWwSHQoKc2Vzc2lvbl9pZBgBIAEoCVIJc2Vzc2lvbklkEhAKA2N3ZBgCIA'
    'EoCVIDY3dkEhQKBXNoZWxsGAMgASgJUgVzaGVsbBISCgRjb2xzGAQgASgFUgRjb2xzEhIKBHJv'
    'd3MYBSABKAVSBHJvd3MSGQoIZW52X2pzb24YBiABKAlSB2Vudkpzb24=');

@$core.Deprecated('Use terminalInputDescriptor instead')
const TerminalInput$json = {
  '1': 'TerminalInput',
  '2': [
    {'1': 'data', '3': 1, '4': 1, '5': 12, '10': 'data'},
  ],
};

/// Descriptor for `TerminalInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalInputDescriptor =
    $convert.base64Decode('Cg1UZXJtaW5hbElucHV0EhIKBGRhdGEYASABKAxSBGRhdGE=');

@$core.Deprecated('Use terminalResizeDescriptor instead')
const TerminalResize$json = {
  '1': 'TerminalResize',
  '2': [
    {'1': 'cols', '3': 1, '4': 1, '5': 5, '10': 'cols'},
    {'1': 'rows', '3': 2, '4': 1, '5': 5, '10': 'rows'},
  ],
};

/// Descriptor for `TerminalResize`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalResizeDescriptor = $convert.base64Decode(
    'Cg5UZXJtaW5hbFJlc2l6ZRISCgRjb2xzGAEgASgFUgRjb2xzEhIKBHJvd3MYAiABKAVSBHJvd3'
    'M=');

@$core.Deprecated('Use terminalServerMessageDescriptor instead')
const TerminalServerMessage$json = {
  '1': 'TerminalServerMessage',
  '2': [
    {
      '1': 'opened',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TerminalOpened',
      '9': 0,
      '10': 'opened'
    },
    {
      '1': 'output',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TerminalOutput',
      '9': 0,
      '10': 'output'
    },
    {
      '1': 'exit',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TerminalExit',
      '9': 0,
      '10': 'exit'
    },
    {
      '1': 'error',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TerminalError',
      '9': 0,
      '10': 'error'
    },
    {
      '1': 'resumed',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.Resumed',
      '9': 0,
      '10': 'resumed'
    },
  ],
  '8': [
    {'1': 'msg'},
  ],
};

/// Descriptor for `TerminalServerMessage`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalServerMessageDescriptor = $convert.base64Decode(
    'ChVUZXJtaW5hbFNlcnZlck1lc3NhZ2USOAoGb3BlbmVkGAEgASgLMh4ucG9ja2V0YWdlbnQudj'
    'EuVGVybWluYWxPcGVuZWRIAFIGb3BlbmVkEjgKBm91dHB1dBgCIAEoCzIeLnBvY2tldGFnZW50'
    'LnYxLlRlcm1pbmFsT3V0cHV0SABSBm91dHB1dBIyCgRleGl0GAMgASgLMhwucG9ja2V0YWdlbn'
    'QudjEuVGVybWluYWxFeGl0SABSBGV4aXQSNQoFZXJyb3IYBCABKAsyHS5wb2NrZXRhZ2VudC52'
    'MS5UZXJtaW5hbEVycm9ySABSBWVycm9yEjMKB3Jlc3VtZWQYBSABKAsyFy5wb2NrZXRhZ2VudC'
    '52MS5SZXN1bWVkSABSB3Jlc3VtZWRCBQoDbXNn');

@$core.Deprecated('Use terminalOpenedDescriptor instead')
const TerminalOpened$json = {
  '1': 'TerminalOpened',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'shell', '3': 2, '4': 1, '5': 9, '10': 'shell'},
    {'1': 'cwd', '3': 3, '4': 1, '5': 9, '10': 'cwd'},
  ],
};

/// Descriptor for `TerminalOpened`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalOpenedDescriptor = $convert.base64Decode(
    'Cg5UZXJtaW5hbE9wZW5lZBIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW9uSWQSFAoFc2hlbG'
    'wYAiABKAlSBXNoZWxsEhAKA2N3ZBgDIAEoCVIDY3dk');

@$core.Deprecated('Use terminalOutputDescriptor instead')
const TerminalOutput$json = {
  '1': 'TerminalOutput',
  '2': [
    {'1': 'data', '3': 1, '4': 1, '5': 12, '10': 'data'},
  ],
};

/// Descriptor for `TerminalOutput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalOutputDescriptor =
    $convert.base64Decode('Cg5UZXJtaW5hbE91dHB1dBISCgRkYXRhGAEgASgMUgRkYXRh');

@$core.Deprecated('Use terminalExitDescriptor instead')
const TerminalExit$json = {
  '1': 'TerminalExit',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'code', '3': 2, '4': 1, '5': 5, '10': 'code'},
  ],
};

/// Descriptor for `TerminalExit`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalExitDescriptor = $convert.base64Decode(
    'CgxUZXJtaW5hbEV4aXQSHQoKc2Vzc2lvbl9pZBgBIAEoCVIJc2Vzc2lvbklkEhIKBGNvZGUYAi'
    'ABKAVSBGNvZGU=');

@$core.Deprecated('Use terminalErrorDescriptor instead')
const TerminalError$json = {
  '1': 'TerminalError',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'message', '3': 2, '4': 1, '5': 9, '10': 'message'},
  ],
};

/// Descriptor for `TerminalError`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalErrorDescriptor = $convert.base64Decode(
    'Cg1UZXJtaW5hbEVycm9yEh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb25JZBIYCgdtZXNzYW'
    'dlGAIgASgJUgdtZXNzYWdl');

@$core.Deprecated('Use resumedDescriptor instead')
const Resumed$json = {
  '1': 'Resumed',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'attached_ms', '3': 2, '4': 1, '5': 3, '10': 'attachedMs'},
  ],
};

/// Descriptor for `Resumed`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resumedDescriptor = $convert.base64Decode(
    'CgdSZXN1bWVkEh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb25JZBIfCgthdHRhY2hlZF9tcx'
    'gCIAEoA1IKYXR0YWNoZWRNcw==');

@$core.Deprecated('Use listTerminalsRequestDescriptor instead')
const ListTerminalsRequest$json = {
  '1': 'ListTerminalsRequest',
};

/// Descriptor for `ListTerminalsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTerminalsRequestDescriptor =
    $convert.base64Decode('ChRMaXN0VGVybWluYWxzUmVxdWVzdA==');

@$core.Deprecated('Use listTerminalsResponseDescriptor instead')
const ListTerminalsResponse$json = {
  '1': 'ListTerminalsResponse',
  '2': [
    {
      '1': 'sessions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.TerminalSessionInfo',
      '10': 'sessions'
    },
  ],
};

/// Descriptor for `ListTerminalsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTerminalsResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0VGVybWluYWxzUmVzcG9uc2USPwoIc2Vzc2lvbnMYASADKAsyIy5wb2NrZXRhZ2VudC'
    '52MS5UZXJtaW5hbFNlc3Npb25JbmZvUghzZXNzaW9ucw==');

@$core.Deprecated('Use terminalSessionInfoDescriptor instead')
const TerminalSessionInfo$json = {
  '1': 'TerminalSessionInfo',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'shell', '3': 2, '4': 1, '5': 9, '10': 'shell'},
    {'1': 'cwd', '3': 3, '4': 1, '5': 9, '10': 'cwd'},
    {'1': 'opened_at_unix_ms', '3': 4, '4': 1, '5': 3, '10': 'openedAtUnixMs'},
    {'1': 'alive', '3': 5, '4': 1, '5': 8, '10': 'alive'},
  ],
};

/// Descriptor for `TerminalSessionInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List terminalSessionInfoDescriptor = $convert.base64Decode(
    'ChNUZXJtaW5hbFNlc3Npb25JbmZvEh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb25JZBIUCg'
    'VzaGVsbBgCIAEoCVIFc2hlbGwSEAoDY3dkGAMgASgJUgNjd2QSKQoRb3BlbmVkX2F0X3VuaXhf'
    'bXMYBCABKANSDm9wZW5lZEF0VW5peE1zEhQKBWFsaXZlGAUgASgIUgVhbGl2ZQ==');

@$core.Deprecated('Use closeTerminalRequestDescriptor instead')
const CloseTerminalRequest$json = {
  '1': 'CloseTerminalRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `CloseTerminalRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List closeTerminalRequestDescriptor = $convert.base64Decode(
    'ChRDbG9zZVRlcm1pbmFsUmVxdWVzdBIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW9uSWQ=');
