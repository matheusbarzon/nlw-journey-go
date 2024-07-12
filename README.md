# nlw-journey-go

Repositório de estudo do NLW Journey da trilha de Go Language

## Configs do Go:
Para utilizar alguns comandos go, como o go install é necessário que o mesmo esteja na variável de ambiente.

### Pela session do terminal
```
export GOPATH=$HOME/go
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
```
### De forma definiva:
Adicione o export no arquivo `~/.bashrc`

## env da aplicação:
Rodar o conteudo do arquivo `.env` (exemplo em `.env.example`)

## Package para criação de código em Go (boilerplate)
### **goapi-gen**  
Pegar swagger e transformar em entidades/struct do go
Instalação
```
go install github.com/discord-gophers/goapi-gen@latest
```
Uso
```
goapi-gen --out ./internal/api/spec/journey.spec.go ./internal/api/spec/journey.spec.json
```

### **tern**  
Criar tabelas no banco, ou seja, fazer o migrate
Instalação
```
go install github.com/jackc/tern/v2@latest
```
Uso
```
tern migrate --migrations ./internal/pgstore/migrations --config ./internal/pgstore/migrations/tern.conf
```

### **sqlc**  
Gerar as models da tabelas do banco
Instalação
```
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```
Uso
```
sqlc generate -f ./internal/pgstore/sqlc.yml
```

## Gerar implementação de interfaces criadas pelo sqlc
No VSCode `ctrl shift p` opção `Go: Generate Interface Stubs`.  
Informar o package com o type e o package com a interface
Exemplo:
```
api API spec.ServerInterface
```
\* Para fazer essa chama o package deve ter `type` neste padrão: 
```
type API struct {}
```

## Automatização do migrate
Dentro do `go.gen` usamos os boilerplate para facilitar manutenção e geração de novos arquivos
Exemplo do arquivo
```
//go:generate tern migrate --migrations ./internal/pgstore/migrations --config ./internal/pgstore/migrations/tern.conf
//go:generate sqlc generate -f ./internal/pgstore/sqlc.yml
```

Para chamar o `go.gen`
```
go generate ./...
```





## Comandos úteis
- Baixar dependencias que o projeto precisa para executar
```
go mod tidy
```
- Atualizar as dependencias
```
go get -u ./...
```

## Subir os container
```
docker compose up -d
```