# Verificação obrigatória

Esta matriz relaciona os cenários do desafio às evidências automatizadas. Testes de
integração usam serviços reais quando as variáveis `TEST_*` correspondentes estão
configuradas.

| Requisito | Evidência principal |
| --- | --- |
| Money, overflow, escala, formato e moedas | `internal/domain/money_test.go` |
| Wallet, saldo não negativo, moeda, versão e timestamps | `internal/domain/wallet_test.go` |
| Estados e cinco operações externas | `wager_transaction_test.go` e testes `process_*` |
| Abertura interna e outbox atômica | `open_wallet_test.go` |
| Migrations, constraints e ledger append-only | testes `*_migration_test.go` |
| Commit e rollback financeiro atômico | `process_bet_postgres_test.go` |
| Inbox, lease, reentrega e recuperação | `postgres_inbox_repository_test.go` e `wager_message_postgres_test.go` |
| Outbox concorrente, ordem, lease e recuperação | `postgres_outbox_delivery_repository_test.go` |
| Referência anterior, retry e expiração | `pending_reference_worker_postgres_test.go` |
| 50 duplicatas simultâneas, um débito | `TestFiftyConcurrentDeliveriesProduceOneDebit` |
| Duas apostas de 80 sobre 100 em três instâncias | `TestThreeInstancesPreserveBalanceAndIdempotencyUnderConcurrentBets` |
| Carteiras distintas em paralelo | `TestDifferentWalletsAdvanceWhileAnotherWalletIsLocked` |
| Replay cruzando HTTP e SQS | `TestWagerTransactionHTTPPersistsBetAndReplaysOriginalResult` |
| Isolamento entre provedores | `TestHTTPContractWithPostgresAndProviderIsolation` |
| Keycloak real e identidades client credentials | `oidc_keycloak_test.go` |
| Grafo Fx, startup e shutdown | `module_test.go`, `bootstrap_oidc_test.go` e testes dos workers |
| Filas FIFO, DLQ e redrive configurado | `sqs_localstack_test.go` |
| Detector de corrida | `go test -race ./...` |

## Preparação

```bash
docker compose up -d postgres keycloak localstack migrate
```

Defina `TEST_DATABASE_URL`, `TEST_OIDC_ISSUER_URL` e `TEST_SQS_ENDPOINT` conforme o
README antes de executar `go test ./test/integration -count=1`.

## Simulações de falha

- commit antes do delete: a inbox concluída aceita a reentrega sem novo débito;
- publicação antes da confirmação: o lease expirado republica o mesmo `eventId`;
- worker interrompido: o shutdown termina o item em andamento ou libera a mensagem;
- referência ausente: o lease e o próximo retry permanecem no PostgreSQL e outra
  instância pode assumir após reinício.

Os testes que dependem de LocalStack são ignorados quando `TEST_SQS_ENDPOINT` não está
definido. Isso é visível na saída do `go test`; não representa substituição por mock.
