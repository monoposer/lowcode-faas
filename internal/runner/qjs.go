package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fastschema/qjs"
)

// Result is the outcome of a single action invoke.
type Result struct {
	Output   any
	Logs     string
	Duration time.Duration
	Error    string
}

// Invoke loads stored ESM, resolves export default | handler, calls it with input, returns JSON-friendly output.
// TypeScript is never compiled here — meta already compiled TS→JS on save.
// Before eval, registered HostBinder(s) inject Go functions / ProxyValues into the JS global scope.
func (r *Runner) Invoke(ctx context.Context, jsESM []byte, input any, timeout time.Duration) (*Result, error) {
	start := time.Now()
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sec := int(timeout.Seconds())
	if sec < 1 {
		sec = 1
	}

	rt, err := qjs.New(qjs.Option{
		MaxExecutionTime: sec,
		Context:          ctx,
	})
	if err != nil {
		return nil, fmt.Errorf("qjs runtime: %w", err)
	}
	defer rt.Close()

	qctx := rt.Context()
	var logs strings.Builder

	for _, bind := range r.hosts {
		if bind == nil {
			continue
		}
		if err := bind(ctx, qctx, &logs); err != nil {
			return &Result{Duration: time.Since(start), Error: "host bind: " + err.Error()}, nil
		}
	}

	if input == nil {
		input = map[string]any{}
	}
	inputVal, err := qjs.ToJsValue(qctx, input)
	if err != nil {
		return &Result{Duration: time.Since(start), Logs: logs.String(), Error: "input to js: " + err.Error()}, nil
	}
	qctx.Global().SetPropertyStr("__input", inputVal)

	if _, err := qctx.Load("action.js", qjs.Code(string(jsESM))); err != nil {
		return &Result{Duration: time.Since(start), Logs: logs.String(), Error: "load action: " + err.Error()}, nil
	}

	result, err := qctx.Eval("invoke.js", qjs.Code(`
		import * as mod from 'action.js';
		const handler = typeof mod.default === 'function'
			? mod.default
			: (typeof mod.handler === 'function' ? mod.handler : null);
		if (typeof handler !== 'function') {
			throw new Error('handler not found (export default or export function handler)');
		}
		export default await Promise.resolve(handler(globalThis.__input));
	`), qjs.TypeModule())
	if err != nil {
		return &Result{Duration: time.Since(start), Logs: logs.String(), Error: err.Error()}, nil
	}
	defer result.Free()

	out, err := jsValueToAny(result)
	if err != nil {
		return &Result{Duration: time.Since(start), Logs: logs.String(), Error: "result convert: " + err.Error()}, nil
	}
	return &Result{
		Output:   out,
		Logs:     strings.TrimSpace(logs.String()),
		Duration: time.Since(start),
	}, nil
}

func jsValueToAny(v *qjs.Value) (any, error) {
	if v == nil || v.IsUndefined() || v.IsNull() {
		return nil, nil
	}
	goVal, err := qjs.ToGoValue[any](v)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(goVal)
	if err != nil {
		return goVal, nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return goVal, nil
	}
	return out, nil
}
