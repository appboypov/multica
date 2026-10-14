-- Migration 577's index has the same key and a broader predicate that matches
-- the overdue sweep, so migration 570's open-only index is no longer read.
DROP INDEX CONCURRENTLY IF EXISTS idx_question_overdue;
