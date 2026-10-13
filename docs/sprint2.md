# Sprint 2 — Rate limit por uuid, dificuldade atômica e request binding

Continuação da [sprint 1](sprint1.md). Os formatos de header e payload estão em [07-contract-payloads.md](07-contract-payloads.md).

## 1. Rate limit por uuid

- Janela fixa por cliente: no máximo `CONTRACT_RATE_LIMIT_REQUESTS` requests em `CONTRACT_RATE_LIMIT_WINDOW_SECONDS`. Default 120 em 60 s; `0` desliga.
- Conta toda request com contrato válido de um cliente não bloqueado, com ou sem challenge.
- Ao estourar, a resposta é 429 `{"code":13,"message":"rate limit exceeded"}` com `Retry-After`. A dificuldade não muda.
- Contador atômico no cache (`Incr`, chave `ct:l:<uuid>:`). O TTL é definido só na criação, então a janela não é empurrada por requests novas. Com Redis, o limite vale somado entre todas as instâncias.
- O rate limit por IP (`RATE_LIMIT_*`) continua existindo e roda depois do contrato.

## 1b. Rate limit por IP atômico

- O limiter por IP também passou a usar o `Incr` atômico (chave `rate:<ip>`). Uma rajada paralela deixa passar exatamente `RATE_LIMIT_REQUESTS` requests. O teste `TestRateLimitAtomic` dispara 80 requests simultâneas com limite 50: passam 50 e 30 recebem 429. Com o código antigo, passavam as 80.
- **Mudança de comportamento:** antes, cada request regravava a chave e renovava o TTL. Com tráfego constante a janela nunca fechava, e o contador só zerava depois de `RATE_LIMIT_WINDOW_SECONDS` sem nenhuma request. Agora é janela fixa: o contador zera `RATE_LIMIT_WINDOW_SECONDS` depois da primeira request da janela, o que bate com o `Retry-After` enviado.
- Erro de cache no limiter por IP responde 500, como antes (falha fechada).

## 2. Dificuldade atômica por cliente

- O aumento da dificuldade virou uma única operação atômica: `Incr(ct:d:<uuid>, base, PUNISHMENT, CONTRACT_PENALTY_SECONDS)`. No Redis é um script Lua (`SET ... PX` se a chave não existe, depois `INCRBY`); na memória, usa o lock do cache.
- Punições simultâneas (por exemplo, replays em instâncias diferentes) não perdem incremento.
- **Mudança de comportamento:** a memória da dificuldade agora dura `CONTRACT_PENALTY_SECONDS` a partir da **primeira** punição, sem renovar a cada nova punição. Um cliente que continua abusando sobe até `CONTRACT_MAX_DIFFICULTY` e é bloqueado de qualquer forma.

## 3. Request binding (payload v2)

```
2:<uuid>:<unix_ts>:<request_hash>[:<prefix>:<nonce>]
request_hash = hex(sha256(METHOD "\n" REQUEST_URI "\n" BODY))
```

- O header só autoriza a request para a qual foi gerado. Um header capturado não serve para outro método, rota, query ou body: a resposta é 406.
- Somado ao replay (header idêntico), um header vazado não tem mais uso fora daquela request exata, dentro da janela de `CONTRACT_MAX_AGE_SECONDS`.
- `CONTRACT_REQUIRE_BINDING`:
  - `false` (default): aceita v1 e v2. Se vier v2, o binding é verificado.
  - `true`: exige v2, e a v1 é rejeitada com 406. Recomendado assim que o front estiver na v2.
- `REQUEST_URI` é o path com a query como trafegam (`r.URL.RequestURI()`). O front deve fazer o hash exatamente da string que coloca na URL.
- Para verificar o hash, o WAF lê o body inteiro e o devolve para os próximos middlewares, do mesmo jeito que o WAF de regras já fazia.

## 4. O que já era configurável

O pedido de configurar quantas requests o cliente faz antes de a dificuldade subir, e quanto tempo o challenge vale, já é atendido por variáveis existentes:

| Configuração | Env | Default |
|---|---|---|
| Requests por challenge resolvido antes de vir outro mais difícil | `CONTRACT_MAX_USES` | 100 |
| Validade do challenge resolvido (token), contada da solução | `CONTRACT_TOKEN_TTL_SECONDS` | 600 |
| Tempo para resolver um challenge emitido | `CONTRACT_CHALLENGE_TTL_SECONDS` | 60 |

## Configuração nova

| Env | Default | Descrição |
|---|---|---|
| `CONTRACT_RATE_LIMIT_REQUESTS` | `120` | Requests por uuid na janela (`0` desliga) |
| `CONTRACT_RATE_LIMIT_WINDOW_SECONDS` | `60` | Tamanho da janela |
| `CONTRACT_REQUIRE_BINDING` | `false` | Exige payload v2 (request binding) |

## Ordem das checagens

```
decripta/valida payload -> binding (v2 ou obrigatório) -> bloqueio -> rate limit por uuid
-> replay -> challenge/token -> [PoW antigo] -> WAF de regras -> rate limit por IP -> backend
```

## Testes

- `internal/cache/atomic_test.go`: `Incr` com 100 goroutines em memória e Redis (miniredis), com `-race`. Sem incremento perdido, partindo do valor inicial, e com TTL definido na criação.
- `web/middleware/contract_test.go`, em memória e Redis:
  - v2 sem challenge e com challenge;
  - body, path e método adulterados;
  - hash inválido;
  - v1 recusada com binding obrigatório;
  - rate limit por uuid (3ª request com limite 2 recebe 429).
- E2E (`make test-e2e`), com `CONTRACT_RATE_LIMIT_REQUESTS=20`:
  - POST com v2 pelo proxy real chega no backend;
  - body ou path adulterado recebe 406;
  - 20 requests alternando as duas instâncias passam e a 21ª recebe 429, ou seja, o limite é compartilhado via Redis.

## Arquivos

- `server/internal/cache/`: `Incr` em `cache.go`, `memory.go` e `redis.go`.
- `server/services/contract/contract.go`: payload v2 e `RequestHash`.
- `server/web/middleware/contract.go`: binding, rate limit por uuid e dificuldade atômica.
- `server/config/config.go`, `.env.example`: as três variáveis novas.
- `server/e2e/e2e_test.go`, `e2e/docker-compose.e2e.yml`: cenários novos.

## Limitações

- O binding não cobre headers da request (por exemplo `Authorization` ou `Content-Type`), só método, URI e body.
- Bodies grandes são lidos inteiros na memória para o hash. O WAF de regras já fazia isso; se virar problema, colocar um limite de tamanho do body.
- Os dois rate limits (por uuid e por IP) são de janela fixa: na virada da janela, um cliente pode fazer até 2× o limite em pouco tempo.

## Correções de teste

- `cache.Initialize` travava na 3ª chamada no mesmo processo, porque mandava num canal com buffer 2 que ninguém lia. Agora o envio não bloqueia, e um novo `Initialize` com Redis reconecta no servidor configurado. Só afetava testes repetidos (`-count=N`); em produção o `Initialize` roda uma vez.
- Os testes de cache e de rate limit limpam o cache em memória (singleton) no início, então passam com `-race -count=3`.
