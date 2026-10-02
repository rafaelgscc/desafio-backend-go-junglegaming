package referenceworker

import (
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var Module = fx.Module(
	"reference-worker",
	fx.Provide(config.LoadReferenceWorkerConfig, NewWorker),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lifecycle fx.Lifecycle, worker *Worker) {
	lifecycle.Append(fx.Hook{OnStart: worker.Start, OnStop: worker.Stop})
}
