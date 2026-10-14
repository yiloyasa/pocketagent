// This is a generated file - do not edit.
//
// Generated from v1/fs.proto.

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

@$core.Deprecated('Use listRootsRequestDescriptor instead')
const ListRootsRequest$json = {
  '1': 'ListRootsRequest',
};

/// Descriptor for `ListRootsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRootsRequestDescriptor =
    $convert.base64Decode('ChBMaXN0Um9vdHNSZXF1ZXN0');

@$core.Deprecated('Use listRootsResponseDescriptor instead')
const ListRootsResponse$json = {
  '1': 'ListRootsResponse',
  '2': [
    {
      '1': 'roots',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.DirectoryEntry',
      '10': 'roots'
    },
  ],
};

/// Descriptor for `ListRootsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRootsResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0Um9vdHNSZXNwb25zZRI0CgVyb290cxgBIAMoCzIeLnBvY2tldGFnZW50LnYxLkRpcm'
    'VjdG9yeUVudHJ5UgVyb290cw==');

@$core.Deprecated('Use listDirectoryRequestDescriptor instead')
const ListDirectoryRequest$json = {
  '1': 'ListDirectoryRequest',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
    {'1': 'include_hidden', '3': 2, '4': 1, '5': 8, '10': 'includeHidden'},
  ],
};

/// Descriptor for `ListDirectoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listDirectoryRequestDescriptor = $convert.base64Decode(
    'ChRMaXN0RGlyZWN0b3J5UmVxdWVzdBISCgRwYXRoGAEgASgJUgRwYXRoEiUKDmluY2x1ZGVfaG'
    'lkZGVuGAIgASgIUg1pbmNsdWRlSGlkZGVu');

@$core.Deprecated('Use listDirectoryResponseDescriptor instead')
const ListDirectoryResponse$json = {
  '1': 'ListDirectoryResponse',
  '2': [
    {
      '1': 'entries',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.pocketagent.v1.DirectoryEntry',
      '10': 'entries'
    },
  ],
};

/// Descriptor for `ListDirectoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listDirectoryResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0RGlyZWN0b3J5UmVzcG9uc2USOAoHZW50cmllcxgBIAMoCzIeLnBvY2tldGFnZW50Ln'
    'YxLkRpcmVjdG9yeUVudHJ5UgdlbnRyaWVz');

@$core.Deprecated('Use directoryEntryDescriptor instead')
const DirectoryEntry$json = {
  '1': 'DirectoryEntry',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'path', '3': 2, '4': 1, '5': 9, '10': 'path'},
    {'1': 'is_dir', '3': 3, '4': 1, '5': 8, '10': 'isDir'},
    {'1': 'size_bytes', '3': 4, '4': 1, '5': 3, '10': 'sizeBytes'},
    {'1': 'modified_unix_ms', '3': 5, '4': 1, '5': 3, '10': 'modifiedUnixMs'},
  ],
};

/// Descriptor for `DirectoryEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List directoryEntryDescriptor = $convert.base64Decode(
    'Cg5EaXJlY3RvcnlFbnRyeRISCgRuYW1lGAEgASgJUgRuYW1lEhIKBHBhdGgYAiABKAlSBHBhdG'
    'gSFQoGaXNfZGlyGAMgASgIUgVpc0RpchIdCgpzaXplX2J5dGVzGAQgASgDUglzaXplQnl0ZXMS'
    'KAoQbW9kaWZpZWRfdW5peF9tcxgFIAEoA1IObW9kaWZpZWRVbml4TXM=');

@$core.Deprecated('Use readFileRequestDescriptor instead')
const ReadFileRequest$json = {
  '1': 'ReadFileRequest',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
    {'1': 'max_bytes', '3': 2, '4': 1, '5': 3, '10': 'maxBytes'},
  ],
};

/// Descriptor for `ReadFileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readFileRequestDescriptor = $convert.base64Decode(
    'Cg9SZWFkRmlsZVJlcXVlc3QSEgoEcGF0aBgBIAEoCVIEcGF0aBIbCgltYXhfYnl0ZXMYAiABKA'
    'NSCG1heEJ5dGVz');

