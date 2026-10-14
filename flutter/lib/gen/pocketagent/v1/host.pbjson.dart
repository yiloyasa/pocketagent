// This is a generated file - do not edit.
//
// Generated from v1/host.proto.

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

@$core.Deprecated('Use getHostInfoRequestDescriptor instead')
const GetHostInfoRequest$json = {
  '1': 'GetHostInfoRequest',
};

/// Descriptor for `GetHostInfoRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getHostInfoRequestDescriptor =
    $convert.base64Decode('ChJHZXRIb3N0SW5mb1JlcXVlc3Q=');

@$core.Deprecated('Use hostInfoDescriptor instead')
const HostInfo$json = {
  '1': 'HostInfo',
  '2': [
    {'1': 'host_id', '3': 1, '4': 1, '5': 9, '10': 'hostId'},
    {'1': 'display_name', '3': 2, '4': 1, '5': 9, '10': 'displayName'},
    {'1': 'os', '3': 3, '4': 1, '5': 9, '10': 'os'},
    {'1': 'arch', '3': 4, '4': 1, '5': 9, '10': 'arch'},
    {'1': 'daemon_version', '3': 5, '4': 1, '5': 9, '10': 'daemonVersion'},
    {'1': 'workspaces', '3': 6, '4': 3, '5': 9, '10': 'workspaces'},
    {'1': 'capabilities', '3': 7, '4': 3, '5': 9, '10': 'capabilities'},
    {'1': 'auth_required', '3': 8, '4': 1, '5': 8, '10': 'authRequired'},
    {'1': 'default_shell', '3': 9, '4': 1, '5': 9, '10': 'defaultShell'},
    {'1': 'tailscale_ipv4', '3': 10, '4': 1, '5': 9, '10': 'tailscaleIpv4'},
    {'1': 'lan_ipv4', '3': 11, '4': 1, '5': 9, '10': 'lanIpv4'},
    {
      '1': 'recovery',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.HostRecoveryReport',
      '10': 'recovery'
    },
    {'1': 'min_app_version', '3': 13, '4': 1, '5': 9, '10': 'minAppVersion'},
  ],
};

/// Descriptor for `HostInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostInfoDescriptor = $convert.base64Decode(
    'CghIb3N0SW5mbxIXCgdob3N0X2lkGAEgASgJUgZob3N0SWQSIQoMZGlzcGxheV9uYW1lGAIgAS'
    'gJUgtkaXNwbGF5TmFtZRIOCgJvcxgDIAEoCVICb3MSEgoEYXJjaBgEIAEoCVIEYXJjaBIlCg5k'
    'YWVtb25fdmVyc2lvbhgFIAEoCVINZGFlbW9uVmVyc2lvbhIeCgp3b3Jrc3BhY2VzGAYgAygJUg'
    'p3b3Jrc3BhY2VzEiIKDGNhcGFiaWxpdGllcxgHIAMoCVIMY2FwYWJpbGl0aWVzEiMKDWF1dGhf'
    'cmVxdWlyZWQYCCABKAhSDGF1dGhSZXF1aXJlZBIjCg1kZWZhdWx0X3NoZWxsGAkgASgJUgxkZW'
    'ZhdWx0U2hlbGwSJQoOdGFpbHNjYWxlX2lwdjQYCiABKAlSDXRhaWxzY2FsZUlwdjQSGQoIbGFu'
    'X2lwdjQYCyABKAlSB2xhbklwdjQSPgoIcmVjb3ZlcnkYDCABKAsyIi5wb2NrZXRhZ2VudC52MS'
    '5Ib3N0UmVjb3ZlcnlSZXBvcnRSCHJlY292ZXJ5EiYKD21pbl9hcHBfdmVyc2lvbhgNIAEoCVIN'
    'bWluQXBwVmVyc2lvbg==');

