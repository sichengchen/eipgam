package sessions

import (
	"os"
	"path/filepath"
	"time"
)

// Test helpers the kept agents share, moved here from the tests of agents
// that were removed.

func ageTree(root string) {
	old := time.Now().Add(-24 * time.Hour)
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil {
			os.Chtimes(p, old, old)
		}
		return nil
	})
}

// antigravitySchema is a conversation database's tables as Antigravity
// 2.23.0 makes them (yygutn's `.schema` on #2190).
const antigravitySchema = "CREATE TABLE `trajectory_meta` (`trajectory_id` text,`cascade_id` text,`trajectory_type` integer,`source` integer,PRIMARY KEY (`trajectory_id`));\n" +
	"CREATE TABLE `steps` (`idx` integer,`step_type` integer NOT NULL DEFAULT 0,`status` integer NOT NULL DEFAULT 0,`has_subtrajectory` numeric NOT NULL DEFAULT false,`metadata` blob,`error_details` blob,`permissions` blob,`task_details` blob,`render_info` blob,`step_payload` blob,`step_format` integer NOT NULL DEFAULT 0,PRIMARY KEY (`idx`));\n" +
	"CREATE INDEX `idx_steps_status` ON `steps`(`status`);\n" +
	"CREATE INDEX `idx_steps_step_type` ON `steps`(`step_type`);\n" +
	"CREATE TABLE `gen_metadata` (`idx` integer,`data` blob,`size` integer NOT NULL DEFAULT 0,PRIMARY KEY (`idx`));\n" +
	"CREATE TABLE `executor_metadata` (`idx` integer,`data` blob,PRIMARY KEY (`idx`));\n" +
	"CREATE TABLE `parent_references` (`idx` integer,`data` blob,PRIMARY KEY (`idx`));\n" +
	"CREATE TABLE `trajectory_metadata_blob` (`id` text DEFAULT \"main\",`data` blob,PRIMARY KEY (`id`));\n" +
	"CREATE TABLE `battle_mode_infos` (`idx` integer,`data` blob,PRIMARY KEY (`idx`));"
