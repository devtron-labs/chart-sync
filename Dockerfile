FROM golang:1.16.2-alpine AS build-env

RUN apk add --no-cache git gcc musl-dev
RUN apk add --update make
RUN go get github.com/google/wire/cmd/wire
WORKDIR /go/src/github.com/devtron-labs/chart-sync
ADD . /go/src/github.com/devtron-labs/chart-sync
RUN GO111MODULE=on GOOS=linux make

FROM alpine:3.9
RUN apk add --no-cache ca-certificates
COPY --from=build-env  /go/src/github.com/devtron-labs/chart-sync/chart-sync .
CMD ["./chart-sync"]
