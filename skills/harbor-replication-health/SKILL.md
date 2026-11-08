---
name: harbor-replication-health
description: Check whether Harbor replication and garbage collection are healthy - last runs, failures, schedules and unreachable registries. Use when asked if replication to another registry worked, why images are missing on a mirror, or whether storage is being reclaimed.
---

# Check replication and garbage collection in Harbor

Uses the Harbor MCP server's read tools. Nothing is changed in Harbor.

1. Call `harbor_list_replications`. Each policy shows its direction (push or pull), the remote registry, its trigger (manual, scheduled or event based), whether it is enabled, and its last run with status and task counts.
2. Call `harbor_list_registries` for the endpoints. A registry whose status is not healthy explains most failing policies.
3. Call `harbor_get_gc` for the garbage collection schedule and the last run.
4. Report:
   - policies whose last run failed or stopped, with the failed and total task counts;
   - enabled scheduled policies that have not run recently, and disabled policies;
   - unhealthy registries and the policies that use them;
   - whether garbage collection is scheduled, when it last ran and whether that run succeeded. Without it, deleted images keep using storage.
5. Suggest the next step for each problem, for example checking the destination registry's credentials or running the policy manually from the Harbor UI.

Harbor keeps per-task logs that these tools do not read; for the exact error of a failed task, point the user to the execution in the Harbor UI (Administration, Replications).
