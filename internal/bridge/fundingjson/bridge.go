// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const filenameFundingJSON = "funding.json"

// Bridge renders funding.json (https://fundingjson.org) from projectfile data.
// The output is validated against the embedded FundingJSON v1.1.0 schema before
// writing. Refuses generation when required data is missing.
type Bridge struct{}

func (Bridge) Name() string             { return "fundingjson" }
func (Bridge) Filename() string         { return filenameFundingJSON }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameFundingJSON, "projectfile" }

// Policy uses no marker — JSON does not support comments, and funding.json is
// a pure-data artefact that should always reflect the projectfile source of
// truth. ScaffoldOnce is also unset: the dispatcher always overwrites.
func (Bridge) Policy() core.Policy { return core.Policy{} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameFundingJSON, filenameFundingJSON))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameFundingJSON, filenameFundingJSON)
}

func (Bridge) Render(pf *projectfile.Document, _ core.Options) (core.Output, error) {
	ext, err := pfmodel.GetFundingExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	if ext == nil {
		ext = &pfmodel.FundingExtension{}
	}

	doc := &fundingDoc{Version: "1.1.0"}

	entity, entityErr := resolveEntity(pf, ext)
	if entityErr != nil {
		return core.Output{}, entityErr
	}
	doc.Entity = entity

	if proj := resolveProject(pf); proj != nil {
		doc.Projects = []fundingProject{*proj}
	}

	if len(ext.Channels) == 0 {
		return core.Output{}, fmt.Errorf("funding.json: no channels configured — add channels to [org.projectfile.funding]:\n  channels = [{guid = \"stripe\", type = \"payment-provider\", address = \"https://...\"}]")
	}
	if len(ext.Plans) == 0 {
		return core.Output{}, fmt.Errorf("funding.json: no plans configured — add plans to [org.projectfile.funding]:\n  plans = [{guid = \"sponsor\", status = \"active\", name = \"Sponsor\", amount = 10.0, currency = \"USD\", frequency = \"monthly\", channels = [\"stripe\"]}]")
	}

	doc.Funding.Channels = buildChannels(ext.Channels)
	doc.Funding.Plans = buildPlans(ext.Plans)
	doc.Funding.History = buildHistory(ext.History)

	emitTrace(entity, ext)

	norm, err := normalizeForValidation(doc)
	if err != nil {
		return core.Output{}, fmt.Errorf("funding.json: normalize: %w", err)
	}
	if err := validateOutput(norm); err != nil {
		return core.Output{}, fmt.Errorf("funding.json: schema validation failed: %w", err)
	}

	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return core.Output{}, fmt.Errorf("funding.json: marshal: %w", err)
	}
	body = append(body, '\n')

	return core.Output{Files: map[string][]byte{filenameFundingJSON: body}}, nil
}

// --- FundingJSON output types ---

type fundingURL struct {
	URL       string `json:"url"`
	WellKnown string `json:"wellKnown,omitempty"`
}

type fundingEntity struct {
	Type        string     `json:"type"`
	Role        string     `json:"role"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone,omitempty"`
	Description string     `json:"description"`
	WebpageURL  fundingURL `json:"webpageUrl"`
}

type fundingProject struct {
	GUID          string     `json:"guid"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	WebpageURL    fundingURL `json:"webpageUrl"`
	RepositoryURL fundingURL `json:"repositoryUrl"`
	Licenses      []string   `json:"licenses"`
	Tags          []string   `json:"tags"`
}

type fundingChannel struct {
	GUID        string `json:"guid"`
	Type        string `json:"type"`
	Address     string `json:"address,omitempty"`
	Description string `json:"description,omitempty"`
}

type fundingPlan struct {
	GUID        string   `json:"guid"`
	Status      string   `json:"status"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Amount      float64  `json:"amount"`
	Currency    string   `json:"currency"`
	Frequency   string   `json:"frequency"`
	Channels    []string `json:"channels"`
}

