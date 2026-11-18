FROM golang:1.22 AS b
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o server .
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=b /app/server ./server
COPY web ./web
COPY data ./data
CMD ["/app/server"]
