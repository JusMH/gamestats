FROM eclipse-temurin:21.0.12_8-jdk-alpine-3.23@sha256:52ca1500049e2e2e7cee85fc4cd8689108792dc2bde18161b7f9236f609f0dea AS build
WORKDIR /work
# Same launcher/version/checksum as sbtw.ps1, from Maven Central only.
RUN wget -q -O /tmp/sbt-launch.jar https://repo.maven.apache.org/maven2/org/scala-sbt/sbt-launch/1.11.7/sbt-launch-1.11.7.jar \
    && echo "f92a2095ac75008764fe3b2b793ffe624c4fbef5bfd9b0022e4bc2daf668c651  /tmp/sbt-launch.jar" | sha256sum -c -
COPY scala-analysis/build.sbt ./
COPY scala-analysis/project/build.properties scala-analysis/project/repositories scala-analysis/project/DockerPackaging.scala ./project/
COPY scala-analysis/src/main ./src/main
RUN java -Xmx1g -Dsbt.supershell=false -Dsbt.server.autostart=false \
    -Dsbt.override.build.repos=true -Dsbt.repository.config=/work/project/repositories \
    -jar /tmp/sbt-launch.jar dockerStage

FROM eclipse-temurin:21.0.12_8-jre-alpine-3.23@sha256:1f165f3a75613b80cc803a2ccc4d865e5c8b845b97f692988d75c4a026609e61 AS runtime
RUN addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app \
    && command -v wget
WORKDIR /app
COPY --from=build /work/target/docker-stage/ /app/
COPY docs/examples/profiles/ /app/profiles/
ENV SCALA_HOST=0.0.0.0 SCALA_PORT=9000 SCALA_PROFILES_DIR=/app/profiles
USER 10001:10001
EXPOSE 9000
HEALTHCHECK --interval=5s --timeout=3s --start-period=15s --retries=12 \
    CMD wget -q -O /dev/null -T 2 "http://127.0.0.1:${SCALA_PORT}/health" || exit 1
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=70.0", "-cp", "/app/app.jar:/app/lib/*", "gamestats.Main"]