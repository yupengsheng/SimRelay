FROM golang:1.25-alpine AS builder

WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/simrelay ./cmd/simrelay

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai \
    SIMRELAY_LISTEN=:7575 \
    SIMRELAY_BAUD=115200 \
    SIMRELAY_TIMEOUT=5s

COPY --from=builder /out/simrelay /usr/local/bin/simrelay

EXPOSE 7575

ENTRYPOINT ["simrelay"]
