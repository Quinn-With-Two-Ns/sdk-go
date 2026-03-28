package internal

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/google/uuid"
	callbackpb "go.temporal.io/api/callback/v1"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/types/known/durationpb"
)

const pollCallbackTimeout = 60 * time.Second

type (
	// ClientStartCallbackOptions contains configuration parameters for starting a callback execution.
	// ID and Callback are required.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.StartCallbackOptions]
	ClientStartCallbackOptions struct {
		// ID - The unique identifier of the callback within its namespace.
		//
		// Mandatory: No default.
		ID string
		// Callback - Information on how this callback should be invoked (e.g. its URL and type).
		//
		// Mandatory: No default.
		Callback *commonpb.Callback
		// ScheduleToCloseTimeout - Total time allowed for the callback to complete.
		//
		// Optional: Defaults to server default.
		ScheduleToCloseTimeout time.Duration
		// TypedSearchAttributes - Search Attributes attached to the callback.
		//
		// Optional: default to none.
		TypedSearchAttributes SearchAttributes
		// Links - Links to be associated with the callback.
		//
		// Optional: default to none.
		Links []*commonpb.Link
	}

	// ClientGetCallbackHandleOptions contains input for GetCallbackHandle call.
	// CallbackID is required.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.GetCallbackHandleOptions]
	ClientGetCallbackHandleOptions struct {
		CallbackID string
	}

	// ClientListCallbacksOptions contains input for ListCallbacks call.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.ListCallbacksOptions]
	ClientListCallbacksOptions struct {
		Query string
	}

	// ClientListCallbacksResult contains the result of the ListCallbacks call.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.ListCallbacksResult]
	ClientListCallbacksResult struct {
		Results iter.Seq2[*ClientCallbackExecutionInfo, error]
	}

	// ClientCountCallbacksOptions contains input for CountCallbacks call.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.CountCallbacksOptions]
	ClientCountCallbacksOptions struct {
		Query string
	}

	// ClientCountCallbacksResult contains the result of the CountCallbacks call.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.CountCallbacksResult]
	ClientCountCallbacksResult struct {
		Count  int64
		Groups []ClientCountCallbacksAggregationGroup
	}

	// ClientCountCallbacksAggregationGroup contains groups of callbacks if
	// CountCallbackExecutions is grouped by a field.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.CountCallbacksAggregationGroup]
	ClientCountCallbacksAggregationGroup struct {
		GroupValues []any
		Count       int64
	}

	// ClientCallbackHandle represents a running or completed standalone callback execution.
	// It can be used to wait for completion, describe, cancel, or terminate the callback.
	//
	// Methods may be added to this interface; implementing it directly is discouraged.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.CallbackHandle]
	ClientCallbackHandle interface {
		// GetID returns the callback ID.
		GetID() string
		// Get waits until the callback finishes. Returns nil on success, or the failure as an error.
		// Note: unlike activities, successful callbacks have no result payload.
		Get(ctx context.Context) error
		// Describe returns detailed information about the callback execution.
		Describe(ctx context.Context, options ClientDescribeCallbackOptions) (*ClientCallbackExecutionDescription, error)
		// Cancel requests cancellation of the callback.
		Cancel(ctx context.Context, options ClientCancelCallbackOptions) error
		// Terminate terminates the callback.
		Terminate(ctx context.Context, options ClientTerminateCallbackOptions) error
	}

	// ClientDescribeCallbackOptions contains options for ClientCallbackHandle.Describe call.
	// For future compatibility, currently unused.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.DescribeCallbackOptions]
	ClientDescribeCallbackOptions struct{}

	// ClientCancelCallbackOptions contains options for ClientCallbackHandle.Cancel call.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.CancelCallbackOptions]
	ClientCancelCallbackOptions struct {
		// Reason is optional description of the reason for cancellation.
		Reason string
	}

	// ClientTerminateCallbackOptions contains options for ClientCallbackHandle.Terminate call.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.TerminateCallbackOptions]
	ClientTerminateCallbackOptions struct {
		// Reason is optional description of the reason for termination.
		Reason string
	}

	// ClientCallbackExecutionInfo contains information about a callback execution.
	// This is returned by ListCallbacks and embedded in ClientCallbackExecutionDescription.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.CallbackExecutionInfo]
	ClientCallbackExecutionInfo struct {
		// Raw PB message this struct was built from. This field is nil in the result of ClientCallbackHandle.Describe call - use
		// ClientCallbackExecutionDescription.RawExecutionInfo instead.
		RawExecutionListInfo     *callbackpb.CallbackExecutionListInfo
		CallbackID               string
		State                    enumspb.CallbackState
		CreateTime               time.Time
		CloseTime                time.Time
		TypedSearchAttributes    SearchAttributes
		StateTransitionCount     int64
	}

	// ClientCallbackExecutionDescription contains detailed information about a callback execution.
	// This is returned by ClientCallbackHandle.Describe.
	//
	// NOTE: Experimental
	//
	// Exposed as: [go.temporal.io/sdk/client.CallbackExecutionDescription]
	ClientCallbackExecutionDescription struct {
		ClientCallbackExecutionInfo
		// Raw PB message this struct was built from.
		RawExecutionInfo        *callbackpb.CallbackExecutionInfo
		Callback                *commonpb.Callback
		Attempt                 int32
		LastAttemptCompleteTime  time.Time
		NextAttemptScheduleTime time.Time
		BlockedReason           string
		ScheduleToCloseTimeout  time.Duration
		Links                   []*commonpb.Link
		failureConverter        converter.FailureConverter
		inboundPayloadVisitor   PayloadVisitor
	}

	// clientCallbackHandleImpl is the default implementation of ClientCallbackHandle.
	clientCallbackHandleImpl struct {
		client     *WorkflowClient
		id         string
		pollResult *ClientPollCallbackResultOutput
	}
)