@$core.Deprecated('Use authorizeRequestDescriptor instead')
const AuthorizeRequest$json = {
  '1': 'AuthorizeRequest',
  '2': [
    {'1': 'pair_code', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'pairCode'},
    {'1': 'token', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'token'},
    {
      '1': 'challenge_request',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'challengeRequest'
    },
    {
      '1': 'challenge_answer',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.ChallengeAnswer',
      '9': 0,
      '10': 'challengeAnswer'
    },
    {'1': 'device_name', '3': 3, '4': 1, '5': 9, '10': 'deviceName'},
    {'1': 'device_id', '3': 4, '4': 1, '5': 9, '10': 'deviceId'},
    {'1': 'device_pub_key', '3': 5, '4': 1, '5': 9, '10': 'devicePubKey'},
  ],
  '8': [
    {'1': 'method'},
  ],
};

/// Descriptor for `AuthorizeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authorizeRequestDescriptor = $convert.base64Decode(
    'ChBBdXRob3JpemVSZXF1ZXN0Eh0KCXBhaXJfY29kZRgBIAEoCUgAUghwYWlyQ29kZRIWCgV0b2'
    'tlbhgCIAEoCUgAUgV0b2tlbhItChFjaGFsbGVuZ2VfcmVxdWVzdBgGIAEoCUgAUhBjaGFsbGVu'
    'Z2VSZXF1ZXN0EkwKEGNoYWxsZW5nZV9hbnN3ZXIYByABKAsyHy5wb2NrZXRhZ2VudC52MS5DaG'
    'FsbGVuZ2VBbnN3ZXJIAFIPY2hhbGxlbmdlQW5zd2VyEh8KC2RldmljZV9uYW1lGAMgASgJUgpk'
    'ZXZpY2VOYW1lEhsKCWRldmljZV9pZBgEIAEoCVIIZGV2aWNlSWQSJAoOZGV2aWNlX3B1Yl9rZX'
    'kYBSABKAlSDGRldmljZVB1YktleUIICgZtZXRob2Q=');

@$core.Deprecated('Use challengeAnswerDescriptor instead')
const ChallengeAnswer$json = {
  '1': 'ChallengeAnswer',
  '2': [
    {'1': 'challenge', '3': 1, '4': 1, '5': 9, '10': 'challenge'},
    {'1': 'signature', '3': 2, '4': 1, '5': 9, '10': 'signature'},
  ],
};

/// Descriptor for `ChallengeAnswer`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List challengeAnswerDescriptor = $convert.base64Decode(
    'Cg9DaGFsbGVuZ2VBbnN3ZXISHAoJY2hhbGxlbmdlGAEgASgJUgljaGFsbGVuZ2USHAoJc2lnbm'
    'F0dXJlGAIgASgJUglzaWduYXR1cmU=');

@$core.Deprecated('Use authorizeResponseDescriptor instead')
const AuthorizeResponse$json = {
  '1': 'AuthorizeResponse',
  '2': [
    {'1': 'token', '3': 1, '4': 1, '5': 9, '10': 'token'},
    {'1': 'expires_at_unix', '3': 2, '4': 1, '5': 3, '10': 'expiresAtUnix'},
    {'1': 'challenge', '3': 3, '4': 1, '5': 9, '10': 'challenge'},
    {'1': 'device_id', '3': 4, '4': 1, '5': 9, '10': 'deviceId'},
  ],
};

/// Descriptor for `AuthorizeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authorizeResponseDescriptor = $convert.base64Decode(
    'ChFBdXRob3JpemVSZXNwb25zZRIUCgV0b2tlbhgBIAEoCVIFdG9rZW4SJgoPZXhwaXJlc19hdF'
    '91bml4GAIgASgDUg1leHBpcmVzQXRVbml4EhwKCWNoYWxsZW5nZRgDIAEoCVIJY2hhbGxlbmdl'
    'EhsKCWRldmljZV9pZBgEIAEoCVIIZGV2aWNlSWQ=');

