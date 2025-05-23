// Package interceptor contains interceptors for client and worker calls.
package interceptor

import (
	"context"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/internal"
	"go.temporal.io/sdk/workflow"
)

// Interceptor is a common interface for all interceptors. It combines
// ClientInterceptor and WorkerInterceptor. If an implementation of this
// interceptor is provided via client options, some client calls and all worker
// calls will be intercepted by it. If an implementation of this interceptor is
// provided via worker options, all worker calls will be intercepted by it.
//
// All implementations of this should embed [InterceptorBase] but are not required
// to.
type Interceptor = internal.Interceptor

// InterceptorBase is a default implementation of Interceptor meant for
// embedding. It simply embeds [ClientInterceptorBase] and [WorkerInterceptorBase].
type InterceptorBase = internal.InterceptorBase

// WorkerInterceptor is an interface for all calls that can be intercepted
// during worker operation. This includes inbound (from server) and outbound
// (from SDK) workflow and activity calls. If an implementation of this is
// provided via client or worker options, all worker calls will be intercepted
// by it.
//
// All implementations must embed WorkerInterceptorBase to safely handle future
// changes.
type WorkerInterceptor = internal.WorkerInterceptor

// WorkerInterceptorBase is a default implementation of [WorkerInterceptor] that
// simply instantiates [ActivityInboundInterceptorBase] or
// [WorkflowInboundInterceptorBase] when called to intercept activities or
// workflows respectively.
//
// This must be embedded into all WorkerInterceptor implementations to safely
// handle future changes.
type WorkerInterceptorBase = internal.WorkerInterceptorBase

// ActivityInboundInterceptor is an interface for all activity calls originating
// from the server. Implementers wanting to intercept outbound (i.e. from SDK)
// activity calls, can change the outbound interceptor in Init before the next
// call in the chain.
//
// All implementations must embed [ActivityInboundInterceptorBase] to safely
// handle future changes.
type ActivityInboundInterceptor = internal.ActivityInboundInterceptor

// ActivityInboundInterceptorBase is a default implementation of
// [ActivityInboundInterceptor] that forwards calls to the next inbound
// interceptor and uses an ActivityOutboundInterceptorBase on Init.
//
// This must be embedded into all [ActivityInboundInterceptor] implementations to
// safely handle future changes.
type ActivityInboundInterceptorBase = internal.ActivityInboundInterceptorBase

// ExecuteActivityInput is input for ActivityInboundInterceptor.ExecuteActivity.
type ExecuteActivityInput = internal.ExecuteActivityInput

// ActivityOutboundInterceptor is an interface for all activity calls
// originating from the SDK.
//
// All implementations must embed [ActivityOutboundInterceptorBase] to safely
// handle future changes.
type ActivityOutboundInterceptor = internal.ActivityOutboundInterceptor

// ActivityOutboundInterceptorBase is a default implementation of
// [ActivityOutboundInterceptor] that forwards calls to the next outbound
// interceptor.
//
// This must be embedded into all ActivityOutboundInterceptor implementations to
// safely handle future changes.
type ActivityOutboundInterceptorBase = internal.ActivityOutboundInterceptorBase

// WorkflowInboundInterceptor is an interface for all workflow calls originating
// from the server. Implementers wanting to intercept outbound (i.e. from SDK)
// workflow calls, can change the outbound interceptor in Init before the next
// call in the chain.
//
// All implementations must embed [WorkflowInboundInterceptorBase] to safely
// handle future changes.
type WorkflowInboundInterceptor = internal.WorkflowInboundInterceptor

// WorkflowInboundInterceptorBase is a default implementation of
// [WorkflowInboundInterceptor] that forwards calls to the next inbound
// interceptor and uses an WorkflowOutboundInterceptorBase on Init.
//
// This must be embedded into all [WorkflowInboundInterceptor] implementations to
// safely handle future changes.
type WorkflowInboundInterceptorBase = internal.WorkflowInboundInterceptorBase

// ExecuteWorkflowInput is input for WorkflowInboundInterceptor.ExecuteWorkflow.
type ExecuteWorkflowInput = internal.ExecuteWorkflowInput

