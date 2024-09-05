FROM golang:1.22 AS build-env

RUN apt update
RUN apt install git gcc musl-dev make -y
RUN go install github.com/google/wire/cmd/wire@latest

WORKDIR /go/src/github.com/devtron-labs/chart-sync
ADD . /go/src/github.com/devtron-labs/chart-sync
RUN GOOS=linux make

FROM ubuntu
RUN apt update
RUN apt install ca-certificates -y
RUN apt clean autoclean
RUN apt autoremove -y && rm -rf /var/lib/apt/lists/*
COPY --from=build-env  /go/src/github.com/devtron-labs/chart-sync/chart-sync .

RUN useradd -ms /bin/bash devtron
RUN chown -R devtron:devtron ./chart-sync
USER devtron

CMD ["./chart-sync"]