// GetLastFailure returns the last attempt failure of the callback execution, using the failure converter of the client used to
// make the Describe call. Returns nil if there was no failure.
func (d *ClientCallbackExecutionDescription) GetLastFailure() error {
	failure := d.RawExecutionInfo.GetLastAttemptFailure()
	if failure == nil {
		return nil
	}
	if err := visitProtoPayloads(context.Background(), d.inboundPayloadVisitor, failure); err != nil {
		return err
	}
	return d.failureConverter.FailureToError(failure)
}

func (h *clientCallbackHandleImpl) GetID() string {
	return h.id
}

func (h *clientCallbackHandleImpl) Get(ctx context.Context) error {
	if h.pollResult != nil {
		return h.pollResult.Error
	}
	if err := h.client.ensureInitialized(ctx); err != nil {
		return err
	}

	// repeatedly poll, the loop repeats until there's an outcome
	for {
		resp, err := h.client.interceptor.PollCallbackResult(ctx, &ClientPollCallbackResultInput{
			CallbackID: h.id,
		})
		if err != nil {
			return err
		}
		if resp != nil {
			h.pollResult = resp
			return resp.Error
		}
	}
}

func (h *clientCallbackHandleImpl) Describe(ctx context.Context, options ClientDescribeCallbackOptions) (*ClientCallbackExecutionDescription, error) {
	if err := h.client.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	out, err := h.client.interceptor.DescribeCallback(ctx, &ClientDescribeCallbackInput{
		CallbackID: h.id,
	})
	if err != nil {
		return nil, err
	}
	return out.Description, nil
}

func (h *clientCallbackHandleImpl) Cancel(ctx context.Context, options ClientCancelCallbackOptions) error {
	if err := h.client.ensureInitialized(ctx); err != nil {
		return err
	}
	return h.client.interceptor.CancelCallback(ctx, &ClientCancelCallbackInput{
		CallbackID: h.id,
		Reason:     options.Reason,
	})
}

func (h *clientCallbackHandleImpl) Terminate(ctx context.Context, options ClientTerminateCallbackOptions) error {
	if err := h.client.ensureInitialized(ctx); err != nil {
		return err
	}
	return h.client.interceptor.TerminateCallback(ctx, &ClientTerminateCallbackInput{
		CallbackID: h.id,
		Reason:     options.Reason,
	})
}

func (wc *WorkflowClient) ExecuteCallback(ctx context.Context, options ClientStartCallbackOptions, completion any) (ClientCallbackHandle, error) {
	if err := wc.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	return wc.interceptor.ExecuteCallback(ctx, &ClientExecuteCallbackInput{
		Options:    &options,
		Completion: completion,
	})
}

func (wc *WorkflowClient) GetCallbackHandle(options ClientGetCallbackHandleOptions) ClientCallbackHandle {
	return wc.interceptor.GetCallbackHandle((*ClientGetCallbackHandleInput)(&options))
}

func (wc *WorkflowClient) ListCallbacks(ctx context.Context, options ClientListCallbacksOptions) (ClientListCallbacksResult, error) {
	return ClientListCallbacksResult{
		Results: func(yield func(*ClientCallbackExecutionInfo, error) bool) {
			if err := wc.ensureInitialized(ctx); err != nil {
				yield(nil, err)
				return
			}

			request := &workflowservice.ListCallbackExecutionsRequest{
				Namespace: wc.namespace,
				Query:     options.Query,
			}

			for {
				resp, err := wc.getListCallbacksPage(ctx, request)
				if err != nil {
					yield(nil, err)
					return
				}

				for _, ex := range resp.Executions {
					if !yield(&ClientCallbackExecutionInfo{
						RawExecutionListInfo:  ex,
						CallbackID:           ex.CallbackId,
						State:                ex.State,
						CreateTime:           ex.CreateTime.AsTime(),
						CloseTime:            ex.CloseTime.AsTime(),
						TypedSearchAttributes: convertToTypedSearchAttributes(wc.logger, ex.SearchAttributes.GetIndexedFields()),
						StateTransitionCount: ex.StateTransitionCount,
					}, nil) {
						return
					}
				}

				if resp.NextPageToken != nil {
					request.NextPageToken = resp.NextPageToken
				} else {
					return
				}
			}
		},
	}, nil
}

