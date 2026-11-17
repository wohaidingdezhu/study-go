# 构建阶段包含 Go 工具链，运行阶段只复制编译后的程序。
FROM golang:1.26-alpine AS build
WORKDIR /src/labs/stack
COPY go/go.mod /src/go/go.mod
COPY labs/stack/go.mod labs/stack/go.sum ./
# 先下载依赖，这样只改 Go 源码时可以复用依赖层的缓存。
RUN go mod download
COPY labs/stack/agentruntime ./agentruntime
COPY labs/stack/cmd/agent-runtime ./cmd/agent-runtime
RUN CGO_ENABLED=0 go build -trimpath -o /out/agent-runtime ./cmd/agent-runtime

FROM alpine:3.23
COPY --from=build /out/agent-runtime /usr/local/bin/agent-runtime
# 使用非 root 用户运行 API 和 worker。
USER 65534
ENTRYPOINT ["agent-runtime"]
