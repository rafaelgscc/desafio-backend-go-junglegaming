# Observabilidade

## Logs

A aplicação configura `log/slog` com saída JSON durante o ciclo de vida do Uber Fx.
Os fluxos financeiros registram somente metadados de rastreamento disponíveis, como
`correlationId`, `messageId`, `transactionId`, `walletId` e `providerId`, além do estado
e do erro. Credenciais, tokens e payloads financeiros completos não são registrados.

Uma divergência encontrada pela reconciliação gera log de nível `WARN` com a carteira
e a quantidade de lançamentos verificados. Falhas e retries dos workers também incluem
o identificador do worker ou da mensagem quando disponível.

## Métricas

`GET /metrics` é público para permitir scraping pelas sondas da infraestrutura e
responde no formato de exposição do Prometheus. O registro é local a cada processo;
o coletor deve agregar todas as réplicas.

| Métrica | Tipo | Significado |
| --- | --- | --- |
| `jungle_wager_results_total{status}` | counter | Resultados financeiros por estado |
| `jungle_duplicates_total{source}` | counter | Replays idempotentes em HTTP ou SQS |
| `jungle_retries_total{component}` | counter | Retries de SQS, outbox e referências |
| `jungle_sqs_dlq_total` | counter | Mensagens que falharam na tentativa limite para redrive |
| `jungle_concurrency_conflicts_total` | counter | Disputas de atualização da carteira |
| `jungle_outbox_lag_seconds` | gauge | Idade do último evento no momento da publicação |
| `jungle_processing_duration_seconds_count` | counter | Operações observadas por entrada |
| `jungle_processing_duration_seconds_sum` | counter | Tempo acumulado de processamento |
| `jungle_reconciliation_divergences_total` | counter | Reconciliações inconsistentes |

Identificadores de alta cardinalidade nunca são usados como labels.

## Health checks

- `GET /health/live` verifica somente se o processo está vivo;
- `GET /health/ready` testa PostgreSQL e as filas SQS de entrada e saída com timeout;
- `GET /metrics` não depende da disponibilidade dos serviços externos.
