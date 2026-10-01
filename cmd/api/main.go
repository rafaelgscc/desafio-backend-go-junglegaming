package main

import (
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/bootstrap"
)

func main() {
	fx.New(bootstrap.Module).Run()
}
