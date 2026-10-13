 Eu quero adaptar esse WAF para receber 2 tipos de header, o primeiro com o id do cliente assinado em pgp com uma pubkey, nela vai ter <contract_version>:<uuid>:<timestamp> vamos usar o id para identificar o client. esse é o primeiro header que tem que ter em todas as requests. O timestamp vai ser usado para verificar se a request é muito velha, onde podemos descartar se ele estiver mais velho que X.

Logo usaremos 2 headers, o primeiro de identificação e o segundo com o challenge resolvido.

Penso também no challenge resolvido estar encriptado em pgp.

O que me diz de usar isso como token ou o back continuar retornando um jwt e esse jwt ser usado como autententicação?

Pq eu quero checar também se o client está extenuando demais esse token, se estiver, ele tem que receber um novo challenge, esse incrementado de dificuldade.