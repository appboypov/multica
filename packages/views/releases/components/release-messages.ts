import type { TFunction } from "i18next";
import { formatVersion, type LinePreview } from "@multica/core/releases";
import type { ReleaseOwnerChangedDetails, ReleaseRefusal } from "@multica/core/types";

/**
 * The red line under Version for a preview that breaks the line: the edited
 * or planned release is too low, or a later release set by hand would be.
 */
export function problemMessage(
  t: TFunction<"releases">,
  preview: LinePreview,
  targetId: string,
): string | null {
  const problem = preview.problem;
  if (problem?.code === "release_shipped") return t(($) => $.refusal.release_shipped);
  if (!problem || problem.code !== "line_not_increasing" || !problem.release) return null;
  if (problem.release.id === targetId) {
    return problem.before
      ? t(($) => $.dialog.error_after_release, {
          version: formatVersion(problem.before.version),
          title: problem.before.title,
        })
      : t(($) => $.dialog.error_after_tag, { tag: formatVersion(problem.lastTag) });
  }
  return problem.before
    ? t(($) => $.dialog.error_later, {
        version: formatVersion(problem.release.version),
        title: problem.release.title,
        before: `${formatVersion(problem.before.version)} ${problem.before.title}`,
      })
    : t(($) => $.dialog.error_later_tag, {
        version: formatVersion(problem.release.version),
        title: problem.release.title,
        tag: formatVersion(problem.lastTag),
      });
}

/** Why Move earlier or Move later is disabled, or null when the move keeps the line. */
export function moveProblemMessage(t: TFunction<"releases">, preview: LinePreview): string | null {
  const problem = preview.problem;
  if (!problem) return null;
  switch (problem.code) {
    case "release_shipped":
      return t(($) => $.refusal.release_shipped);
    case "release_before_shipped":
      return t(($) => $.refusal.release_before_shipped);
    case "first_release":
      return t(($) => $.menu.first_release);
    case "last_release":
      return t(($) => $.menu.last_release);
    case "release_not_on_line":
      return t(($) => $.dialog.not_found);
    case "line_not_increasing":
      if (!problem.release) return null;
      return problem.before
        ? t(($) => $.menu.too_low, {
            version: formatVersion(problem.release.version),
            title: problem.release.title,
            before: `${formatVersion(problem.before.version)} ${problem.before.title}`,
          })
        : t(($) => $.menu.too_low_tag, {
            version: formatVersion(problem.release.version),
            title: problem.release.title,
            tag: formatVersion(problem.lastTag),
          });
  }
}

/** A server refusal of a release write, shown in place. */
export function refusalMessage(
  t: TFunction<"releases">,
  refusal: ReleaseRefusal,
  repository: string,
): string {
  switch (refusal.code) {
    case "release_shipped":
      return t(($) => $.refusal.release_shipped);
    case "release_before_shipped":
      return t(($) => $.refusal.release_before_shipped);
    case "title_required":
      return t(($) => $.refusal.title_required);
    case "version_or_bump":
      return t(($) => $.refusal.version_or_bump);
    case "version_invalid":
      return t(($) => $.refusal.version_invalid);
    case "last_tag_unknown":
      return t(($) => $.refusal.last_tag_unknown);
    case "first_release":
      return t(($) => $.refusal.first_release);
    case "last_release":
      return t(($) => $.refusal.last_release);
    case "does_not_release":
      return t(($) => $.refusal.does_not_release, { repository });
    case "line_not_increasing": {
      const version = refusal.version ? formatVersion(refusal.version) : "";
      const title = refusal.title ?? "";
      const before = refusal.before;
      if (before && "last_tag" in before) {
        return t(($) => $.dialog.error_later_tag, { version, title, tag: formatVersion(before.last_tag) });
      }
      if (before) {
        return t(($) => $.dialog.error_later, {
          version,
          title,
          before: `${formatVersion(before.version)} ${before.title}`,
        });
      }
      return refusal.error ?? t(($) => $.dialog.failed);
    }
    default:
      return refusal.error ?? t(($) => $.dialog.failed);
  }
}

/**
 * One owner change as worklane activity and the release page word it: set,
 * changed (instead of the previous owner) or removed. A name missing from an
 * entry falls back to the actor directory; an entry naming no owner at all is
 * not worded.
 */
export function ownerChangeMessage(
  t: TFunction<"releases">,
  actor: string,
  details: ReleaseOwnerChangedDetails,
  nameOf: (type: string, id: string) => string,
): string | null {
  const owner = details.owner_type && details.owner_id
    ? details.owner_name ?? nameOf(details.owner_type, details.owner_id)
    : null;
  const previous = details.previous_owner_type && details.previous_owner_id
    ? details.previous_owner_name ?? nameOf(details.previous_owner_type, details.previous_owner_id)
    : null;
  const release = { actor, version: formatVersion(details.version), title: details.title };
  if (owner !== null && previous !== null) {
    return t(($) => $.activity.owner_changed, { ...release, owner, previous });
  }
  if (owner !== null) return t(($) => $.activity.owner_set, { ...release, owner });
  if (previous !== null) return t(($) => $.activity.owner_removed, { ...release, owner: previous });
  return null;
}
