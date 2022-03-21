# Control Protocol

The goals of the control protocol are:

- Decouple the data plane from Kubernetes, in order to make components simpler, lighter and easily
  debuggable outside kube
- Allow a data plane to implement multiple resources (e.g. like eventing-kafka-broker and
  knative-gcp already does)
- Change the state of the data plane deployment without redeploy (e.g. the start and stop feature)
- Improve the state of scaling (now scaling is based on resources mostly, but with this system it
  can be based on the actual load)

The control protocol doesn't assume, nor forces any "api style" (it might be resource based or
imperative). The "network layer" of the control protocol is not tied to the user interface, so it
can be reimplemented/swapped at some point with whatever other system we prefer.

For more docs

Note: This project is highly experimental!

Note: This project is an active dependency of the
["distributed" KafkaChannel](https://github.com/knative-sandbox/eventing-kafka/tree/main/pkg/channel/distributed)
implementation. It is used to facilitate communication between the Controller and all Dispatcher
replicas by the
[ResetOffset](https://github.com/knative-sandbox/eventing-kafka/blob/main/config/command/resetoffset)
Controller in order to stop / start Kafka consumers in order to "replay" events.
