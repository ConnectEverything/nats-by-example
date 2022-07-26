FROM gradle:7.5.0-jdk8-jammy AS build
COPY --chown=gradle:gradle . /home/gradle/src
WORKDIR /home/gradle/src
RUN mv Main.java app/src/main/java/src/example
RUN gradle build --no-daemon 

FROM openjdk:8-jre-slim
RUN mkdir /app
COPY --from=build /home/gradle/src/app/build/libs/app.jar /app/example.jar
ENTRYPOINT ["java", "-cp", "/app/example.jar", "example.Main"]
