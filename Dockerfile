FROM golang:1.13-alpine as builder
WORKDIR $GOPATH/src/github.com/loadimpact/k6
ADD . .
RUN apk --no-cache add git
RUN go get github.com/prometheus/client_golang/prometheus && \ 
    go get github.com/prometheus/client_golang/prometheus/promauto && \ 
    go get github.com/prometheus/client_golang/prometheus/promhttp && \
    go get github.com/cloudflare/cfssl/log


RUN CGO_ENABLED=0 go install -a -trimpath -ldflags "-s -w -X github.com/loadimpact/k6/lib/consts.VersionDetails=$(date -u +"%FT%T%z")/$(git describe --always --long --dirty)"


FROM alpine:3.10
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 12345 -g 12345 k6
COPY --from=builder /go/bin/k6 /usr/bin/k6

ENTRYPOINT ["k6"]
