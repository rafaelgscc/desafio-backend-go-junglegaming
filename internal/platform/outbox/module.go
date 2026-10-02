package outboxworker

import (
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var Module = fx.Module(
	"outbox-worker",
	fx.Provide(config.LoadOutboxConfig, NewWorker),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lifecycle fx.Lifecycle, worker *Worker) {
	lifecycle.Append(fx.Hook{OnStart: worker.Start, OnStop: worker.Stop})
}
