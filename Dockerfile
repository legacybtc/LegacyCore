FROM golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder
RUN apk add --no-cache gcc musl-dev linux-headers
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -o /legacycoind ./cmd/legacycoind && \
    CGO_ENABLED=1 go build -trimpath -o /legacycoin-cli ./cmd/legacycoin-cli

FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /legacycoind /usr/local/bin/
COPY --from=builder /legacycoin-cli /usr/local/bin/
EXPOSE 19555/tcp 19556/tcp
ENTRYPOINT ["legacycoind"]