@$core.Deprecated('Use pingRequestDescriptor instead')
const PingRequest$json = {
  '1': 'PingRequest',
  '2': [
    {
      '1': 'client_time_unix_ms',
      '3': 1,
      '4': 1,
      '5': 3,
      '10': 'clientTimeUnixMs'
    },
  ],
};

/// Descriptor for `PingRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pingRequestDescriptor = $convert.base64Decode(
    'CgtQaW5nUmVxdWVzdBItChNjbGllbnRfdGltZV91bml4X21zGAEgASgDUhBjbGllbnRUaW1lVW'
    '5peE1z');

@$core.Deprecated('Use pingResponseDescriptor instead')
const PingResponse$json = {
  '1': 'PingResponse',
  '2': [
    {
      '1': 'server_time_unix_ms',
      '3': 1,
      '4': 1,
      '5': 3,
      '10': 'serverTimeUnixMs'
    },
    {'1': 'status', '3': 2, '4': 1, '5': 9, '10': 'status'},
  ],
};

/// Descriptor for `PingResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pingResponseDescriptor = $convert.base64Decode(
    'CgxQaW5nUmVzcG9uc2USLQoTc2VydmVyX3RpbWVfdW5peF9tcxgBIAEoA1IQc2VydmVyVGltZV'
    'VuaXhNcxIWCgZzdGF0dXMYAiABKAlSBnN0YXR1cw==');

@$core.Deprecated('Use watchEventsRequestDescriptor instead')
const WatchEventsRequest$json = {
  '1': 'WatchEventsRequest',
  '2': [
    {'1': 'after_sequence', '3': 1, '4': 1, '5': 3, '10': 'afterSequence'},
  ],
};

/// Descriptor for `WatchEventsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List watchEventsRequestDescriptor = $convert.base64Decode(
    'ChJXYXRjaEV2ZW50c1JlcXVlc3QSJQoOYWZ0ZXJfc2VxdWVuY2UYASABKANSDWFmdGVyU2VxdW'
    'VuY2U=');

@$core.Deprecated('Use hostEventDescriptor instead')
const HostEvent$json = {
  '1': 'HostEvent',
  '2': [
    {'1': 'timestamp_unix_ms', '3': 1, '4': 1, '5': 3, '10': 'timestampUnixMs'},
    {
      '1': 'turn_finished',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.TurnFinishedNotification',
      '9': 0,
      '10': 'turnFinished'
    },
    {
      '1': 'selection_changed',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.SelectionChangedNotification',
      '9': 0,
      '10': 'selectionChanged'
    },
    {'1': 'sequence', '3': 6, '4': 1, '5': 3, '10': 'sequence'},
  ],
  '8': [
    {'1': 'event'},
  ],
  '9': [
    {'1': 2, '2': 3},
    {'1': 3, '2': 4},
    {'1': 4, '2': 5},
  ],
  '10': ['workspace_changed', 'model_ready', 'update_available'],
};

/// Descriptor for `HostEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostEventDescriptor = $convert.base64Decode(
    'CglIb3N0RXZlbnQSKgoRdGltZXN0YW1wX3VuaXhfbXMYASABKANSD3RpbWVzdGFtcFVuaXhNcx'
    'JPCg10dXJuX2ZpbmlzaGVkGAUgASgLMigucG9ja2V0YWdlbnQudjEuVHVybkZpbmlzaGVkTm90'
    'aWZpY2F0aW9uSABSDHR1cm5GaW5pc2hlZBJbChFzZWxlY3Rpb25fY2hhbmdlZBgHIAEoCzIsLn'
    'BvY2tldGFnZW50LnYxLlNlbGVjdGlvbkNoYW5nZWROb3RpZmljYXRpb25IAFIQc2VsZWN0aW9u'
    'Q2hhbmdlZBIaCghzZXF1ZW5jZRgGIAEoA1IIc2VxdWVuY2VCBwoFZXZlbnRKBAgCEANKBAgDEA'
    'RKBAgEEAVSEXdvcmtzcGFjZV9jaGFuZ2VkUgttb2RlbF9yZWFkeVIQdXBkYXRlX2F2YWlsYWJs'
    'ZQ==');

