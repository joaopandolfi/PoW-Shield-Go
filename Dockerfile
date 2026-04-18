FROM golang:1.22 AS build

WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/pow-shield-go .

FROM node:20 AS client-build

WORKDIR /app/client
COPY client/package.json ./
RUN npm install
COPY client/ ./
RUN npm run build

FROM debian:bookworm-slim

WORKDIR /app
COPY --from=build /out/pow-shield-go /app/pow-shield-go
COPY --from=build /app/server/wafRules.json /app/wafRules.json
COPY --from=build /app/server/wafTypes.json /app/wafTypes.json
COPY --from=client-build /app/client/public /app/client/public

ENV PORT=5656
EXPOSE 5656

CMD ["/app/pow-shield-go"]
