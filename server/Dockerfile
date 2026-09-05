# 多阶段构建：编译阶段用官方 Go 镜像，主版本与 go.mod 的 `go 1.25.9` 对齐
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0：静态链接，运行阶段不需要 glibc/musl 动态库
RUN CGO_ENABLED=0 go build -o /out/blog ./cmd/blog

# 运行阶段：alpine 保留 shell，方便 docker exec 进去排查问题
FROM alpine:3.20

RUN adduser -D -H -u 10001 appuser
COPY --from=build /out/blog /usr/local/bin/blog

USER appuser
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/blog"]
