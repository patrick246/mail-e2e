ARG GO_VERSION=1.24.0
FROM golang:$GO_VERSION AS builder
ENV CGO_ENABLED=0
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN mkdir ./bin && go build -o ./bin -ldflags "-s -w" ./cmd/...

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.url=https://github.com/patrick246/mail-e2e
LABEL org.opencontainers.image.source=https://github.com/patrick246/mail-e2e
LABEL org.opencontainers.image.licenses=AGPL-3.0
COPY --from=builder /app/bin/mail-e2e /
ENTRYPOINT ["/mail-e2e"]
