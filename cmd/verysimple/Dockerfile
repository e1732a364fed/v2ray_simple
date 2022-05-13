FROM golang:alpine AS builder
RUN apk update && apk add --no-cache git make
WORKDIR /build
RUN git clone https://github.com/e1732a364fed/v2ray_simple.git . &&
    make -C ./cmd/verysimple

FROM scratch
COPY --from=builder /build/cmd/verysimple/verysimple /verysimple
ENTRYPOINT ["/verysimple"]
