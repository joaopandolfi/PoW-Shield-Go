# Sprint 3 — Handshake com App Check (Play Integrity / App Attest)

Continuação das sprints [1](sprint1.md) e [2](sprint2.md). Formatos de header e payload em [07-contract-payloads.md](07-contract-payloads.md).

## Problema

No ataque de SMS pumping de 07/10, um script fora do app gerou requests válidas em massa. Com o contrato das sprints anteriores:

- a chave pública do WAF vinha embutida no app e podia ser extraída;
- qualquer UUID era aceito, então trocar de uuid zerava rate limit e dificuldade.

## Solução

O app não traz mais a chave. Ele recebe a chave e um **id token** (JWT assinado pelo WAF, com o uuid no `sub`) num **handshake**, que só responde a aparelhos aprovados pelo **Firebase App Check**: Play Integrity no Android, App Attest/DeviceCheck no iOS.

Com `ATTEST_ACTIVE=true`, o campo de id do payload do contrato passa a ser esse JWT. O WAF valida assinatura e `exp`, sem consultar tabela, e usa o `sub` como id do cliente. Um uuid solto, mesmo o emitido e cifrado com a chave certa, recebe 406.

```
app                                   Firebase                     WAF
 | FirebaseAppCheck.getToken() -------->|                            |
 |<-------- App Check token (JWT) ------|                            |
 | POST /v1/handshake                                                |
 |   X-Firebase-AppCheck: <token> ---------------------------------->| verifica o JWT (JWKS do Firebase)
 |<---------------------------- {"key","id","token","exp"} ---------| assina o id token (Ed25519)
 | guarda key e token só em memória                                  |
 | X-Request-Context: enc_key("2:<token>:<ts>:<hash>[:<prefix>:<nonce>]") -> contrato verifica o token, id = sub
```

## Endpoint

`POST ATTEST_PATH` (default `/v1/handshake`)

| | |
|---|---|
| Header | `ATTEST_HEADER` (default `X-Firebase-AppCheck`): token do Firebase App Check |
| 200 | `{"key":"<armored PGP public key>","id":"<uuid>","token":"<id token>","exp":<unix>}` com `Cache-Control: no-store` |
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

**Um id por token App Check:** o hash do token App Check fica guardado (`ct:h:<sha256>`, valor = o id token emitido) enquanto o token do App Check e o id token forem ambos válidos. Repetir o mesmo token devolve o mesmo id token. Um token tirado de um aparelho real não serve para gerar ids em loop. Esse é o único estado do fluxo, e só existe na emissão.

## Id token (JWT)

```
header : {"alg":"EdDSA","typ":"JWT","kid":"<8 bytes do sha256 da chave pública>"}
claims : {"iss":"pow-shield","sub":"<uuid>","app":"<Firebase app id>","iat":<unix>,"exp":<unix>}
```

