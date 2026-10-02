# Eventos de integração

Os eventos de saída são gravados na tabela `outbox_events` na mesma transação SQL
que altera carteira, ledger, transação de aposta e inbox. Um worker separado reivindica
os registros com lease, publica na fila FIFO `integration-events.fifo` e só então marca
o evento como publicado.

## Envelope

Todos os eventos usam JSON em `camelCase`:

```json
{
  "eventId": "0199...",
  "eventType": "WalletBalanceChanged",
  "aggregateId": "wallet-1",
  "correlationId": "message-1",
  "causationId": "message-1",
  "occurredAt": "2026-10-02T12:00:00Z",
  "version": 1,
  "data": {}
}
```

`occurredAt` é UTC no formato RFC 3339. Valores monetários são objetos com `amount`
decimal serializado como string e `currency` em ISO 4217.

## Tipos publicados

- `WagerTransactionProcessed`: `transactionId`, `providerId`,
  `externalTransactionId`, `kind`, `money` e `balance`;
- `WagerTransactionRejected`: os campos da transação, `money` e `failureCode`;
- `WalletBalanceChanged`: `walletId`, `transactionId`, `direction`, `money`,
  `balanceBefore`, `balanceAfter` e `walletVersion`;
- `WagerTransactionPendingReference`: dados da transação, referência externa,
  número de tentativas e `nextReferenceAttemptAt`.

## Entrega, ordem e recuperação

Na fila FIFO, `MessageGroupId` recebe o `aggregateId` e
`MessageDeduplicationId` recebe o `eventId`. Isso preserva a ordem por agregado e a
identidade do evento em todas as tentativas.

A entrega é pelo menos uma vez. Se o processo cair depois da publicação e antes da
confirmação no PostgreSQL, o lease expira e o mesmo registro é publicado novamente com
o mesmo `eventId`. Consumidores devem, portanto, deduplicar por `eventId`.

Falhas de publicação liberam o lease e agendam nova tentativa com backoff exponencial.
Leases abandonados podem ser assumidos por outra instância; `FOR UPDATE SKIP LOCKED`
permite vários publicadores sem entregar simultaneamente o mesmo registro. No shutdown,
o worker termina a publicação em andamento enquanto houver prazo no contexto de parada.
