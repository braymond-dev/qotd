FROM golang:1.22-alpine AS build
WORKDIR /app/api
# Copy only the API module for better cache, then the rest
COPY api/go.mod .
RUN go mod download
COPY api .
RUN go mod tidy
RUN go build -o /out/server ./cmd/server

FROM alpine:3.19
ENV ADDR=:8080
WORKDIR /app
RUN apk add --no-cache postgresql-client ca-certificates
COPY --from=build /out/server /app/server
# copy migrations and scripts from repo root context
COPY db/migrations /app/migrations
COPY api/entrypoint.sh /app/entrypoint.sh
COPY api/migrate.sh /app/migrate.sh
RUN chmod +x /app/entrypoint.sh /app/migrate.sh
EXPOSE 8080
ENTRYPOINT ["/app/entrypoint.sh"]