@$core.Deprecated('Use fileChunkDescriptor instead')
const FileChunk$json = {
  '1': 'FileChunk',
  '2': [
    {'1': 'data', '3': 1, '4': 1, '5': 12, '10': 'data'},
    {'1': 'total_size', '3': 2, '4': 1, '5': 3, '10': 'totalSize'},
    {'1': 'text', '3': 3, '4': 1, '5': 8, '10': 'text'},
    {'1': 'modified_unix_ms', '3': 4, '4': 1, '5': 3, '10': 'modifiedUnixMs'},
  ],
};

/// Descriptor for `FileChunk`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileChunkDescriptor = $convert.base64Decode(
    'CglGaWxlQ2h1bmsSEgoEZGF0YRgBIAEoDFIEZGF0YRIdCgp0b3RhbF9zaXplGAIgASgDUgl0b3'
    'RhbFNpemUSEgoEdGV4dBgDIAEoCFIEdGV4dBIoChBtb2RpZmllZF91bml4X21zGAQgASgDUg5t'
    'b2RpZmllZFVuaXhNcw==');

@$core.Deprecated('Use writeFileRequestDescriptor instead')
const WriteFileRequest$json = {
  '1': 'WriteFileRequest',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
    {'1': 'data', '3': 2, '4': 1, '5': 12, '10': 'data'},
    {'1': 'create', '3': 3, '4': 1, '5': 8, '10': 'create'},
    {'1': 'truncate', '3': 4, '4': 1, '5': 8, '10': 'truncate'},
    {
      '1': 'expected_modified_at_ms',
      '3': 5,
      '4': 1,
      '5': 3,
      '10': 'expectedModifiedAtMs'
    },
  ],
};

/// Descriptor for `WriteFileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List writeFileRequestDescriptor = $convert.base64Decode(
    'ChBXcml0ZUZpbGVSZXF1ZXN0EhIKBHBhdGgYASABKAlSBHBhdGgSEgoEZGF0YRgCIAEoDFIEZG'
    'F0YRIWCgZjcmVhdGUYAyABKAhSBmNyZWF0ZRIaCgh0cnVuY2F0ZRgEIAEoCFIIdHJ1bmNhdGUS'
    'NQoXZXhwZWN0ZWRfbW9kaWZpZWRfYXRfbXMYBSABKANSFGV4cGVjdGVkTW9kaWZpZWRBdE1z');

@$core.Deprecated('Use writeFileResponseDescriptor instead')
const WriteFileResponse$json = {
  '1': 'WriteFileResponse',
  '2': [
    {
      '1': 'new_modified_at_ms',
      '3': 1,
      '4': 1,
      '5': 3,
      '10': 'newModifiedAtMs'
    },
  ],
};

/// Descriptor for `WriteFileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List writeFileResponseDescriptor = $convert.base64Decode(
    'ChFXcml0ZUZpbGVSZXNwb25zZRIrChJuZXdfbW9kaWZpZWRfYXRfbXMYASABKANSD25ld01vZG'
    'lmaWVkQXRNcw==');

@$core.Deprecated('Use readFileDiffRequestDescriptor instead')
const ReadFileDiffRequest$json = {
  '1': 'ReadFileDiffRequest',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
  ],
};

/// Descriptor for `ReadFileDiffRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readFileDiffRequestDescriptor = $convert
    .base64Decode('ChNSZWFkRmlsZURpZmZSZXF1ZXN0EhIKBHBhdGgYASABKAlSBHBhdGg=');

@$core.Deprecated('Use fileDiffDescriptor instead')
const FileDiff$json = {
  '1': 'FileDiff',
  '2': [
    {'1': 'unified_diff', '3': 1, '4': 1, '5': 9, '10': 'unifiedDiff'},
    {'1': 'has_changes', '3': 2, '4': 1, '5': 8, '10': 'hasChanges'},
  ],
};

/// Descriptor for `FileDiff`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileDiffDescriptor = $convert.base64Decode(
    'CghGaWxlRGlmZhIhCgx1bmlmaWVkX2RpZmYYASABKAlSC3VuaWZpZWREaWZmEh8KC2hhc19jaG'
    'FuZ2VzGAIgASgIUgpoYXNDaGFuZ2Vz');

