package templates

type Var struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Default string `json:"default,omitempty"`
	Secret  bool   `json:"secret"`
}

type Template struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Vars    []Var  `json:"vars"`
	Compose string `json:"-"`
}

func Catalog() []Template {
	return []Template{
		{ID: "postgres", Name: "Postgres", Summary: "PostgreSQL 16 com volume persistente.", Vars: []Var{
			{Key: "POSTGRES_USER", Label: "Usuário", Default: "app"},
			{Key: "POSTGRES_DB", Label: "Banco", Default: "app"},
			{Key: "POSTGRES_PORT", Label: "Porta", Default: "5432"},
			{Key: "POSTGRES_PASSWORD", Label: "Senha", Secret: true},
		}, Compose: pg},
		{ID: "redis", Name: "Redis", Summary: "Redis 7 com senha.", Vars: []Var{
			{Key: "REDIS_PORT", Label: "Porta", Default: "6379"},
			{Key: "REDIS_PASSWORD", Label: "Senha", Secret: true},
		}, Compose: redis},
		{ID: "nginx-ssl", Name: "Nginx + SSL", Summary: "Nginx com Certbot para Let's Encrypt.", Vars: []Var{
			{Key: "DOMAIN", Label: "Domínio", Default: "app.exemplo.com"},
			{Key: "LE_EMAIL", Label: "Email Let's Encrypt", Default: "ops@exemplo.com"},
		}, Compose: nginx},
		{ID: "n8n", Name: "n8n", Summary: "Automação n8n com autenticação básica.", Vars: []Var{
			{Key: "N8N_USER", Label: "Usuário", Default: "admin"},
			{Key: "WEBHOOK_URL", Label: "Webhook URL", Default: "http://localhost:5678"},
			{Key: "N8N_PASSWORD", Label: "Senha", Secret: true},
		}, Compose: n8n},
		{ID: "mysql", Name: "MySQL", Summary: "MySQL 8 com volume.", Vars: []Var{
			{Key: "MYSQL_DATABASE", Label: "Banco", Default: "app"},
			{Key: "MYSQL_USER", Label: "Usuário", Default: "app"},
			{Key: "MYSQL_PORT", Label: "Porta", Default: "3306"},
			{Key: "MYSQL_PASSWORD", Label: "Senha", Secret: true},
			{Key: "MYSQL_ROOT_PASSWORD", Label: "Senha root", Secret: true},
		}, Compose: mysql},
		{ID: "mongodb", Name: "MongoDB", Summary: "MongoDB 7 com usuário root.", Vars: []Var{
			{Key: "MONGO_USER", Label: "Usuário", Default: "root"},
			{Key: "MONGO_PORT", Label: "Porta", Default: "27017"},
			{Key: "MONGO_PASSWORD", Label: "Senha", Secret: true},
		}, Compose: mongo},
		{ID: "traefik", Name: "Traefik", Summary: "Traefik v3 com Let's Encrypt HTTP challenge.", Vars: []Var{
			{Key: "ACME_EMAIL", Label: "Email ACME", Default: "ops@exemplo.com"},
		}, Compose: traefik},
		{ID: "grafana-prometheus", Name: "Grafana + Prometheus", Summary: "Métricas locais com Grafana e Prometheus.", Vars: []Var{
			{Key: "GRAFANA_PORT", Label: "Porta do Grafana", Default: "3000"},
			{Key: "GRAFANA_USER", Label: "Usuário", Default: "admin"},
			{Key: "GRAFANA_PASSWORD", Label: "Senha", Secret: true},
		}, Compose: graf},
	}
}

func ByID(id string) (Template, bool) {
	for _, t := range Catalog() {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

const pg = `services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    ports:
      - "${POSTGRES_PORT}:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata: {}
`

const redis = `services:
  redis:
    image: redis:7
    command: ["redis-server", "--requirepass", "${REDIS_PASSWORD}"]
    ports:
      - "${REDIS_PORT}:6379"
`

const nginx = `services:
  nginx:
    image: nginx:1.27
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf:ro
      - certs:/etc/letsencrypt
      - webroot:/var/www/certbot
  certbot:
    image: certbot/certbot
    volumes:
      - certs:/etc/letsencrypt
      - webroot:/var/www/certbot
    command: certonly --webroot -w /var/www/certbot -d ${DOMAIN} --email ${LE_EMAIL} --agree-tos --non-interactive
volumes:
  certs: {}
  webroot: {}
`

const n8n = `services:
  n8n:
    image: n8nio/n8n:1.68.0
    ports:
      - "5678:5678"
    environment:
      N8N_BASIC_AUTH_ACTIVE: "true"
      N8N_BASIC_AUTH_USER: ${N8N_USER}
      N8N_BASIC_AUTH_PASSWORD: ${N8N_PASSWORD}
      WEBHOOK_URL: ${WEBHOOK_URL}
`

const mysql = `services:
  mysql:
    image: mysql:8.4
    environment:
      MYSQL_DATABASE: ${MYSQL_DATABASE}
      MYSQL_USER: ${MYSQL_USER}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
    ports:
      - "${MYSQL_PORT}:3306"
    volumes:
      - mysqldata:/var/lib/mysql
volumes:
  mysqldata: {}
`

const mongo = `services:
  mongo:
    image: mongo:7
    environment:
      MONGO_INITDB_ROOT_USERNAME: ${MONGO_USER}
      MONGO_INITDB_ROOT_PASSWORD: ${MONGO_PASSWORD}
    ports:
      - "${MONGO_PORT}:27017"
    volumes:
      - mongodata:/data/db
volumes:
  mongodata: {}
`

const traefik = `services:
  traefik:
    image: traefik:v3.1
    command:
      - --providers.docker=true
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --certificatesresolvers.le.acme.email=${ACME_EMAIL}
      - --certificatesresolvers.le.acme.storage=/letsencrypt/acme.json
      - --certificatesresolvers.le.acme.httpchallenge.entrypoint=web
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - le:/letsencrypt
volumes:
  le: {}
`

const graf = `services:
  prometheus:
    image: prom/prometheus:v2.54.1
    ports:
      - "9090:9090"
  grafana:
    image: grafana/grafana:11.2.0
    ports:
      - "${GRAFANA_PORT}:3000"
    environment:
      GF_SECURITY_ADMIN_USER: ${GRAFANA_USER}
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD}
`
