FROM alpine:3.18.3 AS builder

RUN apk add --no-cache ca-certificates

FROM scratch

ENV BIND_PORT=8080

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app
COPY --chmod=775 tapo-prometheus-exporter ./

EXPOSE 8080
ENTRYPOINT [ "/app/tapo-prometheus-exporter" ]