@$core.Deprecated('Use createDirectoryRequestDescriptor instead')
const CreateDirectoryRequest$json = {
  '1': 'CreateDirectoryRequest',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
  ],
};

/// Descriptor for `CreateDirectoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createDirectoryRequestDescriptor =
    $convert.base64Decode(
        'ChZDcmVhdGVEaXJlY3RvcnlSZXF1ZXN0EhIKBHBhdGgYASABKAlSBHBhdGg=');

@$core.Deprecated('Use deleteEntryRequestDescriptor instead')
const DeleteEntryRequest$json = {
  '1': 'DeleteEntryRequest',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
    {'1': 'recursive', '3': 2, '4': 1, '5': 8, '10': 'recursive'},
  ],
};

/// Descriptor for `DeleteEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteEntryRequestDescriptor = $convert.base64Decode(
    'ChJEZWxldGVFbnRyeVJlcXVlc3QSEgoEcGF0aBgBIAEoCVIEcGF0aBIcCglyZWN1cnNpdmUYAi'
    'ABKAhSCXJlY3Vyc2l2ZQ==');

@$core.Deprecated('Use moveEntryRequestDescriptor instead')
const MoveEntryRequest$json = {
  '1': 'MoveEntryRequest',
  '2': [
    {'1': 'from_path', '3': 1, '4': 1, '5': 9, '10': 'fromPath'},
    {'1': 'to_path', '3': 2, '4': 1, '5': 9, '10': 'toPath'},
  ],
};

/// Descriptor for `MoveEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List moveEntryRequestDescriptor = $convert.base64Decode(
    'ChBNb3ZlRW50cnlSZXF1ZXN0EhsKCWZyb21fcGF0aBgBIAEoCVIIZnJvbVBhdGgSFwoHdG9fcG'
    'F0aBgCIAEoCVIGdG9QYXRo');

@$core.Deprecated('Use searchContentRequestDescriptor instead')
const SearchContentRequest$json = {
  '1': 'SearchContentRequest',
  '2': [
    {'1': 'root', '3': 1, '4': 1, '5': 9, '10': 'root'},
    {'1': 'query', '3': 2, '4': 1, '5': 9, '10': 'query'},
    {'1': 'regex', '3': 3, '4': 1, '5': 8, '10': 'regex'},
    {'1': 'glob', '3': 4, '4': 1, '5': 9, '10': 'glob'},
    {'1': 'max_results', '3': 5, '4': 1, '5': 5, '10': 'maxResults'},
  ],
};

/// Descriptor for `SearchContentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchContentRequestDescriptor = $convert.base64Decode(
    'ChRTZWFyY2hDb250ZW50UmVxdWVzdBISCgRyb290GAEgASgJUgRyb290EhQKBXF1ZXJ5GAIgAS'
    'gJUgVxdWVyeRIUCgVyZWdleBgDIAEoCFIFcmVnZXgSEgoEZ2xvYhgEIAEoCVIEZ2xvYhIfCgtt'
    'YXhfcmVzdWx0cxgFIAEoBVIKbWF4UmVzdWx0cw==');

@$core.Deprecated('Use searchMatchDescriptor instead')
const SearchMatch$json = {
  '1': 'SearchMatch',
  '2': [
    {'1': 'path', '3': 1, '4': 1, '5': 9, '10': 'path'},
    {'1': 'line_number', '3': 2, '4': 1, '5': 5, '10': 'lineNumber'},
    {'1': 'line_text', '3': 3, '4': 1, '5': 9, '10': 'lineText'},
  ],
};

/// Descriptor for `SearchMatch`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchMatchDescriptor = $convert.base64Decode(
    'CgtTZWFyY2hNYXRjaBISCgRwYXRoGAEgASgJUgRwYXRoEh8KC2xpbmVfbnVtYmVyGAIgASgFUg'
    'psaW5lTnVtYmVyEhsKCWxpbmVfdGV4dBgDIAEoCVIIbGluZVRleHQ=');