type fundingHistoryEntry struct {
	Year        int     `json:"year"`
	Income      float64 `json:"income,omitempty"`
	Expenses    float64 `json:"expenses,omitempty"`
	Taxes       float64 `json:"taxes,omitempty"`
	Currency    string  `json:"currency"`
	Description string  `json:"description,omitempty"`
}

type fundingSection struct {
	Channels []fundingChannel      `json:"channels"`
	Plans    []fundingPlan         `json:"plans"`
	History  []fundingHistoryEntry `json:"history,omitempty"`
}

type fundingDoc struct {
	Version  string           `json:"version"`
	Entity   fundingEntity    `json:"entity"`
	Projects []fundingProject `json:"projects,omitempty"`
	Funding  fundingSection   `json:"funding"`
}

// --- Entity resolution ---

// resolveEntity determines the FundingJSON entity from projectfile data.
// Preference: organizations with maintainer/owner role, then people.
// Extension overrides (entity-type, entity-role) take precedence.
func resolveEntity(pf *projectfile.Document, ext *pfmodel.FundingExtension) (fundingEntity, error) {
	entity := fundingEntity{}

	entityType, entityName, entityEmail := "", "", ""
	source := ""

	for _, o := range pf.Organizations {
		if o.Email != "" && roleMatches(o.Roles) {
			entityType = "organisation"
			entityName = o.Name
			entityEmail = o.Email
			source = "first [[organizations]] with maintainer/owner role"
			break
		}
	}

	if entityType == "" {
		for _, p := range pf.People {
			if p.Email != "" && roleMatches(p.Roles) {
				entityType = "individual"
				entityName = pfmodel.FlatPersonName(p)
				entityEmail = p.Email
				source = "first [[people]] with maintainer/owner role"
				break
			}
		}
	}

	if entityType == "" {
		for _, p := range pf.People {
			if p.Email != "" {
				entityType = "individual"
				entityName = pfmodel.FlatPersonName(p)
				entityEmail = p.Email
				source = "first [[people]] with email"
				break
			}
		}
	}

	if entityType == "" {
		return entity, fmt.Errorf("funding.json: cannot resolve entity — no [[people]] or [[organizations]] with email found. Add a [[people]] or [[organizations]] entry with role 'maintainer' and email")
	}

	entity.Type = entityType
	entity.Role = bestRole(pf, source)
	entity.Name = entityName
	entity.Email = entityEmail
	entity.Description = entityDescription(pf)

	homepage := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
	if homepage == "" {
		return entity, fmt.Errorf("funding.json: entity.webpageUrl is required — add a links entry with type 'homepage', e.g. links = [{type = \"homepage\", url = \"https://...\"}]")
	}
	entity.WebpageURL = fundingURL{URL: homepage}

	if ext != nil {
		if ext.EntityType != "" {
			entity.Type = ext.EntityType
		}
		if ext.EntityRole != "" {
			entity.Role = ext.EntityRole
		}
	}

	return entity, nil
}

func roleMatches(roles []string) bool {
	for _, r := range roles {
		switch r {
		case "owner", "maintainer", "steward":
			return true
		}
	}
	return false
}

// bestRole picks the most relevant FundingJSON role from people/orgs.
// Priority: owner > steward > maintainer > contributor > other.
func bestRole(pf *projectfile.Document, _ string) string {
	rolePriority := []string{"owner", "steward", "maintainer", "contributor"}
	sources := make([][]string, 0, len(pf.Organizations)+len(pf.People))
	for _, o := range pf.Organizations {
		sources = append(sources, o.Roles)
	}
	for _, p := range pf.People {
		sources = append(sources, p.Roles)
	}
	for _, want := range rolePriority {
		for _, roles := range sources {
			for _, r := range roles {
				if r == want {
					return want
				}
			}
		}
	}
	return "other"
}

func entityDescription(pf *projectfile.Document) string {
	if pf == nil {
		return ""
	}
	if s := projectfile.ExtractLocalizedString(pf.Identity.Summary); s != "" {
		return s
	}
	return projectfile.ExtractLocalizedString(pf.Identity.Description)
}

