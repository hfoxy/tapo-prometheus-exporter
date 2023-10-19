FROM alpine:3.18.3

ENV BIND_PORT=8080

RUN apk add --no-cache ca-certificates
RUN apk add bash

WORKDIR /app
COPY --chmod=775 tapo-prometheus-exporter ./

EXPOSE 8080
ENTRYPOINT [ "/app/tapo-prometheus-exporter" ]
