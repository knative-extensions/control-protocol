# Async Command pattern

The async command pattern allows developing commands where the controller sends a command to the data plane and doesn't expect an immediate response.

This is similar to the request-response, with the crucial difference that the command execution doesn't block the reconciliation, that is the command may take a while to execute, and in the meantime the reconciler can flag the k8s resource as _processing_ such command.

In order to implement this pattern, some utilities are provided:

* service.NewAsyncCommandHandler to create an async command handler