// HandleSignalInput is input for WorkflowInboundInterceptor.HandleSignal.
type HandleSignalInput = internal.HandleSignalInput

// HandleQueryInput is input for WorkflowInboundInterceptor.HandleQuery.
type HandleQueryInput = internal.HandleQueryInput

// UpdateInput is input for WorkflowInboundInterceptor.ExecuteUpdate
// and WorkflowInboundInterceptor.ValidateUpdate.
type UpdateInput = internal.UpdateInput

// ExecuteNexusOperationInput is the input to WorkflowOutboundInterceptor.ExecuteNexusOperation.
//
// NOTE: Experimental
type ExecuteNexusOperationInput = internal.ExecuteNexusOperationInput

// RequestCancelNexusOperationInput is the input to WorkflowOutboundInterceptor.RequestCancelNexusOperation.
//
// NOTE: Experimental
type RequestCancelNexusOperationInput = internal.RequestCancelNexusOperationInput

// WorkflowOutboundInterceptor is an interface for all workflow calls
// originating from the SDK.
//
// All implementations must embed [WorkflowOutboundInterceptorBase] to safely
// handle future changes.
type WorkflowOutboundInterceptor = internal.WorkflowOutboundInterceptor

// WorkflowOutboundInterceptorBase is a default implementation of
// [WorkflowOutboundInterceptor] that forwards calls to the next outbound
// interceptor.
//
// This must be embedded into all [WorkflowOutboundInterceptor] implementations to
// safely handle future changes.
type WorkflowOutboundInterceptorBase = internal.WorkflowOutboundInterceptorBase

// ClientInterceptor for providing a [ClientOutboundInterceptor] to intercept
// certain workflow-specific client calls from the SDK. If an implementation of
// this is provided via client or worker options, certain client calls will be
// intercepted by it.
//
// All implementations must embed [ClientInterceptorBase] to safely handle future
// changes.
type ClientInterceptor = internal.ClientInterceptor

// ClientInterceptorBase is a default implementation of [ClientInterceptor] that
// simply instantiates [ClientOutboundInterceptorBase] when called to intercept
// the client.
//
// This must be embedded into all [ClientInterceptor] implementations to safely
// handle future changes.
type ClientInterceptorBase = internal.ClientInterceptorBase

// ClientOutboundInterceptor is an interface for certain workflow-specific calls
// originating from the SDK.
//
// All implementations must embed [ClientOutboundInterceptorBase] to safely handle
// future changes.
type ClientOutboundInterceptor = internal.ClientOutboundInterceptor

// ClientOutboundInterceptorBase is a default implementation of
// [ClientOutboundInterceptor] that forwards calls to the next outbound
// interceptor.
//
// This must be embedded into all [ClientOutboundInterceptor] implementations to
// safely handle future changes.
type ClientOutboundInterceptorBase = internal.ClientOutboundInterceptorBase

// ClientExecuteWorkflowInput is input for
// ClientOutboundInterceptor.ExecuteWorkflow.
type ClientExecuteWorkflowInput = internal.ClientExecuteWorkflowInput

// ClientSignalWorkflowInput is input for
// ClientOutboundInterceptor.SignalWorkflow.
type ClientSignalWorkflowInput = internal.ClientSignalWorkflowInput

// ClientSignalWithStartWorkflowInput is input for
// ClientOutboundInterceptor.SignalWithStartWorkflow.
type ClientSignalWithStartWorkflowInput = internal.ClientSignalWithStartWorkflowInput

// ClientCancelWorkflowInput is input for
// ClientOutboundInterceptor.CancelWorkflow.
type ClientCancelWorkflowInput = internal.ClientCancelWorkflowInput

// ClientTerminateWorkflowInput is input for
// ClientOutboundInterceptor.TerminateWorkflow.
type ClientTerminateWorkflowInput = internal.ClientTerminateWorkflowInput

// ClientQueryWorkflowInput is input for
// ClientOutboundInterceptor.QueryWorkflow.
type ClientQueryWorkflowInput = internal.ClientQueryWorkflowInput

