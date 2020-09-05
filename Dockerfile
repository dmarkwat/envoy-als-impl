FROM golang:1.14.8-buster

WORKDIR /build
ADD . .
RUN unset GOPATH && go build -trimpath -o envoy-als-impl .

FROM debian:buster
COPY --from=0 /build/envoy-als-impl /envoy-als-impl

ENTRYPOINT [ "/envoy-als-impl" ]
