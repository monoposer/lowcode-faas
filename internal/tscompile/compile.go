package tscompile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
)

const SourceTypeScript = "TYPESCRIPT"

type Options struct {
	Timeout time.Duration
	// Mock when true: emit minimal JS without calling esbuild (dev only).
	Mock bool
}

type Compiler struct {
	opts Options
}

func New(opts Options) *Compiler {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	return &Compiler{opts: opts}
}

func HashSource(sourceType, source string) string {
	sum := sha256.Sum256([]byte(sourceType + "\n" + source))
	return hex.EncodeToString(sum[:])
}

func NeedsCompile(sourceType string) bool {
	return NormalizeSourceType(sourceType) == SourceTypeScript
}

func NormalizeSourceType(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	switch v {
	case SourceTypeScript, "TS", "TSCODE", "JSCODE", "":
		return SourceTypeScript
	default:
		return SourceTypeScript
	}
}

// Compile turns TypeScript into bundled ESM JavaScript.
func (c *Compiler) Compile(ctx context.Context, sourceType, source string) ([]byte, error) {
	st := NormalizeSourceType(sourceType)
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("source is empty")
	}
	if st != SourceTypeScript {
		return nil, fmt.Errorf("only TYPESCRIPT is supported, got %s", st)
	}
	if c.opts.Mock {
		return []byte(minimalJS()), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := api.Build(api.BuildOptions{
		Stdin: &api.StdinOptions{
			Contents:   source,
			Loader:     api.LoaderTS,
			Sourcefile: "action.ts",
		},
		Write:            false,
		Bundle:           true,
		Format:           api.FormatESModule,
		Platform:         api.PlatformNeutral,
		Target:           api.ES2020,
		MinifyWhitespace: false,
		MinifySyntax:     false,
		Sourcemap:        api.SourceMapNone,
	})
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("esbuild failed:\n%s", formatMessages(result.Errors))
	}
	if len(result.OutputFiles) == 0 || len(result.OutputFiles[0].Contents) == 0 {
		return nil, fmt.Errorf("esbuild produced empty output")
	}
	return result.OutputFiles[0].Contents, nil
}

func formatMessages(msgs []api.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

func minimalJS() string {
	return "export default function handler() {\n  return {};\n}\n"
}

// DefaultHandlerSource is the TypeScript template used when create omits content.
// `import type` is erased by esbuild; runtime only needs the default export.
const DefaultHandlerSource = `import type { ActionRequest, ActionResponse } from 'lowcode-faas/runtime'

export default function handler(req: ActionRequest): ActionResponse {
  return {
    status: 200,
    data: {
      ok: true,
      context: req.context ?? null,
      body: req.body ?? null,
      data: req.data ?? null,
      query: req.query ?? null,
    },
  }
}
`
