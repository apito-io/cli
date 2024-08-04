FROM alpine:latest

# RUN apk add --update --no-cache ca-certificates

WORKDIR /go/src/gitlab.com/apito.io/engine

COPY myApp /go/src/gitlab.com/apito.io/engine/

EXPOSE 5050
ENV PORT 5050

ENTRYPOINT /go/src/gitlab.com/apito.io/engine/myApp