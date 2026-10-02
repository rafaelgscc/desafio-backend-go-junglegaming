# Arquitetura

## Concorrência financeira

A coordenação financeira é feita no PostgreSQL, por carteira, sem locks globais ou
estado de sincronização em memória. Cada operação roda em uma transação `READ COMMITTED`.
O adapter carrega a operação com `SELECT ... FOR UPDATE` e, em seguida, bloqueia somente
a linha da carteira correspondente com `SELECT ... FOR UPDATE`. Operações da mesma
carteira são serializadas; carteiras diferentes continuam avançando em paralelo.

A atualização da carteira também compara a versão anterior no `UPDATE`. Esse controle
otimista é uma defesa adicional contra atualização perdida caso algum novo fluxo deixe
de adquirir o lock pessimista. Saldo, versão, estado da operação, ledger e outbox são
confirmados no mesmo commit.

Idempotência é persistente por `(provider_id, idempotency_key)` e por
`(provider_id, external_transaction_id)`. Violações concorrentes das chaves únicas,
inclusive da chave primária interna, provocam uma releitura do registro vencedor. Se
outra instância concluir uma operação entre o aceite e o processamento, a instância
perdedora devolve o resultado terminal persistido sem reaplicar a movimentação.

Os testes de integração usam três pools PostgreSQL independentes, cada um com sua
própria unidade de trabalho e seus próprios casos de uso. Eles cobrem:

- cinquenta entregas simultâneas da mesma aposta, resultando em um único débito;
- duas apostas distintas de 80.00 BRL sobre 100.00 BRL, resultando em saldo 20.00,
  uma operação processada e outra rejeitada;
- processamento de uma carteira enquanto a linha de outra permanece bloqueada;
- comparação do saldo armazenado com a soma dos créditos e débitos do ledger.

## Dinheiro

`Money` usa `int64` em centavos e carrega a moeda ISO 4217. Parsing, aritmética,
JSON e persistência financeira nunca usam ponto flutuante. A entrada externa exige
duas casas decimais, rejeita notação científica, valores especiais, negativos e
overflow. Diferenças internas podem ser negativas; saldos de carteira não podem.

PostgreSQL armazena centavos em `BIGINT` e moeda separadamente. Constraints protegem
saldo não negativo, moedas válidas e coerência dos lançamentos.

## Fronteira transacional

Os repositórios usados pelos casos financeiros recebem a mesma transação `pgx`. Estado
da aposta, carteira, ledger, conclusão da inbox e eventos da outbox são confirmados ou
revertidos juntos. A publicação SQS acontece somente depois desse commit, por um worker
independente.

## Idempotência

O hash SHA-256 é calculado sobre JSON canônico dos campos de negócio, com nomes e ordem
fixos. Metadados de transporte e a própria chave de idempotência não entram no hash.
HTTP e SQS montam o mesmo payload canônico. Reusar uma chave com outro hash é conflito;
repetir a mesma operação devolve o resultado financeiro persistido originalmente.

## Referências e reversões

`REFUND` e `ROLLBACK` resolvem a referência por provedor e ID externo e validam jogador,
carteira, rodada, moeda e valor integral. A unicidade no banco impede duas reversões
bem-sucedidas do mesmo tipo. `REFUND` devolve uma `BET`; `ROLLBACK` aplica o movimento
contrário de `BET`, `WIN` ou `REFUND`. Um débito de rollback sem saldo recebe código de
falha diferente de uma aposta sem saldo.

Referências ausentes ficam em `PENDING_REFERENCE`. Um worker com lease persistente usa
backoff exponencial e limite de tentativas. Depois do limite, a operação termina em
`REJECTED` e emite evento; referências pendentes ou terminais sem sucesso são tratadas
como não processáveis sem movimentação financeira.

## Inbox e outbox

A inbox deduplica por consumidor e mensagem, compara o hash do corpo e usa lease para
recuperação após queda. A outbox mantém um snapshot imutável do evento e permite vários
publishers via `FOR UPDATE SKIP LOCKED`. Uma queda depois do publish e antes da
confirmação pode causar republicação com o mesmo `eventId`; consumidores devem deduplicar.

## Autenticação e autorização

O Keycloak emite tokens `client_credentials`. O middleware valida assinatura, issuer,
audience e validade usando OIDC. A identidade contém exatamente um papel: provedor ou
serviço interno. Consultas são filtradas pelo provedor autenticado; rotas de carteira e
reconciliação são exclusivas do serviço interno.

## Uber Fx e shutdown

Módulos Fx compõem configuração, pool, adapters, casos de uso, handlers, servidor e
workers. Hooks iniciam e encerram componentes em ordem. No shutdown, novas entradas
param, trabalhos em andamento terminam dentro do prazo e, se cancelados, mensagens SQS
têm visibilidade liberada. Leases persistentes permitem que outra instância retome
referências e outbox; o pool fecha depois dos consumidores.

## Limitações

Tracing distribuído, dashboards e ledger de partidas dobradas não foram implementados,
pois são diferenciais opcionais. Métricas são mantidas por processo e devem ser
agregadas pelo scraper. A entrega SQS e da outbox é pelo menos uma vez.
