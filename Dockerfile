FROM ubuntu:18.04
RUN apt-get update
RUN apt-get install -y ca-certificates openssl
RUN useradd auto-private-link
ADD bin/apl /
RUN chown auto-private-link /apl
USER auto-private-link
RUN chmod +x /apl
CMD ["/apl"]