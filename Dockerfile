FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/icinga-kubernetes/
COPY . .
RUN go build -o /go/bin/icinga-kubernetes ./cmd/icinga-kubernetes/main.go

FROM scratch

COPY --from=builder /go/bin/icinga-kubernetes /go/bin/icinga-kubernetes
EXPOSE 8080
ENTRYPOINT ["/go/bin/icinga-kubernetes"]