@$core.Deprecated('Use turnFinishedNotificationDescriptor instead')
const TurnFinishedNotification$json = {
  '1': 'TurnFinishedNotification',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'summary', '3': 2, '4': 1, '5': 9, '10': 'summary'},
  ],
};

/// Descriptor for `TurnFinishedNotification`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List turnFinishedNotificationDescriptor =
    $convert.base64Decode(
        'ChhUdXJuRmluaXNoZWROb3RpZmljYXRpb24SGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZB'
        'IYCgdzdW1tYXJ5GAIgASgJUgdzdW1tYXJ5');

@$core.Deprecated('Use selectionChangedNotificationDescriptor instead')
const SelectionChangedNotification$json = {
  '1': 'SelectionChangedNotification',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {
      '1': 'selection',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.EffectiveModelSelection',
      '10': 'selection'
    },
  ],
};

/// Descriptor for `SelectionChangedNotification`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List selectionChangedNotificationDescriptor =
    $convert.base64Decode(
        'ChxTZWxlY3Rpb25DaGFuZ2VkTm90aWZpY2F0aW9uEhsKCXRocmVhZF9pZBgBIAEoCVIIdGhyZW'
        'FkSWQSRQoJc2VsZWN0aW9uGAIgASgLMicucG9ja2V0YWdlbnQudjEuRWZmZWN0aXZlTW9kZWxT'
        'ZWxlY3Rpb25SCXNlbGVjdGlvbg==');

@$core.Deprecated('Use listDevServersRequestDescriptor instead')
const ListDevServersRequest$json = {
  '1': 'ListDevServersRequest',
};

/// Descriptor for `ListDevServersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listDevServersRequestDescriptor =
    $convert.base64Decode('ChVMaXN0RGV2U2VydmVyc1JlcXVlc3Q=');

@$core.Deprecated('Use devServerDescriptor instead')
const DevServer$json = {
  '1': 'DevServer',
  '2': [
    {'1': 'port', '3': 1, '4': 1, '5': 5, '10': 'port'},
    {'1': 'url', '3': 2, '4': 1, '5': 9, '10': 'url'},
    {'1': 'title', '3': 3, '4': 1, '5': 9, '10': 'title'},
    {'1': 'server_header', '3': 4, '4': 1, '5': 9, '10': 'serverHeader'},
  ],
};

/// Descriptor for `DevServer`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List devServerDescriptor = $convert.base64Decode(
    'CglEZXZTZXJ2ZXISEgoEcG9ydBgBIAEoBVIEcG9ydBIQCgN1cmwYAiABKAlSA3VybBIUCgV0aX'
    'RsZRgDIAEoCVIFdGl0bGUSIwoNc2VydmVyX2hlYWRlchgEIAEoCVIMc2VydmVySGVhZGVy');

@$core.Deprecated('Use devServerListDescriptor instead')
const DevServerList$json = {
  '1': 'DevServerList',
  '2': [
    {
      '1': 'servers',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.DevServer',
      '10': 'servers'
    },
    {
      '1': 'scanned_at_unix_ms',
      '3': 2,
      '4': 1,
      '5': 3,
      '10': 'scannedAtUnixMs'
    },
  ],
};

/// Descriptor for `DevServerList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List devServerListDescriptor = $convert.base64Decode(
    'Cg1EZXZTZXJ2ZXJMaXN0EjMKB3NlcnZlcnMYASADKAsyGS5wb2NrZXRhZ2VudC52MS5EZXZTZX'
    'J2ZXJSB3NlcnZlcnMSKwoSc2Nhbm5lZF9hdF91bml4X21zGAIgASgDUg9zY2FubmVkQXRVbml4'
    'TXM=');

