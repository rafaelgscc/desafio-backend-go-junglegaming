package sqsadapter

import (
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var Module = fx.Module(
	"sqs",
	fx.Provide(config.LoadSQSConfig, NewTransport, NewWorker),
	fx.Invoke(registerWorkerLifecycle),
)

func registerWorkerLifecycle(lifecycle fx.Lifecycle, worker *Worker) {
	lifecycle.Append(fx.Hook{OnStart: worker.Start, OnStop: worker.Stop})
}
