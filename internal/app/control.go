package app

import (
	"context"
	"fmt"
	"os"

	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
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
