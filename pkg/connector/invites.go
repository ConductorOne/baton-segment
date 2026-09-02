package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-segment/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type inviteBuilder struct {
	client *client.Client
}

func (b *inviteBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return inviteResourceType
}

// List returns all pending invitations from the Segment workspace.
func (b *inviteBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	bag, err := parsePageToken(opts.PageToken.Token, &v2.ResourceId{ResourceType: inviteResourceType.Id})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse page token: %w", err)
	}

	pageToken := bag.PageToken()

	outputAnnotations := annotations.New()
	response, rateLimit, err := b.client.ListInvites(ctx, pageToken, client.DefaultPageSize)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to list invites: %w", err)
	}

	var resources []*v2.Resource
	for _, email := range response.Data.Invites {
		r, err := inviteResource(email, parentResourceID)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create invite resource: %w", err)
		}
		resources = append(resources, r)
	}

	nextToken, err := bag.NextToken(response.Data.Pagination.Next)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: outputAnnotations}, fmt.Errorf("failed to create next page token: %w", err)
	}

	l.Debug("listed invites", zap.Int("count", len(resources)), zap.String("next_token", nextToken))

	return resources, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: outputAnnotations}, nil
}

// inviteResource creates a v2.Resource from a Segment invite email.
func inviteResource(email string, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	userTraitOptions := []rs.UserTraitOption{
		rs.WithEmail(email, true),
		rs.WithUserLogin(email),
	}

	// Add "(Invitation)" prefix to make it clear this is a pending invite, not an active user
	displayName := fmt.Sprintf("(Invitation) %s", email)

	// Use email as ID since invites only have email
	return rs.NewUserResource(
		displayName,
		inviteResourceType,
		email, // Use email as ID
		userTraitOptions,
		rs.WithResourceStatus(v2.Status_RESOURCE_STATUS_DISABLED, ""), // Disabled until accepted
		rs.WithParentResourceID(parentResourceID),
	)
}

// Entitlements returns no entitlements for invites.
func (b *inviteBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grants returns no grants for invites.
func (b *inviteBuilder) Grants(ctx context.Context, resource *v2.Resource, opts rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// CreateAccountCapabilityDetails returns the supported credential options for account creation.
func (b *inviteBuilder) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

// CreateAccount sends a new workspace invitation.
func (b *inviteBuilder) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.LocalCredentialOptions,
) (connectorbuilder.CreateAccountResponse, []*v2.PlaintextData, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if accountInfo == nil || accountInfo.Profile == nil {
		return nil, nil, nil, fmt.Errorf("baton-segment: profile is required for creating an invite")
	}

	profile := accountInfo.Profile.AsMap()
	rawEmail, present := profile["email"]
	if !present || rawEmail == nil {
		return nil, nil, nil, fmt.Errorf("baton-segment: email is required for creating an invite")
	}
	email, ok := rawEmail.(string)
	if !ok {
		return nil, nil, nil, fmt.Errorf("baton-segment: invalid email: expected a string, got %T", rawEmail)
	}
	if email == "" {
		return nil, nil, nil, fmt.Errorf("baton-segment: email is required for creating an invite")
	}

	outputAnnotations := annotations.New()
	l.Debug("creating invite", zap.String("email", email))

	inviteReq := []client.InviteRequest{{Email: email}}
	_, rateLimit, err := b.client.CreateInvites(ctx, inviteReq)
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		if !client.IsAlreadyExistsError(err) {
			return nil, nil, outputAnnotations, fmt.Errorf("baton-segment: create invite %s: %w", email, err)
		}

		existingEmail, found, lookupErr := b.client.FindPendingInviteByEmail(ctx, email)
		if lookupErr != nil {
			l.Debug("failed to scan pending invites for duplicate lookup", zap.String("email", email), zap.Error(lookupErr))
		}
		if found {
			existingInvite, resErr := inviteResource(existingEmail, nil)
			if resErr != nil {
				return nil, nil, outputAnnotations, fmt.Errorf("baton-segment: create invite resource %s: %w", email, resErr)
			}
			l.Debug("duplicate confirmed; invitation already pending", zap.String("email", email))
			return &v2.CreateAccountResponse_ActionRequiredResult{
				Resource: existingInvite,
				Message:  fmt.Sprintf("Segment invitation already pending for %s. User must accept the existing email invitation.", email),
			}, nil, outputAnnotations, nil
		}

		l.Debug("duplicate confirmed; not a pending invite, so email already belongs to a workspace member", zap.String("email", email))
		return &v2.CreateAccountResponse_AlreadyExistsResult{}, nil, outputAnnotations, nil
	}

	inviteRes, err := inviteResource(email, nil)
	if err != nil {
		return nil, nil, outputAnnotations, fmt.Errorf("baton-segment: create invite resource %s: %w", email, err)
	}

	l.Debug("invite created successfully", zap.String("email", email))

	return &v2.CreateAccountResponse_ActionRequiredResult{
		Resource: inviteRes,
		Message:  fmt.Sprintf("Invitation sent to %s. User must accept the email invitation to complete account creation.", email),
	}, nil, outputAnnotations, nil
}

// Delete removes a pending invitation.
func (b *inviteBuilder) Delete(ctx context.Context, resourceID *v2.ResourceId) (annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if resourceID.ResourceType != inviteResourceType.Id {
		return nil, fmt.Errorf("baton-segment: invalid resource type: expected %s, got %s", inviteResourceType.Id, resourceID.ResourceType)
	}

	// The resource ID for invites is the email address
	email := resourceID.Resource

	l.Debug("deleting invite", zap.String("email", email))

	outputAnnotations := annotations.New()
	rateLimit, err := b.client.DeleteInvites(ctx, []string{email})
	outputAnnotations.WithRateLimiting(rateLimit)
	if err != nil {
		return outputAnnotations, fmt.Errorf("baton-segment: delete invite %s: %w", email, err)
	}

	l.Debug("invite deleted successfully", zap.String("email", email))

	return outputAnnotations, nil
}

func newInviteBuilder(c *client.Client) *inviteBuilder {
	return &inviteBuilder{client: c}
}
