# epub-translator

Envie um EPUB, acompanhe a tradução **capítulo a capítulo** e baixe o arquivo traduzido — em [Amarra](https://github.com/puppe1990/amarra-cais) (HTML + Drive + SSE + SQLite).

- `GET /` — envio (multipart) + lista de livros com progresso
- `GET /books/{id}` — progresso ao vivo (**SSE** morfa `#book-progress` e `#chapter-list`), tabela de capítulos com status/tentativas/erro
- `POST /books/{id}/retry` — roda a fila (pendentes + falhos)
- `POST /books/{id}/chapters/{cid}/retry` — devolve um capítulo falho à fila e roda
- `GET /books/{id}/download` — reconstrói o EPUB com os capítulos já traduzidos (o que faltar sai no idioma original)
- `GET|POST /settings` — endpoint OpenAI-compatível (base URL, modelo e chave); `POST /settings/test` valida na hora

## Como funciona

1. **Upload** → `internal/epub.Parse` lê o zip: metadados (título/autor/idioma), documentos XHTML do spine (+ nav) e o miolo de `<body>` de cada um. Cada documento vira um **capítulo** em `chapters` (unidade de chamada e de retry).
2. **Tradução** → `internal/pipeline.Runner` processa a fila (pendentes + falhos, em ordem) em um goroutine por livro; cada capítulo é gravado como `running → done|failed` (com `attempts` e a mensagem de erro). Restart do servidor devolve `running` para a fila; o retry manual retoma.
3. **Validação** → `internal/translate.ValidateBody` só aceita resposta com as mesmas tags (sequência normalizada), texto realmente traduzido, tamanho plausível e sem preâmbulo; falhou? o `Service` retenta com um lembrete (3 tentativas), depois marca o capítulo como `failed`.
4. **Download** → `internal/epub.Rebuild` copia o zip original trocando apenas o miolo de `<body>` dos capítulos traduzidos e o `dc:language` do OPF; o resto (CSS, imagens, nav) passa intacto. O arquivo é montado a partir do banco na hora do download.

## Configuração do provedor

A página **/settings** grava `translate_base_url`, `translate_model` e `translate_api_key` no SQLite (a chave fica mascarada na tela). Precedência: settings do app > ambiente > padrão.

| Variável             | Padrão                           |
| -------------------- | -------------------------------- |
| `TRANSLATE_BASE_URL` | `https://api.openai.com/v1`      |
| `TRANSLATE_MODEL`    | `gpt-4.1-mini`                   |
| `TRANSLATE_API_KEY`  | (também aceita `OPENAI_API_KEY`) |

Exemplos: Gemini `https://generativelanguage.googleapis.com/v1beta/openai` + `gemini-3.5-flash-lite`; OpenRouter `https://openrouter.ai/api/v1` + `google/gemini-2.5-flash`.

> A chave é guardada em texto puro no SQLite local — é um app de máquina pessoal. Em produção, prefira a variável de ambiente e restrinja o acesso ao servidor.

## Desenvolvimento

```bash
amarra-cais install   # npm install + go mod tidy
amarra-cais dev       # http://localhost:8080 (auto-pick de porta)
amarra-cais test      # go test ./...
make ci               # test + golangci-lint + prettier --check
```

Dados ficam em `data/` (SQLite `data/app.db`, EPUBs enviados em `data/books/`), fora do git.
