FROM alpine:3.15.0

ARG BINARY
#RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates

COPY ${BINARY} /bin/${BINARY}