// --- Project resolution ---

var guidSanitize = regexp.MustCompile(`[^a-z0-9-]`)

func resolveProject(pf *projectfile.Document) *fundingProject {
	if pf == nil {
		return nil
	}
	name := pfmodel.DisplayName(pf)
	if name == "" || name == "this project" {
		return nil
	}
	desc := projectfile.ExtractLocalizedString(pf.Identity.Summary)
	if desc == "" {
		desc = projectfile.ExtractLocalizedString(pf.Identity.Description)
	}

	homepage := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
	repoURL := pfmodel.CitableRepositoryURL(pf)

	guid := strings.ToLower(pf.Identity.Name)
	guid = strings.ReplaceAll(guid, "_", "-")
	guid = guidSanitize.ReplaceAllString(guid, "")
	if guid == "" {
		guid = "project"
	}

	var licenses []string
	if pf.License != nil && pf.License.Spdx != "" {
		licenses = []string{"spdx:" + pf.License.Spdx}
	}

	tags := filterTags(pf.Keywords)

	proj := &fundingProject{
		GUID:        guid,
		Name:        name,
		Description: desc,
		Licenses:    licenses,
		Tags:        tags,
	}
	if homepage != "" {
		proj.WebpageURL = fundingURL{URL: homepage}
	}
	if repoURL != "" {
		proj.RepositoryURL = fundingURL{URL: repoURL}
	}
	return proj
}

// filterTags keeps only tags matching FundingJSON pattern ^[a-z0-9-]+$ and
// caps at 10 entries.
func filterTags(keywords []string) []string {
	tagRe := regexp.MustCompile(`^[a-z0-9-]+$`)
	out := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		if tagRe.MatchString(kw) {
			out = append(out, kw)
			if len(out) >= 10 {
				break
			}
		}
	}
	return out
}

// --- Builders ---

func buildChannels(in []pfmodel.FundingChannel) []fundingChannel {
	out := make([]fundingChannel, len(in))
	for i, c := range in {
		out[i] = fundingChannel{
			GUID:        c.GUID,
			Type:        c.Type,
			Address:     c.Address,
			Description: c.Description,
		}
	}
	return out
}

func buildPlans(in []pfmodel.FundingPlan) []fundingPlan {
	out := make([]fundingPlan, len(in))
	for i, p := range in {
		out[i] = fundingPlan{
			GUID:        p.GUID,
			Status:      p.Status,
			Name:        p.Name,
			Description: p.Description,
			Amount:      p.Amount,
			Currency:    p.Currency,
			Frequency:   p.Frequency,
			Channels:    p.Channels,
		}
	}
	return out
}

func buildHistory(in []pfmodel.FundingHistory) []fundingHistoryEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]fundingHistoryEntry, len(in))
	for i, h := range in {
		out[i] = fundingHistoryEntry{
			Year:        h.Year,
			Income:      h.Income,
			Expenses:    h.Expenses,
			Taxes:       h.Taxes,
			Currency:    h.Currency,
			Description: h.Description,
		}
	}
	return out
}

// --- Decision trace ---

func emitTrace(entity fundingEntity, ext *pfmodel.FundingExtension) {
	genlog.Decision("entity.type", entity.Type, "auto-resolved + override", "")
	genlog.Decision("entity.role", entity.Role, "auto-resolved + override", "")
	genlog.Decision("entity.name", entity.Name, "auto-resolved", "")
	genlog.Decision("entity.email", entity.Email, "auto-resolved", "")
	genlog.Decision("entity.webpageUrl", entity.WebpageURL.URL, "links[type=homepage]", "")
	genlog.Decision("channels", fmt.Sprintf("%d", len(ext.Channels)), "[org.projectfile.funding].channels", "")
	genlog.Decision("plans", fmt.Sprintf("%d", len(ext.Plans)), "[org.projectfile.funding].plans", "")
	if len(ext.History) > 0 {
		genlog.Decision("history", fmt.Sprintf("%d", len(ext.History)), "[org.projectfile.funding].history", "")
	}
}
