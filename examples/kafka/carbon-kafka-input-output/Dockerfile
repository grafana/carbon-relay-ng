FROM debian:8

# A simple dockerfile to provide gnu netcat.
RUN apt-get update && apt-get -y install netcat && apt-get clean

ENTRYPOINT ["/bin/nc"]
CMD ["-l","-p","8080"]
