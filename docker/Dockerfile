ARG GOPATH=/go
ARG PKG=github.com/amikhailau/medieval-game-server
ARG GODIR=$GOPATH/src/$PKG

# build the server binary
FROM golang:1.15.0 AS builder
ARG PKG
ARG GODIR
LABEL stage=server-intermediate
WORKDIR $GODIR
COPY . $GODIR
RUN go build -v -o /gobin/server ${PKG}/cmd/server
RUN ls /gobin

# copy the server binary from builder stage; run the server binary
FROM alpine:3.11 AS runner

ARG GODIR

RUN mkdir -p /lib64 && \
    ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

COPY --from=builder /gobin/server /gobin/server
COPY --from=builder ${GODIR}/test/* /test/

EXPOSE 9979
ENTRYPOINT ["/gobin/server"]
