# Sprint 3 — Handshake com App Check (Play Integrity / App Attest)

Continuação das sprints [1](sprint1.md) e [2](sprint2.md). Formatos de header e payload em [07-contract-payloads.md](07-contract-payloads.md).

## Problema

No ataque de SMS pumping de 07/10, um script fora do app gerou requests válidas em massa. Com o contrato das sprints anteriores:

- a chave pública do WAF vinha embutida no app e podia ser extraída;
- qualquer UUID era aceito, então trocar de uuid zerava rate limit e dificuldade.

## Solução

O app não traz mais a chave. Ele recebe a chave e um uuid do WAF num **handshake**, que só responde a aparelhos aprovados pelo **Firebase App Check**: Play Integrity no Android, App Attest/DeviceCheck no iOS. Com `ATTEST_ACTIVE=true`, o contrato **só aceita uuids emitidos pelo handshake**. Um uuid inventado recebe 406, mesmo cifrado com a chave certa.

```
app                                   Firebase                     WAF
 | FirebaseAppCheck.getToken() -------->|                            |
 |<-------- App Check token (JWT) ------|                            |
 | POST /v1/handshake                                                |
 |   X-Firebase-AppCheck: <token> ---------------------------------->| verifica o JWT (JWKS do Firebase)
 |<----------------------------------- {"key","id","exp"} ----------| registra ct:a:<id> por ATTEST_ID_TTL_SECONDS
 | guarda key e id só em memória                                     |
 | X-Request-Context: enc_key("2:<id>:<ts>:<hash>[:<prefix>:<nonce>]") -> contrato: id precisa estar em ct:a:
```

## Endpoint

`POST ATTEST_PATH` (default `/v1/handshake`)

| | |
|---|---|
| Header | `ATTEST_HEADER` (default `X-Firebase-AppCheck`): token do Firebase App Check |
| 200 | `{"key":"<armored PGP public key>","id":"<uuid>","exp":<unix>}` com `Cache-Control: no-store` |
| 406 | Token ausente, inválido, expirado, de outro projeto ou de app não permitido |
| 503 | Não foi possível buscar as chaves do Firebase e não há cache |
| 500 | Erro de cache |
| 429 | Rate limit por IP, se `RATE_LIMIT_ACTIVE` |

**Verificação do token** (`services/appcheck`), sem SDK e sem credencial do Google:

- Algoritmo aceito: só RS256 com `typ` JWT. `alg: none` e HS256 são rejeitados.
- Assinatura conferida com o JWKS do Firebase. As chaves ficam em cache por 6 h e o JWKS é buscado de novo se aparecer um `kid` desconhecido, no máximo uma vez por minuto (`kid` aleatório não consegue gerar uma rajada de fetches).
- Se o Firebase estiver fora do ar, o WAF continua usando as chaves em cache.
- Claims conferidas:
  - `iss = https://firebaseappcheck.googleapis.com/<FIREBASE_PROJECT_NUMBER>`;
  - `aud` contém `projects/<FIREBASE_PROJECT_NUMBER>`;
  - `exp` no futuro;
  - `sub` (id do app) dentro de `FIREBASE_APP_IDS` (lista vazia aceita qualquer app do projeto).

**Um uuid por token:** o hash do token App Check fica guardado até o token expirar (`ct:h:<sha256>`). Repetir o mesmo token devolve o mesmo uuid. Um token tirado de um aparelho real não serve para gerar uuids em loop; para ter um uuid novo, é preciso um token novo, o que exige uma nova atestação.

**Validade do uuid:** `ATTEST_ID_TTL_SECONDS` (default 24 h). O app deve refazer o handshake antes de `exp`. Depois disso, o contrato responde 406.

## Uso no Flutter

```dart
await FirebaseAppCheck.instance.activate(
  androidProvider: AndroidProvider.playIntegrity,
  appleProvider: AppleProvider.appAttestWithDeviceCheckFallback,
);

final token = await FirebaseAppCheck.instance.getToken();
final resp = await http.post(Uri.parse('$base/v1/handshake'),
    headers: {'X-Firebase-AppCheck': token!});
final hs = jsonDecode(resp.body);   // {key, id, exp}: manter só em memória
// cifrar o contrato com hs['key'] e usar hs['id'] como uuid
```

## Configuração

| Env | Default | Descrição |
|---|---|---|
| `ATTEST_ACTIVE` | `false` | Liga o handshake e passa a exigir uuid atestado no contrato. Exige `CONTRACT_ACTIVE=true` e `FIREBASE_PROJECT_NUMBER` |
| `ATTEST_PATH` | `/v1/handshake` | Path do endpoint |
| `ATTEST_HEADER` | `X-Firebase-AppCheck` | Header com o token do App Check |
| `FIREBASE_PROJECT_NUMBER` | — | Número (não o id) do projeto Firebase |
| `FIREBASE_APP_IDS` | `[]` | JSON com os ids de app aceitos, ex. `["1:123:android:abc","1:123:ios:def"]`. Para aceitar só Android/Play Integrity, liste só o app Android |
| `ATTEST_JWKS_URL` | `https://firebaseappcheck.googleapis.com/v1/jwks` | Só muda em teste |
| `ATTEST_ID_TTL_SECONDS` | `86400` | Validade do uuid emitido |

Chaves no Redis: `ct:a:<uuid>` (uuid atestado, valor = app id) e `ct:h:<sha256 do token>` (`uuid|exp` emitido para aquele token).

## Testes

- `services/appcheck`: token válido; rejeita lixo, `alg: none`, HS256, assinatura de outra chave, `kid` desconhecido, token expirado, issuer errado, projeto errado e app não permitido. Também confere que `kid` desconhecido não dispara fetch a cada request e que JWKS fora do ar resulta em `ErrUnavailable`.
- `web/controllers/attest`: handshake completo com um Firebase falso (`services/appcheck/appchecktest`):
  - mesmo token devolve o mesmo uuid; token novo gera uuid novo;
  - a chave devolvida cifra um contrato aceito;
  - uuid inventado recebe 406.
- E2E (`make test-e2e`): o teste sobe um JWKS falso na porta 18080 do host, que os containers acessam via `host.docker.internal`.
  - Todos os clientes passam pelo handshake.
  - Handshake sem token, ou com token de outro app, recebe 406.
  - Uuid inventado com a chave certa recebe 406.
  - Todo o fluxo das sprints 1 e 2 roda com uuids atestados.

## Limitações

- **Token reaproveitado:** o token do App Check é bearer. Se for tirado de um aparelho real, ele gera um uuid válido por `ATTEST_ID_TTL_SECONDS`. A partir daí, esse uuid sofre rate limit, dificuldade e bloqueio como qualquer outro. Para a rota de SMS, considerar limited-use tokens com consumo no backend do Firebase.
- **Chave sem rotação:** a chave do WAF é a mesma para todos. Ela não está mais no binário, mas quem passa na atestação e inspeciona a memória ainda consegue lê-la. O que segura o abuso agora é o uuid atestado, não o segredo da chave. A rotação continua no backlog.
- **Aparelhos sem Google Play** (Huawei) e simulador não passam no handshake. Se precisar atendê-los, criar um caminho alternativo restrito.
- **Regra de negócio:** o filtro de DDI no backend (só +55) continua necessário. É ele que impede SMS internacional de verdade.