// ClientDescribeWorkflowInput is input for
// ClientOutboundInterceptor.DescribeWorkflow.
type ClientDescribeWorkflowInput = internal.ClientDescribeWorkflowInput

// ClientDescribeWorkflowOutput is output for
// ClientOutboundInterceptor.DescribeWorkflow.
type ClientDescribeWorkflowOutput = internal.ClientDescribeWorkflowOutput

// ScheduleClientCreateInput is input for
// ScheduleClientInterceptor.CreateSchedule.
type ScheduleClientCreateInput = internal.ScheduleClientCreateInput

// ClientUpdateWorkflowInput is input for
// ClientOutoundInterceptor.UpdateWorkflow.
type ClientUpdateWorkflowInput = internal.ClientUpdateWorkflowInput

// ClientUpdateWithStartWorkflowInput is input for
// ClientOutboundInterceptor.UpdateWithStartWorkflow.
type ClientUpdateWithStartWorkflowInput = internal.ClientUpdateWithStartWorkflowInput

// NexusOperationInboundInterceptor is an interface for intercepting Nexus operation method invocations.
//
// All implementations must embed [NexusOperationInboundInterceptorBase] to safely handle future changes.
//
// Note: Experimental
type NexusOperationInboundInterceptor = internal.NexusOperationInboundInterceptor

// NexusOperationInboundInterceptorBase is a default implementation of [NexusOperationInboundInterceptor] that
// forwards calls to the next inbound interceptor and uses a [NexusOperationOutboundInterceptorBase] on Init.
//
// This must be embedded into all [NexusOperationInboundInterceptor] implementations to safely handle future changes.
//
// Note: Experimental
type NexusOperationInboundInterceptorBase = internal.NexusOperationInboundInterceptorBase

// NexusOperationOutboundInterceptor is an interface for intercepting methods in the temporalnexus package when called
// from within a Nexus Operation handler.
//
// All implementations must embed [NexusOperationOutboundInterceptorBase] to safely handle future changes.
//
// Note: Experimental
type NexusOperationOutboundInterceptor = internal.NexusOperationOutboundInterceptor

// NexusOperationOutboundInterceptorBase is a default implementation of [NexusOperationOutboundInterceptor] that
// forwards calls to the next outbound interceptor.
//
// This must be embedded into all [NexusOperationOutboundInterceptor] implementations to safely handle future changes.
//
// Note: Experimental
type NexusOperationOutboundInterceptorBase = internal.NexusOperationOutboundInterceptorBase

// NexusStartOperationInput is the input to NexusOperationInboundInterceptor.StartOperation.
//
// Note: Experimental
type NexusStartOperationInput = internal.NexusStartOperationInput

// NexusCancelOperationInput is the input to NexusOperationInboundInterceptor.CancelOperation.
//
// Note: Experimental
type NexusCancelOperationInput = internal.NexusCancelOperationInput

// Header provides Temporal header information from the context for reading or
// writing during specific interceptor calls.
//
// This returns a non-nil map only for contexts inside
// ActivityInboundInterceptor.ExecuteActivity,
// ClientOutboundInterceptor.ExecuteWorkflow, and
// ClientOutboundInterceptor.SignalWithStartWorkflow.
func Header(ctx context.Context) map[string]*commonpb.Payload {
	return internal.Header(ctx)
}

// WorkflowHeader provides Temporal header information from the workflow context
// for reading or writing during specific interceptor calls.
//
// This returns a non-nil map only for contexts inside
// WorkflowInboundInterceptor.ExecuteWorkflow,
// WorkflowOutboundInterceptor.ExecuteActivity,
// WorkflowOutboundInterceptor.ExecuteLocalActivity,
// WorkflowOutboundInterceptor.ExecuteChildWorkflow, and
// WorkflowOutboundInterceptor.NewContinueAsNewError.
func WorkflowHeader(ctx workflow.Context) map[string]*commonpb.Payload {
	return internal.WorkflowHeader(ctx)
}
