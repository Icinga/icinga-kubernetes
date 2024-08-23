FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/icinga-kubernetes/
COPY . .
RUN go build -o /go/bin/icinga-kubernetes ./cmd/icinga-kubernetes/main.go

FROM scratch

WORKDIR /go/bin/
COPY --from=alpine /tmp /tmp
COPY --from=builder /go/bin/icinga-kubernetes ./icinga-kubernetes

ENTRYPOINT ["./icinga-kubernetes"]
