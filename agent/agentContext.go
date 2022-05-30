package agent

import (
	"context"
	"sync"

	"github.com/amurzeau/ssh-agent-bridge/log"
)

type AgentContext struct {
	ctx            context.Context
	cancelFunction context.CancelFunc
	wg             sync.WaitGroup

	QueryChannel chan AgentMessageQuery
}

func CreateAgent() AgentContext {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return AgentContext{
		ctx:            ctx,
		cancelFunction: cancelFunc,
		wg:             sync.WaitGroup{},

		QueryChannel: make(chan AgentMessageQuery),
	}
}

func (a *AgentContext) Go(routine func()) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		routine()
	}()
}

func (a *AgentContext) Done() <-chan struct{} {
	return a.ctx.Done()
}

func (a *AgentContext) Stop() {
	select {
	case <-a.QueryChannel:
		// channel already closed
	default:
		log.Debugf("agentContext: stopping forwarding")
		close(a.QueryChannel)
		a.cancelFunction()
	}
}

func (a *AgentContext) Wait() {
	a.wg.Wait()
	log.Debugf("agentContext: all agent forwarding stopped")
}
