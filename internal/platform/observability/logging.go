package observability

import (
	"context"
	"log/slog"
	"os"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"observability",
	fx.Provide(NewMetrics, NewLogger),
	fx.Invoke(registerLogger),
)

func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func registerLogger(lifecycle fx.Lifecycle, logger *slog.Logger) {
	previous := slog.Default()
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			slog.SetDefault(logger)
			return nil
		},
		OnStop: func(context.Context) error {
			slog.SetDefault(previous)
			return nil
		},
	})
}
