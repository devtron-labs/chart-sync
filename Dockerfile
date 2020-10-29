FROM golang:1.12.6-alpine3.9 AS build-env

RUN apk add --no-cache git gcc musl-dev
RUN apk add --update make
RUN go get github.com/google/wire/cmd/wire
WORKDIR /go/src/devtron.ai/chart-sync
ADD . /go/src/devtron.ai/chart-sync
RUN GOOS=linux make

FROM alpine:3.9
RUN apk add --no-cache ca-certificates
COPY --from=build-env  /go/src/devtron.ai/chart-sync/chart-sync .
CMD ["./chart-sync"]