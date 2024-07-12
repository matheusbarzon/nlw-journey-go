FROM golang:1.22.4-alpine

WORKDIR /journey

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY ./bin ./bin
COPY ./cmd ./cmd
COPY ./internal ./internal

RUN go build -o ./bin/journey ./cmd/journey

EXPOSE 8080
ENTRYPOINT [ "./bin/journey" ]
