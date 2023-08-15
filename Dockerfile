FROM paulcager/go-base:latest as build
WORKDIR /app

COPY *.go go.mod ./
RUN go mod tidy && go mod download && CGO_ENABLED=0 go build -o /dump1090_exporter && go test

FROM scratch
WORKDIR /app
COPY --from=build /dump1090_exporter ./
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 9799
CMD ["/app/dump1090_exporter", "--dump1090.files=/dev/shm/rbfeeder_%s",  "--web.disable-exporter-metrics"]

