package bootstrap

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/application"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
	httpadapter "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/http"
	platformpostgres "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/postgres"
	sqsadapter "github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/sqs"
)

var Module = fx.Module(
	"bootstrap",
	httpadapter.Module,
	sqsadapter.Module,
	fx.Provide(
		config.LoadPostgresConfig,
		providePostgresPool,
		provideDatabaseHealthChecker,
		fx.Annotate(
			platformpostgres.NewPostgresWageringUnitOfWork,
			fx.As(new(application.WageringUnitOfWork)),
		),
		fx.Annotate(
			platformpostgres.NewWageringQueryRepository,
			fx.As(new(application.WageringQueryRepository)),
		),
		application.NewWageringQueryService,
		fx.Annotate(
			platformpostgres.NewInboxRepository,
			fx.As(new(application.InboxRepository)),
		),
		application.NewOpenWalletUseCase,
		application.NewSubmitWagerTransactionUseCase,
		application.NewProcessBetUseCase,
		application.NewProcessWinUseCase,
		application.NewProcessLossUseCase,
		fx.Annotate(
			application.NewProcessRefundUseCase,
			fx.ResultTags(`name:"process_refund"`),
		),
		fx.Annotate(
			application.NewProcessRollbackUseCase,
			fx.ResultTags(`name:"process_rollback"`),
		),
		provideExecuteWagerTransactionUseCase,
		provideWagerMessageExecutor,
		application.NewConsumeWagerMessageUseCase,
	),
)

func provideWagerMessageExecutor(
	useCase *application.ExecuteWagerTransactionUseCase,
) application.WagerMessageExecutor {
	return useCase
}

type executeWagerTransactionDependencies struct {
	fx.In

	Submit   *application.SubmitWagerTransactionUseCase
	Bet      *application.ProcessBetUseCase
	Win      *application.ProcessWinUseCase
	Loss     *application.ProcessLossUseCase
	Refund   *application.ProcessReversalUseCase `name:"process_refund"`
	Rollback *application.ProcessReversalUseCase `name:"process_rollback"`
}

func provideExecuteWagerTransactionUseCase(
	dependencies executeWagerTransactionDependencies,
) (*application.ExecuteWagerTransactionUseCase, error) {
	return application.NewExecuteWagerTransactionUseCase(
		dependencies.Submit,
		dependencies.Bet,
		dependencies.Win,
		dependencies.Loss,
		dependencies.Refund,
		dependencies.Rollback,
	)
}

func provideDatabaseHealthChecker(
	pool *pgxpool.Pool,
	transport *sqsadapter.Transport,
) httpadapter.DatabaseHealthChecker {
	return serviceHealthChecker{database: pool, sqs: transport}
}

type serviceHealthChecker struct {
	database *pgxpool.Pool
	sqs      *sqsadapter.Transport
}

func (checker serviceHealthChecker) Ping(ctx context.Context) error {
	if err := checker.database.Ping(ctx); err != nil {
		return err
	}
	return checker.sqs.Ping(ctx)
}

func providePostgresPool(
	lifecycle fx.Lifecycle,
	postgresConfig config.PostgresConfig,
) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := platformpostgres.OpenPool(ctx, platformpostgres.PoolConfig{
		DatabaseURL:       postgresConfig.DatabaseURL,
		MaxConnections:    postgresConfig.MaxConnections,
		MinConnections:    postgresConfig.MinConnections,
		MaxConnectionIdle: postgresConfig.MaxConnectionIdle,
		MaxConnectionLife: postgresConfig.MaxConnectionLife,
	})
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
