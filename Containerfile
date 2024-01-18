FROM golang AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /icinga-kubernetes cmd/icinga-kubernetes/main.go

FROM scratch

COPY --from=build /icinga-kubernetes /icinga-kubernetes

CMD ["/icinga-kubernetes"]
