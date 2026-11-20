FROM gradle:9.9.1-jdk25 AS build
COPY --chown=gradle:gradle . /home/gradle/src
WORKDIR /home/gradle/src
RUN mkdir -p src/main/java/example
RUN mv *.java src/main/java/example/
RUN gradle clean uberJar --no-daemon 

FROM eclipse-temurin:25-jre
RUN mkdir /app
COPY --from=build /home/gradle/src/build/libs/*.jar /app/example.jar
# Bouncy Castle (used by jnats for NKeys) loads native code; allow it
# explicitly so Java 25 does not print a warning.
ENTRYPOINT ["java", "--enable-native-access=ALL-UNNAMED", "-cp", "/app/example.jar", "example.Main"]
