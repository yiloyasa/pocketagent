// This is a generated file - do not edit.
//
// Generated from v1/git.proto.

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
import 'git.pb.dart' as $0;

export 'git.pb.dart';

@$pb.GrpcServiceName('pocketagent.v1.GitService')
class GitServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  GitServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.GitStatus> status(
    $0.GitStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$status, request, options: options);
  }

  $grpc.ResponseFuture<$0.GitDiffResponse> diff(
    $0.GitDiffRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$diff, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> stage(
    $0.GitStageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$stage, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> unstage(
    $0.GitUnstageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unstage, request, options: options);
  }

  $grpc.ResponseFuture<$0.GitCommitResponse> commit(
    $0.GitCommitRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$commit, request, options: options);
  }

  $grpc.ResponseFuture<$0.GitLogResponse> log(
    $0.GitLogRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$log, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListBranchesResponse> listBranches(
    $0.ListBranchesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listBranches, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> checkoutBranch(
    $0.CheckoutBranchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$checkoutBranch, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> createBranch(
    $0.CreateBranchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createBranch, request, options: options);
  }

  $grpc.ResponseFuture<$1.Empty> discardChanges(
    $0.DiscardChangesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$discardChanges, request, options: options);
  }

  // method descriptors

  static final _$status = $grpc.ClientMethod<$0.GitStatusRequest, $0.GitStatus>(
      '/pocketagent.v1.GitService/Status',
      ($0.GitStatusRequest value) => value.writeToBuffer(),
      $0.GitStatus.fromBuffer);
  static final _$diff =
      $grpc.ClientMethod<$0.GitDiffRequest, $0.GitDiffResponse>(
          '/pocketagent.v1.GitService/Diff',
          ($0.GitDiffRequest value) => value.writeToBuffer(),
          $0.GitDiffResponse.fromBuffer);
  static final _$stage = $grpc.ClientMethod<$0.GitStageRequest, $1.Empty>(
      '/pocketagent.v1.GitService/Stage',
      ($0.GitStageRequest value) => value.writeToBuffer(),
      $1.Empty.fromBuffer);
  static final _$unstage = $grpc.ClientMethod<$0.GitUnstageRequest, $1.Empty>(
      '/pocketagent.v1.GitService/Unstage',
      ($0.GitUnstageRequest value) => value.writeToBuffer(),
      $1.Empty.fromBuffer);
  static final _$commit =
      $grpc.ClientMethod<$0.GitCommitRequest, $0.GitCommitResponse>(
          '/pocketagent.v1.GitService/Commit',
          ($0.GitCommitRequest value) => value.writeToBuffer(),
          $0.GitCommitResponse.fromBuffer);
  static final _$log = $grpc.ClientMethod<$0.GitLogRequest, $0.GitLogResponse>(
      '/pocketagent.v1.GitService/Log',
      ($0.GitLogRequest value) => value.writeToBuffer(),
      $0.GitLogResponse.fromBuffer);
  static final _$listBranches =
      $grpc.ClientMethod<$0.ListBranchesRequest, $0.ListBranchesResponse>(
          '/pocketagent.v1.GitService/ListBranches',
          ($0.ListBranchesRequest value) => value.writeToBuffer(),
          $0.ListBranchesResponse.fromBuffer);
  static final _$checkoutBranch =
      $grpc.ClientMethod<$0.CheckoutBranchRequest, $1.Empty>(
          '/pocketagent.v1.GitService/CheckoutBranch',
          ($0.CheckoutBranchRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$createBranch =
      $grpc.ClientMethod<$0.CreateBranchRequest, $1.Empty>(
          '/pocketagent.v1.GitService/CreateBranch',
          ($0.CreateBranchRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
  static final _$discardChanges =
      $grpc.ClientMethod<$0.DiscardChangesRequest, $1.Empty>(
          '/pocketagent.v1.GitService/DiscardChanges',
          ($0.DiscardChangesRequest value) => value.writeToBuffer(),
          $1.Empty.fromBuffer);
}

@$pb.GrpcServiceName('pocketagent.v1.GitService')
abstract class GitServiceBase extends $grpc.Service {
  $core.String get $name => 'pocketagent.v1.GitService';

  GitServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GitStatusRequest, $0.GitStatus>(
        'Status',
        status_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GitStatusRequest.fromBuffer(value),
        ($0.GitStatus value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GitDiffRequest, $0.GitDiffResponse>(
        'Diff',
        diff_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GitDiffRequest.fromBuffer(value),
        ($0.GitDiffResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GitStageRequest, $1.Empty>(
        'Stage',
        stage_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GitStageRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GitUnstageRequest, $1.Empty>(
        'Unstage',
        unstage_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GitUnstageRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GitCommitRequest, $0.GitCommitResponse>(
        'Commit',
        commit_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GitCommitRequest.fromBuffer(value),
        ($0.GitCommitResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GitLogRequest, $0.GitLogResponse>(
        'Log',
        log_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GitLogRequest.fromBuffer(value),
        ($0.GitLogResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListBranchesRequest, $0.ListBranchesResponse>(
            'ListBranches',
            listBranches_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListBranchesRequest.fromBuffer(value),
            ($0.ListBranchesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CheckoutBranchRequest, $1.Empty>(
        'CheckoutBranch',
        checkoutBranch_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CheckoutBranchRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateBranchRequest, $1.Empty>(
        'CreateBranch',
        createBranch_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateBranchRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DiscardChangesRequest, $1.Empty>(
        'DiscardChanges',
        discardChanges_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DiscardChangesRequest.fromBuffer(value),
        ($1.Empty value) => value.writeToBuffer()));
  }

  $async.Future<$0.GitStatus> status_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GitStatusRequest> $request) async {
    return status($call, await $request);
  }

  $async.Future<$0.GitStatus> status(
      $grpc.ServiceCall call, $0.GitStatusRequest request);

  $async.Future<$0.GitDiffResponse> diff_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GitDiffRequest> $request) async {
    return diff($call, await $request);
  }

  $async.Future<$0.GitDiffResponse> diff(
      $grpc.ServiceCall call, $0.GitDiffRequest request);

  $async.Future<$1.Empty> stage_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GitStageRequest> $request) async {
    return stage($call, await $request);
  }

  $async.Future<$1.Empty> stage(
      $grpc.ServiceCall call, $0.GitStageRequest request);

  $async.Future<$1.Empty> unstage_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GitUnstageRequest> $request) async {
    return unstage($call, await $request);
  }

  $async.Future<$1.Empty> unstage(
      $grpc.ServiceCall call, $0.GitUnstageRequest request);

  $async.Future<$0.GitCommitResponse> commit_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GitCommitRequest> $request) async {
    return commit($call, await $request);
  }

  $async.Future<$0.GitCommitResponse> commit(
      $grpc.ServiceCall call, $0.GitCommitRequest request);

  $async.Future<$0.GitLogResponse> log_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.GitLogRequest> $request) async {
    return log($call, await $request);
  }

  $async.Future<$0.GitLogResponse> log(
      $grpc.ServiceCall call, $0.GitLogRequest request);

  $async.Future<$0.ListBranchesResponse> listBranches_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListBranchesRequest> $request) async {
    return listBranches($call, await $request);
  }

  $async.Future<$0.ListBranchesResponse> listBranches(
      $grpc.ServiceCall call, $0.ListBranchesRequest request);

  $async.Future<$1.Empty> checkoutBranch_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CheckoutBranchRequest> $request) async {
    return checkoutBranch($call, await $request);
  }

  $async.Future<$1.Empty> checkoutBranch(
      $grpc.ServiceCall call, $0.CheckoutBranchRequest request);

  $async.Future<$1.Empty> createBranch_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateBranchRequest> $request) async {
    return createBranch($call, await $request);
  }

  $async.Future<$1.Empty> createBranch(
      $grpc.ServiceCall call, $0.CreateBranchRequest request);

  $async.Future<$1.Empty> discardChanges_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DiscardChangesRequest> $request) async {
    return discardChanges($call, await $request);
  }

  $async.Future<$1.Empty> discardChanges(
      $grpc.ServiceCall call, $0.DiscardChangesRequest request);
}
