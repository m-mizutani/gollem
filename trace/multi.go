package trace

import (
	"context"
	"errors"
)

// multiHandler fans out trace events to multiple Handler implementations.
// Each handler receives its own isolated context to prevent interference
// (e.g., two Recorders sharing the same context key).
type multiHandler struct {
	handlers []Handler
}

// Multi creates a Handler that forwards all events to the given handlers.
// Each handler maintains its own context state, so multiple Recorders
// or any combination of handlers work correctly without interference.
func Multi(handlers ...Handler) Handler {
	return &multiHandler{handlers: handlers}
}

// multiCtxKey is the context key for per-handler contexts.
type multiCtxKey struct{}

// getContexts retrieves per-handler contexts from the context.
// If not found, returns the base context for each handler.
func (m *multiHandler) getContexts(ctx context.Context) []context.Context {
	if v, ok := ctx.Value(multiCtxKey{}).([]context.Context); ok {
		return v
	}
	ctxs := make([]context.Context, len(m.handlers))
	for i := range ctxs {
		ctxs[i] = ctx
	}
	return ctxs
}

// wrapContexts stores per-handler contexts into a new context.
func (m *multiHandler) wrapContexts(base context.Context, handlerCtxs []context.Context) context.Context {
	return context.WithValue(base, multiCtxKey{}, handlerCtxs)
}

func (m *multiHandler) StartAgentExecute(ctx context.Context) context.Context {
	handlerCtxs := make([]context.Context, len(m.handlers))
	for i, h := range m.handlers {
		handlerCtxs[i] = h.StartAgentExecute(ctx)
	}
	return m.wrapContexts(ctx, handlerCtxs)
}

func (m *multiHandler) EndAgentExecute(ctx context.Context, err error) {
	for i, h := range m.handlers {
		h.EndAgentExecute(m.getContexts(ctx)[i], err)
	}
}

func (m *multiHandler) StartLLMCall(ctx context.Context) context.Context {
	parentCtxs := m.getContexts(ctx)
	handlerCtxs := make([]context.Context, len(m.handlers))
	for i, h := range m.handlers {
		handlerCtxs[i] = h.StartLLMCall(parentCtxs[i])
	}
	return m.wrapContexts(ctx, handlerCtxs)
}

func (m *multiHandler) EndLLMCall(ctx context.Context, data *LLMCallData, err error) {
	for i, h := range m.handlers {
		h.EndLLMCall(m.getContexts(ctx)[i], data, err)
	}
}

func (m *multiHandler) StartToolExec(ctx context.Context, toolName string, args map[string]any) context.Context {
	parentCtxs := m.getContexts(ctx)
	handlerCtxs := make([]context.Context, len(m.handlers))
	for i, h := range m.handlers {
		handlerCtxs[i] = h.StartToolExec(parentCtxs[i], toolName, args)
	}
	return m.wrapContexts(ctx, handlerCtxs)
}

func (m *multiHandler) EndToolExec(ctx context.Context, result map[string]any, err error) {
	for i, h := range m.handlers {
		h.EndToolExec(m.getContexts(ctx)[i], result, err)
	}
}

func (m *multiHandler) StartSubAgent(ctx context.Context, name string) context.Context {
	parentCtxs := m.getContexts(ctx)
	handlerCtxs := make([]context.Context, len(m.handlers))
	for i, h := range m.handlers {
		handlerCtxs[i] = h.StartSubAgent(parentCtxs[i], name)
	}
	return m.wrapContexts(ctx, handlerCtxs)
}

func (m *multiHandler) EndSubAgent(ctx context.Context, err error) {
	for i, h := range m.handlers {
		h.EndSubAgent(m.getContexts(ctx)[i], err)
	}
}

func (m *multiHandler) AddEvent(ctx context.Context, kind string, data any) {
	for i, h := range m.handlers {
		h.AddEvent(m.getContexts(ctx)[i], kind, data)
	}
}

func (m *multiHandler) Finish(ctx context.Context) error {
	var errs []error
	for _, h := range m.handlers {
		if err := h.Finish(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