func (wc *WorkflowClient) getListCallbacksPage(ctx context.Context, request *workflowservice.ListCallbackExecutionsRequest) (*workflowservice.ListCallbackExecutionsResponse, error) {
	grpcCtx, cancel := newGRPCContext(ctx, defaultGrpcRetryParameters(ctx))
	defer cancel()

	return wc.WorkflowService().ListCallbackExecutions(grpcCtx, request)
}

func (wc *WorkflowClient) CountCallbacks(ctx context.Context, options ClientCountCallbacksOptions) (*ClientCountCallbacksResult, error) {
	grpcCtx, cancel := newGRPCContext(ctx, defaultGrpcRetryParameters(ctx))
	defer cancel()

	request := &workflowservice.CountCallbackExecutionsRequest{
		Namespace: wc.namespace,
		Query:     options.Query,
	}
	resp, err := wc.WorkflowService().CountCallbackExecutions(grpcCtx, request)
	if err != nil {
		return nil, err
	}

	groups := make([]ClientCountCallbacksAggregationGroup, len(resp.Groups))
	for i, group := range resp.Groups {
		groupValues := make([]any, len(group.GroupValues))
		for j, groupValue := range group.GroupValues {
			// should never fail, and if it does, leaving nil behind
			_ = converter.GetDefaultDataConverter().FromPayload(groupValue, &groupValues[j])
		}
		groups[i] = ClientCountCallbacksAggregationGroup{
			GroupValues: groupValues,
			Count:       group.Count,
		}
	}

	return &ClientCountCallbacksResult{
		Count:  resp.Count,
		Groups: groups,
	}, nil
}

func (w *workflowClientInterceptor) ExecuteCallback(
	ctx context.Context,
	in *ClientExecuteCallbackInput,
) (ClientCallbackHandle, error) {
	ctx = contextWithNewHeader(ctx)
	dataConverter := WithContext(ctx, w.client.dataConverter)
	if dataConverter == nil {
		dataConverter = converter.GetDefaultDataConverter()
	}

	if in.Options.ID == "" {
		return nil, fmt.Errorf("callback ID is required")
	}
	if in.Options.Callback == nil {
		return nil, fmt.Errorf("callback is required")
	}

	searchAttrs, err := serializeTypedSearchAttributes(in.Options.TypedSearchAttributes.GetUntypedValues())
	if err != nil {
		return nil, err
	}

	// Build the completion proto. A nil completion is treated as success with nil payload.
	// If completion is an error, convert to failure. Otherwise, serialize as success payload.
	completion := &callbackpb.CallbackExecutionCompletion{}
	if completionErr, ok := in.Completion.(error); ok {
		completion.Failure = w.client.failureConverter.ErrorToFailure(completionErr)
	} else if in.Completion != nil {
		payload, err := dataConverter.ToPayload(in.Completion)
		if err != nil {
			return nil, err
		}
		completion.Success = payload
	}

	request := &workflowservice.StartCallbackExecutionRequest{
		Namespace:              w.client.namespace,
		Identity:               w.client.identity,
		RequestId:              uuid.NewString(),
		CallbackId:             in.Options.ID,
		Callback:               in.Options.Callback,
		ScheduleToCloseTimeout: durationpb.New(in.Options.ScheduleToCloseTimeout),
		SearchAttributes:       searchAttrs,
		Completion:             completion,
		Links:                  in.Options.Links,
	}

	if request.Header, err = headerPropagated(ctx, w.client.contextPropagators); err != nil {
		return nil, err
	}

	if err := visitProtoPayloads(ctx, w.client.outboundPayloadVisitor, request); err != nil {
		return nil, err
	}

	grpcCtx, cancel := newGRPCContext(ctx, defaultGrpcRetryParameters(ctx))
	defer cancel()

	_, err = w.client.WorkflowService().StartCallbackExecution(grpcCtx, request)
	if err != nil {
		return nil, err
	}

	return &clientCallbackHandleImpl{
		client: w.client,
		id:     in.Options.ID,
	}, nil
}

func (w *workflowClientInterceptor) GetCallbackHandle(
	in *ClientGetCallbackHandleInput,
) ClientCallbackHandle {
	return &clientCallbackHandleImpl{
		client: w.client,
		id:     in.CallbackID,
	}
}

