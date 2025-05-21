FROM golang AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o /icinga-kubernetes cmd/icinga-kubernetes/main.go

FROM scratch

COPY <<EOF /etc/group
icinga-kubernetes:x:101:
EOF

COPY <<EOF /etc/passwd
icinga-kubernetes:*:101:101::/nonexistent:/usr/sbin/nologin
EOF

COPY --from=build /icinga-kubernetes /icinga-kubernetes

USER icinga-kubernetes
CMD ["/icinga-kubernetes"]
