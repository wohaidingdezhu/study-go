# 第一阶段使用 Go 工具链，下载依赖并编译。
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go/go.mod go/go.mod
COPY labs/stack/go.mod labs/stack/go.sum labs/stack/
WORKDIR /src/labs/stack
RUN go mod download
COPY go/order /src/go/order
COPY labs/stack/*.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/stack .

# 第二阶段只复制编译好的程序，容器启动时由 Compose 传入 api 或 relay。
FROM alpine:3.23
COPY --from=build /out/stack /usr/local/bin/stack
USER 65534
ENTRYPOINT ["stack"]
