FROM golang:1.15 as build
RUN apt-get update
RUN apt-get install -y ca-certificates openssl
WORKDIR /build
ADD . .

RUN go build -o auto-private-link cmd/apl/main.go
RUN useradd auto-private-link
RUN cat /etc/passwd | grep auto-private-link > passwd.apl

FROM scratch
COPY --from=build /build/passwd.apl /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /build/auto-private-link .

USER auto-private-link
CMD ["./auto-private-link"]