- Assinado com Ed25519 (`ATTEST_JWT_KEY_FILE`, PEM PKCS#8). Todas as instâncias precisam da mesma chave.
- Cerca de 320 bytes. O limite do payload decriptado subiu de 512 para 1024 bytes.
- Base64url separado por `.`, sem `:`, então cabe no campo de id do payload sem mudar o formato (v1 e v2).
- **Validação no contrato, sem estado:**
  - `alg` precisa ser `EdDSA` e o `kid` precisa ser o da chave atual;
  - assinatura válida;
  - `iss` igual a `pow-shield`;
  - `exp` no futuro;
  - `sub` precisa ser um uuid.
- **O que continua com estado** (no cache, com chave pelo `sub`): rate limit por id, dificuldade progressiva, punições e bloqueio. O bloqueio também é a forma de invalidar um id antes do `exp`.
- **Validade:** `ATTEST_ID_TTL_SECONDS` (default 24 h). O app deve refazer o handshake antes de `exp`; depois disso, o contrato responde 406.
- **Gerar a chave:** `openssl genpkey -algorithm ed25519 -out id.key.pem`. Os arquivos `*.pem` de chave ficam fora do git, assim como `*.key.asc`.

## Uso no Flutter

```dart
await FirebaseAppCheck.instance.activate(
  androidProvider: AndroidProvider.playIntegrity,
  appleProvider: AppleProvider.appAttestWithDeviceCheckFallback,
);

final token = await FirebaseAppCheck.instance.getToken();
final resp = await http.post(Uri.parse('$base/v1/handshake'),
    headers: {'X-Firebase-AppCheck': token!});
final hs = jsonDecode(resp.body);   // {key, id, token, exp}: manter só em memória
// cifrar o contrato com hs['key'] e colocar hs['token'] no campo de id do payload:
// "2:${hs['token']}:<ts>:<request_hash>[:<prefix>:<nonce>]"
```

## Configuração

| Env | Default | Descrição |
|---|---|---|
| `ATTEST_ACTIVE` | `false` | Liga o handshake e passa a exigir o id token no campo de id do contrato. Exige `CONTRACT_ACTIVE=true` e `FIREBASE_PROJECT_NUMBER` |
| `ATTEST_PATH` | `/v1/handshake` | Path do endpoint |
| `ATTEST_HEADER` | `X-Firebase-AppCheck` | Header com o token do App Check |
| `FIREBASE_PROJECT_NUMBER` | — | Número (não o id) do projeto Firebase |
| `FIREBASE_APP_IDS` | `[]` | JSON com os ids de app aceitos, ex. `["1:123:android:abc","1:123:ios:def"]`. Para aceitar só Android/Play Integrity, liste só o app Android |
| `ATTEST_JWKS_URL` | `https://firebaseappcheck.googleapis.com/v1/jwks` | Só muda em teste |
| `ATTEST_ID_TTL_SECONDS` | `86400` | Validade (`exp`) do id token |
| `ATTEST_JWT_KEY_FILE` | `id.key.pem` | Chave Ed25519 (PEM PKCS#8) que assina os id tokens |

Chave no Redis: `ct:h:<sha256 do token App Check>`, com o id token emitido para aquele token.

## Testes

- `services/appcheck`: token válido; rejeita lixo, `alg: none`, HS256, assinatura de outra chave, `kid` desconhecido, token expirado, issuer errado, projeto errado e app não permitido. Também confere que `kid` desconhecido não dispara fetch a cada request e que JWKS fora do ar resulta em `ErrUnavailable`.
- `web/controllers/attest`: handshake completo com um Firebase falso (`services/appcheck/appchecktest`):
  - mesmo token devolve o mesmo id token; token novo gera id novo;
  - a chave devolvida cifra um contrato aceito com o id token;
  - uuid solto (inclusive o emitido), uuid inventado, id token assinado por outra chave e id token expirado recebem 406.
- `services/idtoken`: assinatura e verificação. Rejeita outra chave, token expirado, `sub` que não é uuid, claims trocadas, token sem assinatura e uuid puro.
- E2E (`make test-e2e`): o teste sobe um JWKS falso na porta 18080 do host, que os containers acessam via `host.docker.internal`.
  - Todos os clientes passam pelo handshake.
  - Handshake sem token, ou com token de outro app, recebe 406.
  - Uuid inventado com a chave certa recebe 406; todos os clientes usam o id token no payload.
  - Todo o fluxo das sprints 1 e 2 roda com uuids atestados.

## Dificuldade progressiva por janela

Pensada para quando o WAF fica na frente do autenticador: quanto mais requests o mesmo uuid faz dentro da janela, mais caro fica o próximo challenge.

```
dificuldade = base (ou a dificuldade de punição, se houver)
            + CONTRACT_DIFFICULTY_STEP * floor((requests_na_janela - 1) / CONTRACT_DIFFICULTY_EVERY)
            limitada a CONTRACT_MAX_DIFFICULTY
```

- **Janela:** `CONTRACT_RATE_LIMIT_WINDOW_SECONDS` (default 60 s, ou seja 1 minuto). Usa o mesmo contador atômico do rate limit por uuid (`ct:l:<uuid>:`) e zera quando a janela expira.
- **O que conta:** toda request do uuid com contrato válido, inclusive as que só pedem challenge. Pegar challenges adiantados para usar depois também encarece.
- **Teto:** limitada a `CONTRACT_MAX_DIFFICULTY` e **nunca bloqueia**, porque é carga, não abuso. Bloqueio continua vindo só das punições (solução errada, replay, token esgotado). Quem é parado pelo volume é o rate limit.
- **Exemplo** com `DEFAULT_PREFIX_SIZE=15`, `STEP=2`, `EVERY=3` e janela de 60 s: as requests 1 a 3 pedem 15 bits, 4 a 6 pedem 17, 7 a 9 pedem 19, e assim por diante até o teto.
- `CONTRACT_DIFFICULTY_STEP=0` (default) desliga.

Configuração sugerida para o autenticador:

```
CONTRACT_RATE_LIMIT_WINDOW_SECONDS=60
CONTRACT_DIFFICULTY_STEP=1
CONTRACT_DIFFICULTY_EVERY=1     # ou X
CONTRACT_MAX_USES=1             # cada chamada paga um challenge novo
CONTRACT_TOKEN_TTL_SECONDS=60
```

Com `CONTRACT_MAX_USES=1`, cada chamada ao autenticador custa duas requests (a que recebe o 401 com o challenge e a resolvida). As duas contam na janela, então ajuste `EVERY` levando isso em conta.

## Limitações

- **Token reaproveitado:** o token do App Check é bearer. Se for tirado de um aparelho real, ele gera um uuid válido por `ATTEST_ID_TTL_SECONDS`. A partir daí, esse uuid sofre rate limit, dificuldade e bloqueio como qualquer outro. Para a rota de SMS, considerar limited-use tokens com consumo no backend do Firebase.
- **Chave sem rotação:** a chave do WAF é a mesma para todos. Ela não está mais no binário, mas quem passa na atestação e inspeciona a memória ainda consegue lê-la. O que segura o abuso agora é o uuid atestado, não o segredo da chave. A rotação continua no backlog.
- **Aparelhos sem Google Play** (Huawei) e simulador não passam no handshake. Se precisar atendê-los, criar um caminho alternativo restrito.
- **Regra de negócio:** o filtro de DDI no backend (só +55) continua necessário. É ele que impede SMS internacional de verdade.
