FROM gradle:9.9.1-jdk25 AS build
COPY --chown=gradle:gradle . /home/gradle/src
WORKDIR /home/gradle/src
RUN mkdir -p src/main/java/example
RUN mv *.java src/main/java/example/
RUN gradle clean uberJar --no-daemon 

FROM golang:1.27-alpine3.23

RUN apk update && apk add git bash curl openjdk25-jre-headless

RUN go install github.com/nats-io/nats-server/v2@v2.15.3
RUN go install github.com/nats-io/natscli/nats@v0.5.2
RUN go install github.com/nats-io/nsc/v2@v2.15.0

COPY --from=build /home/gradle/src/build/libs/*.jar /app/example.jar
COPY main.sh main.sh

ENTRYPOINT ["bash"]
CMD ["main.sh"]
