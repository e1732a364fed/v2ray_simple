FROM golang:latest AS build
WORKDIR /go/src
RUN git clone https://github.com/e1732a364fed/v2ray_simple.git
WORKDIR /go/src/v2ray_simple
RUN git fetch --tags & git checkout $(git describe --tags `git rev-list --tags --max-count=1`)
WORKDIR /go/src/v2ray_simple/cmd/verysimple
RUN make
FROM scratch
COPY --from=build /go/src/v2ray_simple/cmd/verysimple/verysimple /bin/verysimple
WORKDIR /data
ENTRYPOINT ["/bin/verysimple"]
