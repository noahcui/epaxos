FROM golang:1.15.7
WORKDIR .
COPY src src
COPY tla+ tla+
RUN export GOPATH=/go
RUN go install master
RUN go install server
RUN go install client
ENTRYPOINT $CMD
EXPOSE $PORT
EXPOSE $P
