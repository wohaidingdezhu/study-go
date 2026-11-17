# 第一阶段用 Maven 和 JDK 编译，顺便把运行需要的依赖复制到 target。
FROM maven:3.9.11-eclipse-temurin-21 AS build
WORKDIR /src
COPY java/pom.xml .
COPY java/src src
RUN mvn -B -q package dependency:copy-dependencies -DincludeScope=runtime

# 第二阶段用 Java 运行环境，通过 classpath 同时加载自己的类和依赖。
FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /src/target/classes /app/classes
COPY --from=build /src/target/dependency /app/dependency
USER 65534
ENTRYPOINT ["java", "-cp", "/app/classes:/app/dependency/*", "study.NotificationWorker"]
