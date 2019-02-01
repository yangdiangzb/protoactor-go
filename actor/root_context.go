package actor

import "time"

type RootContext struct {
	senderMiddleware SenderFunc
	headers          messageHeader
}

var emptyRootContext = &RootContext{
	senderMiddleware: nil,
	headers:          emptyMessageHeader,
}

func EmptyRootContext() *RootContext {
	return emptyRootContext
}

func NewRootContext(header map[string]string, middleware ...SenderMiddleware) *RootContext {
	return &RootContext{
		senderMiddleware: makeSenderMiddlewareChain(middleware, func(_ SenderContext, target *PID, envelope *MessageEnvelope) {
			target.sendUserMessage(envelope)
		}),
		headers: messageHeader(header),
	}
}

func (rc *RootContext) WithHeaders(headers map[string]string) *RootContext {
	rc.headers = headers
	return rc
}

func (rc *RootContext) WithSenderMiddleware(middleware ...SenderMiddleware) *RootContext {
	rc.senderMiddleware = makeSenderMiddlewareChain(middleware, func(_ SenderContext, target *PID, envelope *MessageEnvelope) {
		target.sendUserMessage(envelope)
	})
	return rc
}

//
// Interface: SenderContext
//

func (rc *RootContext) Message() interface{} {
	return nil
}

func (rc *RootContext) MessageHeader() ReadonlyMessageHeader {
	return rc.headers
}

func (rc *RootContext) Send(pid *PID, message interface{}) {
	rc.sendUserMessage(pid, message)
}

func (rc *RootContext) Request(pid *PID, message interface{}) {
	rc.sendUserMessage(pid, message)
}

func (rc *RootContext) RequestWithCustomSender(pid *PID, message interface{}, sender *PID) {
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  sender,
	}
	rc.sendUserMessage(pid, env)
}

// RequestFuture sends a message to a given PID and returns a Future
func (rc *RootContext) RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future {
	future := NewFuture(timeout)
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  future.PID(),
	}
	rc.sendUserMessage(pid, env)
	return future
}

func (rc *RootContext) sendUserMessage(pid *PID, message interface{}) {
	if rc.senderMiddleware != nil {
		// Request based middleware
		rc.senderMiddleware(rc, pid, WrapEnvelope(message))
	} else {
		// tell based middleware
		pid.sendUserMessage(message)
	}
}

//
// Interface: SpawnerContext
//

func (rc *RootContext) Spawn(props *Props) (*PID, error) {
	name := ProcessRegistry.NextId()
	return rc.SpawnNamed(props, name)
}

func (rc *RootContext) SpawnPrefix(props *Props, prefix string) (*PID, error) {
	name := prefix + ProcessRegistry.NextId()
	return rc.SpawnNamed(props, name)
}

func (rc *RootContext) SpawnNamed(props *Props, name string) (*PID, error) {
	var parent *PID = nil
	if props.guardianStrategy != nil {
		parent = guardians.getGuardianPid(props.guardianStrategy)
	}
	return props.spawn(name, parent)
}

//
// Interface: StopperContext
//

// Stop will stop actor immediately regardless of existing user messages in mailbox.
func (rc *RootContext) Stop(pid *PID) {
	pid.ref().Stop(pid)
}

// StopFuture will stop actor immediately regardless of existing user messages in mailbox, and return its future.
func (rc *RootContext) StopFuture(pid *PID) *Future {
	future := NewFuture(10 * time.Second)

	pid.sendSystemMessage(&Watch{Watcher: future.pid})
	rc.Stop(pid)

	return future
}

// Poison will tell actor to stop after processing current user messages in mailbox.
func (rc *RootContext) Poison(pid *PID) {
	pid.sendUserMessage(&PoisonPill{})
}

// PoisonFuture will tell actor to stop after processing current user messages in mailbox, and return its future.
func (rc *RootContext) PoisonFuture(pid *PID) *Future {
	future := NewFuture(10 * time.Second)

	pid.sendSystemMessage(&Watch{Watcher: future.pid})
	rc.Poison(pid)

	return future
}
