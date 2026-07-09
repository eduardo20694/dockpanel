-- Setup inicial do PostgreSQL para o dockpanel (rode como superuser: postgres)
-- Depois o app cria a tabela users automaticamente no primeiro start.

CREATE USER dockpanel WITH PASSWORD 'TROQUE_SENHA';
CREATE DATABASE dockpanel OWNER dockpanel;

-- Se a tabela users já existir de setup manual:
-- ALTER TABLE public.users OWNER TO dockpanel;

-- pg_hba.conf — permitir containers Docker (ajuste a faixa se necessário):
-- host    dockpanel    dockpanel    172.16.0.0/12    scram-sha-256
--
-- UFW na VPS:
-- ufw allow from 172.16.0.0/12 to any port 5432 comment 'PostgreSQL Docker'
