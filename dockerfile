FROM golang:alpine AS build
RUN apk add --no-cache git
WORKDIR /go/src
RUN git clone https://github.com/e1732a364fed/v2ray_simple.git
WORKDIR /go/src/v2ray_simple/cmd/verysimple
RUN go build
FROM alpine
COPY --from=build /go/src/v2ray_simple/cmd/verysimple/verysimple /bin/verysimple
WORKDIR /data
ENTRYPOINT ["/bin/verysimple"]
