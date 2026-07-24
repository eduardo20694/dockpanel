# Build + push (no PC):
#   docker build -t redecoop/dockwatch:1.0.0 .
#   docker push redecoop/dockwatch:1.0.0

FROM golang:1.25-alpine AS backend-build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,https://goproxy.io,direct
ENV GOSUMDB=sum.golang.org
COPY backend/go.mod backend/go.sum ./
RUN for i in 1 2 3 4 5; do go mod download && break || sleep $((i * 3)); done
COPY backend/ .
RUN CGO_ENABLED=0 go build -o /dockpanel ./cmd/server

FROM node:20-alpine AS frontend-build
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM nginx:1.27-alpine
# Docker CLI + compose plugin: tela Deploy via socket montado
# Trivy: scan de imagens
ARG COMPOSE_VERSION=2.32.4
RUN apk add --no-cache ca-certificates curl docker-cli \
 && mkdir -p /usr/local/lib/docker/cli-plugins \
 && curl -fsSL "https://github.com/docker/compose/releases/download/v${COMPOSE_VERSION}/docker-compose-linux-x86_64" \
      -o /usr/local/lib/docker/cli-plugins/docker-compose \
 && chmod +x /usr/local/lib/docker/cli-plugins/docker-compose \
 && curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin \
 && apk del curl
COPY --from=backend-build /dockpanel /usr/local/bin/dockpanel
COPY --from=frontend-build /app/dist /usr/share/nginx/html
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 80
ENTRYPOINT ["/entrypoint.sh"]
