package main

import (
	"context"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

func resolveCommentSubject(ctx context.Context, client *cli.APIClient, input, subject string) (resolvedID, error) {
	if subject == "release" {
		return resolveReleaseID(ctx, client, input)
	}
	return resolveIssueRef(ctx, client, input)
}

func newReleaseCommentCommand() *cobra.Command {
	root := &cobra.Command{Use: "comment", Short: "Read and report on a release discussion"}
	list := &cobra.Command{Use: "list <release>", Args: exactArgs(1), Short: "Read bounded release comment threads", RunE: func(cmd *cobra.Command, args []string) error { return runSubjectCommentList(cmd, args, "release") }}
	list.Flags().String("output", "json", "Output format: table or json")
	list.Flags().Bool("compact", false, "Omit echoed and bookkeeping fields from JSON")
	list.Flags().Bool("roots-only", false, "Read thread roots only")
	list.Flags().Bool("summary", false, "Clip comment bodies")
	list.Flags().Bool("full", false, "Read resolved threads in full")
	list.Flags().String("thread", "", "Read this comment thread")
	list.Flags().Int("tail", 0, "Read the newest N replies with the root")
	list.Flags().Int("recent", 0, "Read the newest N complete threads")
	list.Flags().String("since", "", "Read comments after this RFC3339 timestamp")
	list.Flags().String("before", "", "Older page timestamp cursor")
	list.Flags().String("before-id", "", "Older page UUID cursor")
	add := &cobra.Command{Use: "add <release>", Args: exactArgs(1), Short: "Post a release comment or reply", RunE: func(cmd *cobra.Command, args []string) error { return runSubjectCommentAdd(cmd, args, "release") }}
	add.Flags().String("output", "json", "Output format: table or json")
	add.Flags().String("content", "", "Comment body")
	add.Flags().String("content-file", "", "Read body from a UTF-8 file in the working directory")
	add.Flags().Bool("content-stdin", false, "Read body from standard input")
	add.Flags().String("parent", "", "Reply to a comment on this release")
	add.Flags().StringSlice("attachment", nil, "Attach local files (repeatable)")
	root.AddCommand(list, add)
	return root
}