@$core.Deprecated('Use hostRecoveryReportDescriptor instead')
const HostRecoveryReport$json = {
  '1': 'HostRecoveryReport',
  '2': [
    {'1': 'status', '3': 1, '4': 1, '5': 9, '10': 'status'},
    {'1': 'summary', '3': 2, '4': 1, '5': 9, '10': 'summary'},
    {
      '1': 'store',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.HostStoreHealth',
      '10': 'store'
    },
    {
      '1': 'turn_journal',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.HostTurnJournalHealth',
      '10': 'turnJournal'
    },
    {
      '1': 'devices',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.HostDeviceHealth',
      '10': 'devices'
    },
    {
      '1': 'identity',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.pocketagent.v1.HostIdentityHealth',
      '10': 'identity'
    },
  ],
};

/// Descriptor for `HostRecoveryReport`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostRecoveryReportDescriptor = $convert.base64Decode(
    'ChJIb3N0UmVjb3ZlcnlSZXBvcnQSFgoGc3RhdHVzGAEgASgJUgZzdGF0dXMSGAoHc3VtbWFyeR'
    'gCIAEoCVIHc3VtbWFyeRI1CgVzdG9yZRgDIAEoCzIfLnBvY2tldGFnZW50LnYxLkhvc3RTdG9y'
    'ZUhlYWx0aFIFc3RvcmUSSAoMdHVybl9qb3VybmFsGAQgASgLMiUucG9ja2V0YWdlbnQudjEuSG'
    '9zdFR1cm5Kb3VybmFsSGVhbHRoUgt0dXJuSm91cm5hbBI6CgdkZXZpY2VzGAUgASgLMiAucG9j'
    'a2V0YWdlbnQudjEuSG9zdERldmljZUhlYWx0aFIHZGV2aWNlcxI+CghpZGVudGl0eRgGIAEoCz'
    'IiLnBvY2tldGFnZW50LnYxLkhvc3RJZGVudGl0eUhlYWx0aFIIaWRlbnRpdHk=');

@$core.Deprecated('Use hostStoreHealthDescriptor instead')
const HostStoreHealth$json = {
  '1': 'HostStoreHealth',
  '2': [
    {'1': 'outcome', '3': 1, '4': 1, '5': 9, '10': 'outcome'},
    {'1': 'has_backup', '3': 2, '4': 1, '5': 8, '10': 'hasBackup'},
    {'1': 'quarantine_path', '3': 3, '4': 1, '5': 9, '10': 'quarantinePath'},
  ],
};

/// Descriptor for `HostStoreHealth`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostStoreHealthDescriptor = $convert.base64Decode(
    'Cg9Ib3N0U3RvcmVIZWFsdGgSGAoHb3V0Y29tZRgBIAEoCVIHb3V0Y29tZRIdCgpoYXNfYmFja3'
    'VwGAIgASgIUgloYXNCYWNrdXASJwoPcXVhcmFudGluZV9wYXRoGAMgASgJUg5xdWFyYW50aW5l'
    'UGF0aA==');

@$core.Deprecated('Use hostTurnJournalHealthDescriptor instead')
const HostTurnJournalHealth$json = {
  '1': 'HostTurnJournalHealth',
  '2': [
    {'1': 'pending_count', '3': 1, '4': 1, '5': 5, '10': 'pendingCount'},
    {'1': 'thread_ids', '3': 2, '4': 3, '5': 9, '10': 'threadIds'},
    {
      '1': 'lost_inputs',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.HostLostQueueInput',
      '10': 'lostInputs'
    },
  ],
};

/// Descriptor for `HostTurnJournalHealth`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostTurnJournalHealthDescriptor = $convert.base64Decode(
    'ChVIb3N0VHVybkpvdXJuYWxIZWFsdGgSIwoNcGVuZGluZ19jb3VudBgBIAEoBVIMcGVuZGluZ0'
    'NvdW50Eh0KCnRocmVhZF9pZHMYAiADKAlSCXRocmVhZElkcxJDCgtsb3N0X2lucHV0cxgDIAMo'
    'CzIiLnBvY2tldGFnZW50LnYxLkhvc3RMb3N0UXVldWVJbnB1dFIKbG9zdElucHV0cw==');

