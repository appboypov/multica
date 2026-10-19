CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS skill_folder_workspace_daemon_path_unique ON skill_folder (workspace_id, daemon_id, path);
