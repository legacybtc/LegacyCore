FROM golang:1.26-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -o /legacycoind ./cmd/legacycoind && \
    CGO_ENABLED=1 go build -trimpath -o /legacycoin-cli ./cmd/legacycoin-cli

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /legacycoind /usr/local/bin/
COPY --from=builder /legacycoin-cli /usr/local/bin/
EXPOSE 19555/tcp 19556/tcp
ENTRYPOINT ["legacycoind"]