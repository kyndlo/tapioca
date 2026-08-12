package app

import (
	"context"
	"fmt"
	"os"

	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/modellicense"
)

type PullProgress struct {
	Stage      string `json:"stage"`
	Message    string `json:"message,omitempty"`
	Path       string `json:"path,omitempty"`
	File       string `json:"file,omitempty"`
	Index      int    `json:"index,omitempty"`
	Count      int    `json:"count,omitempty"`
	Bytes      int64  `json:"bytes,omitempty"`
	TotalBytes int64  `json:"total_bytes,omitempty"`
}

type PullReporter func(PullProgress)

type huggingFaceTokenContextKey struct{}

// WithHuggingFaceToken attaches an ephemeral provider token to one operation.
// The token is never persisted by Tapioca.
func WithHuggingFaceToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, huggingFaceTokenContextKey{}, token)
}

func huggingFaceToken(ctx context.Context) string {
	if token, ok := ctx.Value(huggingFaceTokenContextKey{}).(string); ok && token != "" {
		return token
	}
	if token := os.Getenv("HF_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("HUGGING_FACE_HUB_TOKEN")
}

func PullModel(
	ctx context.Context,
	ref string,
	force bool,
	report PullReporter,
) (config.Model, error) {
	resolved, err := catalog.Resolve(ref)
	if err != nil {
		return config.Model{}, err
	}
	return pullResolvedWithContext(ctx, resolved, force, report)
}

// AcceptModelLicense records that the user reviewed and accepted the terms for
// a gated catalog model. The provider may still require a separate account-side
// acceptance and an access token.
func AcceptModelLicense(ref string) error {
	resolved, err := catalog.Resolve(ref)
	if err != nil {
		return err
	}
	if !resolved.Gated {
		return fmt.Errorf("%s does not require explicit license acceptance", resolved.Name)
	}
	return modellicense.Accept(resolved.Name, resolved.License, resolved.LicenseURL)
}

func reportPull(report PullReporter, progress PullProgress) {
	if report != nil {
		report(progress)
	}
}

func cliPullReporter(progress PullProgress) {
	if progress.Stage == "transfer_complete" {
		fmt.Fprintln(os.Stderr)
		return
	}
	if progress.Stage == "progress" {
		if progress.TotalBytes > 0 {
			fmt.Fprintf(os.Stderr, "\r%.1f%%  %.1f / %.1f GB",
				float64(progress.Bytes)*100/float64(progress.TotalBytes),
				float64(progress.Bytes)/1e9,
				float64(progress.TotalBytes)/1e9,
			)
		} else {
			fmt.Fprintf(os.Stderr, "\r%.1f GB", float64(progress.Bytes)/1e9)
		}
		return
	}
	if progress.Message != "" {
		fmt.Println(progress.Message)
	}
}
