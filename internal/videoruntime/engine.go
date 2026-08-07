package videoruntime

import (
	"context"
	"fmt"
)

// Engine is Tapioca's stable boundary around a video inference implementation.
// Backends such as ComfyUI, MLX, or Diffusers remain replaceable details behind
// this interface; callers only depend on Tapioca's Request contract.
type Engine interface {
	Name() string
	Run(context.Context, string, Request) error
}

type engineFunc struct {
	name string
	run  func(context.Context, string, Request) error
}

func (e engineFunc) Name() string { return e.name }

func (e engineFunc) Run(ctx context.Context, cacheDir string, request Request) error {
	return e.run(ctx, cacheDir, request)
}

func engineFor(backend string) (Engine, error) {
	switch backend {
	case "mlx-video":
		return engineFunc{name: "mlx", run: func(ctx context.Context, cache string, request Request) error {
			return runPython(ctx, cache, request, "mlx")
		}}, nil
	case "diffusers-video":
		return engineFunc{name: "diffusers", run: func(ctx context.Context, cache string, request Request) error {
			return runPython(ctx, cache, request, "diffusers")
		}}, nil
	case "comfy-h3-mps", "comfy-h3-cuda":
		return engineFunc{name: "comfy-h3", run: runH3}, nil
	default:
		return nil, fmt.Errorf("unsupported video backend %q", backend)
	}
}
