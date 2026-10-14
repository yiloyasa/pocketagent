// This is a generated file - do not edit.
//
// Generated from v1/fs.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'common.pb.dart' as $1;
import 'fs.pb.dart' as $0;

export 'fs.pb.dart';

@$pb.GrpcServiceName('pocketagent.v1.FileSystemService')
class FileSystemServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  FileSystemServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.ListDirectoryResponse> listDirectory(
    $0.ListDirectoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listDirectory, request, options: options);
  }

  /// "此电脑":返回所有盘符/挂载点
  $grpc.ResponseFuture<$0.ListRootsResponse> listRoots(
    $0.ListRootsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listRoots, request, options: options);
  }

  /// 大文件分块流
  $grpc.ResponseStream<$0.FileChunk> readFile(
    $0.ReadFileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$readFile, $async.Stream.fromIterable([request]),
        options: options);
  }

  $grpc.ResponseFuture<$0.WriteFileResponse> writeFile(
    $0.WriteFileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$writeFile, request, options: options);
  }

  $grpc.ResponseFuture<$0.FileDiff> readFileDiff(
    $0.ReadFileDiffRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$readFileDiff, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> createDirectory(
    $0.CreateDirectoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createDirectory, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> deleteEntry(
    $0.DeleteEntryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteEntry, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> moveEntry(
    $0.MoveEntryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$moveEntry, request, options: options);
  }

  /// 内容搜索(内置 ripgrep 语义,子串或正则)
  $grpc.ResponseStream<$0.SearchMatch> searchContent(
    $0.SearchContentRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$searchContent, $async.Stream.fromIterable([request]),
        options: options);
  }

  // method descriptors

  static final _$listDirectory =
      $grpc.ClientMethod<$0.ListDirectoryRequest, $0.ListDirectoryResponse>(
          '/pocketagent.v1.FileSystemService/ListDirectory',
          ($0.ListDirectoryRequest value) => value.writeToBuffer(),
          $0.ListDirectoryResponse.fromBuffer);
  static final _$listRoots =
      $grpc.ClientMethod<$0.ListRootsRequest, $0.ListRootsResponse>(
          '/pocketagent.v1.FileSystemService/ListRoots',
          ($0.ListRootsRequest value) => value.writeToBuffer(),
          $0.ListRootsResponse.fromBuffer);
  static final _$readFile =
      $grpc.ClientMethod<$0.ReadFileRequest, $0.FileChunk>(
          '/pocketagent.v1.FileSystemService/ReadFile',
          ($0.ReadFileRequest value) => value.writeToBuffer(),
          $0.FileChunk.fromBuffer);
  static final _$writeFile =
      $grpc.ClientMethod<$0.WriteFileRequest, $0.WriteFileResponse>(
          '/pocketagent.v1.FileSystemService/WriteFile',
          ($0.WriteFileRequest value) => value.writeToBuffer(),
          $0.WriteFileResponse.fromBuffer);
  static final _$readFileDiff =
      $grpc.ClientMethod<$0.ReadFileDiffRequest, $0.FileDiff>(
          '/pocketagent.v1.FileSystemService/ReadFileDiff',
          ($0.ReadFileDiffRequest value) => value.writeToBuffer(),
          $0.FileDiff.fromBuffer);
  static final _$createDirectory =
      $grpc.ClientMethod<$0.CreateDirectoryRequest, $1.Empty>(
          '/pocketagent.v1.FileSystemService/CreateDirectory',
          ($0.CreateDirectoryRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$deleteEntry =
      $grpc.ClientMethod<$0.DeleteEntryRequest, $1.Empty>(
          '/pocketagent.v1.FileSystemService/DeleteEntry',
          ($0.DeleteEntryRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$moveEntry = $grpc.ClientMethod<$0.MoveEntryRequest, $1.Empty>(
      '/pocketagent.v1.FileSystemService/MoveEntry',
      ($0.MoveEntryRequest value) => value.writeToBuffer(),
      $1.Empty.fromBuffer);
  static final _$searchContent =
      $grpc.ClientMethod<$0.SearchContentRequest, $0.SearchMatch>(
          '/pocketagent.v1.FileSystemService/SearchContent',
          ($0.SearchContentRequest value) => value.writeToBuffer(),
          $0.SearchMatch.fromBuffer);
}

@$pb.GrpcServiceName('pocketagent.v1.FileSystemService')
abstract class FileSystemServiceBase extends $grpc.Service {
  $core.String get $name => 'pocketagent.v1.FileSystemService';

  FileSystemServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.ListDirectoryRequest, $0.ListDirectoryResponse>(
            'ListDirectory',
            listDirectory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListDirectoryRequest.fromBuffer(value),
            ($0.ListDirectoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListRootsRequest, $0.ListRootsResponse>(
        'ListRoots',
        listRoots_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListRootsRequest.fromBuffer(value),
        ($0.ListRootsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ReadFileRequest, $0.FileChunk>(
        'ReadFile',
        readFile_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.ReadFileRequest.fromBuffer(value),
        ($0.FileChunk value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.WriteFileRequest, $0.WriteFileResponse>(
        'WriteFile',
        writeFile_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.WriteFileRequest.fromBuffer(value),
        ($0.WriteFileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ReadFileDiffRequest, $0.FileDiff>(
        'ReadFileDiff',
        readFileDiff_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReadFileDiffRequest.fromBuffer(value),
        ($0.FileDiff value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateDirectoryRequest, $1.Empty>(
        'CreateDirectory',
        createDirectory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateDirectoryRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteEntryRequest, $1.Empty>(
        'DeleteEntry',
        deleteEntry_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeleteEntryRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MoveEntryRequest, $1.Empty>(
        'MoveEntry',
        moveEntry_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.MoveEntryRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SearchContentRequest, $0.SearchMatch>(
        'SearchContent',
        searchContent_Pre,
        false,
        true,
        ($core.List<$core.int> value) =>
            $0.SearchContentRequest.fromBuffer(value),
        ($0.SearchMatch value) => value.writeToBuffer()));
  }

  $async.Future<$0.ListDirectoryResponse> listDirectory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListDirectoryRequest> $request) async {
    return listDirectory($call, await $request);
  }

  $async.Future<$0.ListDirectoryResponse> listDirectory(
      $grpc.ServiceCall call, $0.ListDirectoryRequest request);

  $async.Future<$0.ListRootsResponse> listRoots_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListRootsRequest> $request) async {
    return listRoots($call, await $request);
  }

  $async.Future<$0.ListRootsResponse> listRoots(
      $grpc.ServiceCall call, $0.ListRootsRequest request);

  $async.Stream<$0.FileChunk> readFile_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ReadFileRequest> $request) async* {
    yield* readFile($call, await $request);
  }

  $async.Stream<$0.FileChunk> readFile(
      $grpc.ServiceCall call, $0.ReadFileRequest request);

  $async.Future<$0.WriteFileResponse> writeFile_Pre($grpc.ServiceCall $call,
      $async.Future<$0.WriteFileRequest> $request) async {
    return writeFile($call, await $request);
  }

  $async.Future<$0.WriteFileResponse> writeFile(
      $grpc.ServiceCall call, $0.WriteFileRequest request);

  $async.Future<$0.FileDiff> readFileDiff_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ReadFileDiffRequest> $request) async {
    return readFileDiff($call, await $request);
  }

  $async.Future<$0.FileDiff> readFileDiff(
      $grpc.ServiceCall call, $0.ReadFileDiffRequest request);

  $async.Future<$1.Empty> createDirectory_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateDirectoryRequest> $request) async {
    return createDirectory($call, await $request);
  }

  $async.Future<$1.Empty> createDirectory(
      $grpc.ServiceCall call, $0.CreateDirectoryRequest request);

  $async.Future<$1.Empty> deleteEntry_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteEntryRequest> $request) async {
    return deleteEntry($call, await $request);
  }

  $async.Future<$1.Empty> deleteEntry(
      $grpc.ServiceCall call, $0.DeleteEntryRequest request);

  $async.Future<$1.Empty> moveEntry_Pre($grpc.ServiceCall $call,
      $async.Future<$0.MoveEntryRequest> $request) async {
    return moveEntry($call, await $request);
  }

  $async.Future<$1.Empty> moveEntry(
      $grpc.ServiceCall call, $0.MoveEntryRequest request);

  $async.Stream<$0.SearchMatch> searchContent_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SearchContentRequest> $request) async* {
    yield* searchContent($call, await $request);
  }

  $async.Stream<$0.SearchMatch> searchContent(
      $grpc.ServiceCall call, $0.SearchContentRequest request);
}
