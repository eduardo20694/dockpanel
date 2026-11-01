# Auditoria das tools MCP contra Docker real

Ambiente: dockerd 29.1.3, socket local, containers `dp-sleep` (busybox em execução, volume `dp-data`) e `dp-exit` (exit 1), compose em `/tmp/dp-compose`. Cliente: SDK `mcp-go` in-process (`TestLiveDockerTools`).

| Tool | Funciona | Bug | Corrigido |
|---|---|---|---|
| list_containers | sim | — | — |
| list_images | sim | não existia; o painel lista imagens | adicionada |
| list_volumes | sim | não existia | adicionada |
| list_networks | sim | não existia | adicionada |
| container_logs | sim | — | — |
| container_stats | sim | não existia | adicionada |
| container_action | sim | — | — |
| diagnose_container | sim | — | — |
| scan_problems | sim | id curto podia estourar o slice | corte seguro |
| investigate_container | sim | — | — |
| security_audit | sim | — | — |
| safe_prune_report | sim | mesmo corte de id | corte seguro |
| system_overview | sim | — | — |
| executive_summary | sim | — | — |
| stack_health | sim | — | — |
| what_broke | sim | ignorava o Docker e só lia last-ship | passa a varrer problemas ao vivo |
| cost_insights | sim | não lia reservas dos containers | lê o daemon |
| list_hosts / list_environments | sim | — | — |
| list_alerts | sim | não existia | adicionada |
| search_logs | sim | — | — |
| list_log_incidents | sim | não existia | adicionada |
| log_retention | sim | não existia | adicionada |
| dockpilot_guidance | sim | — | — |
| list_users / list_teams / create_user | sim | — | — |
| list_audit | sim | — | — |
| check_compose_drift / deep_compose_drift | sim | — | — |
| deploy_compose | sim | — | — |
| create_stack | sim | — | — |
| scan_image_vulnerabilities | sim | sem binário Trivy devolve nota, sem panic | — |
| backup_volume | sim | — | — |
| docker_proxy | sim | — | — |
| kubernetes_proxy | sim | sem API configurada responde isso, sem inventar cluster | — |
| resolve_problem | sim | — | — |

HTTP streamable (`/mcp`) e SSE (`/sse`) exigem `Authorization: Bearer`. Coberto por `TestMCPHTTPKeyAuth`: chave válida, inválida, revogada e viewer sem permissão de escrita.