@$core.Deprecated('Use hostLostQueueInputDescriptor instead')
const HostLostQueueInput$json = {
  '1': 'HostLostQueueInput',
  '2': [
    {'1': 'thread_id', '3': 1, '4': 1, '5': 9, '10': 'threadId'},
    {'1': 'preview', '3': 2, '4': 1, '5': 9, '10': 'preview'},
    {
      '1': 'created_at_unix_ms',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'createdAtUnixMs'
    },
  ],
};

/// Descriptor for `HostLostQueueInput`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostLostQueueInputDescriptor = $convert.base64Decode(
    'ChJIb3N0TG9zdFF1ZXVlSW5wdXQSGwoJdGhyZWFkX2lkGAEgASgJUgh0aHJlYWRJZBIYCgdwcm'
    'V2aWV3GAIgASgJUgdwcmV2aWV3EisKEmNyZWF0ZWRfYXRfdW5peF9tcxgDIAEoA1IPY3JlYXRl'
    'ZEF0VW5peE1z');

@$core.Deprecated('Use hostDeviceHealthDescriptor instead')
const HostDeviceHealth$json = {
  '1': 'HostDeviceHealth',
  '2': [
    {'1': 'degraded', '3': 1, '4': 1, '5': 8, '10': 'degraded'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `HostDeviceHealth`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostDeviceHealthDescriptor = $convert.base64Decode(
    'ChBIb3N0RGV2aWNlSGVhbHRoEhoKCGRlZ3JhZGVkGAEgASgIUghkZWdyYWRlZBIWCgZyZWFzb2'
    '4YAiABKAlSBnJlYXNvbg==');

@$core.Deprecated('Use hostIdentityHealthDescriptor instead')
const HostIdentityHealth$json = {
  '1': 'HostIdentityHealth',
  '2': [
    {'1': 'degraded', '3': 1, '4': 1, '5': 8, '10': 'degraded'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `HostIdentityHealth`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List hostIdentityHealthDescriptor = $convert.base64Decode(
    'ChJIb3N0SWRlbnRpdHlIZWFsdGgSGgoIZGVncmFkZWQYASABKAhSCGRlZ3JhZGVkEhYKBnJlYX'
    'NvbhgCIAEoCVIGcmVhc29u');

@$core.Deprecated('Use getPairingInfoRequestDescriptor instead')
const GetPairingInfoRequest$json = {
  '1': 'GetPairingInfoRequest',
};

/// Descriptor for `GetPairingInfoRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPairingInfoRequestDescriptor =
    $convert.base64Decode('ChVHZXRQYWlyaW5nSW5mb1JlcXVlc3Q=');

@$core.Deprecated('Use getPairingInfoResponseDescriptor instead')
const GetPairingInfoResponse$json = {
  '1': 'GetPairingInfoResponse',
  '2': [
    {'1': 'pairing_enabled', '3': 1, '4': 1, '5': 8, '10': 'pairingEnabled'},
    {'1': 'pair_code', '3': 2, '4': 1, '5': 9, '10': 'pairCode'},
    {'1': 'daemon_version', '3': 3, '4': 1, '5': 9, '10': 'daemonVersion'},
  ],
};

/// Descriptor for `GetPairingInfoResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPairingInfoResponseDescriptor = $convert.base64Decode(
    'ChZHZXRQYWlyaW5nSW5mb1Jlc3BvbnNlEicKD3BhaXJpbmdfZW5hYmxlZBgBIAEoCFIOcGFpcm'
    'luZ0VuYWJsZWQSGwoJcGFpcl9jb2RlGAIgASgJUghwYWlyQ29kZRIlCg5kYWVtb25fdmVyc2lv'
    'bhgDIAEoCVINZGFlbW9uVmVyc2lvbg==');
