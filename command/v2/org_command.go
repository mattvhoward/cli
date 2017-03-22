package v2

import (
	"sort"
	"strings"

	"code.cloudfoundry.org/cli/actor/sharedaction"
	"code.cloudfoundry.org/cli/actor/v2action"
	"code.cloudfoundry.org/cli/actor/v3action"
	"code.cloudfoundry.org/cli/command"
	"code.cloudfoundry.org/cli/command/flag"
	"code.cloudfoundry.org/cli/command/v2/shared"
	sharedV3 "code.cloudfoundry.org/cli/command/v3/shared"
)

//go:generate counterfeiter . OrgActor

type OrgActor interface {
	GetOrganizationByName(orgName string) (v2action.Organization, v2action.Warnings, error)
	GetOrganizationSummaryByName(orgName string) (v2action.OrganizationSummary, v2action.Warnings, error)
}

//go:generate counterfeiter . OrgActorV3

type OrgActorV3 interface {
	GetIsolationSegmentsByOrganization(orgName string) ([]v3action.IsolationSegment, v3action.Warnings, error)
	CloudControllerAPIVersion() string
}

type OrgCommand struct {
	RequiredArgs    flag.Organization `positional-args:"yes"`
	GUID            bool              `long:"guid" description:"Retrieve and display the given org's guid.  All other output for the org is suppressed."`
	usage           interface{}       `usage:"CF_NAME org ORG [--guid]"`
	relatedCommands interface{}       `related_commands:"org-users, orgs"`

	UI          command.UI
	Config      command.Config
	SharedActor command.SharedActor
	Actor       OrgActor
	ActorV3     OrgActorV3
}

func (cmd *OrgCommand) Setup(config command.Config, ui command.UI) error {
	cmd.Config = config
	cmd.UI = ui
	cmd.SharedActor = sharedaction.NewActor()

	ccClient, uaaClient, err := shared.NewClients(config, ui, true)
	if err != nil {
		return err
	}
	cmd.Actor = v2action.NewActor(ccClient, uaaClient)

	ccClientV3, err := sharedV3.NewClients(config, ui, true)
	if err != nil {
		return err
	}
	cmd.ActorV3 = v3action.NewActor(ccClientV3)

	return nil
}

func (cmd OrgCommand) Execute(args []string) error {
	err := cmd.SharedActor.CheckTarget(cmd.Config, false, false)
	if err != nil {
		return shared.HandleError(err)
	}

	if cmd.GUID {
		return cmd.displayOrgGUID()
	} else {
		return cmd.displayOrgSummary()
	}
}

func (cmd OrgCommand) displayOrgGUID() error {
	org, warnings, err := cmd.Actor.GetOrganizationByName(cmd.RequiredArgs.Organization)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		return shared.HandleError(err)
	}

	cmd.UI.DisplayText(org.GUID)

	return nil
}

func (cmd OrgCommand) displayOrgSummary() error {
	user, err := cmd.Config.CurrentUser()
	if err != nil {
		return shared.HandleError(err)
	}

	cmd.UI.DisplayTextWithFlavor(
		"Getting info for org {{.OrgName}} as {{.Username}}...",
		map[string]interface{}{
			"OrgName":  cmd.RequiredArgs.Organization,
			"Username": user.Name,
		})
	cmd.UI.DisplayNewline()

	orgSummary, warnings, err := cmd.Actor.GetOrganizationSummaryByName(cmd.RequiredArgs.Organization)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		return shared.HandleError(err)
	}

	table := [][]string{
		{cmd.UI.TranslateText("name:"), orgSummary.Name},
		{cmd.UI.TranslateText("domains:"), strings.Join(orgSummary.DomainNames, ", ")},
		{cmd.UI.TranslateText("quota:"), orgSummary.QuotaName},
		{cmd.UI.TranslateText("spaces:"), strings.Join(orgSummary.SpaceNames, ", ")},
	}

	apiCheck := command.MinimumAPIVersionCheck(cmd.ActorV3.CloudControllerAPIVersion(), "3.11.0")
	if apiCheck == nil {
		isolationSegments, v3Warnings, err := cmd.ActorV3.GetIsolationSegmentsByOrganization(orgSummary.GUID)
		cmd.UI.DisplayWarnings(v3Warnings)
		if err != nil {
			return shared.HandleError(err)
		}

		isolationSegmentNames := []string{}
		for _, iso := range isolationSegments {
			isolationSegmentNames = append(isolationSegmentNames, iso.Name)
		}

		sort.Strings(isolationSegmentNames)
		table = append(table, []string{cmd.UI.TranslateText("isolation segments:"), strings.Join(isolationSegmentNames, ", ")})
	}

	cmd.UI.DisplayKeyValueTable("", table, 3)

	return nil
}
