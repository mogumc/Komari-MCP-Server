FROM gcr.io/distroless/static-debian12

COPY komari-mcp /komari-mcp

ENV TZ=Asia/Shanghai
USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/komari-mcp"]
