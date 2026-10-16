package main

import "github.com/spf13/cobra"

func newReleaseWakeupCommand() *cobra.Command {
	root := &cobra.Command{Use: "wakeup", Short: "Set waits for the release's current owner"}
	for _, action := range []string{"create", "list", "delete"} {
		action := action
		count := 1
		use := action + " <release>"
		if action == "delete" {
			count = 2
			use += " <wakeup-id>"
		}
		cmd := &cobra.Command{Use: use, Short: action + " release wakeups", Args: exactArgs(count), RunE: func(cmd *cobra.Command, args []string) error { return runSubjectWakeup(cmd, args, action, "release") }}
		cmd.Flags().String("output", "json", "Output format: table or json")
		if action == "create" {
			cmd.Long = "Wake the release's owner at firing time, using one same-workspace issue condition or a time schedule. Shipment ends these waits."
			cmd.Flags().String("instruction", "", "Instruction for the next run")
			cmd.Flags().String("instruction-file", "", "Read instruction from a UTF-8 file")
			cmd.Flags().String("kind", "event", "event (condition), at, every or cron")
			cmd.Flags().String("mode", "", "once or continuous")
			cmd.Flags().String("parent", "", "Release comment thread for result delivery")
			cmd.Flags().String("after", "", "Delay for at, e.g. 10m")
			cmd.Flags().String("at", "", "Single RFC3339 timestamp")
			cmd.Flags().String("every", "", "Fixed interval, e.g. 1h (minimum 1m)")
			cmd.Flags().String("cron", "", "Five-field cron expression")
			cmd.Flags().String("timezone", "UTC", "IANA timezone for cron")
			cmd.Flags().String("expires-in", "", "End the rule after this duration")
			cmd.Flags().String("expires-at", "", "End the rule at this RFC3339 time")
			cmd.Flags().String("on-timeout", "", "At the deadline: end or wake")
			cmd.Flags().String("until-issue", "", "Wait for an issue in this workspace")
			cmd.Flags().String("until-issue-state", "done", "done, ended or in_review")
			cmd.Flags().Int32("max-fires", 0, "Stop repeating after this many runs (1-1000)")
		}
		root.AddCommand(cmd)
	}
	return root
}
