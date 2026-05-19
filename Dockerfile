FROM gcr.io/distroless/static-debian12

ARG TARGETOS
ARG TARGETARCH

COPY komari-mcp-${TARGETOS}-${TARGETARCH} /komari-mcp

ENV TZ=Asia/Shanghai

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/komari-mcp"]
