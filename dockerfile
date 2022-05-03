FROM golang:bullseye AS build
COPY . .
WORKDIR v2ray_simple
RUN git clone https://github.com/e1732a364fed/v2ray_simple.git
RUN cd v2ray_simple/cmd/verysimple
RUN go build
FROM debian:bullseye
COPY --from=build /go/v2ray_simple/cmd/verysimple/verysimple /bin/verysimple
WORKDIR /data
ENTRYPOINT ["/bin/verysimple"]