# 在独立模块构建 Spring Boot 的可执行 JAR，原有 Java 消费者不增加依赖。
FROM maven:3.9.11-eclipse-temurin-21 AS java21

FROM java21 AS build
WORKDIR /src
COPY labs/spring-orders/pom.xml .
COPY labs/spring-orders/src src
RUN --mount=type=cache,target=/root/.m2 mvn -B -q package

# 本机实验复用同一个 JDK 基础镜像，最终阶段只带入编译好的应用。
FROM java21 AS runtime
WORKDIR /app
COPY --from=build /src/target/spring-orders.jar .
USER 65534:65534
ENTRYPOINT ["java", "-jar", "/app/spring-orders.jar"]
