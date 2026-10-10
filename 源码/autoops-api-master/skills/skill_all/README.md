# Built-in Skill Catalog

`skill_all` is the built-in registration catalog. A future loader can read this
file or scan the sibling skill groups to register enabled skills into
`SkillRegistry`.

## Registration Order

1. `cmdb_skill`: resolve assets and inventory context first.
2. `monitor_skill`: collect alerts, metrics and pressure evidence.
3. `log_skill`: collect log evidence after target resources are resolved.
4. `k8s_skill`: collect Kubernetes evidence.
5. `db_skill`: collect database context and health evidence.
6. `knowledge_skill`: enrich answers with runbooks and incident documents.
7. `task_skill`: expose async task status and controlled runbook task flows.

## Current Built-in Skills

| Group | Skill | Status | Runtime | Main Tools |
| --- | --- | --- | --- | --- |
| cmdb_skill | host-query | enabled | declarative | get_cmdb_host, search_cmdb_hosts |
| cmdb_skill | asset-context | enabled | declarative | aiops_context_hosts |
| monitor_skill | host-pressure | enabled | declarative | get_cmdb_host, get_agent_status, query_host_metrics, query_host_services |
| monitor_skill | alert-overview | enabled | declarative | query_active_alerts, query_alert_detail |
| monitor_skill | alert-diagnosis | enabled | declarative | query_alert_detail, get_cmdb_host, query_host_metrics, query_host_logs, knowledge_search |
| log_skill | host-log-query | enabled | declarative | get_cmdb_host, query_host_logs |
| k8s_skill | k8s-context | enabled | declarative | aiops_context_k8s |
| k8s_skill | pod-diagnosis | enabled | go | AnalyzeK8sPod |
| k8s_skill | ingress-diagnosis | enabled | go | AnalyzeK8sIngress |
| db_skill | database-overview | enabled | declarative | aiops_context_database |
| db_skill | slow-sql-analysis | enabled | go | db_get_slow_sql_top |
| knowledge_skill | knowledge-search | enabled | declarative | knowledge_search |
| task_skill | async-task-status | draft | go | existing task query APIs |
| task_skill | runbook-execution | planned | declarative | knowledge_search |

## Future Loader Behavior

The loader should:

1. Scan `skills/*_skill/*/SKILL.md`.
2. Parse YAML frontmatter into a manifest.
3. Validate required fields, tool names and risk level.
4. Register enabled skills in `SkillRegistry`.
5. Keep disabled, draft and planned skills visible in the catalog but not executable.
