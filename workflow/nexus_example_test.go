package workflow_test

import (
	"context"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/workflow"
)

type MyInput struct{}
type MyOutput struct{}

var myOperationRef = nexus.NewOperationReference[MyInput, MyOutput]("my-operation")

var myOperation = nexus.NewSyncOperation("my-operation", func(ctx context.Context, input MyInput, options nexus.StartOperationOptions) (MyOutput, error) {
	return MyOutput{}, nil
})

func ExampleNexusClient() {
	myWorkflow := func(ctx workflow.Context) (MyOutput, error) {
		client := workflow.NewNexusClient("my-endpoint", "my-service")
		// Execute an operation using an operation name.
		fut := client.ExecuteOperation(ctx, "my-operation", MyInput{}, workflow.NexusOperationOptions{
			ScheduleToCloseTimeout: time.Hour,
		})
		// Or using an OperationReference.
		fut = client.ExecuteOperation(ctx, myOperationRef, MyInput{}, workflow.NexusOperationOptions{
			ScheduleToCloseTimeout: time.Hour,
		})
		// Or using a defined operation (which is also an OperationReference).
		fut = client.ExecuteOperation(ctx, myOperation, MyInput{}, workflow.NexusOperationOptions{
			ScheduleToCloseTimeout: time.Hour,
		})

		var exec workflow.NexusOperationExecution
		// Optionally wait for the operation to be started.
		_ = fut.GetNexusOperationExecution().Get(ctx, &exec)
		// OperationToken will be empty if the operation completed synchronously.
		workflow.GetLogger(ctx).Info("operation started", "token", exec.OperationToken)

		// Get the result of the operation.
		var output MyOutput
		return output, fut.Get(ctx, &output)
	}

	_ = myWorkflow
}
