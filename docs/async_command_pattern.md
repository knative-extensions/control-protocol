# Async Command pattern

The async command pattern allows developing commands where the controller sends a command to the data plane and doesn't expect an immediate response.

This is similar to the request-response, with the important difference that the command execution doesn't block the reconciliation, that is the command may take a while to execute, and in the meantime the reconciler can flag the k8s resource as _processing_ such command.

The flow of the async command pattern looks like:

<!-- PlantUML source:

@startuml

activate Controller
Controller -> "Data plane" ++: Send AsyncCommand

"Data plane" -> Controller: Ack
note left: Flag the Resource as processing
deactivate Controller
note over Controller: Short-circuit reconciliation

note right of "Data plane": Process the command

"Data plane" -> Controller ++: Send AsyncCommandResult

Controller -> "Data plane": Ack
note left: Flag the Resource as succeded/failed
deactivate "Data plane"

note over Controller: Continue reconciliation
deactivate Controller

@enduml

-->

![](https://plantuml-server.kkeisuke.dev/svg/ZP7BRW8n34Nt_WgBBKBTpmA1gBgkoXTOZcV6QYP6YOFKls-01GEQFdPUnDVtdEoAK_OwHG1YrpEvuC6IPujHCjn7t6nnzKfEU8gKP8NhTOT7IG7tvIlnmQQ9KW1uUDDsxWaTxlaJahKBKNhly2tIW3uAVaYncbcG2fwoiPIYQO0WIvMk0NPkZURHnz6oRrWpLtNCmfPOevAh9RZjP1r6H-iVC3fylnsy5k6_APQv6q6D3h_u-XzzgSmI9Bpqf572NC4y37wmS9arLNaMi6mITWsZVVqt.svg)

In order to implement this pattern, some utilities are provided:

* `service.NewAsyncCommandHandler` which wraps `ServiceMessage` in order to simplify sending back the async result:

```go
dataPlaneService.MessageHandler(service.NewAsyncCommandHandler(
    dataPlaneService, // The service to use to send the AsyncCommandResult
    &myAsyncCommand{}, // The type of the async command
    control.OpCode(2), // The opcode to use when sending the AsyncCommandResult
    func(ctx context.Context, commandMessage service.AsyncCommandMessage) {
        // This is safe, the handler will take care of parsing
    	var cmd *myAsyncCommand = commandMessage.ParsedCommand().(*myAsyncCommand)
        // Do stuff to process command
        
        // Notify the result:
        command.NotifySuccess()
    	// Or:
    	command.NotifyFailure(err)
    },
))
```

* `reconciler.NewAsyncCommandNotificationStore` which wraps `NotificationStore` in order to provide a specialized interface to handle `AsyncCommandResult` messages:

```go
// Control plane sends the command
err := controlPlaneService.SendAndWaitForAck(
	control.OpCode(1), 
	myCommand
)

// Flag the resource as processing and shortcircuit the controller

// Control plane checks the notification store to see if there's any notification of command done
commandResult := commandNotificationStore.GetCommandResult(resourceNamespacedName, podIp, myCommand)

if commandResult == nil {
	// Still no notification: the data plane is still processing the message, 
	// shortcircuit the controller
} else if commandResult.IsFailed() {
	// Command failed
} else {
	// Command succeeded
}
```

