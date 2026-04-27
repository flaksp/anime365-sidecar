package embyclient

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// fieldsFromDocumentation is a list of fields from the API documentation that may be requested.
// They are not used by the app but item update API may fail if some of these fields are missing in request.
var fieldsFromDocumentation = map[string]struct{}{
	"Budget":                  {},
	"Chapters":                {},
	"DateCreated":             {},
	"Genres":                  {},
	"HomePageUrl":             {},
	"IndexOptions":            {},
	"MediaStreams":            {},
	"Overview":                {},
	"ParentId":                {},
	"Path":                    {},
	"People":                  {},
	"PrimaryImageAspectRatio": {},
	"ProviderIds":             {},
	"Revenue":                 {},
	"SortName":                {},
	"Studios":                 {},
	"Taglines":                {},
}

// baseItemDTOFields is a list of fields the app reads
var baseItemDTOFields = map[string]struct{}{
	"CommunityRating": {},
	"DisplayOrder":    {},
	"EndDate":         {},
	"EpisodeNumber":   {},
	"ForcedSortName":  {},
	"Genres":          {},
	"Id":              {},
	"IndexNumber":     {},
	"LockData":        {},
	"LockedFields":    {},
	"MediaSources":    {},
	"Name":            {},
	"OfficialRating":  {},
	"OriginalTitle":   {},
	"Overview":        {},
	"Path":            {},
	"People":          {},
	"PremiereDate":    {},
	"ProductionYear":  {},
	"RunTimeTicks":    {},
	"SeriesName":      {},
	"ServerId":        {},
	"SortName":        {},
	"Status":          {},
	"Studios":         {},
	"TagItems":        {},
	"Taglines":        {},
	"Tags":            {},
}

type GetItemsOptionalParams struct {
	Recursive           *bool
	AnyProviderIdEquals map[string]string
	IsLocked            *bool
	IsFolder            *bool
	ParentID            string
	Path                string
	Ids                 []string
	Filters             []string
	IncludeItemTypes    []string
	Limit               int
}

func (c *Client) GetItems(
	ctx context.Context,
	optionalParams *GetItemsOptionalParams,
) (QueryResultBaseItemDto, error) {
	fieldsMap := make(map[string]struct{})

	maps.Copy(fieldsMap, fieldsFromDocumentation)
	maps.Copy(fieldsMap, baseItemDTOFields)

	fields := make([]string, 0, len(fieldsMap))

	for field := range fieldsMap {
		fields = append(fields, field)
	}

	slices.Sort(fields)

	queryParams := url.Values{
		"Fields": {strings.Join(fields, ",")},
	}

	if optionalParams.Filters != nil {
		queryParams.Add("Filters", strings.Join(optionalParams.Filters, ","))
	}

	if optionalParams.Ids != nil {
		queryParams.Add("Ids", strings.Join(optionalParams.Ids, ","))
	}

	if optionalParams.ParentID != "" {
		queryParams.Add("ParentId", optionalParams.ParentID)
	}

	if optionalParams.Path != "" {
		queryParams.Add("Path", optionalParams.Path)
	}

	if optionalParams.Limit > 0 {
		queryParams.Add("Limit", strconv.Itoa(optionalParams.Limit))
	}

	if optionalParams.AnyProviderIdEquals != nil {
		providerIDs := make([]string, 0, len(optionalParams.AnyProviderIdEquals))

		for providerName, id := range optionalParams.AnyProviderIdEquals {
			providerIDs = append(providerIDs, fmt.Sprintf("%s.%s", providerName, id))
		}

		queryParams.Add("AnyProviderIdEquals", strings.Join(providerIDs, ","))
	}

	if optionalParams.IncludeItemTypes != nil {
		queryParams.Add("IncludeItemTypes", strings.Join(optionalParams.IncludeItemTypes, ","))
	}

	if optionalParams.Recursive != nil {
		queryParams.Add("Recursive", strconv.FormatBool(*optionalParams.Recursive))
	}

	if optionalParams.IsLocked != nil {
		queryParams.Add("IsLocked", strconv.FormatBool(*optionalParams.IsLocked))
	}

	if optionalParams.IsFolder != nil {
		queryParams.Add("IsFolder", strconv.FormatBool(*optionalParams.IsFolder))
	}

	var response QueryResultBaseItemDto

	err := c.sendGETRequestToAPI(ctx, "/Items", queryParams, &response)

	return response, err
}

type GetUserItemsOptionalParams struct {
	Recursive           *bool
	AnyProviderIdEquals map[string]string
	IsPlayed            *bool
	ParentID            string
	Path                string
	SortOrder           string
	Filters             []string
	IncludeItemTypes    []string
	SortBy              []string
	Limit               int
}

func (c *Client) GetUserItems(
	ctx context.Context,
	userID string,
	optionalParams *GetUserItemsOptionalParams,
) (QueryResultBaseItemDto, error) {
	fieldsMap := make(map[string]struct{})

	maps.Copy(fieldsMap, fieldsFromDocumentation)
	maps.Copy(fieldsMap, baseItemDTOFields)

	fields := make([]string, 0, len(fieldsMap))

	for field := range fieldsMap {
		fields = append(fields, field)
	}

	slices.Sort(fields)

	queryParams := url.Values{
		"Fields": {strings.Join(fields, ",")},
	}

	if optionalParams.Filters != nil {
		queryParams.Add("Filters", strings.Join(optionalParams.Filters, ","))
	}

	if optionalParams.ParentID != "" {
		queryParams.Add("ParentId", optionalParams.ParentID)
	}

	if optionalParams.Path != "" {
		queryParams.Add("Path", optionalParams.Path)
	}

	if optionalParams.Limit > 0 {
		queryParams.Add("Limit", strconv.Itoa(optionalParams.Limit))
	}

	if optionalParams.AnyProviderIdEquals != nil {
		providerIDs := make([]string, 0, len(optionalParams.AnyProviderIdEquals))

		for providerName, id := range optionalParams.AnyProviderIdEquals {
			providerIDs = append(providerIDs, fmt.Sprintf("%s.%s", providerName, id))
		}

		queryParams.Add("AnyProviderIdEquals", strings.Join(providerIDs, ","))
	}

	if optionalParams.IncludeItemTypes != nil {
		queryParams.Add("IncludeItemTypes", strings.Join(optionalParams.IncludeItemTypes, ","))
	}

	if optionalParams.SortBy != nil {
		queryParams.Add("SortBy", strings.Join(optionalParams.SortBy, ","))
	}

	if optionalParams.SortOrder != "" {
		queryParams.Add("SortOrder", optionalParams.SortOrder)
	}

	if optionalParams.IsPlayed != nil {
		queryParams.Add("IsPlayed", strconv.FormatBool(*optionalParams.IsPlayed))
	}

	if optionalParams.Recursive != nil {
		queryParams.Add("Recursive", strconv.FormatBool(*optionalParams.Recursive))
	}

	var response QueryResultBaseItemDto

	err := c.sendGETRequestToAPI(ctx, fmt.Sprintf("/Users/%s/Items", userID), queryParams, &response)

	return response, err
}
