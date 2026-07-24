package observability

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// Init configures slog JSON logging and optional Sentry (SENTRY_DSN).
// When SENTRY_DSN is empty the process runs with structured logs only.
func Init() {
	level := slog.LevelInfo
	if os.Getenv("DOCKPANEL_LOG_DEBUG") == "1" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      first(os.Getenv("DOCKPANEL_ENV"), os.Getenv("APP_ENV"), "development"),
		TracesSampleRate: 0,
	})
	if err != nil {
		slog.Error("sentry init failed", "error", err.Error())
		return
	}
	slog.Info("sentry enabled")
}

func Flush() {
	sentry.Flush(2 * time.Second)
}

func CaptureError(ctx context.Context, err error, attrs map[string]string) {
	if err == nil {
		return
	}
	args := []any{"error", err.Error()}
	for k, v := range attrs {
		args = append(args, k, v)
	}
	slog.ErrorContext(ctx, "error", args...)
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.WithScope(func(scope *sentry.Scope) {
			for k, v := range attrs {
				scope.SetTag(k, v)
			}
			hub.CaptureException(err)
		})
		return
	}
	if os.Getenv("SENTRY_DSN") != "" {
		sentry.WithScope(func(scope *sentry.Scope) {
			for k, v := range attrs {
				scope.SetTag(k, v)
			}
			sentry.CaptureException(err)
		})
	}
}

func CapturePanic(ctx context.Context, recovered any, attrs map[string]string) {
	args := []any{"panic", recovered}
	for k, v := range attrs {
		args = append(args, k, v)
	}
	slog.ErrorContext(ctx, "panic recovered", args...)
	if os.Getenv("SENTRY_DSN") == "" {
		return
	}
	sentry.CurrentHub().RecoverWithContext(ctx, recovered)
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