func (w *workflowClientInterceptor) PollCallbackResult(
	ctx context.Context,
	in *ClientPollCallbackResultInput,
) (*ClientPollCallbackResultOutput, error) {
	request := &workflowservice.PollCallbackExecutionRequest{
		Namespace:  w.client.namespace,
		CallbackId: in.CallbackID,
	}

	var resp *workflowservice.PollCallbackExecutionResponse
	for resp.GetOutcome() == nil {
		grpcCtx, cancel := newGRPCContext(ctx, grpcLongPoll(true), grpcTimeout(pollCallbackTimeout), defaultGrpcRetryParameters(ctx))
		var err error
		resp, err = w.client.WorkflowService().PollCallbackExecution(grpcCtx, request)
		cancel()
		if err != nil {
			return nil, err
		}
	}

	if err := visitProtoPayloads(ctx, w.client.inboundPayloadVisitor, resp); err != nil {
		return nil, err
	}

	switch v := resp.GetOutcome().GetValue().(type) {
	case *callbackpb.CallbackExecutionOutcome_Success:
		return &ClientPollCallbackResultOutput{Error: nil}, nil
	case *callbackpb.CallbackExecutionOutcome_Failure:
		return &ClientPollCallbackResultOutput{Error: w.client.failureConverter.FailureToError(v.Failure)}, nil
	default:
		return nil, fmt.Errorf("unexpected callback outcome type: %T", v)
	}
}

func (w *workflowClientInterceptor) DescribeCallback(
	ctx context.Context,
	in *ClientDescribeCallbackInput,
) (*ClientDescribeCallbackOutput, error) {
	grpcCtx, cancel := newGRPCContext(ctx, defaultGrpcRetryParameters(ctx))
	defer cancel()

	request := &workflowservice.DescribeCallbackExecutionRequest{
		Namespace:  w.client.namespace,
		CallbackId: in.CallbackID,
	}
	resp, err := w.client.WorkflowService().DescribeCallbackExecution(grpcCtx, request)
	if err != nil {
		return nil, err
	}
	info := resp.GetInfo()
	if info == nil {
		return nil, fmt.Errorf("DescribeCallbackExecution response doesn't contain info")
	}

	if err := visitProtoPayloads(ctx, w.client.inboundPayloadVisitor, resp); err != nil {
		return nil, err
	}

	return &ClientDescribeCallbackOutput{
		Description: &ClientCallbackExecutionDescription{
			ClientCallbackExecutionInfo: ClientCallbackExecutionInfo{
				RawExecutionListInfo:  nil,
				CallbackID:           info.CallbackId,
				State:                info.State,
				CreateTime:           info.CreateTime.AsTime(),
				CloseTime:            info.CloseTime.AsTime(),
				TypedSearchAttributes: convertToTypedSearchAttributes(w.client.logger, info.SearchAttributes.GetIndexedFields()),
				StateTransitionCount: info.StateTransitionCount,
			},
			RawExecutionInfo:        info,
			Callback:                info.Callback,
			Attempt:                 info.Attempt,
			LastAttemptCompleteTime:  info.LastAttemptCompleteTime.AsTime(),
			NextAttemptScheduleTime: info.NextAttemptScheduleTime.AsTime(),
			BlockedReason:           info.BlockedReason,
			ScheduleToCloseTimeout:  info.ScheduleToCloseTimeout.AsDuration(),
			Links:                   info.Links,
			failureConverter:        w.client.failureConverter,
			inboundPayloadVisitor:   w.client.inboundPayloadVisitor,
		},
	}, nil
}

func (w *workflowClientInterceptor) CancelCallback(
	ctx context.Context,
	in *ClientCancelCallbackInput,
) error {
	grpcCtx, cancel := newGRPCContext(ctx, defaultGrpcRetryParameters(ctx))
	defer cancel()

	request := &workflowservice.RequestCancelCallbackExecutionRequest{
		Namespace:  w.client.namespace,
		CallbackId: in.CallbackID,
		Identity:   w.client.identity,
		RequestId:  uuid.NewString(),
		Reason:     in.Reason,
	}
	_, err := w.client.WorkflowService().RequestCancelCallbackExecution(grpcCtx, request)
	return err
}

func (w *workflowClientInterceptor) TerminateCallback(
	ctx context.Context,
	in *ClientTerminateCallbackInput,
) error {
	grpcCtx, cancel := newGRPCContext(ctx, defaultGrpcRetryParameters(ctx))
	defer cancel()

	request := &workflowservice.TerminateCallbackExecutionRequest{
		Namespace:  w.client.namespace,
		CallbackId: in.CallbackID,
		Identity:   w.client.identity,
		RequestId:  uuid.NewString(),
		Reason:     in.Reason,
	}
	_, err := w.client.WorkflowService().TerminateCallbackExecution(grpcCtx, request)
	return err
}

