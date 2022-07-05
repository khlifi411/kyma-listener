
# Kyma Listener

Listener component that listens to events sent by the Kyma [watcher](https://github.com/kyma-project/kyma-watcher) component

## Overview

The listener module is typically used with operators built using kube-builder but its use is not resticted to that case only.
### Use

1. For operators built using the kube-builder framework, you might leverage your `SetupWithManager()` method to initialize the listener by calling `StartListenerComponent()`.

2. You might also setup your controller to watch for changes sent through the `source.Channel{}` returned by the listener component and react to them calling the `(blder *Builder) Watches()` and providing your `handler.EventHandler` implementation.
