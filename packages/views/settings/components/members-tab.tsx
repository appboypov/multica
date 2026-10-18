"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  Building2,
  Check,
  Copy,
  Crown,
  Eye,
  Link,
  Loader2,
  Mail,
  MoreHorizontal,
  Plus,
  Search,
  Shield,
  Trash2,
  User,
  UserMinus,
  X,
} from "lucide-react";
import { ActorAvatar } from "../../common/actor-avatar";
import { AppLink, useOptionalNavigation } from "../../navigation";
import type {
  Invitation,
  MemberRole,
  StaffRole,
  MemberWithUser,
  PurchaseWorkspaceSeatsRequest,
  ShareLink,
  WorkspaceSeatPurchasePreview,
} from "@multica/core/types";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Label } from "@multica/ui/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Badge } from "@multica/ui/components/ui/badge";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from "@multica/ui/components/ui/alert-dialog";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@multica/ui/components/ui/select";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
} from "@multica/ui/components/ui/dropdown-menu";
import { toast } from "sonner";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import {
  usePreviewWorkspaceSeatPurchase,
  usePurchaseWorkspaceSeats,
  workspaceSubscriptionSummaryOptions,
} from "@multica/core/billing";
import { useWorkspaceId } from "@multica/core/hooks";
import { useFeatureEnabled } from "@multica/core/config";
import { BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG } from "@multica/core/feature-flags";
import { useCurrentWorkspace } from "@multica/core/paths";
import {
  invitationListOptions,
  memberListOptions,
  shareLinkListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { toggleClientOrganisation, useOrganisations } from "@multica/core/organisations";
import { api, errorCode } from "@multica/core/api";
import { useLocale, useT } from "../../i18n";
import { OrganisationMultiSelect } from "../../projects/components/organisation-multi-select";
import {
  SettingsCard,
  SettingsReadOnlyNotice,
  SettingsTab,
  SettingsViewTabs,
} from "./settings-layout";
import { resolveSettingsLocation, settingsHref } from "./settings-navigation";
import { formatStripeMinorAmount } from "./billing-format";
import {
  isSingleSeatInvitePreview,
  purchasedSeatIsReadyForInvitation,
  seatInvitationCanRetryAfterPurchase,
  seatInvitationCapacityFailure,
  seatPurchaseCanRetryWithSameQuote,
  seatPurchaseMatchesPreview,
} from "./seat-invite-purchase";

const SEAT_PURCHASE_CONFIRM_TIMEOUT_MS = 2 * 60_000;

type MembersView = "members" | "invitations" | "links";

type InviteSeatPurchase = {
  workspaceId: string;
  email: string;
  role: MemberRole;
  organisationIds: string[];
  preview: WorkspaceSeatPurchasePreview;
  idempotencyKey: string;
  phase: "review" | "purchasing" | "waiting" | "inviting" | "error";
  submittedAt?: number;
  error?: string;
  retryable?: boolean;
};

function createSeatPurchaseKey(workspaceId: string): string {
  const suffix =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  return `invite-seat-${workspaceId}-${suffix}`.slice(0, 200);
}

const ROLE_ICONS: Record<MemberRole, typeof Crown> = {
  owner: Crown,
  admin: Shield,
  member: User,
  client: Eye,
};

// Builds the shareable URL for a share-link invite. Prefers the navigation
// adapter's getShareableUrl (works on desktop where window.location.origin is
// not the public web origin), falling back to the browser origin on web.
function buildShareLinkUrl(
  navigation: ReturnType<typeof useOptionalNavigation>,
  code: string,
): string {
  const joinPath = `/join?code=${code}`;
  if (navigation?.getShareableUrl) {
    return navigation.getShareableUrl(joinPath);
  }
  return `${typeof window !== "undefined" ? window.location.origin : ""}${joinPath}`;
}

function useRoleLabels() {
  const { t } = useT("settings");
  return {
    owner: {
      label: t(($) => $.members.roles.owner.label),
      description: t(($) => $.members.roles.owner.description),
      icon: ROLE_ICONS.owner,
    },
    admin: {
      label: t(($) => $.members.roles.admin.label),
      description: t(($) => $.members.roles.admin.description),
      icon: ROLE_ICONS.admin,
    },
    member: {
      label: t(($) => $.members.roles.member.label),
      description: t(($) => $.members.roles.member.description),
      icon: ROLE_ICONS.member,
    },
    client: {
      label: t(($) => $.members.roles.client.label),
      description: t(($) => $.members.roles.client.description),
      icon: ROLE_ICONS.client,
    },
  } as const;
}

/** The organisations of a client, one badge each. Unknown ids are skipped. Long names truncate and keep their full name as the title. */
function OrganisationBadges({ organisationIds }: { organisationIds: readonly string[] }) {
  const wsId = useWorkspaceId();
  const { data: organisations = [] } = useOrganisations(wsId);
  return organisations
    .filter((o) => organisationIds.includes(o.id))
    .map((o) => (
      <Badge key={o.id} variant="secondary" className="min-w-0 max-w-full" title={o.name}>
        <Building2 aria-hidden="true" />
        <span className="truncate">{o.name}</span>
      </Badge>
    ));
}

function MemberRow({
  member,
  canManage,
  canManageOwners,
  ownerCount,
  isSelf,
  busy,
  onRoleChange,
  onOrganisationsChange,
  onRemove,
}: {
  member: MemberWithUser;
  canManage: boolean;
  canManageOwners: boolean;
  /** Total number of owners in this workspace — needed to gate demoting the
   *  last owner per `workspace.go:497-507`. */
  ownerCount: number;
  isSelf: boolean;
  busy: boolean;
  onRoleChange: (role: MemberRole) => void;
  onRemove: () => void;
  onOrganisationsChange: (organisationIds: string[]) => void;
}) {
  const { t } = useT("settings");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  const { data: organisations = [] } = useOrganisations(wsId);
  const roleConfig = useRoleLabels();
  const rc = roleConfig[member.role];
  const RoleIcon = rc.icon;
  const canEditRole = canManage && !isSelf && (member.role !== "owner" || canManageOwners);
  const canRemove = canManage && !isSelf && (member.role !== "owner" || canManageOwners);
  const isLastOwner = member.role === "owner" && ownerCount <= 1;
  const showMenu = canEditRole || canRemove;

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <ActorAvatar actorType="member" actorId={member.user_id} size="lg" />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <span className="min-w-0 max-w-full truncate text-body font-medium">{member.name}</span>
          {isSelf ? (
            <Badge variant="secondary" className="shrink-0">
              {t(($) => $.members.you)}
            </Badge>
          ) : null}
          <OrganisationBadges organisationIds={member.organisation_ids} />
        </div>
        <div className="truncate text-caption text-muted-foreground">{member.email}</div>
      </div>
      <span className="hidden w-32 shrink-0 text-caption tabular-nums text-muted-foreground md:block">
        {t(($) => $.members.joined, {
          date: new Date(member.created_at).toLocaleDateString(locale),
        })}
      </span>
      <span className="flex w-24 shrink-0 items-center gap-1.5 text-body">
        <RoleIcon className="size-3.5 text-muted-foreground" aria-hidden="true" />
        {rc.label}
      </span>
      <span className="flex size-7 shrink-0 items-center justify-center">
      {showMenu && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                disabled={busy}
                aria-label={t(($) => $.members.member_actions, { name: member.name })}
              >
                <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
              </Button>
            }
          />
          <DropdownMenuContent align="end" className="w-auto">
            {canEditRole && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <Shield className="h-3.5 w-3.5" />
                  {t(($) => $.members.change_role)}
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="w-auto">
                  {(Object.entries(roleConfig) as [MemberRole, (typeof roleConfig)[MemberRole]][]).map(
                    ([role, config]) => {
                      if (role === "owner" && !canManageOwners) return null;
                      if (role === "client" && member.role !== "client") return null;
                      const Icon = config.icon;
                      const wouldDemoteLastOwner =
                        isLastOwner && role !== "owner";
                      // A client row's Client option is its current role. Organisations changes it.
                      const unavailable = wouldDemoteLastOwner || role === "client";
                      return (
                        <DropdownMenuItem
                          key={role}
                          onClick={() =>
                            unavailable ? undefined : onRoleChange(role)
                          }
                          disabled={unavailable}
                          title={
                            wouldDemoteLastOwner
                              ? t(($) => $.members.cannot_demote_last_owner_title)
                              : undefined
                          }
                        >
                          <Icon className="h-3.5 w-3.5" />
                          <div className="flex flex-col">
                            <span>{config.label}</span>
                            <span className="text-caption text-muted-foreground font-normal">
                              {wouldDemoteLastOwner
                                ? t(($) => $.members.cannot_demote_last_owner)
                                : config.description}
                            </span>
                          </div>
                          {member.role === role && (
                            <span className="ml-auto text-caption text-muted-foreground">{"✓"}</span>
                          )}
                        </DropdownMenuItem>
                      );
                    }
                  )}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
            {canEditRole && member.role === "client" && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <Building2 className="h-3.5 w-3.5" />
                  {t(($) => $.members.organisations_label)}
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="w-56">
                  {organisations.map((organisation) => {
                    const held = member.organisation_ids.includes(organisation.id);
                    const isLast = held && member.organisation_ids.length === 1;
                    return (
                      <DropdownMenuItem
                        key={organisation.id}
                        disabled={isLast}
                        title={isLast ? t(($) => $.members.last_organisation_title) : undefined}
                        onClick={() =>
                          onOrganisationsChange(
                            toggleClientOrganisation(member.organisation_ids, organisation.id),
                          )
                        }
                      >
                        <Building2 className="h-3.5 w-3.5" />
                        <span className="min-w-0 flex-1 truncate">{organisation.name}</span>
                        {held && <Check className="size-3.5 shrink-0" />}
                      </DropdownMenuItem>
                    );
                  })}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
            {canEditRole && canRemove && <DropdownMenuSeparator />}
            {canRemove && (
              <DropdownMenuItem variant="destructive" onClick={onRemove}>
                <UserMinus className="h-3.5 w-3.5" />
                {t(($) => $.members.remove_action)}
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      </span>
    </div>
  );
}

function InvitationRow({
  invitation,
  canManage,
  onRevoke,
  busy,
}: {
  invitation: Invitation;
  canManage: boolean;
  onRevoke: () => void;
  busy: boolean;
}) {
  const { t } = useT("settings");
  const locale = useLocale();
  const roleConfig = useRoleLabels();
  const rc = roleConfig[invitation.role];

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted">
        <Mail className="size-4 text-muted-foreground" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <span className="min-w-0 max-w-full truncate text-body font-medium">{invitation.invitee_email}</span>
          <OrganisationBadges organisationIds={invitation.organisation_ids} />
        </div>
        <div className="truncate text-caption text-muted-foreground">
          {t(($) => $.members.invitation_meta, {
            sent: new Date(invitation.created_at).toLocaleDateString(locale),
            expires: new Date(invitation.expires_at).toLocaleDateString(locale),
          })}
        </div>
      </div>
      <span className="w-24 shrink-0 text-body">{rc.label}</span>
      <span className="flex size-7 shrink-0 items-center justify-center">
        {canManage && (
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={busy}
            onClick={onRevoke}
            aria-label={t(($) => $.members.revoke_invitation_aria, {
              email: invitation.invitee_email,
            })}
          >
            <X className="size-4 text-muted-foreground" />
          </Button>
        )}
      </span>
    </div>
  );
}

function ShareLinkRow({
  link,
  onRevoke,
  busy,
  onCopy,
}: {
  link: ShareLink;
  onRevoke: () => void;
  busy: boolean;
  onCopy: () => void;
}) {
  const { t } = useT("settings");
  const locale = useLocale();
  const roleConfig = useRoleLabels();
  const rc = roleConfig[link.role];
  const navigation = useOptionalNavigation();
  const joinUrl = buildShareLinkUrl(navigation, link.code);

  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted">
        <Link className="size-4 text-muted-foreground" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-1 text-body font-medium">
          <span>{t(($) => $.members.share_link_uses, { used: link.use_count, max: link.max_uses ?? "∞" })}</span>
          {link.expires_at && <span>· {t(($) => $.members.share_link_expires, { date: new Date(link.expires_at).toLocaleDateString(locale) })}</span>}
        </div>
        <div
          className="truncate font-mono text-caption text-muted-foreground"
          title={joinUrl}
        >
          {joinUrl}
        </div>
      </div>
      <span className="w-24 shrink-0 text-body">{rc.label}</span>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={onCopy}
        aria-label={t(($) => $.members.share_link_copy_tooltip)}
      >
        <Copy className="size-4 text-muted-foreground" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        disabled={busy}
        onClick={onRevoke}
        aria-label={t(($) => $.members.share_link_revoke_tooltip)}
      >
        <Trash2 className="size-4 text-muted-foreground" />
      </Button>
    </div>
  );
}

export function MembersTab() {
  const { t } = useT("settings");
  const { t: billingT } = useT("billing");
  const locale = useLocale();
  const roleConfig = useRoleLabels();
  const user = useAuthStore((s) => s.user);
  const workspace = useCurrentWorkspace();
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const navigation = useOptionalNavigation();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: invitations = [] } = useQuery(invitationListOptions(wsId));

  const [inviteOpen, setInviteOpen] = useState(false);
  const [shareLinkOpen, setShareLinkOpen] = useState(false);
  const requestedView = resolveSettingsLocation(
    navigation?.searchParams ?? new URLSearchParams(),
  ).section;
  const urlView: MembersView =
    requestedView === "invitations" || requestedView === "links"
      ? requestedView
      : "members";
  const [localView, setLocalView] = useState<MembersView>(urlView);
  // With a router the URL owns the open list, so search results that land on
  // this page and back/forward switch it too. Without one it is local state.
  const view = navigation ? urlView : localView;
  const setView = (next: MembersView) => {
    if (!navigation) {
      setLocalView(next);
      return;
    }
    navigation.replace(
      settingsHref(navigation.pathname, navigation.searchParams, "members", {
        section: next === "members" ? undefined : next,
      }),
    );
  };
  const [memberQuery, setMemberQuery] = useState("");
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<MemberRole>("member");
  const [inviteOrganisationIds, setInviteOrganisationIds] = useState<string[]>([]);
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteSeatPurchase, setInviteSeatPurchase] =
    useState<InviteSeatPurchase | null>(null);
  const dispatchedInvitePurchaseKey = useRef<string | null>(null);
  const [memberActionId, setMemberActionId] = useState<string | null>(null);
  const [invitationActionId, setInvitationActionId] = useState<string | null>(null);
  const [shareLinkActionId, setShareLinkActionId] = useState<string | null>(null);
  const [shareLinkLoading, setShareLinkLoading] = useState(false);
  const [shareLinkRole, setShareLinkRole] = useState<StaffRole>("member");
  const [shareLinkExpiry, setShareLinkExpiry] = useState<string>("168"); // default 7 days
  const [confirmAction, setConfirmAction] = useState<{
    title: string;
    description: string;
    variant?: "destructive";
    onConfirm: () => Promise<void>;
  } | null>(null);
  const previewSeatPurchase = usePreviewWorkspaceSeatPurchase();
  const purchaseSeats = usePurchaseWorkspaceSeats(wsId);
  const seatPurchaseSummary = useQuery({
    ...workspaceSubscriptionSummaryOptions(wsId),
    enabled:
      inviteSeatPurchase?.phase === "waiting" &&
      inviteSeatPurchase.workspaceId === wsId,
    staleTime: 0,
    refetchInterval:
      inviteSeatPurchase?.phase === "waiting" ? 2_000 : false,
  });

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManageWorkspace = currentMember?.role === "owner" || currentMember?.role === "admin";
  const isOwner = currentMember?.role === "owner";
  const ownerCount = members.filter((m) => m.role === "owner").length;
  // Only owners/admins may list share links; skip the request for plain
  // members (the server would 403) once the current member's role is known.
  const { data: shareLinks = [] } = useQuery(shareLinkListOptions(wsId, canManageWorkspace));
  const billingEnabled = useFeatureEnabled(BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG, false);
  const seats = useQuery({
    ...workspaceSubscriptionSummaryOptions(wsId),
    enabled: billingEnabled && canManageWorkspace,
  }).data?.seatCapacity;
  const needle = memberQuery.trim().toLocaleLowerCase();
  const visibleMembers = needle
    ? members.filter(
        (m) =>
          m.name.toLocaleLowerCase().includes(needle) ||
          m.email.toLocaleLowerCase().includes(needle),
      )
    : members;

  const sendInvitation = useCallback(
    async (email: string, role: MemberRole, organisationIds: string[]) => {
      if (!workspace) return;
      await api.createMember(
        workspace.id,
        role === "client" ? { email, role, organisation_ids: organisationIds } : { email, role },
      );
      setInviteEmail("");
      setInviteRole("member");
      setInviteOrganisationIds([]);
      await qc.invalidateQueries({
        queryKey: workspaceKeys.invitations(wsId),
      });
      toast.success(t(($) => $.members.toast_invitation_sent));
    },
    [qc, t, workspace, wsId],
  );

  const handleInviteMember = async () => {
    if (!workspace) return;
    const email = inviteEmail.trim();
    const role = inviteRole;
    const organisationIds = inviteOrganisationIds;
    setInviteLoading(true);
    try {
      await sendInvitation(email, role, organisationIds);
      setInviteOpen(false);
    } catch (e) {
      const code = errorCode(e);
      const capacityFailure = seatInvitationCapacityFailure(code);
      if (code === "seat_capacity_overcommitted") {
        try {
          const summary = await qc.fetchQuery({
            ...workspaceSubscriptionSummaryOptions(wsId),
            staleTime: 0,
          });
          const capacity = summary?.seatCapacity;
          toast.error(
            billingT(($) => $.workspace.seats.members_over_capacity_title),
            capacity
              ? {
                  description: billingT(
                    ($) => $.workspace.seats.occupancy_over_capacity_description,
                    {
                      // Cloud refuses on used + reserved, so pending
                      // invitations have to appear here: the most common
                      // trigger is a first ledger snapshot whose member count
                      // alone still fits inside the purchased seats.
                      occupied: capacity.used + capacity.reserved,
                      purchased: capacity.purchased,
                      members: capacity.used,
                      reserved: capacity.reserved,
                    },
                  ),
                }
              : undefined,
          );
        } catch {
          toast.error(
            billingT(($) => $.workspace.seats.members_over_capacity_title),
          );
        }
        return;
      }
      if (capacityFailure === "full") {
        try {
          const preview = await previewSeatPurchase.mutateAsync({
            additionalSeats: 1,
          });
          if (!isSingleSeatInvitePreview(preview)) {
            toast.error(
              billingT(($) => $.workspace.seat_purchase.preview_unreadable),
            );
            return;
          }
          // Hand over to the purchase confirmation rather than stacking a
          // second modal on the invite dialog.
          setInviteOpen(false);
          setInviteSeatPurchase({
            workspaceId: workspace.id,
            email,
            role,
            organisationIds,
            preview,
            idempotencyKey: createSeatPurchaseKey(workspace.id),
            phase: "review",
          });
          dispatchedInvitePurchaseKey.current = null;
        } catch (previewError) {
          toast.error(
            errorCode(previewError) === "seat_purchase_in_progress"
              ? billingT(($) => $.workspace.seat_purchase.in_progress)
              : billingT(($) => $.workspace.seat_purchase.preview_failed),
          );
        }
        return;
      }
      if (capacityFailure === "unavailable") {
        toast.error(t(($) => $.members.toast_seat_capacity_unavailable));
        return;
      }
      if (capacityFailure === "rate_limited") {
        toast.error(t(($) => $.members.toast_seat_capacity_rate_limited));
        return;
      }
      toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_invitation_failed));
    } finally {
      setInviteLoading(false);
    }
  };

  const handlePurchaseSeatAndInvite = async () => {
    if (
      !inviteSeatPurchase ||
      (inviteSeatPurchase.phase !== "review" &&
        !(inviteSeatPurchase.phase === "error" && inviteSeatPurchase.retryable))
    ) {
      return;
    }
    const current = inviteSeatPurchase;
    if (!workspace || current.workspaceId !== workspace.id) {
      setInviteSeatPurchase(null);
      return;
    }
    // A transient failure may happen after the purchase succeeded and the
    // first invitation dispatch ran. Reusing the same idempotency key is safe,
    // but the dispatch guard must be reopened for the retried invitation.
    dispatchedInvitePurchaseKey.current = null;
    setInviteSeatPurchase({
      ...current,
      phase: "purchasing",
      error: undefined,
      retryable: false,
    });
    const request: PurchaseWorkspaceSeatsRequest = {
      additionalSeats: current.preview.additionalSeats,
      expectedCurrentSeats: current.preview.currentSeats,
      expectedPurchaseVersion: current.preview.purchaseVersion,
      acceptedProrationAmount: current.preview.prorationAmount,
      currency: current.preview.currency,
      idempotencyKey: current.idempotencyKey,
    };
    try {
      const response = await purchaseSeats.mutateAsync(request);
      if (!seatPurchaseMatchesPreview(response, current.preview)) {
        setInviteSeatPurchase({
          ...current,
          phase: "error",
          error: billingT(
            ($) => $.workspace.seat_purchase.purchase_unreadable,
          ),
          retryable: true,
        });
        return;
      }
      const submittedAt = Date.now();
      setInviteSeatPurchase({
        ...current,
        phase: "waiting",
        submittedAt,
        error: undefined,
        retryable: false,
      });
      await qc.invalidateQueries({
        queryKey: workspaceSubscriptionSummaryOptions(wsId).queryKey,
      });
    } catch (error) {
      const code = errorCode(error);
      const message =
        code === "seat_purchase_payment_failed"
          ? billingT(($) => $.workspace.seat_purchase.payment_failed)
          : code === "seat_purchase_in_progress"
            ? billingT(($) => $.workspace.seat_purchase.in_progress)
            : code === "seat_quote_changed" || code === "seat_capacity_changed"
              ? billingT(($) => $.workspace.seat_purchase.quote_changed)
              : billingT(($) => $.workspace.seat_purchase.purchase_failed);
      setInviteSeatPurchase({
        ...current,
        phase: "error",
        error: message,
        retryable: seatPurchaseCanRetryWithSameQuote(code),
      });
    }
  };

  useEffect(() => {
    setInviteSeatPurchase((current) =>
      current && current.workspaceId !== wsId ? null : current,
    );
  }, [wsId]);

  useEffect(() => {
    const purchase = inviteSeatPurchase;
    if (
      purchase?.phase !== "waiting" ||
      purchase.workspaceId !== wsId ||
      purchase.submittedAt == null ||
      dispatchedInvitePurchaseKey.current === purchase.idempotencyKey ||
      !purchasedSeatIsReadyForInvitation(
        seatPurchaseSummary.data,
        purchase.preview,
        purchase.submittedAt,
        seatPurchaseSummary.dataUpdatedAt,
      )
    ) {
      return;
    }

    dispatchedInvitePurchaseKey.current = purchase.idempotencyKey;
    setInviteSeatPurchase({ ...purchase, phase: "inviting" });
    void sendInvitation(purchase.email, purchase.role, purchase.organisationIds)
      .then(() => setInviteSeatPurchase(null))
      .catch((error) => {
        const code = errorCode(error);
        const capacityFailure = seatInvitationCapacityFailure(code);
        let message: string;
        switch (capacityFailure) {
          case "full":
            message = t(($) => $.members.seat_purchase_capacity_taken);
            break;
          case "unavailable":
            message = t(($) => $.members.toast_seat_capacity_unavailable);
            break;
          case "rate_limited":
            message = t(($) => $.members.toast_seat_capacity_rate_limited);
            break;
          default:
            message =
              error instanceof Error
                ? error.message
                : t(($) => $.members.toast_invitation_failed);
        }
        setInviteSeatPurchase({
          ...purchase,
          phase: "error",
          retryable: seatInvitationCanRetryAfterPurchase(code),
          error: message,
        });
      });
  }, [
    inviteSeatPurchase,
    seatPurchaseSummary.data,
    seatPurchaseSummary.dataUpdatedAt,
    sendInvitation,
    t,
    wsId,
  ]);

  useEffect(() => {
    if (
      inviteSeatPurchase?.phase !== "waiting" ||
      inviteSeatPurchase.submittedAt == null
    ) {
      return;
    }
    const elapsed = Date.now() - inviteSeatPurchase.submittedAt;
    const timeout = window.setTimeout(() => {
      setInviteSeatPurchase((current) =>
        current?.phase === "waiting"
          ? {
              ...current,
              phase: "error",
              error: t(($) => $.members.seat_purchase_timeout),
              retryable: false,
            }
          : current,
      );
    }, Math.max(0, SEAT_PURCHASE_CONFIRM_TIMEOUT_MS - elapsed));
    return () => window.clearTimeout(timeout);
  }, [inviteSeatPurchase?.phase, inviteSeatPurchase?.submittedAt, t]);

  const handleRevokeInvitation = (invitation: Invitation) => {
    if (!workspace) return;
    setConfirmAction({
      title: t(($) => $.members.revoke_invitation_title),
      description: t(($) => $.members.revoke_invitation_description, { email: invitation.invitee_email }),
      variant: "destructive",
      onConfirm: async () => {
        setInvitationActionId(invitation.id);
        try {
          await api.revokeInvitation(workspace.id, invitation.id);
          qc.invalidateQueries({ queryKey: workspaceKeys.invitations(wsId) });
          toast.success(t(($) => $.members.toast_invitation_revoked));
        } catch (e) {
          toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_invitation_revoke_failed));
        } finally {
          setInvitationActionId(null);
        }
      },
    });
  };

  const handleRoleChange = async (memberId: string, role: MemberRole) => {
    if (!workspace) return;
    setMemberActionId(memberId);
    try {
      await api.updateMember(workspace.id, memberId, { role });
      qc.invalidateQueries({ queryKey: workspaceKeys.members(wsId) });
      toast.success(t(($) => $.members.toast_role_updated));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_role_failed));
    } finally {
      setMemberActionId(null);
    }
  };

  const handleOrganisationsChange = async (memberId: string, organisationIds: string[]) => {
    if (!workspace) return;
    setMemberActionId(memberId);
    try {
      await api.updateMember(workspace.id, memberId, {
        role: "client",
        organisation_ids: organisationIds,
      });
      toast.success(t(($) => $.members.toast_organisations_updated));
      // The row stays busy until the fresh set arrives, so the next edit starts from it.
      await qc.invalidateQueries({ queryKey: workspaceKeys.members(wsId) });
    } catch (e) {
      toast.error(
        e instanceof Error ? e.message : t(($) => $.members.toast_organisations_failed),
      );
    } finally {
      setMemberActionId(null);
    }
  };

  const handleRemoveMember = (member: MemberWithUser) => {
    if (!workspace) return;
    setConfirmAction({
      title: t(($) => $.members.remove_member_title, { name: member.name }),
      description: t(($) => $.members.remove_member_description, { name: member.name, workspace: workspace.name }),
      variant: "destructive",
      onConfirm: async () => {
        setMemberActionId(member.id);
        try {
          await api.deleteMember(workspace.id, member.id);
          qc.invalidateQueries({ queryKey: workspaceKeys.members(wsId) });
          toast.success(t(($) => $.members.toast_member_removed));
        } catch (e) {
          toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_member_remove_failed));
        } finally {
          setMemberActionId(null);
        }
      },
    });
  };

  const handleCreateShareLink = async () => {
    if (!workspace) return;
    setShareLinkLoading(true);
    try {
      await api.createShareLink(workspace.id, { role: shareLinkRole, expires_in: parseInt(shareLinkExpiry) || undefined });
      qc.invalidateQueries({ queryKey: workspaceKeys.shareLinks(wsId) });
      setShareLinkOpen(false);
      toast.success(t(($) => $.members.toast_share_link_created));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_share_link_failed));
    } finally {
      setShareLinkLoading(false);
    }
  };

  const handleRevokeShareLink = (link: ShareLink) => {
    if (!workspace) return;
    setShareLinkActionId(link.id);
    api.revokeShareLink(workspace.id, link.id)
      .then(() => {
        qc.invalidateQueries({ queryKey: workspaceKeys.shareLinks(wsId) });
        toast.success(t(($) => $.members.toast_share_link_revoked));
      })
      .catch((e) => {
        toast.error(e instanceof Error ? e.message : t(($) => $.members.toast_share_link_revoke_failed));
      })
      .finally(() => setShareLinkActionId(null));
  };

  const handleCopyShareLink = (link: ShareLink) => {
    const joinUrl = buildShareLinkUrl(navigation, link.code);
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(joinUrl).then(
        () => toast.success(t(($) => $.members.toast_share_link_copied)),
        () => toast.error(t(($) => $.members.toast_share_link_copy_failed)),
      );
    } else {
      const textArea = document.createElement("textarea");
      textArea.value = joinUrl;
      textArea.style.position = "fixed";
      textArea.style.left = "-9999px";
      document.body.appendChild(textArea);
      textArea.select();
      try {
        document.execCommand("copy");
        toast.success(t(($) => $.members.toast_share_link_copied));
      } catch {
        toast.error(t(($) => $.members.toast_share_link_copy_failed));
      }
      document.body.removeChild(textArea);
    }
  };

  if (!workspace) return null;

  const inviteRoleItems = (["member", "admin", "client"] as const).map((value) => ({
    value,
    label: roleConfig[value].label,
  }));
  const shareLinkRoleItems = (["member", "admin"] as const).map((value) => ({
    value,
    label: roleConfig[value].label,
  }));
  const inviteNeedsOrganisation = inviteRole === "client" && inviteOrganisationIds.length === 0;
  const expiryOptions = [
    { value: "24", label: t(($) => $.members.expiry_24h) },
    { value: "168", label: t(($) => $.members.expiry_7d) },
    { value: "720", label: t(($) => $.members.expiry_30d) },
    { value: "0", label: t(($) => $.members.expiry_never) },
  ];
  const views: { value: MembersView; label: string; count: number }[] = [
    { value: "members", label: t(($) => $.members.members_label), count: members.length },
    { value: "invitations", label: t(($) => $.members.pending_label), count: invitations.length },
    ...(canManageWorkspace
      ? [{ value: "links" as const, label: t(($) => $.members.share_links_label), count: shareLinks.length }]
      : []),
  ];
  const activeView = views.some((item) => item.value === view) ? view : "members";

  return (
    <SettingsTab
      title={t(($) => $.page.tabs.members)}
      scope="workspace"
      actions={
        canManageWorkspace ? (
          <Button onClick={() => setInviteOpen(true)}>
            <Plus />
            {t(($) => $.members.invite_title)}
          </Button>
        ) : undefined
      }
    >
      {currentMember && !canManageWorkspace ? (
        <SettingsReadOnlyNotice wsId={wsId} />
      ) : null}
      <section className="space-y-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <SettingsViewTabs
            label={t(($) => $.page.tabs.members)}
            items={views}
            value={activeView}
            onChange={setView}
          />
          <div className="flex min-w-0 items-center gap-3">
            {seats && billingEnabled ? (
              <span className="shrink-0 text-caption text-muted-foreground">
                {t(($) => $.members.seats_used, {
                  used: seats.used + seats.reserved,
                  purchased: seats.purchased,
                })}
                {" · "}
                <AppLink
                  href={
                    navigation
                      ? settingsHref(navigation.pathname, navigation.searchParams, "billing")
                      : "?tab=billing"
                  }
                  className="text-foreground underline-offset-4 hover:underline"
                >
                  {t(($) => $.members.manage_seats)}
                </AppLink>
              </span>
            ) : null}
            {activeView === "members" ? (
              <div className="relative w-full sm:w-56">
                <Search
                  aria-hidden="true"
                  className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
                />
                <Input
                  type="search"
                  value={memberQuery}
                  onChange={(event) => setMemberQuery(event.target.value)}
                  placeholder={t(($) => $.members.search_placeholder)}
                  aria-label={t(($) => $.members.search_placeholder)}
                  className="pl-8"
                />
              </div>
            ) : activeView === "links" ? (
              <Button variant="outline" size="sm" onClick={() => setShareLinkOpen(true)}>
                <Plus />
                {t(($) => $.members.share_links_create_title)}
              </Button>
            ) : null}
          </div>
        </div>

        {activeView === "members" ? (
          visibleMembers.length > 0 ? (
            <SettingsCard>
              {visibleMembers.map((m) => (
                <div key={m.id}>
                  <MemberRow
                    member={m}
                    canManage={canManageWorkspace}
                    canManageOwners={isOwner}
                    ownerCount={ownerCount}
                    isSelf={m.user_id === user?.id}
                    busy={memberActionId === m.id}
                    onRoleChange={(role) => handleRoleChange(m.id, role)}
                    onOrganisationsChange={(organisationIds) =>
                      handleOrganisationsChange(m.id, organisationIds)
                    }
                    onRemove={() => handleRemoveMember(m)}
                  />
                </div>
              ))}
            </SettingsCard>
          ) : (
            <EmptyList>
              {needle ? t(($) => $.members.no_results) : t(($) => $.members.no_members)}
            </EmptyList>
          )
        ) : activeView === "invitations" ? (
          invitations.length > 0 ? (
            <SettingsCard>
              {invitations.map((inv) => (
                <div key={inv.id}>
                  <InvitationRow
                    invitation={inv}
                    canManage={canManageWorkspace}
                    onRevoke={() => handleRevokeInvitation(inv)}
                    busy={invitationActionId === inv.id}
                  />
                </div>
              ))}
            </SettingsCard>
          ) : (
            <EmptyList>{t(($) => $.members.no_invitations)}</EmptyList>
          )
        ) : shareLinks.length > 0 ? (
          <SettingsCard>
            {shareLinks.map((link) => (
              <div key={link.id}>
                <ShareLinkRow
                  link={link}
                  onRevoke={() => handleRevokeShareLink(link)}
                  busy={shareLinkActionId === link.id}
                  onCopy={() => handleCopyShareLink(link)}
                />
              </div>
            ))}
          </SettingsCard>
        ) : (
          <EmptyList>{t(($) => $.members.no_share_links)}</EmptyList>
        )}
      </section>

      <Dialog
        open={inviteOpen}
        onOpenChange={(open) => {
          if (!open && !inviteLoading) setInviteOpen(false);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(($) => $.members.invite_title)}</DialogTitle>
          </DialogHeader>
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              if (inviteEmail.trim() && !inviteNeedsOrganisation) void handleInviteMember();
            }}
          >
            <div className="space-y-1.5">
              <Label htmlFor="invite-email">{t(($) => $.members.email_label)}</Label>
              <Input
                id="invite-email"
                type="email"
                name="invite-email"
                autoComplete="email"
                spellCheck={false}
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                placeholder={t(($) => $.members.invite_email_placeholder)}
                autoFocus
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="invite-role">{t(($) => $.members.role_label)}</Label>
              <Select
                items={inviteRoleItems}
                value={inviteRole}
                onValueChange={(value) => setInviteRole(value as MemberRole)}
              >
                <SelectTrigger id="invite-role" className="w-full">
                  <SelectValue>{() => roleConfig[inviteRole].label}</SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {inviteRoleItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      <span className="flex flex-col">
                        <span>{item.label}</span>
                        <span className="text-caption text-muted-foreground">
                          {roleConfig[item.value].description}
                        </span>
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {inviteRole === "client" && (
                <p className="text-caption text-muted-foreground">
                  {roleConfig.client.description}
                </p>
              )}
            </div>
            {inviteRole === "client" && (
              <div className="space-y-1.5">
                <Label htmlFor="invite-organisations">
                  {t(($) => $.members.organisations_label)}
                </Label>
                <OrganisationMultiSelect
                  id="invite-organisations"
                  selectedIds={inviteOrganisationIds}
                  onChange={setInviteOrganisationIds}
                />
              </div>
            )}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setInviteOpen(false)}
                disabled={inviteLoading}
              >
                {t(($) => $.members.confirm_cancel)}
              </Button>
              <Button
                type="submit"
                disabled={inviteLoading || !inviteEmail.trim() || inviteNeedsOrganisation}
                aria-busy={inviteLoading || undefined}
              >
                {inviteLoading ? t(($) => $.members.inviting) : t(($) => $.members.invite_button)}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog
        open={shareLinkOpen}
        onOpenChange={(open) => {
          if (!open && !shareLinkLoading) setShareLinkOpen(false);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t(($) => $.members.share_links_create_title)}</DialogTitle>
            <DialogDescription>{t(($) => $.members.share_links_description)}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="share-link-role">{t(($) => $.members.role_label)}</Label>
              <Select
                items={shareLinkRoleItems}
                value={shareLinkRole}
                onValueChange={(value) => setShareLinkRole(value as StaffRole)}
              >
                <SelectTrigger id="share-link-role" className="w-full">
                  <SelectValue>{() => roleConfig[shareLinkRole].label}</SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {shareLinkRoleItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="share-link-expiry">{t(($) => $.members.expiry_label)}</Label>
              <Select
                items={expiryOptions}
                value={shareLinkExpiry}
                onValueChange={(v) => v && setShareLinkExpiry(v)}
              >
                <SelectTrigger id="share-link-expiry" className="w-full">
                  <SelectValue>
                    {() =>
                      expiryOptions.find((option) => option.value === shareLinkExpiry)?.label ??
                      shareLinkExpiry
                    }
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {expiryOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setShareLinkOpen(false)}
              disabled={shareLinkLoading}
            >
              {t(($) => $.members.confirm_cancel)}
            </Button>
            <Button
              onClick={() => void handleCreateShareLink()}
              disabled={shareLinkLoading}
              aria-busy={shareLinkLoading || undefined}
            >
              {shareLinkLoading
                ? t(($) => $.members.share_links_creating)
                : t(($) => $.members.share_links_create_button)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={inviteSeatPurchase !== null}
        onOpenChange={(open) => {
          if (
            !open &&
            (inviteSeatPurchase?.phase === "review" ||
              inviteSeatPurchase?.phase === "error")
          ) {
            setInviteSeatPurchase(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(($) => $.members.seat_purchase_title)}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {inviteSeatPurchase?.phase === "review"
                ? t(($) => $.members.seat_purchase_description, {
                    email: inviteSeatPurchase.email,
                  })
                : inviteSeatPurchase?.phase === "error"
                  ? inviteSeatPurchase.error
                  : t(($) => $.members.seat_purchase_waiting)}
            </AlertDialogDescription>
          </AlertDialogHeader>

          {inviteSeatPurchase?.phase === "review" && (
            <div className="divide-y rounded-lg border text-body">
              <div className="flex items-center justify-between gap-4 px-4 py-3">
                <span className="text-muted-foreground">
                  {billingT(($) => $.workspace.seat_purchase.seats_after)}
                </span>
                <span className="font-medium">
                  {billingT(($) => $.workspace.seats.seat_count, {
                    count: inviteSeatPurchase.preview.resultingSeats,
                  })}
                </span>
              </div>
              <div className="flex items-center justify-between gap-4 px-4 py-3">
                <span className="text-muted-foreground">
                  {billingT(($) => $.workspace.seat_purchase.charge_today)}
                </span>
                <span className="font-medium">
                  {formatStripeMinorAmount(
                    inviteSeatPurchase.preview.prorationAmount,
                    inviteSeatPurchase.preview.currency,
                    locale,
                  ) ?? "—"}
                </span>
              </div>
              <div className="flex items-center justify-between gap-4 px-4 py-3">
                <span className="text-muted-foreground">
                  {billingT(($) => $.workspace.seat_purchase.next_invoice)}
                </span>
                <span className="font-medium">
                  {formatStripeMinorAmount(
                    inviteSeatPurchase.preview.nextInvoiceAmount,
                    inviteSeatPurchase.preview.currency,
                    locale,
                  ) ?? "—"}
                </span>
              </div>
              <p className="px-4 py-3 text-caption text-muted-foreground">
                {billingT(($) => $.workspace.seat_purchase.tax_notice)}
              </p>
            </div>
          )}

          {(inviteSeatPurchase?.phase === "purchasing" ||
            inviteSeatPurchase?.phase === "waiting" ||
            inviteSeatPurchase?.phase === "inviting") && (
            <div className="flex items-center gap-2 text-body text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              {inviteSeatPurchase.phase === "inviting"
                ? t(($) => $.members.inviting)
                : t(($) => $.members.seat_purchase_waiting)}
            </div>
          )}

          <AlertDialogFooter>
            {(inviteSeatPurchase?.phase === "review" ||
              inviteSeatPurchase?.phase === "error") && (
              <AlertDialogCancel>
                {t(($) => $.members.confirm_cancel)}
              </AlertDialogCancel>
            )}
            {inviteSeatPurchase?.phase === "review" && (
              <AlertDialogAction
                onClick={(event) => {
                  event.preventDefault();
                  void handlePurchaseSeatAndInvite();
                }}
              >
                {t(($) => $.members.purchase_seat_and_invite)}
              </AlertDialogAction>
            )}
            {inviteSeatPurchase?.phase === "error" &&
              inviteSeatPurchase.retryable && (
                <AlertDialogAction
                  onClick={(event) => {
                    event.preventDefault();
                    void handlePurchaseSeatAndInvite();
                  }}
                >
                  {t(($) => $.members.retry_seat_purchase)}
                </AlertDialogAction>
              )}
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={!!confirmAction} onOpenChange={(v) => { if (!v) setConfirmAction(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmAction?.title}</AlertDialogTitle>
            <AlertDialogDescription>{confirmAction?.description}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t(($) => $.members.confirm_cancel)}</AlertDialogCancel>
            <AlertDialogAction
              variant={confirmAction?.variant === "destructive" ? "destructive" : "default"}
              onClick={async () => {
                await confirmAction?.onConfirm();
                setConfirmAction(null);
              }}
            >
              {t(($) => $.members.confirm_action)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsTab>
  );
}

function EmptyList({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-surface-border px-4 py-10 text-center text-body text-muted-foreground">
      {children}
    </div>
  );
}
